package accommodation

import (
	"context"
	"errors"
	"unicode/utf8"

	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// foreignKeyViolation は PostgreSQL の外部キー制約違反のエラーコード
const foreignKeyViolation = "23503"

var (
	ErrInvalidRoomTypeName    = errors.New("部屋タイプ名は1文字以上60文字以内にしてください")
	ErrInvalidCapacity        = errors.New("定員は1名以上にしてください")
	ErrFailedToSaveRoomType   = errors.New("部屋タイプの登録に失敗しました")
	ErrRoomTypeNotFound       = errors.New("部屋タイプが見つかりませんでした")
	ErrInvalidAccommodationId = errors.New("宿に部屋タイプを連携できませんでした")
)

type RoomType struct {
	id              uuid.UUID
	accommodationId uuid.UUID
	name            string
	capacity        int
	hasPrivateBath  bool
	hasBalcony      bool
}

type RoomTypeCreateInput struct {
	AccommodationId uuid.UUID
	Name            string
	Capacity        int
	HasPrivateBath  bool
	HasBalcony      bool
}

func NewRoomType(input RoomTypeCreateInput) (*RoomType, error) {
	if err := validCapacity(input.Capacity); err != nil {
		return nil, err
	}

	if err := validRoomTypeName(input.Name); err != nil {
		return nil, err
	}

	if err := validAccommodationId(input.AccommodationId); err != nil {
		return nil, err
	}

	return &RoomType{
		id:              uuid.New(),
		accommodationId: input.AccommodationId,
		name:            input.Name,
		capacity:        input.Capacity,
		hasPrivateBath:  input.HasPrivateBath,
		hasBalcony:      input.HasBalcony,
	}, nil
}

func (r *RoomType) Save(ctx context.Context, db accommodationdb.DBTX) error {
	q := accommodationdb.New(db)

	err := q.UpsertRoomType(ctx, accommodationdb.UpsertRoomTypeParams{
		ID:              r.id,
		AccommodationID: r.accommodationId,
		Name:            r.name,
		Capacity:        int32(r.capacity),
		HasPrivateBath:  r.hasPrivateBath,
		HasBalcony:      r.hasBalcony,
	})

	if err != nil {
		// 宿が存在するかを判定できるのはDBだけなので、外部キー違反をここで業務の言葉に翻訳する
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation {
			return ErrInvalidAccommodationId
		}
		return ErrFailedToSaveRoomType
	}

	return nil
}

func FindRoomTypeById(ctx context.Context, db accommodationdb.DBTX, id uuid.UUID) (*RoomType, error) {
	q := accommodationdb.New(db)

	roomTypeFromDB, err := q.GetRoomType(ctx, id)

	if err != nil {
		return nil, ErrRoomTypeNotFound
	}
	result := reconstructRoomType(roomTypeFromDB)
	return &result, nil
}

func (r *RoomType) ID() uuid.UUID {
	return r.id
}

func (r *RoomType) AccommodationId() uuid.UUID {
	return r.accommodationId
}

func (r *RoomType) Name() string {
	return r.name
}

func (r *RoomType) Capacity() int {
	return r.capacity
}

func (r *RoomType) HasPrivateBath() bool {
	return r.hasPrivateBath
}

func (r *RoomType) HasBalcony() bool {
	return r.hasBalcony
}

func validAccommodationId(accommodationId uuid.UUID) error {
	if accommodationId == uuid.Nil {
		return ErrInvalidAccommodationId
	}
	return nil
}

func validCapacity(capacity int) error {
	if capacity < 1 {
		return ErrInvalidCapacity
	}
	return nil
}

func validRoomTypeName(name string) error {
	nameLength := utf8.RuneCountInString(name)
	if nameLength < 1 || nameLength > 60 {
		return ErrInvalidRoomTypeName
	}
	return nil
}

func reconstructRoomType(row accommodationdb.RoomType) RoomType {
	return RoomType{
		id:              row.ID,
		accommodationId: row.AccommodationID,
		name:            row.Name,
		capacity:        int(row.Capacity),
		hasPrivateBath:  row.HasPrivateBath,
		hasBalcony:      row.HasBalcony,
	}
}
