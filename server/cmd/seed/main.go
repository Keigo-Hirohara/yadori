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
	"github.com/jackc/pgx/v5/pgxpool"
)

const days = 60

type roomTypeSeed struct {
	name           string
	capacity       int
	fee            int
	quantity       int
	hasPrivateBath bool
	hasBalcony     bool
}

type accommodationSeed struct {
	name       string
	prefecture string
	city       string
	roomTypes  []roomTypeSeed
}

var seeds = []accommodationSeed{
	{
		name: "やどり館山", prefecture: "千葉県", city: "館山市",
		roomTypes: []roomTypeSeed{
			{"海side和室", 4, 15000, 3, true, true},
			{"ツイン", 2, 11000, 5, false, false},
		},
	},
	{
		name: "やどり箱根", prefecture: "神奈川県", city: "足柄下郡箱根町",
		roomTypes: []roomTypeSeed{
			{"露天風呂付き客室", 2, 28000, 2, true, true},
			{"和室10畳", 5, 18000, 4, false, false},
		},
	},
	{
		name: "やどり金沢", prefecture: "石川県", city: "金沢市",
		roomTypes: []roomTypeSeed{
			{"町家スイート", 6, 32000, 1, true, false},
			{"シングル", 1, 8000, 8, false, false},
		},
	},
}

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

	inventory := inventoryapp.NewService(inventorypostgres.NewTransactor(pool))
	now := time.Now()

	for _, s := range seeds {
		a, err := accommodation.NewAccommodation(accommodation.AccommodationCreateInput{
			Name:          s.name,
			PhoneNumber:   "0312345678",
			PostalCode:    "1000001",
			Prefecture:    s.prefecture,
			City:          s.city,
			StreetAddress: "1-1-1",
		})
		if err != nil {
			return err
		}
		if err := a.Save(ctx, pool); err != nil {
			return err
		}
		fmt.Printf("%s（%s%s）\n", a.Name(), a.Prefecture(), a.City())

		for _, rt := range s.roomTypes {
			roomType, err := accommodation.NewRoomType(accommodation.RoomTypeCreateInput{
				AccommodationId: a.ID(),
				Name:            rt.name,
				Capacity:        rt.capacity,
				HasPrivateBath:  rt.hasPrivateBath,
				HasBalcony:      rt.hasBalcony,
			})
			if err != nil {
				return err
			}
			if err := roomType.Save(ctx, pool); err != nil {
				return err
			}

			registered := 0
			for i := 0; i < days; i++ {
				date := now.AddDate(0, 0, i)
				err := inventory.Register(ctx, inventoryapp.RegisterInput{
					RoomTypeId: roomType.ID(),
					Date:       date,
					Quantity:   rt.quantity,
					FeeAmount:  rt.fee,
					Now:        now,
				})
				switch {
				case err == nil:
					registered++
				case errors.Is(err, domain.ErrAlreadyRegistered), errors.Is(err, domain.ErrPast):

				default:
					return err
				}
			}
			fmt.Printf("  %s 定員%d名 %d円 ×%d枠 → %d日分\n",
				rt.name, rt.capacity, rt.fee, rt.quantity, registered)
		}
	}

	fmt.Println("\nデモデータを投入しました")
	return nil
}
