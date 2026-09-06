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
}

type InventoryId struct {
	date       time.Time
	roomTypeId uuid.UUID
}

func NewInventoryId(input CreateNewInventoryIdInput) (*InventoryId, error) {
	if err := validDate(input.Date); err != nil {
		return nil, err
	}
	return &InventoryId{
		input.Date,
		input.RoomTypeId,
	}, nil
}

func (i *InventoryId) Date() time.Time {
	return i.date
}

func (i *InventoryId) RoomTypeId() uuid.UUID {
	return i.roomTypeId
}

func validDate(date time.Time) error {
	if date.Before(time.Now()) {
		return ErrPast
	}
	return nil
}
