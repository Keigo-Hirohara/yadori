package postgres

import (
	"fmt"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	inventorydb "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func toInventory(row inventorydb.Inventory, holdRows []inventorydb.Hold) (*domain.Inventory, error) {
	fee, err := domain.NewFee(domain.CreateNewFeeInput{Amount: int(row.Fee)})
	if err != nil {
		return nil, err
	}

	holds := make([]domain.Hold, 0, len(holdRows))
	for _, h := range holdRows {
		status, err := toDomainHoldStatus(h.Status)
		if err != nil {
			return nil, err
		}
		holds = append(holds, domain.ReconstructHold(
			h.ID,
			h.BookingID,
			int(h.SlotNo),
			status,
			toTimePointer(h.ExpiredAt),
		))
	}

	id := domain.ReconstructInventoryId(row.RoomTypeID, row.Date)
	return domain.Reconstruct(id, int(row.QuantityAvailable), fee, row.IsClosed, holds), nil
}

func toUpsertInventoryParams(inv *domain.Inventory) inventorydb.UpsertInventoryParams {
	return inventorydb.UpsertInventoryParams{
		RoomTypeID:        inv.Id().RoomTypeId(),
		Date:              inv.Id().Date(),
		Fee:               int32(inv.Fee().Amount()),
		QuantityAvailable: int32(inv.QuantityAvailable()),
		IsClosed:          inv.IsClosed(),
	}
}

func toUpsertHoldParams(inv *domain.Inventory, h domain.Hold) (inventorydb.UpsertHoldParams, error) {
	status, err := toDBHoldStatus(h.Status())
	if err != nil {
		return inventorydb.UpsertHoldParams{}, err
	}
	return inventorydb.UpsertHoldParams{
		ID:         h.Id(),
		Date:       inv.Id().Date(),
		RoomTypeID: inv.Id().RoomTypeId(),
		BookingID:  h.BookingId(),
		Status:     status,
		ExpiredAt:  toTimestamptz(h.ExpiredAt()),
		SlotNo:     int32(h.SlotNo()),
	}, nil
}

func toDBHoldStatus(s domain.HoldStatus) (inventorydb.HoldStatus, error) {
	switch s {
	case domain.TemporaryHold:
		return inventorydb.HoldStatusTemporaryHold, nil
	case domain.ProcessingPayment:
		return inventorydb.HoldStatusProcessingPayment, nil
	case domain.Confirmed:
		return inventorydb.HoldStatusConfirmed, nil
	default:
		return "", fmt.Errorf("保存できない確保の状態です: %d", s)
	}
}

func toDomainHoldStatus(s inventorydb.HoldStatus) (domain.HoldStatus, error) {
	switch s {
	case inventorydb.HoldStatusTemporaryHold:
		return domain.TemporaryHold, nil
	case inventorydb.HoldStatusProcessingPayment:
		return domain.ProcessingPayment, nil
	case inventorydb.HoldStatusConfirmed:
		return domain.Confirmed, nil
	default:
		return 0, fmt.Errorf("未知の確保の状態です: %s", s)
	}
}

func toTimePointer(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func toTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
