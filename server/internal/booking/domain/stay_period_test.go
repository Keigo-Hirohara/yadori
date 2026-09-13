package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestNewStayPeriod(t *testing.T) {
	t.Run("チェックアウトがチェックインより後なら作成できる", func(t *testing.T) {
		p, err := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 25))
		require.NoError(t, err)
		require.Equal(t, 1, p.Nights())
	})

	t.Run("チェックアウトがチェックインと同日なら作成できない", func(t *testing.T) {
		_, err := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 24))
		require.ErrorIs(t, err, ErrInvalidStayPeriod)
	})

	t.Run("チェックアウトがチェックインより前なら作成できない", func(t *testing.T) {
		_, err := NewStayPeriod(date(2026, 12, 25), date(2026, 12, 24))
		require.ErrorIs(t, err, ErrInvalidStayPeriod)
	})

	t.Run("時刻が含まれていても日付に丸められる", func(t *testing.T) {
		in := time.Date(2026, 12, 24, 15, 30, 0, 0, time.UTC)
		out := time.Date(2026, 12, 25, 9, 0, 0, 0, time.UTC)

		p, err := NewStayPeriod(in, out)

		require.NoError(t, err)
		require.True(t, p.CheckinDate().Equal(date(2026, 12, 24)))
		require.True(t, p.CheckoutDate().Equal(date(2026, 12, 25)))
	})
}

func TestStayPeriod_Nights(t *testing.T) {
	tests := []struct {
		name     string
		checkin  time.Time
		checkout time.Time
		want     int
	}{
		{"1泊", date(2026, 12, 24), date(2026, 12, 25), 1},
		{"3泊", date(2026, 12, 24), date(2026, 12, 27), 3},
		{"月をまたぐ", date(2026, 12, 30), date(2027, 1, 2), 3},
		{"年をまたぐ", date(2026, 12, 31), date(2027, 1, 1), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewStayPeriod(tt.checkin, tt.checkout)
			require.NoError(t, err)
			require.Equal(t, tt.want, p.Nights())
		})
	}
}

func TestStayPeriod_Dates(t *testing.T) {
	t.Run("宿泊日の一覧を返す", func(t *testing.T) {
		p, _ := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 27))

		got := p.Dates()

		require.Len(t, got, 3)
		require.True(t, got[0].Equal(date(2026, 12, 24)))
		require.True(t, got[1].Equal(date(2026, 12, 25)))
		require.True(t, got[2].Equal(date(2026, 12, 26)))
	})

	t.Run("チェックアウト日は含まれない", func(t *testing.T) {
		p, _ := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 25))

		got := p.Dates()

		require.Len(t, got, 1)
		require.True(t, got[0].Equal(date(2026, 12, 24)))
	})

	t.Run("日付は昇順で返る", func(t *testing.T) {
		p, _ := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 28))

		got := p.Dates()

		for i := 1; i < len(got); i++ {
			require.True(t, got[i].After(got[i-1]), "昇順でなければデッドロックの原因になる")
		}
	})

	t.Run("泊数と日付の件数が一致する", func(t *testing.T) {
		p, _ := NewStayPeriod(date(2026, 12, 24), date(2026, 12, 30))

		require.Equal(t, p.Nights(), len(p.Dates()))
	})
}
