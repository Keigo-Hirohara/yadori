package booker

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	bookerdb "github.com/Keigo-Hirohara/yadori/internal/booker/db"
	"github.com/google/uuid"
)

var (
	phoneNumberPattern = regexp.MustCompile(`^0\d{9,10}$`)
	postalCodePattern  = regexp.MustCompile(`^\d{7}$`)
)

var (
	ErrInvalidFirstName   = errors.New("名は1文字以上60文字以内にしてください")
	ErrInvalidLastName    = errors.New("姓は1文字以上60文字以内にしてください")
	ErrInvalidPhoneNumber = errors.New("電話番号は0から始まる10桁または11桁の数字にしてください")
	ErrInvalidPostalCode  = errors.New("郵便番号は7桁の数字にしてください")
	ErrPrefectureRequired = errors.New("都道府県を入力してください")
	ErrCityRequired       = errors.New("市区町村を入力してください")
	ErrFailedToSave       = errors.New("会員の登録に失敗しました")
	ErrBookerNotFound     = errors.New("会員が見つかりませんでした")
)

type Booker struct {
	id            uuid.UUID
	firstName     string
	lastName      string
	postalCode    string
	phoneNumber   string
	prefecture    string
	city          string
	streetAddress string
	building      string
}

type BookerCreateInput struct {
	FirstName     string
	LastName      string
	PostalCode    string
	PhoneNumber   string
	Prefecture    string
	City          string
	StreetAddress string
	Building      string
}

func NewBooker(input BookerCreateInput) (Booker, error) {
	if err := validFirstName(input.FirstName); err != nil {
		return Booker{}, err
	}

	if err := validLastName(input.LastName); err != nil {
		return Booker{}, err
	}

	phoneNumber, err := validPhoneNumber(input.PhoneNumber)
	if err != nil {
		return Booker{}, err
	}

	postalCode, err := validPostalCode(input.PostalCode)
	if err != nil {
		return Booker{}, err
	}

	if err := validPrefecture(input.Prefecture); err != nil {
		return Booker{}, err
	}

	if err := validCity(input.City); err != nil {
		return Booker{}, err
	}

	return Booker{
		id:            uuid.New(),
		firstName:     input.FirstName,
		lastName:      input.LastName,
		postalCode:    postalCode,
		phoneNumber:   phoneNumber,
		prefecture:    input.Prefecture,
		city:          input.City,
		streetAddress: input.StreetAddress,
		building:      input.Building,
	}, nil
}

func (b *Booker) Save(ctx context.Context, db bookerdb.DBTX) error {
	q := bookerdb.New(db)

	err := q.UpsertBooker(ctx, bookerdb.UpsertBookerParams{
		ID:            b.id,
		FirstName:     b.firstName,
		LastName:      b.lastName,
		PostalCode:    b.postalCode,
		PhoneNumber:   b.phoneNumber,
		Prefecture:    b.prefecture,
		City:          b.city,
		StreetAddress: b.streetAddress,
		Building:      b.building,
	})

	if err != nil {
		return ErrFailedToSave
	}

	return nil
}

func FindById(ctx context.Context, db bookerdb.DBTX, id uuid.UUID) (*Booker, error) {
	q := bookerdb.New(db)

	bookerFromDB, err := q.GetBooker(ctx, id)

	if err != nil {
		return nil, ErrBookerNotFound
	}
	result := reconstruct(bookerFromDB)
	return &result, nil
}

func (b *Booker) ID() uuid.UUID {
	return b.id
}

func (b *Booker) FirstName() string {
	return b.firstName
}

func (b *Booker) LastName() string {
	return b.lastName
}

func (b *Booker) PostalCode() string {
	return b.postalCode
}

func (b *Booker) PhoneNumber() string {
	return b.phoneNumber
}

func (b *Booker) Prefecture() string {
	return b.prefecture
}

func (b *Booker) City() string {
	return b.city
}

func validFirstName(firstName string) error {
	length := utf8.RuneCountInString(firstName)
	if length < 1 || length > 60 {
		return ErrInvalidFirstName
	}
	return nil
}

func validLastName(lastName string) error {
	length := utf8.RuneCountInString(lastName)
	if length < 1 || length > 60 {
		return ErrInvalidLastName
	}
	return nil
}

func validPhoneNumber(phoneNumber string) (string, error) {
	phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
	if !phoneNumberPattern.MatchString(phoneNumber) {
		return "", ErrInvalidPhoneNumber
	}
	return phoneNumber, nil
}

func validPostalCode(postalCode string) (string, error) {
	postalCode = strings.ReplaceAll(postalCode, "-", "")
	if !postalCodePattern.MatchString(postalCode) {
		return "", ErrInvalidPostalCode
	}
	return postalCode, nil
}

func validPrefecture(prefecture string) error {
	if prefecture == "" {
		return ErrPrefectureRequired
	}
	return nil
}

func validCity(city string) error {
	if city == "" {
		return ErrCityRequired
	}
	return nil
}

func reconstruct(row bookerdb.Booker) Booker {
	return Booker{
		id:            row.ID,
		firstName:     row.FirstName,
		lastName:      row.LastName,
		postalCode:    row.PostalCode,
		phoneNumber:   row.PhoneNumber,
		prefecture:    row.Prefecture,
		city:          row.City,
		streetAddress: row.StreetAddress,
		building:      row.Building,
	}
}
