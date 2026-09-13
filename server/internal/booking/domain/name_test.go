package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewName(t *testing.T) {
	t.Run("正常系", func(t *testing.T) {
		tests := []struct {
			name      string
			firstName string
			lastName  string
		}{
			{"姓名が揃っていれば作れる", "太郎", "山田"},
			{"1文字ずつでも作れる", "太", "山"},
			{"アルファベットでも作れる", "Taro", "Yamada"},
			{"50文字ずつでも作れる", strings.Repeat("あ", 50), strings.Repeat("い", 50)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := NewName(tt.firstName, tt.lastName)
				require.NoError(t, err)
				require.Equal(t, tt.firstName, got.FirstName())
				require.Equal(t, tt.lastName, got.LastName())
			})
		}
	})

	t.Run("異常系", func(t *testing.T) {
		tests := []struct {
			name      string
			firstName string
			lastName  string
			wantErr   error
		}{
			{"名が空なら作れない", "", "山田", ErrInvalidName},
			{"姓が空なら作れない", "太郎", "", ErrInvalidName},
			{"両方空なら作れない", "", "", ErrInvalidName},
			{"名が空白のみなら作れない", "   ", "山田", ErrInvalidName},
			{"姓が空白のみなら作れない", "太郎", "   ", ErrInvalidName},
			{"名が51文字なら作れない", strings.Repeat("あ", 51), "山田", ErrInvalidName},
			{"姓が51文字なら作れない", "太郎", strings.Repeat("い", 51), ErrInvalidName},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewName(tt.firstName, tt.lastName)
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	})

	t.Run("正規化", func(t *testing.T) {
		t.Run("前後の空白が除去される", func(t *testing.T) {
			got, err := NewName("  太郎  ", "  山田  ")

			require.NoError(t, err)
			require.Equal(t, "太郎", got.FirstName())
			require.Equal(t, "山田", got.LastName())
		})
	})
}

func TestName_FullName(t *testing.T) {
	t.Run("姓名が連結される", func(t *testing.T) {
		n, err := NewName("太郎", "山田")
		require.NoError(t, err)

		require.Equal(t, "山田 太郎", n.FullName())
	})
}
