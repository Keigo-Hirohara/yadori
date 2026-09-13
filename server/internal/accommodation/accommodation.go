package accommodation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	"github.com/google/uuid"
)

var (
	phoneNumberPattern = regexp.MustCompile(`^0\d{9,10}$`)
	postalCodePattern  = regexp.MustCompile(`^\d{7}$`)
)

var (
	ErrInvalidName           = errors.New("名前は1文字以上60文字以内にしてください")
	ErrInvalidPhoneNumber    = errors.New("電話番号は0から始まる10桁または11桁の数字にしてください")
	ErrInvalidPostalCode     = errors.New("郵便番号は7桁の数字にしてください")
	ErrPrefectureRequired    = errors.New("都道府県を入力してください")
	ErrCityRequired          = errors.New("市区町村を入力してください")
	ErrFailedToSave          = errors.New("宿の登録に失敗しました")
	ErrAccommodationNotFound = errors.New("宿が見つかりませんでした")
)

type Accommodation struct {
	id            uuid.UUID
	name          string
	phoneNumber   string
	postalCode    string
	prefecture    string
	city          string
	streetAddress string
	building      string
}

type AccommodationCreateInput struct {
	Name          string
	PhoneNumber   string
	PostalCode    string
	Prefecture    string
	City          string
	StreetAddress string
	Building      string
}

func NewAccommodation(input AccommodationCreateInput) (Accommodation, error) {
	if err := validName(input.Name); err != nil {
		return Accommodation{}, err
	}

	phoneNumber, err := validPhoneNumber(input.PhoneNumber)
	if err != nil {
		return Accommodation{}, err
	}

	input.PostalCode, err = validPostalCode(input.PostalCode)
	if err != nil {
		return Accommodation{}, err
	}

	if err := validPrefecture(input.Prefecture); err != nil {
		return Accommodation{}, err
	}

	if err := validCity(input.City); err != nil {
		return Accommodation{}, err
	}

	return Accommodation{
		id:            uuid.New(),
		name:          input.Name,
		phoneNumber:   phoneNumber,
		postalCode:    input.PostalCode,
		prefecture:    input.Prefecture,
		city:          input.City,
		streetAddress: input.StreetAddress,
		building:      input.Building,
	}, nil
}

func (a *Accommodation) Save(ctx context.Context, db accommodationdb.DBTX) error {
	q := accommodationdb.New(db)

	err := q.UpsertAccommodation(ctx, accommodationdb.UpsertAccommodationParams{
		ID:            a.id,
		Name:          a.name,
		PhoneNumber:   a.phoneNumber,
		PostalCode:    a.postalCode,
		Prefecture:    a.prefecture,
		City:          a.city,
		StreetAddress: a.streetAddress,
		Building:      a.building,
	})

	if err != nil {
		return ErrFailedToSave
	}

	return nil
}

func FindAccommodationById(ctx context.Context, db accommodationdb.DBTX, id uuid.UUID) (*Accommodation, error) {
	q := accommodationdb.New(db)

	accommodationFromDB, err := q.GetAccommodation(ctx, id)

	if err != nil {
		return nil, ErrAccommodationNotFound
	}
	result := reconstruct(accommodationFromDB)
	return &result, nil
}

func (a *Accommodation) ID() uuid.UUID {
	return a.id
}

func (a *Accommodation) Name() string {
	return a.name
}

func (a *Accommodation) PhoneNumber() string {
	return a.phoneNumber
}

func (a *Accommodation) PostalCode() string {
	return a.postalCode
}

func (a *Accommodation) Prefecture() string {
	return a.prefecture
}

func ListAccommodations(ctx context.Context, db accommodationdb.DBTX) ([]Accommodation, error) {
	q := accommodationdb.New(db)

	rows, err := q.ListAccommodations(ctx)
	if err != nil {
		return nil, ErrAccommodationNotFound
	}

	accommodations := make([]Accommodation, 0, len(rows))
	for _, row := range rows {
		accommodations = append(accommodations, reconstruct(row))
	}
	return accommodations, nil
}

func (a *Accommodation) StreetAddress() string {
	return a.streetAddress
}

func (a *Accommodation) Building() string {
	return a.building
}

func (a *Accommodation) City() string {
	return a.city
}

func validName(name string) error {
	nameLength := utf8.RuneCountInString(name)
	if nameLength < 1 || nameLength > 60 {
		return ErrInvalidName
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

func reconstruct(row accommodationdb.Accommodation) Accommodation {
	return Accommodation{
		id:            row.ID,
		name:          row.Name,
		phoneNumber:   row.PhoneNumber,
		postalCode:    row.PostalCode,
		prefecture:    row.Prefecture,
		city:          row.City,
		streetAddress: row.StreetAddress,
		building:      row.Building,
	}
}
