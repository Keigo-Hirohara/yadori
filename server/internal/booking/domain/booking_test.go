package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var bookingNow = time.Date(2026, 12, 20, 10, 0, 0, 0, time.UTC)

func testGuests(t *testing.T, n int) []Guest {
	t.Helper()
	guests := make([]Guest, 0, n)
	for i := 0; i < n; i++ {
		name, err := NewName("太郎", "山田")
		require.NoError(t, err)
		guests = append(guests, NewGuest(uuid.New(), name))
	}
	return guests
}

func newTestBooking(t *testing.T, guestCount, capacity int) *Booking {
	t.Helper()
	period, err := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 27))
	require.NoError(t, err)
	fee, err := NewTotalFee(30000)
	require.NoError(t, err)

	b, err := Book(uuid.New(), uuid.New(), uuid.New(), period,
		testGuests(t, guestCount), fee, capacity, bookingNow)
	require.NoError(t, err)
	return b
}

func TestBook(t *testing.T) {
	period, _ := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 27))
	fee, _ := NewTotalFee(30000)

	t.Run("必要な情報が揃っていれば予約できる", func(t *testing.T) {
		b, err := Book(uuid.New(), uuid.New(), uuid.New(), period,
			testGuests(t, 2), fee, 2, bookingNow)

		require.NoError(t, err)
		require.Equal(t, TemporaryHold, b.Status())
		require.Equal(t, 2, b.GuestCount())
	})

	t.Run("宿泊者が定員ちょうどなら予約できる", func(t *testing.T) {
		_, err := Book(uuid.New(), uuid.New(), uuid.New(), period,
			testGuests(t, 2), fee, 2, bookingNow)

		require.NoError(t, err)
	})

	t.Run("宿泊者が定員を超えると予約できない", func(t *testing.T) {
		_, err := Book(uuid.New(), uuid.New(), uuid.New(), period,
			testGuests(t, 3), fee, 2, bookingNow)

		require.ErrorIs(t, err, ErrOverCapacity)
	})

	t.Run("宿泊者が0人なら予約できない", func(t *testing.T) {
		_, err := Book(uuid.New(), uuid.New(), uuid.New(), period,
			testGuests(t, 0), fee, 2, bookingNow)

		require.ErrorIs(t, err, ErrNoGuest)
	})

	t.Run("過去の日程は予約できない", func(t *testing.T) {
		past, _ := NewStayPeriod(date(2026, 12, 18), date(2026, 12, 19))

		_, err := Book(uuid.New(), uuid.New(), uuid.New(), past,
			testGuests(t, 2), fee, 2, bookingNow)

		require.ErrorIs(t, err, ErrPastStayPeriod)
	})

	t.Run("当日の予約はできる", func(t *testing.T) {
		today, _ := NewStayPeriod(date(2026, 12, 20), date(2026, 12, 21))

		_, err := Book(uuid.New(), uuid.New(), uuid.New(), today,
			testGuests(t, 2), fee, 2, bookingNow)

		require.NoError(t, err)
	})
}

func TestBooking_StartPayment(t *testing.T) {
	t.Run("仮予約から決済処理中にできる", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)

		err := b.StartPayment()

		require.NoError(t, err)
		require.Equal(t, ProcessingPayment, b.Status())
	})

	t.Run("確定済みからは決済処理中にできない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		require.NoError(t, b.StartPayment())
		require.NoError(t, b.Confirm())

		err := b.StartPayment()

		require.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("キャンセル済みからは決済処理中にできない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		_, err := b.Cancel(ByGuest, bookingNow)
		require.NoError(t, err)

		err = b.StartPayment()

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBooking_Confirm(t *testing.T) {
	t.Run("決済処理中から確定にできる", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		require.NoError(t, b.StartPayment())

		err := b.Confirm()

		require.NoError(t, err)
		require.Equal(t, Confirmed, b.Status())
	})

	t.Run("仮予約から直接確定にはできない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)

		err := b.Confirm()

		require.ErrorIs(t, err, ErrInvalidTransition)
	})

	t.Run("二重に確定できない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		require.NoError(t, b.StartPayment())
		require.NoError(t, b.Confirm())

		err := b.Confirm()

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBooking_Cancel(t *testing.T) {
	t.Run("仮予約をキャンセルできる", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)

		fee, err := b.Cancel(ByGuest, bookingNow)

		require.NoError(t, err)
		require.Equal(t, Cancelled, b.Status())
		require.Equal(t, 0, fee.Amount())
	})

	t.Run("確定済みをキャンセルできる", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		require.NoError(t, b.StartPayment())
		require.NoError(t, b.Confirm())

		fee, err := b.Cancel(ByGuest, date(2026, 12, 22))

		require.NoError(t, err)
		require.Equal(t, Cancelled, b.Status())
		require.Equal(t, 30000, fee.Amount())
	})

	t.Run("宿都合ならキャンセル料は発生しない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		require.NoError(t, b.StartPayment())
		require.NoError(t, b.Confirm())

		fee, err := b.Cancel(ByAccommodation, date(2026, 12, 24))

		require.NoError(t, err)
		require.Equal(t, 0, fee.Amount())
	})

	t.Run("二重キャンセルは冪等に成功する", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		_, err := b.Cancel(ByGuest, bookingNow)
		require.NoError(t, err)

		fee, err := b.Cancel(ByGuest, bookingNow)

		require.NoError(t, err)
		require.Equal(t, 0, fee.Amount())
		require.Equal(t, Cancelled, b.Status())
	})
}

func TestBooking_ChangeGuests(t *testing.T) {
	t.Run("宿泊者を変更できる", func(t *testing.T) {
		b := newTestBooking(t, 2, 3)

		err := b.ChangeGuests(testGuests(t, 3), 3)

		require.NoError(t, err)
		require.Equal(t, 3, b.GuestCount())
	})

	t.Run("定員を超える変更はできない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)

		err := b.ChangeGuests(testGuests(t, 3), 2)

		require.ErrorIs(t, err, ErrOverCapacity)
		require.Equal(t, 2, b.GuestCount())
	})

	t.Run("0人には変更できない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)

		err := b.ChangeGuests(testGuests(t, 0), 2)

		require.ErrorIs(t, err, ErrNoGuest)
	})

	t.Run("キャンセル済みは変更できない", func(t *testing.T) {
		b := newTestBooking(t, 2, 2)
		_, err := b.Cancel(ByGuest, bookingNow)
		require.NoError(t, err)

		err = b.ChangeGuests(testGuests(t, 1), 2)

		require.ErrorIs(t, err, ErrInvalidTransition)
	})
}

func TestBooking_GuestCount(t *testing.T) {
	t.Run("宿泊者の件数から人数が導出される", func(t *testing.T) {
		b := newTestBooking(t, 3, 4)
		require.Equal(t, 3, b.GuestCount())
	})
}

func TestCalculateCancellationFee(t *testing.T) {
	checkin := date(2026, 12, 24)
	period, _ := NewStayPeriod(checkin, date(2026, 12, 27))
	total, _ := NewTotalFee(30000)

	t.Run("客都合", func(t *testing.T) {
		tests := []struct {
			name string
			now  time.Time
			want int
		}{
			{"8日前は無料", date(2026, 12, 16), 0},
			{"7日前は無料", date(2026, 12, 17), 0},
			{"6日前は50%", date(2026, 12, 18), 15000},
			{"3日前は50%", date(2026, 12, 21), 15000},
			{"2日前は全額", date(2026, 12, 22), 30000},
			{"前日は全額", date(2026, 12, 23), 30000},
			{"当日は全額", date(2026, 12, 24), 30000},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := CalculateCancellationFee(period, total, ByGuest, tt.now)
				require.Equal(t, tt.want, got.Amount())
			})
		}
	})

	t.Run("宿都合は当日でも無料", func(t *testing.T) {
		got := CalculateCancellationFee(period, total, ByAccommodation, date(2026, 12, 24))
		require.Equal(t, 0, got.Amount())
	})

	t.Run("決済失敗は無料", func(t *testing.T) {
		got := CalculateCancellationFee(period, total, PaymentFailed, date(2026, 12, 23))
		require.Equal(t, 0, got.Amount())
	})

	t.Run("期限切れは無料", func(t *testing.T) {
		got := CalculateCancellationFee(period, total, Expired, date(2026, 12, 23))
		require.Equal(t, 0, got.Amount())
	})
}
