package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFee(t *testing.T) {
	t.Run("金額が自然数であれば、作成できる", func(t *testing.T) {
		_, err := NewFee(CreateNewFeeInput{
			Amount: 5000,
		})
		require.NoError(t, err)
		t.Run("金額がゼロの場合でも作成できる", func(t *testing.T) {
			_, err := NewFee(CreateNewFeeInput{
				Amount: 0,
			})
			require.NoError(t, err)
		})
	})
	t.Run("金額がマイナスの場合、作成できない", func(t *testing.T) {
		_, err := NewFee(CreateNewFeeInput{
			Amount: -1,
		})
		if err == nil {
			t.Fatal("エラーが返りませんでした")
		}
		if err.Error() != ErrMinusFee.Error() {
			t.Errorf("エラーメッセージが違います: got %q", err.Error())
		}
	})
}
