package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	inventorypostgres "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	operatorSubject = "loadtest-rush"
	rushDays        = 14
	vacancy         = 3
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
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

	a, err := findOrCreateAccommodation(ctx, pool)
	if err != nil {
		return err
	}
	roomType, err := findOrCreateRoomType(ctx, pool, a)
	if err != nil {
		return err
	}

	inventory := inventoryapp.NewService(inventorypostgres.NewTransactor(pool))
	now := time.Now()
	date := now.AddDate(0, 0, rushDays)
	if err := prepareVacancy(ctx, inventory, roomType.ID(), date, now); err != nil {
		return err
	}

	fmt.Println(roomType.ID(), date.Format("2006-01-02"))
	return nil
}

func prepareVacancy(ctx context.Context, inventory *inventoryapp.Service, roomTypeId uuid.UUID, date, now time.Time) error {
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

func findOrCreateAccommodation(ctx context.Context, pool *pgxpool.Pool) (accommodation.Accommodation, error) {
	accommodations, err := accommodation.ListAccommodationsByOperator(ctx, pool, operatorSubject)
	if err != nil {
		return accommodation.Accommodation{}, err
	}
	if len(accommodations) > 0 {
		return accommodations[0], nil
	}

	a, err := accommodation.NewAccommodation(accommodation.AccommodationCreateInput{
		OperatorSubject: operatorSubject,
		Name:            "負荷試験（集中）",
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

func findOrCreateRoomType(ctx context.Context, pool *pgxpool.Pool, a accommodation.Accommodation) (*accommodation.RoomType, error) {
	roomTypes, err := accommodation.ListRoomTypesByAccommodationId(ctx, pool, a.ID())
	if err != nil {
		return nil, err
	}
	if len(roomTypes) > 0 {
		return &roomTypes[0], nil
	}

	roomType, err := accommodation.NewRoomType(accommodation.RoomTypeCreateInput{
		AccommodationId: a.ID(),
		Name:            "集中試験用",
		Capacity:        2,
	})
	if err != nil {
		return nil, err
	}
	if err := roomType.Save(ctx, pool); err != nil {
		return nil, err
	}
	return roomType, nil
}
