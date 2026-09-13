package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	"github.com/Keigo-Hirohara/yadori/internal/booker"
	bookerdb "github.com/Keigo-Hirohara/yadori/internal/booker/db"
	bookingapp "github.com/Keigo-Hirohara/yadori/internal/booking/app"
	bookinginfra "github.com/Keigo-Hirohara/yadori/internal/booking/infra"
	bookinghttp "github.com/Keigo-Hirohara/yadori/internal/booking/infra/http"
	bookingpostgres "github.com/Keigo-Hirohara/yadori/internal/booking/infra/postgres"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	inventoryhttp "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/http"
	inventorypostgres "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres"
	"github.com/Keigo-Hirohara/yadori/internal/search"
	searchdb "github.com/Keigo-Hirohara/yadori/internal/search/db"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultPort = "8080"

	requestTimeout = 5 * time.Second

	maxRequestBody = 1 << 20

	shutdownTimeout = 10 * time.Second
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("サーバーを起動できませんでした", "error", err)
		os.Exit(1)
	}
}

func allowedOrigins() []string {
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		origins := strings.Split(v, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		return origins
	}
	return []string{
		"http://localhost:5173",
		"http://localhost:5174",
	}
}

func run() error {
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

	inventoryService := inventoryapp.NewService(inventorypostgres.NewTransactor(pool))

	bookingService := bookingapp.NewService(
		bookingpostgres.NewTransactor(pool),
		bookinginfra.NewRoomTypeFinder(accommodationdb.DBTX(pool)),
		bookinginfra.NewInventoryHolder(inventoryService),
	)

	mux := routes(handlers{
		accommodation: accommodation.NewHandler(accommodationdb.DBTX(pool)),
		booker:        booker.NewHandler(bookerdb.DBTX(pool)),
		inventory:     inventoryhttp.NewHandler(inventoryService),
		booking:       bookinghttp.NewHandler(bookingService),
		search:        search.NewHandler(searchdb.DBTX(pool)),
	})

	handler := sharedhttp.Chain(mux,
		sharedhttp.Recover,
		sharedhttp.RequestID,
		sharedhttp.AccessLog,
		sharedhttp.CORS(allowedOrigins()),
		sharedhttp.Timeout(requestTimeout),
		sharedhttp.MaxBytes(maxRequestBody),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("サーバーを起動しました", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("停止信号を受け取りました。処理中のリクエストを待ちます")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
