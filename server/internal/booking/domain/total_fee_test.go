package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTotalFee(t *testing.T) {
	t.Run("正常系", func(t *testing.T) {
		tests := []struct {
			name  string
			input int
		}{
			{"0円を作れる", 0},
			{"正の金額を作れる", 30000},
			{"1円を作れる", 1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := NewTotalFee(tt.input)
				require.NoError(t, err)
				require.Equal(t, tt.input, got.Amount())
			})
		}
	})

	t.Run("異常系", func(t *testing.T) {
		tests := []struct {
			name    string
			input   int
			wantErr error
		}{
			{"負の金額は作れない", -1, ErrInvalidTotalFee},
			{"大きな負の金額も作れない", -30000, ErrInvalidTotalFee},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewTotalFee(tt.input)
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	})
}
