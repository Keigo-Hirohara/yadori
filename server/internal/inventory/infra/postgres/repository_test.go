package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/infra/postgres"
	"github.com/Keigo-Hirohara/yadori/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func stayDate() time.Time {
	d := time.Now().AddDate(0, 0, 7)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func insertRoomType(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	accommodationId := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO accommodations (id, name, phone_number, postal_code, prefecture, city, street_address)
		VALUES ($1, 'やどり旅館', '0312345678', '1000001', '東京都', '千代田区', '1-1-1')`,
		accommodationId)
	require.NoError(t, err)

	roomTypeId := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO room_types (id, accommodation_id, name, capacity)
		VALUES ($1, $2, '和室', 4)`,
		roomTypeId, accommodationId)
	require.NoError(t, err)

	return roomTypeId
}

func insertBooking(t *testing.T, pool *pgxpool.Pool, roomTypeId uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	bookerId := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO bookers (id, first_name, last_name, postal_code, phone_number, prefecture, city, street_address)
		VALUES ($1, '太郎', '山田', '1000001', '09012345678', '東京都', '千代田区', '1-1-1')`,
		bookerId)
	require.NoError(t, err)

	bookingId := uuid.New()
	date := stayDate()
	_, err = pool.Exec(ctx, `
		INSERT INTO bookings (id, booker_id, room_type_id, total_fee, checkin_date, checkout_date)
		VALUES ($1, $2, $3, 15000, $4, $5)`,
		bookingId, bookerId, roomTypeId, date, date.AddDate(0, 0, 1))
	require.NoError(t, err)

	return bookingId
}

func countHolds(t *testing.T, pool *pgxpool.Pool, roomTypeId uuid.UUID, date time.Time) int {
	t.Helper()
	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM holds WHERE room_type_id = $1 AND date = $2`,
		roomTypeId, date).Scan(&count)
	require.NoError(t, err)
	return count
}

func newInventory(t *testing.T, roomTypeId uuid.UUID, date time.Time, quantity int, holds ...domain.Hold) *domain.Inventory {
	t.Helper()
	fee, err := domain.NewFee(domain.CreateNewFeeInput{Amount: 15000})
	require.NoError(t, err)
	return domain.Reconstruct(domain.ReconstructInventoryId(roomTypeId, date), quantity, fee, false, holds)
}

func temporaryHold(bookingId uuid.UUID, slotNo int, expiredAt time.Time) domain.Hold {
	return domain.ReconstructHold(uuid.New(), bookingId, slotNo, domain.TemporaryHold, &expiredAt)
}

func addInventory(t *testing.T, pool *pgxpool.Pool, repo *postgres.Repository, quantity int) (uuid.UUID, *domain.Inventory) {
	t.Helper()
	roomTypeId := insertRoomType(t, pool)
	inv := newInventory(t, roomTypeId, stayDate(), quantity)
	require.NoError(t, repo.Add(context.Background(), inv))
	return roomTypeId, inv
}

func TestRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なため -short では実行しない")
	}
	ctx := context.Background()
	pool := testutil.SetupDB(t)
	repo := postgres.NewRepository(pool)
	date := stayDate()

	t.Run("追加した在庫を読み戻せる", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)

		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))

		require.NoError(t, err)
		require.Equal(t, 3, got.QuantityAvailable())
		require.Equal(t, 15000, got.Fee().Amount())
		require.False(t, got.IsClosed())
		require.Equal(t, 0, got.HoldCount())
		require.Equal(t, roomTypeId, got.Id().RoomTypeId())
		require.True(t, got.Id().Date().Equal(date), "日付が往復で変わってはいけない")
	})

	t.Run("同じ日を二重に追加すると主キー違反が業務の言葉に翻訳される", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)

		err := repo.Add(ctx, newInventory(t, roomTypeId, date, 5))

		require.ErrorIs(t, err, domain.ErrAlreadyRegistered)
	})

	t.Run("二重追加を試みても元の行は変わらない", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)

		_ = repo.Add(ctx, newInventory(t, roomTypeId, date, 5))

		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 3, got.QuantityAvailable())
	})

	t.Run("無い在庫は見つからないエラーになる", func(t *testing.T) {
		id := domain.ReconstructInventoryId(uuid.New(), date)

		_, err := repo.Find(ctx, id)
		require.ErrorIs(t, err, domain.ErrInventoryNotFound)

		_, err = repo.FindForUpdate(ctx, id)
		require.ErrorIs(t, err, domain.ErrInventoryNotFound)
	})

	t.Run("在庫の変更が保存される", func(t *testing.T) {
		roomTypeId, inv := addInventory(t, pool, repo, 3)
		require.NoError(t, inv.ChangeQuantity(5))
		require.NoError(t, inv.Close())

		require.NoError(t, repo.Save(ctx, inv))

		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 5, got.QuantityAvailable())
		require.True(t, got.IsClosed())
	})

	t.Run("確保つきで保存すると枠番号・状態・期限が復元される", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)
		bookingId := insertBooking(t, pool, roomTypeId)
		expiredAt := time.Now().Add(30 * time.Minute).Truncate(time.Microsecond)
		inv := newInventory(t, roomTypeId, date, 3, temporaryHold(bookingId, 2, expiredAt))

		require.NoError(t, repo.Save(ctx, inv))

		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 1, got.HoldCount())
		h, ok := got.FindHold(bookingId)
		require.True(t, ok)
		require.Equal(t, 2, h.SlotNo())
		require.Equal(t, domain.TemporaryHold, h.Status())
		require.NotNil(t, h.ExpiredAt())
		require.True(t, h.ExpiredAt().Equal(expiredAt))
	})

	t.Run("確保の3つの状態がDBのenumと往復し、期限なしはNULLになる", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)
		expiredAt := time.Now().Add(30 * time.Minute)
		cases := []struct {
			status    domain.HoldStatus
			expiredAt *time.Time
		}{
			{domain.TemporaryHold, &expiredAt},
			{domain.ProcessingPayment, nil},
			{domain.Confirmed, nil},
		}
		holds := make([]domain.Hold, 0, len(cases))
		bookingIds := make([]uuid.UUID, 0, len(cases))
		for i, c := range cases {
			bookingId := insertBooking(t, pool, roomTypeId)
			bookingIds = append(bookingIds, bookingId)
			holds = append(holds, domain.ReconstructHold(uuid.New(), bookingId, i+1, c.status, c.expiredAt))
		}

		require.NoError(t, repo.Save(ctx, newInventory(t, roomTypeId, date, 3, holds...)))

		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		for i, c := range cases {
			h, ok := got.FindHold(bookingIds[i])
			require.True(t, ok)
			require.Equal(t, c.status, h.Status())
			if c.expiredAt == nil {
				require.Nil(t, h.ExpiredAt(), "決済中・確定済みは期限なしで保存される")
			} else {
				require.NotNil(t, h.ExpiredAt())
			}
		}
	})

	t.Run("集約から消えた確保は行ごと削除される", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)
		b1 := insertBooking(t, pool, roomTypeId)
		b2 := insertBooking(t, pool, roomTypeId)
		expiredAt := time.Now().Add(30 * time.Minute)
		require.NoError(t, repo.Save(ctx, newInventory(t, roomTypeId, date, 3,
			temporaryHold(b1, 1, expiredAt),
			temporaryHold(b2, 2, expiredAt),
		)))
		require.Equal(t, 2, countHolds(t, pool, roomTypeId, date))

		inv, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.NoError(t, inv.Release(b1))
		require.NoError(t, repo.Save(ctx, inv))

		require.Equal(t, 1, countHolds(t, pool, roomTypeId, date), "確保の数は行数そのもの。カウンタは持たない")
		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		_, ok := got.FindHold(b1)
		require.False(t, ok)
		_, ok = got.FindHold(b2)
		require.True(t, ok)
	})

	t.Run("解放した枠番号を別の確保が再利用しても一意制約に触れない", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 1)
		b1 := insertBooking(t, pool, roomTypeId)
		b2 := insertBooking(t, pool, roomTypeId)
		expiredAt := time.Now().Add(30 * time.Minute)
		require.NoError(t, repo.Save(ctx, newInventory(t, roomTypeId, date, 1, temporaryHold(b1, 1, expiredAt))))

		err := repo.Save(ctx, newInventory(t, roomTypeId, date, 1, temporaryHold(b2, 1, expiredAt)))

		require.NoError(t, err)
		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 1, got.HoldCount())
		h, ok := got.FindHold(b2)
		require.True(t, ok)
		require.Equal(t, 1, h.SlotNo())
	})

	t.Run("期限切れの仮確保を持つ在庫だけが日付の昇順で返る", func(t *testing.T) {
		roomTypeId := insertRoomType(t, pool)
		now := time.Now()
		later := date.AddDate(0, 0, 1)

		expiredA := newInventory(t, roomTypeId, later, 3, temporaryHold(insertBooking(t, pool, roomTypeId), 1, now.Add(-time.Minute)))
		expiredB := newInventory(t, roomTypeId, date, 3, temporaryHold(insertBooking(t, pool, roomTypeId), 1, now.Add(-time.Minute)))
		processing := newInventory(t, roomTypeId, date.AddDate(0, 0, 2), 3,
			domain.ReconstructHold(uuid.New(), insertBooking(t, pool, roomTypeId), 1, domain.ProcessingPayment, nil))
		unexpired := newInventory(t, roomTypeId, date.AddDate(0, 0, 3), 3,
			temporaryHold(insertBooking(t, pool, roomTypeId), 1, now.Add(time.Hour)))
		for _, inv := range []*domain.Inventory{expiredA, expiredB, processing, unexpired} {
			require.NoError(t, repo.Add(ctx, inv))
			require.NoError(t, repo.Save(ctx, inv))
		}

		ids, err := repo.ListExpiredInventoryIds(ctx, now)

		require.NoError(t, err)

		mine := make([]time.Time, 0)
		for _, id := range ids {
			if id.RoomTypeId() == roomTypeId {
				mine = append(mine, id.Date())
			}
		}
		require.Len(t, mine, 2)
		require.True(t, mine[0].Equal(date), "昇順でなければロックの順序が揃わない")
		require.True(t, mine[1].Equal(later))
	})

	t.Run("表示用の読み取りは確保数を数えて期間で絞る", func(t *testing.T) {
		roomTypeId := insertRoomType(t, pool)
		expiredAt := time.Now().Add(30 * time.Minute)
		inRange := newInventory(t, roomTypeId, date, 3,
			temporaryHold(insertBooking(t, pool, roomTypeId), 1, expiredAt),
			temporaryHold(insertBooking(t, pool, roomTypeId), 2, expiredAt),
		)
		outOfRange := newInventory(t, roomTypeId, date.AddDate(0, 0, 5), 3)
		for _, inv := range []*domain.Inventory{inRange, outOfRange} {
			require.NoError(t, repo.Add(ctx, inv))
			require.NoError(t, repo.Save(ctx, inv))
		}

		summaries, err := repo.ListSummaries(ctx, roomTypeId, date, date.AddDate(0, 0, 2))

		require.NoError(t, err)
		require.Len(t, summaries, 1, "to は含まないので範囲外の日は入らない")
		require.True(t, summaries[0].Date.Equal(date))
		require.Equal(t, 3, summaries[0].QuantityAvailable)
		require.Equal(t, 2, summaries[0].HeldCount)
	})
}

func TestTransactor(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なため -short では実行しない")
	}
	ctx := context.Background()
	pool := testutil.SetupDB(t)
	repo := postgres.NewRepository(pool)
	tx := postgres.NewTransactor(pool)
	date := stayDate()

	t.Run("エラーを返すと書き込みが取り消される", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)
		boom := errors.New("途中で失敗")

		err := tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
			inv, err := repo.FindForUpdate(ctx, domain.ReconstructInventoryId(roomTypeId, date))
			if err != nil {
				return err
			}
			if err := inv.ChangeQuantity(10); err != nil {
				return err
			}
			if err := repo.Save(ctx, inv); err != nil {
				return err
			}
			return boom
		})

		require.ErrorIs(t, err, boom)
		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 3, got.QuantityAvailable(), "保存したあとに失敗しても、なかったことになる")
	})

	t.Run("正常に抜けると書き込みが確定する", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)

		err := tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
			inv, err := repo.FindForUpdate(ctx, domain.ReconstructInventoryId(roomTypeId, date))
			if err != nil {
				return err
			}
			if err := inv.ChangeQuantity(10); err != nil {
				return err
			}
			return repo.Save(ctx, inv)
		})

		require.NoError(t, err)
		got, err := repo.Find(ctx, domain.ReconstructInventoryId(roomTypeId, date))
		require.NoError(t, err)
		require.Equal(t, 10, got.QuantityAvailable())
	})

	t.Run("ロック待ちが上限を超えると混雑として返る", func(t *testing.T) {
		roomTypeId, _ := addInventory(t, pool, repo, 3)
		id := domain.ReconstructInventoryId(roomTypeId, date)

		locked := make(chan struct{})
		release := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
				_, err := repo.FindForUpdate(ctx, id)
				close(locked)
				<-release
				return err
			})
		}()
		<-locked

		err := tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
			_, err := repo.FindForUpdate(ctx, id)
			return err
		})

		close(release)
		wg.Wait()
		require.ErrorIs(t, err, inventoryapp.ErrBusy, "pgx のエラーではなく業務側の言葉で返す")
	})
}

func TestDoubleBooking(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なため -short では実行しない")
	}
	ctx := context.Background()
	pool := testutil.SetupDB(t)
	repo := postgres.NewRepository(pool)
	tx := postgres.NewTransactor(pool)
	date := stayDate()

	roomTypeId, _ := addInventory(t, pool, repo, 1)
	id := domain.ReconstructInventoryId(roomTypeId, date)

	const attempts = 100
	bookingIds := make([]uuid.UUID, attempts)
	for i := range bookingIds {
		bookingIds[i] = insertBooking(t, pool, roomTypeId)
	}

	var (
		mu        sync.Mutex
		succeeded int
		soldOut   int
		others    []error
		wg        sync.WaitGroup
	)
	now := time.Now()

	for _, bookingId := range bookingIds {
		wg.Add(1)
		go func(bookingId uuid.UUID) {
			defer wg.Done()
			err := tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
				inv, err := repo.FindForUpdate(ctx, id)
				if err != nil {
					return err
				}
				if _, err := inv.Hold(domain.HoldInput{
					HoldId:     uuid.New(),
					BookingId:  bookingId,
					RoomTypeId: roomTypeId,
					ExpiredAt:  now.Add(30 * time.Minute),
					Date:       now,
				}); err != nil {
					return err
				}
				return repo.Save(ctx, inv)
			})

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, domain.ErrSoldOut):
				soldOut++
			default:
				others = append(others, err)
			}
		}(bookingId)
	}
	wg.Wait()

	require.Empty(t, others, "満室以外のエラーが出てはいけない")
	require.Equal(t, 1, succeeded, "成功はちょうど1件でなければならない")
	require.Equal(t, attempts-1, soldOut)
	require.Equal(t, 1, countHolds(t, pool, roomTypeId, date), "確保の行数が枠数を超えてはいけない")
}

func TestNoDeadlockAcrossDates(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なため -short では実行しない")
	}
	ctx := context.Background()
	pool := testutil.SetupDB(t)
	repo := postgres.NewRepository(pool)
	tx := postgres.NewTransactor(pool)

	roomTypeId := insertRoomType(t, pool)
	const nights = 3
	const attempts = 20
	dates := make([]time.Time, 0, nights)
	for i := 0; i < nights; i++ {
		d := stayDate().AddDate(0, 0, i)
		dates = append(dates, d)
		require.NoError(t, repo.Add(ctx, newInventory(t, roomTypeId, d, attempts)))
	}

	bookingIds := make([]uuid.UUID, attempts)
	for i := range bookingIds {
		bookingIds[i] = insertBooking(t, pool, roomTypeId)
	}

	var (
		mu     sync.Mutex
		failed []error
		wg     sync.WaitGroup
	)
	now := time.Now()

	for i, bookingId := range bookingIds {
		wg.Add(1)
		go func(i int, bookingId uuid.UUID) {
			defer wg.Done()
			order := dates
			if i%2 == 1 {
				order = []time.Time{dates[2], dates[1], dates[0]}
			}
			for _, date := range order {
				err := tx.WithinTx(ctx, func(ctx context.Context, repo inventoryapp.Repository) error {
					inv, err := repo.FindForUpdate(ctx, domain.ReconstructInventoryId(roomTypeId, date))
					if err != nil {
						return err
					}
					if _, err := inv.Hold(domain.HoldInput{
						HoldId:     uuid.New(),
						BookingId:  bookingId,
						RoomTypeId: roomTypeId,
						ExpiredAt:  now.Add(30 * time.Minute),
						Date:       now,
					}); err != nil {
						return err
					}
					return repo.Save(ctx, inv)
				})
				if err != nil {
					mu.Lock()
					failed = append(failed, err)
					mu.Unlock()
					return
				}
			}
		}(i, bookingId)
	}
	wg.Wait()

	require.Empty(t, failed, "ロック待ち超過やデッドロックが起きてはいけない")
	for _, d := range dates {
		require.Equal(t, attempts, countHolds(t, pool, roomTypeId, d))
	}
}
