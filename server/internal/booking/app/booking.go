package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	"github.com/google/uuid"
)

const HoldTTL = 30 * time.Minute

var (
	ErrCompensationFailed = errors.New("確保の解放に失敗しました")
)

type Service struct {
	tx        Transactor
	roomTypes RoomTypeFinder
	inventory InventoryHolder
}

func NewService(tx Transactor, roomTypes RoomTypeFinder, inventory InventoryHolder) *Service {
	return &Service{tx: tx, roomTypes: roomTypes, inventory: inventory}
}

type GuestInput struct {
	FirstName string
	LastName  string
}

type BookInput struct {
	BookerId     uuid.UUID
	RoomTypeId   uuid.UUID
	CheckinDate  time.Time
	CheckoutDate time.Time
	Guests       []GuestInput
}

func (s *Service) Book(ctx context.Context, in BookInput, now time.Time) (*domain.Booking, error) {
	capacity, err := s.roomTypes.Capacity(ctx, in.RoomTypeId)
	if err != nil {
		return nil, err
	}

	period, err := domain.NewStayPeriod(in.CheckinDate, in.CheckoutDate)
	if err != nil {
		return nil, err
	}

	guests, err := buildGuests(in.Guests)
	if err != nil {
		return nil, err
	}

	zeroFee, err := domain.NewTotalFee(0)
	if err != nil {
		return nil, err
	}

	booking, err := domain.Book(uuid.New(), in.BookerId, in.RoomTypeId, period, guests, zeroFee, capacity, now)
	if err != nil {
		return nil, err
	}

	if err := s.save(ctx, booking); err != nil {
		return nil, err
	}

	total, err := s.holdAllDates(ctx, booking, now)
	if err != nil {
		return nil, err
	}

	totalFee, err := domain.NewTotalFee(total)
	if err != nil {
		return nil, err
	}
	if err := booking.FixTotalFee(totalFee); err != nil {
		return nil, err
	}
	if err := s.save(ctx, booking); err != nil {
		return nil, err
	}

	return booking, nil
}

func (s *Service) holdAllDates(ctx context.Context, booking *domain.Booking, now time.Time) (int, error) {
	expiredAt := now.Add(HoldTTL)
	roomTypeId := booking.RoomTypeId()

	total := 0
	held := make([]time.Time, 0, booking.StayPeriod().Nights())

	for _, date := range booking.StayPeriod().Dates() {
		slot, err := s.inventory.Hold(ctx, roomTypeId, date, uuid.New(), booking.Id(), expiredAt, now)
		if err != nil {
			s.compensate(ctx, booking, held, now)
			return 0, err
		}
		held = append(held, date)
		total += slot.FeeAmount
	}
	return total, nil
}

func (s *Service) Confirm(ctx context.Context, bookingId uuid.UUID) error {
	booking, err := s.startPayment(ctx, bookingId)
	if err != nil {
		return err
	}

	roomTypeId := booking.RoomTypeId()
	for _, date := range booking.StayPeriod().Dates() {
		if err := s.inventory.StartPayment(ctx, roomTypeId, date, bookingId); err != nil {
			return err
		}
	}
	for _, date := range booking.StayPeriod().Dates() {
		if err := s.inventory.Confirm(ctx, roomTypeId, date, bookingId); err != nil {
			return err
		}
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		b, err := repo.FindForUpdate(ctx, bookingId)
		if err != nil {
			return err
		}
		if err := b.Confirm(); err != nil {
			return err
		}
		return repo.Save(ctx, b)
	})
}

func (s *Service) Cancel(
	ctx context.Context,
	bookingId uuid.UUID,
	reason domain.CancellationReason,
	now time.Time,
) (domain.CancellationFee, error) {
	var fee domain.CancellationFee
	var booking *domain.Booking

	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		b, err := repo.FindForUpdate(ctx, bookingId)
		if err != nil {
			return err
		}
		f, err := b.Cancel(reason, now)
		if err != nil {
			return err
		}
		if err := repo.Save(ctx, b); err != nil {
			return err
		}
		fee, booking = f, b
		return nil
	})
	if err != nil {
		return domain.CancellationFee{}, err
	}

	if err := s.releaseAll(ctx, booking); err != nil {
		return fee, err
	}
	return fee, nil
}

func (s *Service) ChangeGuests(ctx context.Context, bookingId uuid.UUID, in []GuestInput) error {
	guests, err := buildGuests(in)
	if err != nil {
		return err
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		b, err := repo.FindForUpdate(ctx, bookingId)
		if err != nil {
			return err
		}
		capacity, err := s.roomTypes.Capacity(ctx, b.RoomTypeId())
		if err != nil {
			return err
		}
		if err := b.ChangeGuests(guests, capacity); err != nil {
			return err
		}
		return repo.Save(ctx, b)
	})
}

func (s *Service) Find(ctx context.Context, bookingId uuid.UUID) (*domain.Booking, error) {
	var booking *domain.Booking
	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		b, err := repo.Find(ctx, bookingId)
		if err != nil {
			return err
		}
		booking = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return booking, nil
}

func (s *Service) ListByBooker(ctx context.Context, bookerId uuid.UUID) ([]BookingSummary, error) {
	var summaries []BookingSummary
	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		var err error
		summaries, err = repo.ListSummariesByBookerId(ctx, bookerId)
		return err
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *Service) startPayment(ctx context.Context, bookingId uuid.UUID) (*domain.Booking, error) {
	var booking *domain.Booking
	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		b, err := repo.FindForUpdate(ctx, bookingId)
		if err != nil {
			return err
		}
		if err := b.StartPayment(); err != nil {
			return err
		}
		if err := repo.Save(ctx, b); err != nil {
			return err
		}
		booking = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return booking, nil
}

func (s *Service) save(ctx context.Context, b *domain.Booking) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		return repo.Save(ctx, b)
	})
}

func (s *Service) compensate(ctx context.Context, booking *domain.Booking, held []time.Time, now time.Time) {
	roomTypeId := booking.RoomTypeId()
	for _, date := range held {
		_ = s.inventory.Release(ctx, roomTypeId, date, booking.Id())
	}
	if _, err := booking.Cancel(domain.Expired, now); err == nil {
		_ = s.save(ctx, booking)
	}
}

func (s *Service) releaseAll(ctx context.Context, booking *domain.Booking) error {
	roomTypeId := booking.RoomTypeId()
	for _, date := range booking.StayPeriod().Dates() {
		if err := s.inventory.Release(ctx, roomTypeId, date, booking.Id()); err != nil {
			return fmt.Errorf("%w: %s", ErrCompensationFailed, date.Format(time.DateOnly))
		}
	}
	return nil
}

func buildGuests(in []GuestInput) ([]domain.Guest, error) {
	guests := make([]domain.Guest, 0, len(in))
	for _, g := range in {
		name, err := domain.NewName(g.FirstName, g.LastName)
		if err != nil {
			return nil, err
		}
		guests = append(guests, domain.NewGuest(uuid.New(), name))
	}
	return guests, nil
}
