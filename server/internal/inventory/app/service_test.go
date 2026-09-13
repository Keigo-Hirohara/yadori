package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	inventories map[string]*domain.Inventory
	expiredIds  []*domain.InventoryId

	lockedReads int
	failSave    bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{inventories: map[string]*domain.Inventory{}}
}

func key(id *domain.InventoryId) string {
	return id.RoomTypeId().String() + "|" + id.Date().Format(time.DateOnly)
}

func clone(inv *domain.Inventory) *domain.Inventory {
	return domain.Reconstruct(inv.Id(), inv.QuantityAvailable(), inv.Fee(), inv.IsClosed(), inv.Holds())
}

func (s *fakeStore) WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error {
	snapshot := make(map[string]*domain.Inventory, len(s.inventories))
	for k, v := range s.inventories {
		snapshot[k] = v
	}
	if err := fn(ctx, s); err != nil {
		s.inventories = snapshot
		return err
	}
	return nil
}

func (s *fakeStore) Add(ctx context.Context, inv *domain.Inventory) error {
	if s.failSave {
		return errSaveFailed
	}
	if _, ok := s.inventories[key(inv.Id())]; ok {
		return domain.ErrAlreadyRegistered
	}
	s.inventories[key(inv.Id())] = clone(inv)
	return nil
}

func (s *fakeStore) Find(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error) {
	return s.get(id)
}

func (s *fakeStore) FindForUpdate(ctx context.Context, id *domain.InventoryId) (*domain.Inventory, error) {
	s.lockedReads++
	return s.get(id)
}

func (s *fakeStore) Save(ctx context.Context, inv *domain.Inventory) error {
	if s.failSave {
		return errSaveFailed
	}
	s.inventories[key(inv.Id())] = clone(inv)
	return nil
}

func (s *fakeStore) ListExpiredInventoryIds(ctx context.Context, now time.Time) ([]*domain.InventoryId, error) {
	return s.expiredIds, nil
}

func (s *fakeStore) ListSummaries(
	ctx context.Context,
	roomTypeId uuid.UUID,
	from, to time.Time,
) ([]InventorySummary, error) {
	summaries := make([]InventorySummary, 0)
	for _, inv := range s.inventories {
		id := inv.Id()
		if id.RoomTypeId() != roomTypeId || id.Date().Before(from) || !id.Date().Before(to) {
			continue
		}
		summaries = append(summaries, InventorySummary{
			Date:              id.Date(),
			Fee:               inv.Fee().Amount(),
			QuantityAvailable: inv.QuantityAvailable(),
			HeldCount:         inv.HoldCount(),
			IsClosed:          inv.IsClosed(),
		})
	}
	return summaries, nil
}

func (s *fakeStore) get(id *domain.InventoryId) (*domain.Inventory, error) {
	inv, ok := s.inventories[key(id)]
	if !ok {
		return nil, domain.ErrInventoryNotFound
	}
	return clone(inv), nil
}

var errSaveFailed = errors.New("保存に失敗しました")

const feeAmount = 15000

var (
	appNow   = time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC)
	stayDate = time.Date(2026, 12, 24, 0, 0, 0, 0, time.UTC)
)

func newTestService(t *testing.T) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	return NewService(store), store
}

func registeredInventory(t *testing.T, quantity int) (*Service, *fakeStore, uuid.UUID) {
	t.Helper()
	store := newFakeStore()
	service := NewService(store)
	roomTypeId := uuid.New()

	err := service.Register(context.Background(), RegisterInput{
		RoomTypeId: roomTypeId,
		Date:       stayDate,
		Quantity:   quantity,
		FeeAmount:  feeAmount,
		Now:        appNow,
	})
	require.NoError(t, err)

	return service, store, roomTypeId
}

func stored(t *testing.T, store *fakeStore, roomTypeId uuid.UUID) *domain.Inventory {
	t.Helper()
	inv, err := store.Find(context.Background(), domain.ReconstructInventoryId(roomTypeId, stayDate))
	require.NoError(t, err)
	return inv
}

func holdInput(roomTypeId, bookingId uuid.UUID) HoldInput {
	return HoldInput{
		HoldId:     uuid.New(),
		RoomTypeId: roomTypeId,
		Date:       stayDate,
		BookingId:  bookingId,
		ExpiredAt:  appNow.Add(30 * time.Minute),
		Now:        appNow,
	}
}

func addHold(t *testing.T, s *Service, roomTypeId uuid.UUID) uuid.UUID {
	t.Helper()
	bookingId := uuid.New()
	_, err := s.Hold(context.Background(), holdInput(roomTypeId, bookingId))
	require.NoError(t, err)
	return bookingId
}

func TestService_Register(t *testing.T) {
	t.Run("登録した販売枠をあとから読み出せる", func(t *testing.T) {
		_, store, roomTypeId := registeredInventory(t, 3)

		inv := stored(t, store, roomTypeId)
		require.Equal(t, 3, inv.QuantityAvailable())
		require.Equal(t, feeAmount, inv.Fee().Amount())
		require.False(t, inv.IsClosed())
		require.Equal(t, 3, inv.Available())
	})

	t.Run("過去の日付には登録できない", func(t *testing.T) {
		s, _ := newTestService(t)

		err := s.Register(context.Background(), RegisterInput{
			RoomTypeId: uuid.New(),
			Date:       appNow.AddDate(0, 0, -1),
			Quantity:   3,
			FeeAmount:  feeAmount,
			Now:        appNow,
		})

		require.ErrorIs(t, err, domain.ErrPast)
	})

	t.Run("料金が負なら登録できない", func(t *testing.T) {
		s, _ := newTestService(t)

		err := s.Register(context.Background(), RegisterInput{
			RoomTypeId: uuid.New(), Date: stayDate, Quantity: 3, FeeAmount: -1, Now: appNow,
		})

		require.ErrorIs(t, err, domain.ErrMinusFee)
	})

	t.Run("販売可能数が負なら登録できない", func(t *testing.T) {
		s, _ := newTestService(t)

		err := s.Register(context.Background(), RegisterInput{
			RoomTypeId: uuid.New(), Date: stayDate, Quantity: -1, FeeAmount: feeAmount, Now: appNow,
		})

		require.ErrorIs(t, err, domain.ErrInvalidQuantity)
	})

	t.Run("同じ日を二重に登録できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)

		err := s.Register(context.Background(), RegisterInput{
			RoomTypeId: roomTypeId, Date: stayDate, Quantity: 5, FeeAmount: 20000, Now: appNow,
		})

		require.ErrorIs(t, err, domain.ErrAlreadyRegistered)
	})

	t.Run("二重登録を試みても既存の確保は消えない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		_ = s.Register(context.Background(), RegisterInput{
			RoomTypeId: roomTypeId, Date: stayDate, Quantity: 5, FeeAmount: 20000, Now: appNow,
		})

		inv := stored(t, store, roomTypeId)
		require.Equal(t, 1, inv.HoldCount())
		_, ok := inv.FindHold(bookingId)
		require.True(t, ok)
		require.Equal(t, 3, inv.QuantityAvailable(), "枠数も上書きされない")
	})

	t.Run("登録に失敗した在庫は保存されない", func(t *testing.T) {
		store := newFakeStore()
		s := NewService(store)
		roomTypeId := uuid.New()

		err := s.Register(context.Background(), RegisterInput{
			RoomTypeId: roomTypeId, Date: stayDate, Quantity: -1, FeeAmount: feeAmount, Now: appNow,
		})

		require.Error(t, err)
		require.Empty(t, store.inventories)
	})
}

func TestService_Hold(t *testing.T) {
	t.Run("空きがあれば確保でき、枠番号とその日の料金が返る", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)

		slot, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.NoError(t, err)
		require.Equal(t, 1, slot.SlotNo)
		require.Equal(t, feeAmount, slot.FeeAmount)
	})

	t.Run("確保した内容が保存される", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := uuid.New()

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, bookingId))
		require.NoError(t, err)

		inv := stored(t, store, roomTypeId)
		require.Equal(t, 1, inv.HoldCount())
		require.Equal(t, 2, inv.Available())
		_, ok := inv.FindHold(bookingId)
		require.True(t, ok)
	})

	t.Run("販売可能数まで確保できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)

		for i := 1; i <= 3; i++ {
			slot, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))
			require.NoError(t, err)
			require.Equal(t, i, slot.SlotNo)
		}

		require.Equal(t, 0, stored(t, store, roomTypeId).Available())
	})

	t.Run("満室なら確保できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 1)
		addHold(t, s, roomTypeId)

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.ErrorIs(t, err, domain.ErrSoldOut)
	})

	t.Run("確保に失敗しても在庫は変化しない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 1)
		addHold(t, s, roomTypeId)

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))
		require.Error(t, err)

		inv := stored(t, store, roomTypeId)
		require.Equal(t, 1, inv.HoldCount(), "失敗した確保が残ってはいけない")
	})

	t.Run("登録されていない在庫は確保できない", func(t *testing.T) {
		s, _ := newTestService(t)

		_, err := s.Hold(context.Background(), holdInput(uuid.New(), uuid.New()))

		require.ErrorIs(t, err, domain.ErrInventoryNotFound)
	})

	t.Run("販売停止中は確保できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		require.NoError(t, s.Close(context.Background(), roomTypeId, stayDate))

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.ErrorIs(t, err, domain.ErrClosed)
	})

	t.Run("解放された枠番号は再利用される", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		first := addHold(t, s, roomTypeId)
		addHold(t, s, roomTypeId)
		require.NoError(t, s.Release(context.Background(), roomTypeId, stayDate, first))

		slot, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.NoError(t, err)
		require.Equal(t, 1, slot.SlotNo)
	})

	t.Run("状態を変えるときはロックを取って読む", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		store.lockedReads = 0

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.NoError(t, err)
		require.Equal(t, 1, store.lockedReads, "ロックなしで読んでいるとダブルブッキングを防げない")
	})
}

func TestService_StartPayment(t *testing.T) {
	t.Run("仮確保を決済中にできる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		err := s.StartPayment(context.Background(), roomTypeId, stayDate, bookingId)

		require.NoError(t, err)
		h, ok := stored(t, store, roomTypeId).FindHold(bookingId)
		require.True(t, ok)
		require.Equal(t, domain.ProcessingPayment, h.Status())
	})

	t.Run("決済中になると期限が外れる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		require.NoError(t, s.StartPayment(context.Background(), roomTypeId, stayDate, bookingId))

		h, _ := stored(t, store, roomTypeId).FindHold(bookingId)
		require.Nil(t, h.ExpiredAt(), "期限が残っていると決済中に回収されてしまう")
	})

	t.Run("存在しない確保では失敗する", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)

		err := s.StartPayment(context.Background(), roomTypeId, stayDate, uuid.New())

		require.ErrorIs(t, err, domain.ErrHoldNotFound)
	})
}

func TestService_Confirm(t *testing.T) {
	t.Run("決済中を確定にできる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)
		require.NoError(t, s.StartPayment(context.Background(), roomTypeId, stayDate, bookingId))

		err := s.Confirm(context.Background(), roomTypeId, stayDate, bookingId)

		require.NoError(t, err)
		h, _ := stored(t, store, roomTypeId).FindHold(bookingId)
		require.Equal(t, domain.Confirmed, h.Status())
	})

	t.Run("仮確保から直接は確定できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		err := s.Confirm(context.Background(), roomTypeId, stayDate, bookingId)

		require.ErrorIs(t, err, domain.ErrInvalidTransition)
	})

	t.Run("確定に失敗しても確保の状態は変わらない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		_ = s.Confirm(context.Background(), roomTypeId, stayDate, bookingId)

		h, _ := stored(t, store, roomTypeId).FindHold(bookingId)
		require.Equal(t, domain.TemporaryHold, h.Status())
	})
}

func TestService_Release(t *testing.T) {
	t.Run("解放すると枠が戻る", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)

		err := s.Release(context.Background(), roomTypeId, stayDate, bookingId)

		require.NoError(t, err)
		inv := stored(t, store, roomTypeId)
		require.Equal(t, 0, inv.HoldCount())
		require.Equal(t, 3, inv.Available())
	})

	t.Run("確定済みも解放できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)
		require.NoError(t, s.StartPayment(context.Background(), roomTypeId, stayDate, bookingId))
		require.NoError(t, s.Confirm(context.Background(), roomTypeId, stayDate, bookingId))

		err := s.Release(context.Background(), roomTypeId, stayDate, bookingId)

		require.NoError(t, err)
		require.Equal(t, 0, stored(t, store, roomTypeId).HoldCount())
	})

	t.Run("存在しない確保では失敗する", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)

		err := s.Release(context.Background(), roomTypeId, stayDate, uuid.New())

		require.ErrorIs(t, err, domain.ErrHoldNotFound)
	})
}

func TestService_ChangeQuantity(t *testing.T) {
	t.Run("販売可能数を増やせる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)

		err := s.ChangeQuantity(context.Background(), roomTypeId, stayDate, 5)

		require.NoError(t, err)
		require.Equal(t, 5, stored(t, store, roomTypeId).QuantityAvailable())
	})

	t.Run("確保数を下回る変更はできない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)
		addHold(t, s, roomTypeId)

		err := s.ChangeQuantity(context.Background(), roomTypeId, stayDate, 1)

		require.ErrorIs(t, err, domain.ErrQuantityBelowHolds)
	})

	t.Run("変更に失敗したら元の枠数のまま", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)
		addHold(t, s, roomTypeId)

		_ = s.ChangeQuantity(context.Background(), roomTypeId, stayDate, 1)

		require.Equal(t, 3, stored(t, store, roomTypeId).QuantityAvailable())
	})

	t.Run("登録されていない在庫は変更できない", func(t *testing.T) {
		s, _ := newTestService(t)

		err := s.ChangeQuantity(context.Background(), uuid.New(), stayDate, 5)

		require.ErrorIs(t, err, domain.ErrInventoryNotFound)
	})
}

func TestService_ChangeFee(t *testing.T) {
	t.Run("料金を変更できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)

		err := s.ChangeFee(context.Background(), roomTypeId, stayDate, 20000)

		require.NoError(t, err)
		require.Equal(t, 20000, stored(t, store, roomTypeId).Fee().Amount())
	})

	t.Run("確保があっても料金を変更できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)

		err := s.ChangeFee(context.Background(), roomTypeId, stayDate, 20000)

		require.NoError(t, err)
		require.Equal(t, 1, stored(t, store, roomTypeId).HoldCount(), "既存の確保は残る")
	})

	t.Run("料金が負なら変更できない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)

		err := s.ChangeFee(context.Background(), roomTypeId, stayDate, -1)

		require.ErrorIs(t, err, domain.ErrMinusFee)
		require.Equal(t, feeAmount, stored(t, store, roomTypeId).Fee().Amount())
	})
}

func TestService_Close(t *testing.T) {
	t.Run("販売を停止できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)

		err := s.Close(context.Background(), roomTypeId, stayDate)

		require.NoError(t, err)
		require.True(t, stored(t, store, roomTypeId).IsClosed())
	})

	t.Run("停止しても枠数と既存の確保は残る", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)

		require.NoError(t, s.Close(context.Background(), roomTypeId, stayDate))

		inv := stored(t, store, roomTypeId)
		require.Equal(t, 3, inv.QuantityAvailable())
		require.Equal(t, 1, inv.HoldCount())
	})

	t.Run("既に停止中なら停止できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		require.NoError(t, s.Close(context.Background(), roomTypeId, stayDate))

		err := s.Close(context.Background(), roomTypeId, stayDate)

		require.ErrorIs(t, err, domain.ErrAlreadyClosed)
	})
}

func TestService_Reopen(t *testing.T) {
	t.Run("販売を再開できる", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		require.NoError(t, s.Close(context.Background(), roomTypeId, stayDate))

		err := s.Reopen(context.Background(), roomTypeId, stayDate)

		require.NoError(t, err)
		require.False(t, stored(t, store, roomTypeId).IsClosed())
	})

	t.Run("再開すれば確保できる", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)
		require.NoError(t, s.Close(context.Background(), roomTypeId, stayDate))
		require.NoError(t, s.Reopen(context.Background(), roomTypeId, stayDate))

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.NoError(t, err)
	})

	t.Run("停止していなければ再開できない", func(t *testing.T) {
		s, _, roomTypeId := registeredInventory(t, 3)

		err := s.Reopen(context.Background(), roomTypeId, stayDate)

		require.ErrorIs(t, err, domain.ErrNotClosed)
	})
}

func TestService_CollectExpired(t *testing.T) {
	t.Run("期限切れの仮確保が回収され、件数が返る", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)
		store.expiredIds = []*domain.InventoryId{domain.ReconstructInventoryId(roomTypeId, stayDate)}

		collected, err := s.CollectExpired(context.Background(), appNow.Add(31*time.Minute))

		require.NoError(t, err)
		require.Equal(t, 1, collected)
		require.Equal(t, 0, stored(t, store, roomTypeId).HoldCount())
	})

	t.Run("期限内の仮確保は回収されない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		addHold(t, s, roomTypeId)
		store.expiredIds = []*domain.InventoryId{domain.ReconstructInventoryId(roomTypeId, stayDate)}

		collected, err := s.CollectExpired(context.Background(), appNow.Add(time.Minute))

		require.NoError(t, err)
		require.Equal(t, 0, collected)
		require.Equal(t, 1, stored(t, store, roomTypeId).HoldCount())
	})

	t.Run("決済中の確保は回収されない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		bookingId := addHold(t, s, roomTypeId)
		require.NoError(t, s.StartPayment(context.Background(), roomTypeId, stayDate, bookingId))
		store.expiredIds = []*domain.InventoryId{domain.ReconstructInventoryId(roomTypeId, stayDate)}

		collected, err := s.CollectExpired(context.Background(), appNow.Add(24*time.Hour))

		require.NoError(t, err)
		require.Equal(t, 0, collected, "決済中に在庫を取り上げると、課金だけが成立する")
		require.Equal(t, 1, stored(t, store, roomTypeId).HoldCount())
	})

	t.Run("複数の在庫にまたがって回収できる", func(t *testing.T) {
		store := newFakeStore()
		s := NewService(store)

		ids := make([]*domain.InventoryId, 0, 2)
		for i := 0; i < 2; i++ {
			roomTypeId := uuid.New()
			require.NoError(t, s.Register(context.Background(), RegisterInput{
				RoomTypeId: roomTypeId, Date: stayDate, Quantity: 3, FeeAmount: feeAmount, Now: appNow,
			}))
			addHold(t, s, roomTypeId)
			ids = append(ids, domain.ReconstructInventoryId(roomTypeId, stayDate))
		}
		store.expiredIds = ids

		collected, err := s.CollectExpired(context.Background(), appNow.Add(31*time.Minute))

		require.NoError(t, err)
		require.Equal(t, 2, collected)
	})

	t.Run("回収対象がなければ0件を返す", func(t *testing.T) {
		s, _ := newTestService(t)

		collected, err := s.CollectExpired(context.Background(), appNow)

		require.NoError(t, err)
		require.Equal(t, 0, collected)
	})
}

func TestService_保存に失敗したとき(t *testing.T) {
	t.Run("失敗がそのまま返る", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		store.failSave = true

		_, err := s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		require.ErrorIs(t, err, errSaveFailed)
	})

	t.Run("在庫は変化しない", func(t *testing.T) {
		s, store, roomTypeId := registeredInventory(t, 3)
		store.failSave = true

		_, _ = s.Hold(context.Background(), holdInput(roomTypeId, uuid.New()))

		store.failSave = false
		require.Equal(t, 0, stored(t, store, roomTypeId).HoldCount())
	})
}
