package accommodation

import (
	"context"
	"strings"
	"testing"

	"github.com/Keigo-Hirohara/yadori/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewRoomType(t *testing.T) {
	t.Run("部屋タイプ名", func(t *testing.T) {
		t.Run("1文字以上60文字以内なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "1文字なら登録できる", input: "松"},
				{name: "60文字なら登録できる", input: strings.Repeat("あ", 60)},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewRoomType(RoomTypeCreateInput{
						AccommodationId: uuid.New(),
						Name:            tt.input,
						Capacity:        10,
						HasPrivateBath:  true,
						HasBalcony:      true,
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.Name() != tt.input {
						t.Errorf("部屋タイプ名が保持されていません: got %q, want %q", got.Name(), tt.input)
					}
				})
			}
		})

		t.Run("1文字未満または60文字を超えると、登録できない", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "空だと登録できない", input: ""},
				{name: "61文字だと登録できない", input: strings.Repeat("あ", 61)},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewRoomType(RoomTypeCreateInput{
						AccommodationId: uuid.New(),
						Name:            tt.input,
						Capacity:        10,
						HasPrivateBath:  true,
						HasBalcony:      true,
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidRoomTypeName.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("定員", func(t *testing.T) {
		t.Run("1名以上なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input int
			}{
				{name: "1名なら登録できる", input: 1},
				{name: "10名なら登録できる", input: 10},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewRoomType(RoomTypeCreateInput{
						AccommodationId: uuid.New(),
						Name:            "スーペリアルーム",
						Capacity:        tt.input,
						HasPrivateBath:  true,
						HasBalcony:      true,
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.Capacity() != tt.input {
						t.Errorf("定員が保持されていません: got %d, want %d", got.Capacity(), tt.input)
					}
				})
			}
		})

		t.Run("1名未満だと、登録できない", func(t *testing.T) {
			tests := []struct {
				name  string
				input int
			}{
				{name: "0名だと登録できない", input: 0},
				{name: "負の数だと登録できない", input: -1},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewRoomType(RoomTypeCreateInput{
						AccommodationId: uuid.New(),
						Name:            "スーペリアルーム",
						Capacity:        tt.input,
						HasPrivateBath:  true,
						HasBalcony:      true,
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidCapacity.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("宿のID", func(t *testing.T) {
		t.Run("指定されていないと、登録できない", func(t *testing.T) {
			_, err := NewRoomType(RoomTypeCreateInput{
				AccommodationId: uuid.Nil,
				Name:            "スーペリアルーム",
				Capacity:        10,
				HasPrivateBath:  true,
				HasBalcony:      true,
			})
			if err == nil {
				t.Fatal("エラーが返りませんでした")
			}
			if err.Error() != ErrInvalidAccommodationId.Error() {
				t.Errorf("エラーメッセージが違います: got %q", err.Error())
			}
		})
	})
}

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
