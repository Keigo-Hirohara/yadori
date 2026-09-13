package app

import (
	"context"
	"errors"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Find(ctx context.Context, id uuid.UUID) (*domain.Booking, error)

	FindForUpdate(ctx context.Context, id uuid.UUID) (*domain.Booking, error)

	Save(ctx context.Context, b *domain.Booking) error
	ListStaleTemporaryHoldIds(ctx context.Context, before time.Time) ([]uuid.UUID, error)

	ListSummariesByBookerId(ctx context.Context, bookerId uuid.UUID) ([]BookingSummary, error)
}

type BookingSummary struct {
	BookingId       uuid.UUID
	RoomTypeId      uuid.UUID
	CheckinDate     time.Time
	CheckoutDate    time.Time
	TotalFee        int
	CancellationFee int
	Status          domain.Status
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error
}

type RoomTypeFinder interface {
	Capacity(ctx context.Context, roomTypeId uuid.UUID) (int, error)
}

type HeldSlot struct {
	SlotNo    int
	FeeAmount int
}

var ErrHoldAlreadyReleased = errors.New("確保はすでに解放されています")

type InventoryHolder interface {
	Hold(ctx context.Context, roomTypeId uuid.UUID, date time.Time, holdId, bookingId uuid.UUID, expiredAt, now time.Time) (HeldSlot, error)
	StartPayment(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error
	Confirm(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error
	Release(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error
}
