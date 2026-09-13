package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPast = errors.New("過去の日付は予約できません")
)

type CreateNewInventoryIdInput struct {
	Date       time.Time
	RoomTypeId uuid.UUID
	Now        time.Time
}

type InventoryId struct {
	date       time.Time
	roomTypeId uuid.UUID
}

func NewInventoryId(input CreateNewInventoryIdInput) (*InventoryId, error) {
	date := truncateToDate(input.Date)
	if err := validDate(date, input.Now); err != nil {
		return nil, err
	}
	return &InventoryId{
		date:       date,
		roomTypeId: input.RoomTypeId,
	}, nil
}

func ReconstructInventoryId(roomTypeId uuid.UUID, date time.Time) *InventoryId {
	return &InventoryId{
		date:       truncateToDate(date),
		roomTypeId: roomTypeId,
	}
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (i *InventoryId) Date() time.Time {
	return i.date
}

func (i *InventoryId) RoomTypeId() uuid.UUID {
	return i.roomTypeId
}

func validDate(date, now time.Time) error {
	if date.Before(now) {
		return ErrPast
	}
	return nil
}
