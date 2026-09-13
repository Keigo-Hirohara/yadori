package postgres

import (
	"context"
	"errors"

	bookingapp "github.com/Keigo-Hirohara/yadori/internal/booking/app"
	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	bookingdb "github.com/Keigo-Hirohara/yadori/internal/booking/infra/postgres/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	q *bookingdb.Queries
}

func NewRepository(db bookingdb.DBTX) *Repository {
	return &Repository{q: bookingdb.New(db)}
}

var _ bookingapp.Repository = (*Repository)(nil)

func (r *Repository) Find(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	row, err := r.q.GetBookingById(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, err
	}
	return r.load(ctx, row)
}

func (r *Repository) FindForUpdate(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	row, err := r.q.GetBookingByIdForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, err
	}
	return r.load(ctx, row)
}

func (r *Repository) Save(ctx context.Context, b *domain.Booking) error {
	params, err := toUpsertBookingParams(b)
	if err != nil {
		return err
	}
	if err := r.q.UpsertBooking(ctx, params); err != nil {
		return err
	}

	existing, err := r.q.ListGuestsByBookingId(ctx, b.Id())
	if err != nil {
		return err
	}

	guests := b.Guests()
	kept := make(map[uuid.UUID]struct{}, len(guests))
	for _, g := range guests {
		kept[g.Id()] = struct{}{}
	}
	for _, e := range existing {
		if _, ok := kept[e.ID]; ok {
			continue
		}
		if err := r.q.DeleteGuestById(ctx, e.ID); err != nil {
			return err
		}
	}

	for _, g := range guests {
		err := r.q.UpsertGuest(ctx, bookingdb.UpsertGuestParams{
			ID:        g.Id(),
			BookingID: b.Id(),
			FirstName: g.Name().FirstName(),
			LastName:  g.Name().LastName(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListSummariesByBookerId(
	ctx context.Context,
	bookerId uuid.UUID,
) ([]bookingapp.BookingSummary, error) {
	rows, err := r.q.ListBookingSummariesByBookerId(ctx, bookerId)
	if err != nil {
		return nil, err
	}
	summaries := make([]bookingapp.BookingSummary, 0, len(rows))
	for _, row := range rows {
		status, err := toDomainStatus(row.Status)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, bookingapp.BookingSummary{
			BookingId:       row.ID,
			RoomTypeId:      row.RoomTypeID,
			CheckinDate:     row.CheckinDate,
			CheckoutDate:    row.CheckoutDate,
			TotalFee:        int(row.TotalFee),
			CancellationFee: int(row.CancellationFee),
			Status:          status,
		})
	}
	return summaries, nil
}

func (r *Repository) load(ctx context.Context, row bookingdb.Booking) (*domain.Booking, error) {
	guests, err := r.q.ListGuestsByBookingId(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return toBooking(row, guests)
}

type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

var _ bookingapp.Transactor = (*Transactor)(nil)

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context, repo bookingapp.Repository) error) error {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(ctx, NewRepository(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
