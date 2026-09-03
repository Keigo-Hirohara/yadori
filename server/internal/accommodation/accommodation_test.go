package accommodation

import (
	"context"
	"strings"
	"testing"

	"github.com/Keigo-Hirohara/yadori/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewAccommodation(t *testing.T) {
	t.Run("必須項目が揃っていれば、宿を登録できる", func(t *testing.T) {
		_, err := New(AccommodationCreateInput{
			Name:          "舞浜ホテル",
			PhoneNumber:   "04712344321",
			PostalCode:    "1000000",
			Prefecture:    "千葉県",
			City:          "浦安市",
			StreetAddress: "舞浜1-1",
			Building:      "舞浜ホテル",
		})
		if err != nil {
			t.Errorf("Error occured: %s", err)
		}
	})

	t.Run("名前", func(t *testing.T) {
		t.Run("1文字以上60文字以内なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "1文字なら登録できる", input: "宿"},
				{name: "60文字なら登録できる", input: strings.Repeat("あ", 60)},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := New(AccommodationCreateInput{
						Name:          tt.input,
						PhoneNumber:   "04712344321",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.Name() != tt.input {
						t.Errorf("名前が保持されていません: got %q, want %q", got.Name(), tt.input)
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
					_, err := New(AccommodationCreateInput{
						Name:          tt.input,
						PhoneNumber:   "04712344321",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidName.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("電話番号", func(t *testing.T) {
		t.Run("0から始まる10桁または11桁の数字なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
				want  string
			}{
				{name: "10桁なら登録できる", input: "0312345678", want: "0312345678"},
				{name: "11桁なら登録できる", input: "04712344321", want: "04712344321"},
				{name: "ハイフン入りならハイフンが除かれて登録できる", input: "047-1234-4321", want: "04712344321"},
				{name: "10桁のハイフン入りならハイフンが除かれて登録できる", input: "03-1234-5678", want: "0312345678"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := New(AccommodationCreateInput{
						Name:          "舞浜ホテル",
						PhoneNumber:   tt.input,
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.PhoneNumber() != tt.want {
						t.Errorf("電話番号が正規化されていません: got %q, want %q", got.PhoneNumber(), tt.want)
					}
				})
			}
		})

		t.Run("形式が違うと、登録できない", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "空だと登録できない", input: ""},
				{name: "0から始まらないと登録できない", input: "9012345678"},
				{name: "9桁だと登録できない", input: "047123443"},
				{name: "12桁だと登録できない", input: "047123443210"},
				{name: "数字以外が含まれると登録できない", input: "0471234432a"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := New(AccommodationCreateInput{
						Name:          "舞浜ホテル",
						PhoneNumber:   tt.input,
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidPhoneNumber.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("郵便番号", func(t *testing.T) {
		t.Run("7桁の数字なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
				want  string
			}{
				{name: "7桁なら登録できる", input: "1000000", want: "1000000"},
				{name: "ハイフン入りならハイフンが除かれて登録できる", input: "100-0000", want: "1000000"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := New(AccommodationCreateInput{
						Name:          "舞浜ホテル",
						PhoneNumber:   "04712344321",
						PostalCode:    tt.input,
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.PostalCode() != tt.want {
						t.Errorf("郵便番号が正規化されていません: got %q, want %q", got.PostalCode(), tt.want)
					}
				})
			}
		})

		t.Run("形式が違うと、登録できない", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "空だと登録できない", input: ""},
				{name: "6桁だと登録できない", input: "100000"},
				{name: "8桁だと登録できない", input: "10000000"},
				{name: "数字以外が含まれると登録できない", input: "100000a"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := New(AccommodationCreateInput{
						Name:          "舞浜ホテル",
						PhoneNumber:   "04712344321",
						PostalCode:    tt.input,
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜ホテル",
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidPostalCode.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("都道府県", func(t *testing.T) {
		t.Run("入力されていれば、登録できる", func(t *testing.T) {
			got, err := New(AccommodationCreateInput{
				Name:          "舞浜ホテル",
				PhoneNumber:   "04712344321",
				PostalCode:    "1000000",
				Prefecture:    "東京都",
				City:          "浦安市",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜ホテル",
			})
			if err != nil {
				t.Fatalf("Error occured: %s", err)
			}
			if got.Prefecture() != "東京都" {
				t.Errorf("都道府県が保持されていません: got %q, want %q", got.Prefecture(), "東京都")
			}
		})

		t.Run("空だと、登録できない", func(t *testing.T) {
			_, err := New(AccommodationCreateInput{
				Name:          "舞浜ホテル",
				PhoneNumber:   "04712344321",
				PostalCode:    "1000000",
				Prefecture:    "",
				City:          "浦安市",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜ホテル",
			})
			if err == nil {
				t.Fatal("エラーが返りませんでした")
			}
			if err.Error() != ErrPrefectureRequired.Error() {
				t.Errorf("エラーメッセージが違います: got %q", err.Error())
			}
		})
	})

	t.Run("市区町村", func(t *testing.T) {
		t.Run("入力されていれば、登録できる", func(t *testing.T) {
			got, err := New(AccommodationCreateInput{
				Name:          "舞浜ホテル",
				PhoneNumber:   "04712344321",
				PostalCode:    "1000000",
				Prefecture:    "千葉県",
				City:          "千代田区",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜ホテル",
			})
			if err != nil {
				t.Fatalf("Error occured: %s", err)
			}
			if got.City() != "千代田区" {
				t.Errorf("市区町村が保持されていません: got %q, want %q", got.City(), "千代田区")
			}
		})

		t.Run("空だと、登録できない", func(t *testing.T) {
			_, err := New(AccommodationCreateInput{
				Name:          "舞浜ホテル",
				PhoneNumber:   "04712344321",
				PostalCode:    "1000000",
				Prefecture:    "千葉県",
				City:          "",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜ホテル",
			})
			if err == nil {
				t.Fatal("エラーが返りませんでした")
			}
			if err.Error() != ErrCityRequired.Error() {
				t.Errorf("エラーメッセージが違います: got %q", err.Error())
			}
		})
	})
}

func TestUpdateAccommodation(t *testing.T) {
	t.Run("宿情報は編集できる", func(t *testing.T) {
	})
}

func TestSaveAccomodation(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なテストをスキップ")
	}

	pool := testutil.SetupDB(t)
	ctx := context.Background()

	t.Run("宿を登録し、保存ができる", func(t *testing.T) {
		accommodation, err := New(AccommodationCreateInput{
			Name:          "舞浜ホテル",
			PhoneNumber:   "04700000000",
			PostalCode:    "1000000",
			Prefecture:    "千葉県",
			City:          "浦安市",
			StreetAddress: "舞浜1-1",
			Building:      "舞浜ホテル",
		})

		require.NoError(t, err)

		require.NoError(t, accommodation.Save(ctx, pool))

		got, err := FindById(ctx, pool, accommodation.ID())

		require.NoError(t, err)
		require.Equal(t, accommodation.Name(), got.Name())
	})
}
