package app

import (
	"context"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	"github.com/google/uuid"
)

type Service struct {
	tx Transactor
}

func NewService(tx Transactor) *Service {
	return &Service{tx: tx}
}

type RegisterInput struct {
	RoomTypeId uuid.UUID
	Date       time.Time
	Quantity   int
	FeeAmount  int
	Now        time.Time
}

func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	id, err := domain.NewInventoryId(domain.CreateNewInventoryIdInput{
		RoomTypeId: in.RoomTypeId,
		Date:       in.Date,
		Now:        in.Now,
	})
	if err != nil {
		return err
	}
	fee, err := domain.NewFee(domain.CreateNewFeeInput{Amount: in.FeeAmount})
	if err != nil {
		return err
	}
	inv, err := domain.Register(id, in.Quantity, fee)
	if err != nil {
		return err
	}

	return s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		return repo.Add(ctx, inv)
	})
}

func (s *Service) ChangeQuantity(ctx context.Context, roomTypeId uuid.UUID, date time.Time, quantity int) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.ChangeQuantity(quantity)
	})
}

func (s *Service) ChangeFee(ctx context.Context, roomTypeId uuid.UUID, date time.Time, feeAmount int) error {
	fee, err := domain.NewFee(domain.CreateNewFeeInput{Amount: feeAmount})
	if err != nil {
		return err
	}
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.ChangeFee(fee)
	})
}

func (s *Service) Close(ctx context.Context, roomTypeId uuid.UUID, date time.Time) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.Close()
	})
}

func (s *Service) Reopen(ctx context.Context, roomTypeId uuid.UUID, date time.Time) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.Reopen()
	})
}

type HoldInput struct {
	HoldId     uuid.UUID
	RoomTypeId uuid.UUID
	Date       time.Time
	BookingId  uuid.UUID
	ExpiredAt  time.Time
	Now        time.Time
}

type HeldSlot struct {
	SlotNo    int
	FeeAmount int
}

func (s *Service) Hold(ctx context.Context, in HoldInput) (HeldSlot, error) {
	var held HeldSlot

	err := s.mutate(ctx, in.RoomTypeId, in.Date, func(inv *domain.Inventory) error {
		slotNo, err := inv.Hold(domain.HoldInput{
			HoldId:     in.HoldId,
			BookingId:  in.BookingId,
			RoomTypeId: in.RoomTypeId,
			ExpiredAt:  in.ExpiredAt,
			Date:       in.Now,
		})
		if err != nil {
			return err
		}
		held = HeldSlot{SlotNo: slotNo, FeeAmount: inv.Fee().Amount()}
		return nil
	})
	if err != nil {
		return HeldSlot{}, err
	}
	return held, nil
}

func (s *Service) StartPayment(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.StartPayment(bookingId)
	})
}

func (s *Service) Confirm(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.Confirm(bookingId)
	})
}

func (s *Service) Release(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	return s.mutate(ctx, roomTypeId, date, func(inv *domain.Inventory) error {
		return inv.Release(bookingId)
	})
}

func (s *Service) CollectExpired(ctx context.Context, now time.Time) (int, error) {
	var ids []*domain.InventoryId
	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		var err error
		ids, err = repo.ListExpiredInventoryIds(ctx, now)
		return err
	})
	if err != nil {
		return 0, err
	}

	collected := 0
	for _, id := range ids {
		err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
			inv, err := repo.FindForUpdate(ctx, id)
			if err != nil {
				return err
			}
			n := inv.CollectExpired(now)
			if n == 0 {
				return nil
			}
			if err := repo.Save(ctx, inv); err != nil {
				return err
			}
			collected += n
			return nil
		})
		if err != nil {
			return collected, err
		}
	}
	return collected, nil
}

func (s *Service) ListSummaries(
	ctx context.Context,
	roomTypeId uuid.UUID,
	from, to time.Time,
) ([]InventorySummary, error) {
	var summaries []InventorySummary
	err := s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		var err error
		summaries, err = repo.ListSummaries(ctx, roomTypeId, from, to)
		return err
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *Service) mutate(
	ctx context.Context,
	roomTypeId uuid.UUID,
	date time.Time,
	apply func(inv *domain.Inventory) error,
) error {
	id := domain.ReconstructInventoryId(roomTypeId, date)

	return s.tx.WithinTx(ctx, func(ctx context.Context, repo Repository) error {
		inv, err := repo.FindForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if err := apply(inv); err != nil {
			return err
		}
		return repo.Save(ctx, inv)
	})
}
