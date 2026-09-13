package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewInventoryId(t *testing.T) {
	t.Run("未来の日時に対して在庫を作成できる", func(t *testing.T) {
		_, err := NewInventoryId(CreateNewInventoryIdInput{
			Date:       time.Now().AddDate(0, 0, 1),
			RoomTypeId: uuid.New(),
			Now:        time.Now(),
		})
		require.NoError(t, err)
	})
	t.Run("過去の日付は作成できない", func(t *testing.T) {
		_, err := NewInventoryId(CreateNewInventoryIdInput{
			Date:       time.Date(2026, 9, 6, 0, 0, 0, 0, time.Local),
			RoomTypeId: uuid.New(),
			Now:        time.Now(),
		})
		if err == nil {
			t.Fatal("エラーが返りませんでした")
		}
		if err.Error() != ErrPast.Error() {
			t.Errorf("エラーメッセージが違います: got %q", err.Error())
		}
	})
}
