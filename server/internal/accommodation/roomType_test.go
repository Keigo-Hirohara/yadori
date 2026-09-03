package accommodation

import (
	"context"
	"testing"

	"github.com/Keigo-Hirohara/yadori/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreateRoomType(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なテストをスキップ")
	}

	pool := testutil.SetupDB(t)
	ctx := context.Background()

	t.Run("宿が提供する部屋のタイプを作成できる", func(t *testing.T) {
		_, err := NewRoomType(RoomTypeCreateInput{
			AccommodationId: uuid.New(),
			HasPrivateBath:  true,
			HasBalcony:      true,
			Capacity:        10,
			Name:            "スーペリアルーム",
		})

		if err != nil {
			t.Errorf("部屋タイプの登録ができませんでした: %s", err)
		}
	})

	t.Run("作成した宿を保存できる", func(t *testing.T) {

		accommodation, err := NewAccommodation(AccommodationCreateInput{
			Name:          "舞浜ホテル",
			PhoneNumber:   "04712344321",
			PostalCode:    "1000000",
			Prefecture:    "千葉県",
			City:          "浦安市",
			StreetAddress: "舞浜1-1",
			Building:      "舞浜ホテル",
		})
		require.NoError(t, err)
		require.NoError(t, accommodation.Save(ctx, pool))

		roomType, err := NewRoomType(RoomTypeCreateInput{
			AccommodationId: accommodation.ID(),
			HasPrivateBath:  true,
			HasBalcony:      true,
			Capacity:        10,
			Name:            "スーペリアルーム",
		})

		if err != nil {
			t.Errorf("部屋タイプの登録ができませんでした: %s", err)
		}

		require.NoError(t, roomType.Save(ctx, pool))

		got, err := FindRoomTypeById(ctx, pool, roomType.ID())

		require.NoError(t, err)
		require.Equal(t, roomType.Name(), got.Name())
	})

	t.Run("存在しない宿の登録をしようとしていた場合、登録が失敗する", func(t *testing.T) {
		roomType, err := NewRoomType(RoomTypeCreateInput{
			AccommodationId: uuid.New(),
			HasPrivateBath:  true,
			HasBalcony:      true,
			Capacity:        10,
			Name:            "スーペリアルーム",
		})
		require.NoError(t, err)

		err = roomType.Save(ctx, pool)

		if err == nil {
			t.Fatalf("エラーが返りませんでした")
		}

		if err.Error() != ErrInvalidAccommodationId.Error() {
			t.Errorf("エラーメッセージが違います: got %q", err.Error())
		}
	})
}

func TestListRoomTypesByAccommodation(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なテストをスキップ")
	}

	ctx := context.Background()
	pool := testutil.SetupDB(t)

	accommodation, err := NewAccommodation(AccommodationCreateInput{
		Name:          "舞浜ホテル",
		PhoneNumber:   "04712344321",
		PostalCode:    "1000000",
		Prefecture:    "千葉県",
		City:          "浦安市",
		StreetAddress: "舞浜1-1",
		Building:      "舞浜ホテル",
	})

	if err != nil {
		t.Fatalf("宿が登録できませんでした: %s", err)
	}

	require.NoError(t, accommodation.Save(ctx, pool))

	roomTypeInputs := []RoomTypeCreateInput{
		{
			AccommodationId: accommodation.ID(),
			HasPrivateBath:  true,
			HasBalcony:      true,
			Capacity:        10,
			Name:            "スーペリアルーム",
		},
		{
			AccommodationId: accommodation.ID(),
			HasPrivateBath:  true,
			HasBalcony:      false,
			Capacity:        8,
			Name:            "スタンダードルーム",
		},
		{
			AccommodationId: accommodation.ID(),
			HasPrivateBath:  true,
			HasBalcony:      true,
			Capacity:        20,
			Name:            "スイートルーム",
		},
	}

	for _, roomTypeInput := range roomTypeInputs {
		roomType, err := NewRoomType(roomTypeInput)
		if err != nil {
			t.Fatalf("Error occured: %s", err)
		}
		require.NoError(t, roomType.Save(ctx, pool))
	}

	t.Run("宿が提供する部屋タイプの一覧を取得できる", func(t *testing.T) {

	})
}
