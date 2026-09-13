package app

import (
	"context"
	"errors"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	"github.com/google/uuid"
)

var ErrBusy = errors.New("在庫が混み合っています。しばらくしてからお試しください")

type Repository interface {
	Add(ctx context.Context, inv *domain.Inventory) error
	Find(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error)
	FindForUpdate(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error)
	Save(ctx context.Context, inv *domain.Inventory) error
	ListExpiredInventoryIds(ctx context.Context, now time.Time) ([]*domain.InventoryId, error)
	ListSummaries(ctx context.Context, roomTypeId uuid.UUID, from, to time.Time) ([]InventorySummary, error)
}

type InventorySummary struct {
	Date              time.Time
	Fee               int
	QuantityAvailable int
	HeldCount         int
	IsClosed          bool
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error
}
