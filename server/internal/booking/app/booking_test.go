package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	bookings map[uuid.UUID]*domain.Booking
}

func newFakeStore() *fakeStore {
	return &fakeStore{bookings: map[uuid.UUID]*domain.Booking{}}
}

func (s *fakeStore) WithinTx(ctx context.Context, fn func(ctx context.Context, repo Repository) error) error {
	return fn(ctx, s)
}

func (s *fakeStore) Find(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	b, ok := s.bookings[id]
	if !ok {
		return nil, domain.ErrBookingNotFound
	}
	return b, nil
}

func (s *fakeStore) FindForUpdate(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	return s.Find(ctx, id)
}

func (s *fakeStore) Save(ctx context.Context, b *domain.Booking) error {
	s.bookings[b.Id()] = b
	return nil
}

func (s *fakeStore) ListSummariesByBookerId(ctx context.Context, bookerId uuid.UUID) ([]BookingSummary, error) {
	summaries := make([]BookingSummary, 0)
	for _, b := range s.bookings {
		if b.BookerId() != bookerId {
			continue
		}
		summaries = append(summaries, BookingSummary{
			BookingId:       b.Id(),
			RoomTypeId:      b.RoomTypeId(),
			CheckinDate:     b.StayPeriod().CheckinDate(),
			CheckoutDate:    b.StayPeriod().CheckoutDate(),
			TotalFee:        b.TotalFee().Amount(),
			CancellationFee: b.CancellationFee().Amount(),
			Status:          b.Status(),
		})
	}
	return summaries, nil
}

type fixedCapacity int

func (c fixedCapacity) Capacity(ctx context.Context, roomTypeId uuid.UUID) (int, error) {
	return int(c), nil
}

type fakeInventory struct {
	feePerNight int
	soldOutOn   *time.Time

	held     []time.Time
	holdIds  []uuid.UUID
	released []time.Time
	calls    []string
}

func (f *fakeInventory) Hold(
	ctx context.Context,
	roomTypeId uuid.UUID,
	date time.Time,
	holdId, bookingId uuid.UUID,
	expiredAt, now time.Time,
) (HeldSlot, error) {
	f.calls = append(f.calls, "hold")
	if f.soldOutOn != nil && date.Equal(*f.soldOutOn) {
		return HeldSlot{}, errSoldOut
	}
	f.held = append(f.held, date)
	f.holdIds = append(f.holdIds, holdId)
	return HeldSlot{SlotNo: len(f.held), FeeAmount: f.feePerNight}, nil
}

func (f *fakeInventory) StartPayment(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	f.calls = append(f.calls, "startPayment")
	return nil
}

func (f *fakeInventory) Confirm(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	f.calls = append(f.calls, "confirm")
	return nil
}

func (f *fakeInventory) Release(ctx context.Context, roomTypeId uuid.UUID, date time.Time, bookingId uuid.UUID) error {
	f.calls = append(f.calls, "release")
	f.released = append(f.released, date)
	return nil
}

var errSoldOut = errors.New("満室です")

var appNow = time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func newTestService(t *testing.T, capacity int, inv *fakeInventory) (*Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	return NewService(store, fixedCapacity(capacity), inv), store
}

func threeNights() BookInput {
	return BookInput{
		BookerId:     uuid.New(),
		RoomTypeId:   uuid.New(),
		CheckinDate:  date(2026, 12, 24),
		CheckoutDate: date(2026, 12, 27),
		Guests:       []GuestInput{{FirstName: "太郎", LastName: "山田"}},
	}
}

func TestService_Book(t *testing.T) {
	t.Run("全日程の在庫が取れれば予約できる", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, store := newTestService(t, 2, inv)

		b, err := s.Book(context.Background(), threeNights(), appNow)

		require.NoError(t, err)
		require.Equal(t, domain.TemporaryHold, b.Status())
		require.Len(t, inv.held, 3)
		require.Contains(t, store.bookings, b.Id())
	})

	t.Run("宿泊料金は確保できた日の料金の合計になる", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)

		b, err := s.Book(context.Background(), threeNights(), appNow)

		require.NoError(t, err)
		require.Equal(t, 30000, b.TotalFee().Amount())
	})

	t.Run("在庫は日付の昇順で確保する", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)

		_, err := s.Book(context.Background(), threeNights(), appNow)

		require.NoError(t, err)
		for i := 1; i < len(inv.held); i++ {
			require.True(t, inv.held[i].After(inv.held[i-1]), "昇順でなければデッドロックの原因になる")
		}
	})

	t.Run("確保IDは日ごとに別のものが採番される", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)

		_, err := s.Book(context.Background(), threeNights(), appNow)

		require.NoError(t, err)
		require.Len(t, inv.holdIds, 3)
		seen := map[uuid.UUID]struct{}{}
		for _, id := range inv.holdIds {
			require.NotEqual(t, uuid.Nil, id, "確保IDが採番されていない")
			seen[id] = struct{}{}
		}
		require.Len(t, seen, 3, "日ごとに別の確保IDでなければならない")
	})

	t.Run("途中の日が満室なら、確保済みの分が解放される", func(t *testing.T) {
		soldOut := date(2026, 12, 26)
		inv := &fakeInventory{feePerNight: 10000, soldOutOn: &soldOut}
		s, _ := newTestService(t, 2, inv)

		_, err := s.Book(context.Background(), threeNights(), appNow)

		require.ErrorIs(t, err, errSoldOut)

		require.Equal(t, []time.Time{date(2026, 12, 24), date(2026, 12, 25)}, inv.released)
	})

	t.Run("途中で失敗した仮予約はキャンセル済みになる", func(t *testing.T) {
		soldOut := date(2026, 12, 24)
		inv := &fakeInventory{feePerNight: 10000, soldOutOn: &soldOut}
		s, store := newTestService(t, 2, inv)

		_, err := s.Book(context.Background(), threeNights(), appNow)

		require.ErrorIs(t, err, errSoldOut)
		require.Len(t, store.bookings, 1)
		for _, b := range store.bookings {
			require.Equal(t, domain.Cancelled, b.Status())
		}
	})

	t.Run("定員を超えていれば在庫に手を付けない", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, store := newTestService(t, 1, inv)

		in := threeNights()
		in.Guests = []GuestInput{
			{FirstName: "太郎", LastName: "山田"},
			{FirstName: "花子", LastName: "山田"},
		}

		_, err := s.Book(context.Background(), in, appNow)

		require.ErrorIs(t, err, domain.ErrOverCapacity)
		require.Empty(t, inv.calls)
		require.Empty(t, store.bookings)
	})

	t.Run("宿泊期間が不正なら在庫に手を付けない", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)

		in := threeNights()
		in.CheckoutDate = in.CheckinDate

		_, err := s.Book(context.Background(), in, appNow)

		require.ErrorIs(t, err, domain.ErrInvalidStayPeriod)
		require.Empty(t, inv.calls)
	})
}

func TestService_Confirm(t *testing.T) {
	t.Run("在庫の期限を外してから予約を確定する", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)
		b, err := s.Book(context.Background(), threeNights(), appNow)
		require.NoError(t, err)
		inv.calls = nil

		require.NoError(t, s.Confirm(context.Background(), b.Id()))

		require.Equal(t, domain.Confirmed, b.Status())

		require.Equal(t, []string{
			"startPayment", "startPayment", "startPayment",
			"confirm", "confirm", "confirm",
		}, inv.calls)
	})

	t.Run("存在しない予約は確定できない", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)

		err := s.Confirm(context.Background(), uuid.New())

		require.ErrorIs(t, err, domain.ErrBookingNotFound)
	})
}

func TestService_Cancel(t *testing.T) {
	t.Run("キャンセルすると全日程の在庫が解放される", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)
		b, err := s.Book(context.Background(), threeNights(), appNow)
		require.NoError(t, err)

		fee, err := s.Cancel(context.Background(), b.Id(), domain.ByGuest, appNow)

		require.NoError(t, err)
		require.Equal(t, domain.Cancelled, b.Status())
		require.Len(t, inv.released, 3)

		require.Equal(t, 0, fee.Amount())
	})

	t.Run("確定済みをキャンセルするとキャンセル料が出る", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 2, inv)
		b, err := s.Book(context.Background(), threeNights(), appNow)
		require.NoError(t, err)
		require.NoError(t, s.Confirm(context.Background(), b.Id()))

		fee, err := s.Cancel(context.Background(), b.Id(), domain.ByGuest, date(2026, 12, 22))

		require.NoError(t, err)
		require.Equal(t, 30000, fee.Amount())
	})
}

func TestService_ChangeGuests(t *testing.T) {
	t.Run("宿泊者を変更できる", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 3, inv)
		b, err := s.Book(context.Background(), threeNights(), appNow)
		require.NoError(t, err)

		err = s.ChangeGuests(context.Background(), b.Id(), []GuestInput{
			{FirstName: "太郎", LastName: "山田"},
			{FirstName: "花子", LastName: "山田"},
		})

		require.NoError(t, err)
		require.Equal(t, 2, b.GuestCount())
	})

	t.Run("定員を超える変更はできない", func(t *testing.T) {
		inv := &fakeInventory{feePerNight: 10000}
		s, _ := newTestService(t, 1, inv)
		b, err := s.Book(context.Background(), threeNights(), appNow)
		require.NoError(t, err)

		err = s.ChangeGuests(context.Background(), b.Id(), []GuestInput{
			{FirstName: "太郎", LastName: "山田"},
			{FirstName: "花子", LastName: "山田"},
		})

		require.ErrorIs(t, err, domain.ErrOverCapacity)
		require.Equal(t, 1, b.GuestCount())
	})
}
