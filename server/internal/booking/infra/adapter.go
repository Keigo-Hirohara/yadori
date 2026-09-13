package infra

import (
	"context"
	"errors"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	bookingapp "github.com/Keigo-Hirohara/yadori/internal/booking/app"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	inventorydomain "github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	"github.com/google/uuid"
)

type InventoryHolder struct {
	inventory *inventoryapp.Service
}

func NewInventoryHolder(inventory *inventoryapp.Service) *InventoryHolder {
	return &InventoryHolder{inventory: inventory}
}

var _ bookingapp.InventoryHolder = (*InventoryHolder)(nil)

func (h *InventoryHolder) Hold(
	ctx context.Context,
	roomTypeId uuid.UUID,
	date time.Time,
	holdId, bookingId uuid.UUID,
	expiredAt, now time.Time,
) (bookingapp.HeldSlot, error) {
	slot, err := h.inventory.Hold(ctx, inventoryapp.HoldInput{
		HoldId:     holdId,
		RoomTypeId: roomTypeId,
		Date:       date,
		BookingId:  bookingId,
		ExpiredAt:  expiredAt,
		Now:        now,
	})
	if err != nil {
		return bookingapp.HeldSlot{}, err
	}
	return bookingapp.HeldSlot{SlotNo: slot.SlotNo, FeeAmount: slot.FeeAmount}, nil
}

func (h *InventoryHolder) StartPayment(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	return h.inventory.StartPayment(ctx, roomTypeId, date, bookingId)
}

func (h *InventoryHolder) Confirm(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	return h.inventory.Confirm(ctx, roomTypeId, date, bookingId)
}

func (h *InventoryHolder) Release(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	err := h.inventory.Release(ctx, roomTypeId, date, bookingId)
	if errors.Is(err, inventorydomain.ErrHoldNotFound) || errors.Is(err, inventorydomain.ErrInventoryNotFound) {
		return bookingapp.ErrHoldAlreadyReleased
	}
	return err
}

type RoomTypeFinder struct {
	db accommodationdb.DBTX
}

func NewRoomTypeFinder(db accommodationdb.DBTX) *RoomTypeFinder {
	return &RoomTypeFinder{db: db}
}

var _ bookingapp.RoomTypeFinder = (*RoomTypeFinder)(nil)

func (f *RoomTypeFinder) Capacity(ctx context.Context, roomTypeId uuid.UUID) (int, error) {
	roomType, err := accommodation.FindRoomTypeById(ctx, f.db, roomTypeId)
	if err != nil {
		return 0, err
	}
	return roomType.Capacity(), nil
}
