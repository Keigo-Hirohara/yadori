package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	inventorypostgres "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const startAfterDays = 14

type plan struct {
	subject   string
	name      string
	roomTypes int
	days      int
	vacancy   int
}

func main() {
	var p plan
	flag.StringVar(&p.subject, "subject", "loadtest-rush", "宿の目印（operator_subject）")
	flag.StringVar(&p.name, "name", "負荷試験（集中）", "宿の名前")
	flag.IntVar(&p.roomTypes, "room-types", 1, "部屋タイプの数")
	flag.IntVar(&p.days, "days", 1, "実行日の2週間後から何日分の在庫を用意するか")
	flag.IntVar(&p.vacancy, "vacancy", 3, "各在庫の空き枠の数")
	flag.Parse()

	if err := run(p); err != nil {
		log.Fatal(err)
	}
}

func run(p plan) error {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL が設定されていません")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	a, err := findOrCreateAccommodation(ctx, pool, p)
	if err != nil {
		return err
	}
	roomTypes, err := findOrCreateRoomTypes(ctx, pool, a, p)
	if err != nil {
		return err
	}

	inventory := inventoryapp.NewService(inventorypostgres.NewTransactor(pool))
	now := time.Now()
	start := now.AddDate(0, 0, startAfterDays)

	ids := make([]string, 0, len(roomTypes))
	for _, rt := range roomTypes {
		for i := 0; i < p.days; i++ {
			if err := prepareVacancy(ctx, inventory, rt.ID(), start.AddDate(0, 0, i), now, p.vacancy); err != nil {
				return err
			}
		}
		ids = append(ids, rt.ID().String())
	}

	fmt.Println(start.Format("2006-01-02"), strings.Join(ids, ","))
	return nil
}

func prepareVacancy(ctx context.Context, inventory *inventoryapp.Service, roomTypeId uuid.UUID, date, now time.Time, vacancy int) error {
	err := inventory.Register(ctx, inventoryapp.RegisterInput{
		RoomTypeId: roomTypeId,
		Date:       date,
		Quantity:   vacancy,
		FeeAmount:  10000,
		Now:        now,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrAlreadyRegistered) {
		return err
	}

	summaries, err := inventory.ListSummaries(ctx, roomTypeId, date, date.AddDate(0, 0, 1))
	if err != nil {
		return err
	}
	if len(summaries) != 1 {
		return fmt.Errorf("%s の在庫が見つかりません", date.Format("2006-01-02"))
	}
	return inventory.ChangeQuantity(ctx, roomTypeId, date, summaries[0].HeldCount+vacancy)
}

func findOrCreateAccommodation(ctx context.Context, pool *pgxpool.Pool, p plan) (accommodation.Accommodation, error) {
	accommodations, err := accommodation.ListAccommodationsByOperator(ctx, pool, p.subject)
	if err != nil {
		return accommodation.Accommodation{}, err
	}
	if len(accommodations) > 0 {
		return accommodations[0], nil
	}

	a, err := accommodation.NewAccommodation(accommodation.AccommodationCreateInput{
		OperatorSubject: p.subject,
		Name:            p.name,
		PhoneNumber:     "0312345678",
		PostalCode:      "1000001",
		Prefecture:      "東京都",
		City:            "千代田区",
		StreetAddress:   "1-1-1",
	})
	if err != nil {
		return accommodation.Accommodation{}, err
	}
	if err := a.Save(ctx, pool); err != nil {
		return accommodation.Accommodation{}, err
	}
	return a, nil
}

func findOrCreateRoomTypes(ctx context.Context, pool *pgxpool.Pool, a accommodation.Accommodation, p plan) ([]accommodation.RoomType, error) {
	roomTypes, err := accommodation.ListRoomTypesByAccommodationId(ctx, pool, a.ID())
	if err != nil {
		return nil, err
	}

	for len(roomTypes) < p.roomTypes {
		roomType, err := accommodation.NewRoomType(accommodation.RoomTypeCreateInput{
			AccommodationId: a.ID(),
			Name:            fmt.Sprintf("試験用 %d", len(roomTypes)+1),
			Capacity:        2,
		})
		if err != nil {
			return nil, err
		}
		if err := roomType.Save(ctx, pool); err != nil {
			return nil, err
		}
		roomTypes = append(roomTypes, *roomType)
	}
	return roomTypes[:p.roomTypes], nil
}
