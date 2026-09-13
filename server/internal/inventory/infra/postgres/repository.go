package postgres

import (
	"context"
	"errors"
	"time"

	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	inventorydb "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	q *inventorydb.Queries
}

func NewRepository(db inventorydb.DBTX) *Repository {
	return &Repository{q: inventorydb.New(db)}
}

var _ inventoryapp.Repository = (*Repository)(nil)

const uniqueViolation = "23505"

func (r *Repository) Add(ctx context.Context, inv *domain.Inventory) error {
	err := r.q.InsertInventory(ctx, inventorydb.InsertInventoryParams{
		RoomTypeID:        inv.Id().RoomTypeId(),
		Date:              inv.Id().Date(),
		Fee:               int32(inv.Fee().Amount()),
		QuantityAvailable: int32(inv.QuantityAvailable()),
		IsClosed:          inv.IsClosed(),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.ErrAlreadyRegistered
		}
		return err
	}
	return nil
}

func (r *Repository) Find(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error) {
	row, err := r.q.GetInventoriesByDateAndRoomTypeId(ctx, inventorydb.GetInventoriesByDateAndRoomTypeIdParams{
		Date:       id.Date(),
		RoomTypeID: id.RoomTypeId(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInventoryNotFound
		}
		return nil, err
	}
	return r.load(ctx, row)
}

func (r *Repository) FindForUpdate(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error) {
	row, err := r.q.GetInventoryByRoomTypeIdAndDateForUpdate(ctx, inventorydb.GetInventoryByRoomTypeIdAndDateForUpdateParams{
		RoomTypeID: id.RoomTypeId(),
		Date:       id.Date(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInventoryNotFound
		}
		return nil, err
	}
	return r.load(ctx, row)
}

func (r *Repository) Save(ctx context.Context, inv *domain.Inventory) error {
	if err := r.q.UpsertInventory(ctx, toUpsertInventoryParams(inv)); err != nil {
		return err
	}

	existing, err := r.listHolds(ctx, inv.Id())
	if err != nil {
		return err
	}

	holds := inv.Holds()
	kept := make(map[uuid.UUID]struct{}, len(holds))
	for _, h := range holds {
		kept[h.Id()] = struct{}{}
	}

	for _, e := range existing {
		if _, ok := kept[e.ID]; ok {
			continue
		}
		if err := r.q.DeleteHoldById(ctx, e.ID); err != nil {
			return err
		}
	}

	for _, h := range holds {
		params, err := toUpsertHoldParams(inv, h)
		if err != nil {
			return err
		}
		if err := r.q.UpsertHold(ctx, params); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListExpiredInventoryIds(ctx context.Context, now time.Time) ([]*domain.InventoryId, error) {
	rows, err := r.q.ListExpiredHoldInventoryIds(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		return nil, err
	}
	ids := make([]*domain.InventoryId, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, domain.ReconstructInventoryId(row.RoomTypeID, row.Date))
	}
	return ids, nil
}

func (r *Repository) ListSummaries(
	ctx context.Context,
	roomTypeId uuid.UUID,
	from, to time.Time,
) ([]inventoryapp.InventorySummary, error) {
	rows, err := r.q.ListInventorySummaries(ctx, inventorydb.ListInventorySummariesParams{
		RoomTypeID: roomTypeId,
		Date:       from,
		Date_2:     to,
	})
	if err != nil {
		return nil, err
	}
	summaries := make([]inventoryapp.InventorySummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, inventoryapp.InventorySummary{
			Date:              row.Date,
			Fee:               int(row.Fee),
			QuantityAvailable: int(row.QuantityAvailable),
			HeldCount:         int(row.HeldCount),
			IsClosed:          row.IsClosed,
		})
	}
	return summaries, nil
}

func (r *Repository) load(ctx context.Context, row inventorydb.Inventory) (*domain.Inventory, error) {
	holds, err := r.q.ListHoldsByRoomTypeIdAndDate(ctx, inventorydb.ListHoldsByRoomTypeIdAndDateParams{
		Date:       row.Date,
		RoomTypeID: row.RoomTypeID,
	})
	if err != nil {
		return nil, err
	}
	return toInventory(row, holds)
}

func (r *Repository) listHolds(ctx context.Context, id *domain.InventoryId) ([]inventorydb.Hold, error) {
	return r.q.ListHoldsByRoomTypeIdAndDate(ctx, inventorydb.ListHoldsByRoomTypeIdAndDateParams{
		Date:       id.Date(),
		RoomTypeID: id.RoomTypeId(),
	})
}

type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

var _ inventoryapp.Transactor = (*Transactor)(nil)

const lockNotAvailable = "55P03"

const LockTimeout = "3s"

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context, repo inventoryapp.Repository) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '"+LockTimeout+"'"); err != nil {
		return err
	}

	if err := fn(ctx, NewRepository(tx)); err != nil {
		return translateLockTimeout(err)
	}
	return tx.Commit(ctx)
}

func translateLockTimeout(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == lockNotAvailable {
		return inventoryapp.ErrBusy
	}
	return err
}
