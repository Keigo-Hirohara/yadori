package search

import (
	"context"
	"errors"
	"time"

	searchdb "github.com/Keigo-Hirohara/yadori/internal/search/db"
	"github.com/google/uuid"
)

var (
	ErrInvalidPeriod = errors.New("チェックアウト日はチェックイン日より後にしてください")
	ErrInvalidGuests = errors.New("宿泊人数は1名以上にしてください")
	ErrFailedToRead  = errors.New("空室の検索に失敗しました")
)

type Criteria struct {
	Prefecture   string
	CheckinDate  time.Time
	CheckoutDate time.Time
	Guests       int
	MaxFee       int
}

type Result struct {
	RoomTypeId        uuid.UUID
	RoomTypeName      string
	Capacity          int
	HasPrivateBath    bool
	HasBalcony        bool
	AccommodationId   uuid.UUID
	AccommodationName string
	Prefecture        string
	City              string
	TotalFee          int
	Nights            int
}

func Search(ctx context.Context, db searchdb.DBTX, c Criteria) ([]Result, error) {
	nights, err := validCriteria(c)
	if err != nil {
		return nil, err
	}

	rows, err := searchdb.New(db).SearchRoomTypes(ctx, searchdb.SearchRoomTypesParams{
		CheckinDate:  truncateToDate(c.CheckinDate),
		CheckoutDate: truncateToDate(c.CheckoutDate),
		Guests:       int32(c.Guests),
		Prefecture:   c.Prefecture,
		Nights:       int32(nights),
		MaxFee:       int32(c.MaxFee),
	})
	if err != nil {
		return nil, ErrFailedToRead
	}

	results := make([]Result, 0, len(rows))
	for _, row := range rows {
		results = append(results, Result{
			RoomTypeId:        row.RoomTypeID,
			RoomTypeName:      row.RoomTypeName,
			Capacity:          int(row.Capacity),
			HasPrivateBath:    row.HasPrivateBath,
			HasBalcony:        row.HasBalcony,
			AccommodationId:   row.AccommodationID,
			AccommodationName: row.AccommodationName,
			Prefecture:        row.Prefecture,
			City:              row.City,
			TotalFee:          int(row.TotalFee),
			Nights:            nights,
		})
	}
	return results, nil
}

func validCriteria(c Criteria) (int, error) {
	checkin := truncateToDate(c.CheckinDate)
	checkout := truncateToDate(c.CheckoutDate)
	if !checkout.After(checkin) {
		return 0, ErrInvalidPeriod
	}
	if c.Guests < 1 {
		return 0, ErrInvalidGuests
	}
	return int(checkout.Sub(checkin).Hours() / 24), nil
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
