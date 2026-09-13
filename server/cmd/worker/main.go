package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	bookingapp "github.com/Keigo-Hirohara/yadori/internal/booking/app"
	bookinginfra "github.com/Keigo-Hirohara/yadori/internal/booking/infra"
	bookingpostgres "github.com/Keigo-Hirohara/yadori/internal/booking/infra/postgres"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	inventorypostgres "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultInterval = time.Minute

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	once := flag.Bool("once", false, "1回だけ回収して終了する")
	flag.Parse()

	if err := run(*once); err != nil {
		slog.Error("ワーカーを起動できませんでした", "error", err)
		os.Exit(1)
	}
}

func run(once bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL が設定されていません")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}

	inventory := inventoryapp.NewService(inventorypostgres.NewTransactor(pool))
	booking := bookingapp.NewService(
		bookingpostgres.NewTransactor(pool),
		bookinginfra.NewRoomTypeFinder(accommodationdb.DBTX(pool)),
		bookinginfra.NewInventoryHolder(inventory),
	)
	collector := &collector{booking: booking, inventory: inventory}

	if once {
		collector.collect(ctx)
		return nil
	}

	interval := defaultInterval
	if v := os.Getenv("WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return err
		}
		interval = d
	}

	slog.Info("期限切れ回収ワーカーを起動しました", "interval", interval.String())
	collector.collect(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("停止信号を受け取りました")
			return nil
		case <-ticker.C:
			collector.collect(ctx)
		}
	}
}

type collector struct {
	booking   *bookingapp.Service
	inventory *inventoryapp.Service
}

func (c *collector) collect(ctx context.Context) {
	now := time.Now()

	bookings, err := c.booking.ExpireStale(ctx, now)
	if err != nil {
		slog.Error("仮予約の期限切れ処理に失敗しました", "error", err, "expired", bookings)
	}

	holds, err := c.inventory.CollectExpired(ctx, now)
	if err != nil {
		slog.Error("確保の期限切れ回収に失敗しました", "error", err, "collected", holds)
	}

	if bookings > 0 || holds > 0 {
		slog.Info("期限切れを回収しました", "expiredBookings", bookings, "collectedHolds", holds)
	}
}
