package booker

import (
	"context"
	"strings"
	"testing"

	"github.com/Keigo-Hirohara/yadori/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewBooker(t *testing.T) {
	t.Run("必須項目が揃っていれば、会員を登録できる", func(t *testing.T) {
		_, err := NewBooker(BookerCreateInput{
			FirstName:     "太郎",
			LastName:      "山田",
			PhoneNumber:   "09012345678",
			PostalCode:    "1000000",
			Prefecture:    "千葉県",
			City:          "浦安市",
			StreetAddress: "舞浜1-1",
			Building:      "舞浜マンション",
		})
		if err != nil {
			t.Errorf("Error occured: %s", err)
		}
	})

	t.Run("名", func(t *testing.T) {
		t.Run("1文字以上60文字以内なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "1文字なら登録できる", input: "太"},
				{name: "60文字なら登録できる", input: strings.Repeat("あ", 60)},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewBooker(BookerCreateInput{
						FirstName:     tt.input,
						LastName:      "山田",
						PhoneNumber:   "09012345678",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.FirstName() != tt.input {
						t.Errorf("名が保持されていません: got %q, want %q", got.FirstName(), tt.input)
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
					_, err := NewBooker(BookerCreateInput{
						FirstName:     tt.input,
						LastName:      "山田",
						PhoneNumber:   "09012345678",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidFirstName.Error() {
						t.Errorf("エラーメッセージが違います: got %q", err.Error())
					}
				})
			}
		})
	})

	t.Run("姓", func(t *testing.T) {
		t.Run("1文字以上60文字以内なら、登録できる", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "1文字なら登録できる", input: "山"},
				{name: "60文字なら登録できる", input: strings.Repeat("あ", 60)},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      tt.input,
						PhoneNumber:   "09012345678",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
					})
					if err != nil {
						t.Fatalf("Error occured: %s", err)
					}
					if got.LastName() != tt.input {
						t.Errorf("姓が保持されていません: got %q, want %q", got.LastName(), tt.input)
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
					_, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      tt.input,
						PhoneNumber:   "09012345678",
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
					})
					if err == nil {
						t.Fatal("エラーが返りませんでした")
					}
					if err.Error() != ErrInvalidLastName.Error() {
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
				{name: "11桁なら登録できる", input: "09012345678", want: "09012345678"},
				{name: "ハイフン入りならハイフンが除かれて登録できる", input: "090-1234-5678", want: "09012345678"},
				{name: "10桁のハイフン入りならハイフンが除かれて登録できる", input: "03-1234-5678", want: "0312345678"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      "山田",
						PhoneNumber:   tt.input,
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
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
				{name: "9桁だと登録できない", input: "090123456"},
				{name: "12桁だと登録できない", input: "090123456789"},
				{name: "数字以外が含まれると登録できない", input: "0901234567a"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      "山田",
						PhoneNumber:   tt.input,
						PostalCode:    "1000000",
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
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
				{name: "先頭が0でも0が消えない", input: "0600000", want: "0600000"},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      "山田",
						PhoneNumber:   "09012345678",
						PostalCode:    tt.input,
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
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
					_, err := NewBooker(BookerCreateInput{
						FirstName:     "太郎",
						LastName:      "山田",
						PhoneNumber:   "09012345678",
						PostalCode:    tt.input,
						Prefecture:    "千葉県",
						City:          "浦安市",
						StreetAddress: "舞浜1-1",
						Building:      "舞浜マンション",
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
			got, err := NewBooker(BookerCreateInput{
				FirstName:     "太郎",
				LastName:      "山田",
				PhoneNumber:   "09012345678",
				PostalCode:    "1000000",
				Prefecture:    "東京都",
				City:          "浦安市",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜マンション",
			})
			if err != nil {
				t.Fatalf("Error occured: %s", err)
			}
			if got.Prefecture() != "東京都" {
				t.Errorf("都道府県が保持されていません: got %q, want %q", got.Prefecture(), "東京都")
			}
		})

		t.Run("空だと、登録できない", func(t *testing.T) {
			_, err := NewBooker(BookerCreateInput{
				FirstName:     "太郎",
				LastName:      "山田",
				PhoneNumber:   "09012345678",
				PostalCode:    "1000000",
				Prefecture:    "",
				City:          "浦安市",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜マンション",
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
			got, err := NewBooker(BookerCreateInput{
				FirstName:     "太郎",
				LastName:      "山田",
				PhoneNumber:   "09012345678",
				PostalCode:    "1000000",
				Prefecture:    "千葉県",
				City:          "千代田区",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜マンション",
			})
			if err != nil {
				t.Fatalf("Error occured: %s", err)
			}
			if got.City() != "千代田区" {
				t.Errorf("市区町村が保持されていません: got %q, want %q", got.City(), "千代田区")
			}
		})

		t.Run("空だと、登録できない", func(t *testing.T) {
			_, err := NewBooker(BookerCreateInput{
				FirstName:     "太郎",
				LastName:      "山田",
				PhoneNumber:   "09012345678",
				PostalCode:    "1000000",
				Prefecture:    "千葉県",
				City:          "",
				StreetAddress: "舞浜1-1",
				Building:      "舞浜マンション",
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

func TestSaveBooker(t *testing.T) {
	if testing.Short() {
		t.Skip("DBが必要なテストをスキップ")
	}

	pool := testutil.SetupDB(t)
	ctx := context.Background()

	t.Run("会員を登録し、保存ができる", func(t *testing.T) {
		booker, err := NewBooker(BookerCreateInput{
			FirstName:     "太郎",
			LastName:      "山田",
			PhoneNumber:   "09012345678",
			PostalCode:    "1000000",
			Prefecture:    "千葉県",
			City:          "浦安市",
			StreetAddress: "舞浜1-1",
			Building:      "舞浜マンション",
		})

		require.NoError(t, err)

		require.NoError(t, booker.Save(ctx, pool))

		got, err := FindById(ctx, pool, booker.ID())

		require.NoError(t, err)
		require.Equal(t, booker.FirstName(), got.FirstName())
		require.Equal(t, booker.LastName(), got.LastName())
	})

	t.Run("存在しない会員は取得できない", func(t *testing.T) {
		_, err := FindById(ctx, pool, uuid.New())

		if err == nil {
			t.Fatal("エラーが返りませんでした")
		}
		if err.Error() != ErrBookerNotFound.Error() {
			t.Errorf("エラーメッセージが違います: got %q", err.Error())
		}
	})
}
