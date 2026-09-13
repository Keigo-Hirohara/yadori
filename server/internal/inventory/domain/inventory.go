package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidQuantity    = errors.New("販売可能数に負の値は設定できません")
	ErrQuantityBelowHolds = errors.New("確保済みの枠数を下回る販売可能数には変更できません")
	ErrSoldOut            = errors.New("空きがないため確保できません")
	ErrClosed             = errors.New("販売停止中のため確保できません")
	ErrAlreadyClosed      = errors.New("すでに販売を停止しています")
	ErrNotClosed          = errors.New("販売を停止していません")
	ErrDuplicatedHold     = errors.New("すでに確保しています")
	ErrInvalidExpiration  = errors.New("確保の期限は現在より後でなければなりません")
	ErrHoldNotFound       = errors.New("確保が見つかりません")
	ErrInvalidTransition  = errors.New("この状態からは遷移できません")
	ErrInventoryNotFound  = errors.New("在庫が見つかりません")
	ErrAlreadyRegistered  = errors.New("この日の販売枠はすでに登録されています")
)

type Inventory struct {
	id                *InventoryId
	holds             []Hold
	fee               *Fee
	quantityAvailable int
	isClosed          bool
}

type HoldInput struct {
	HoldId     uuid.UUID
	BookingId  uuid.UUID
	RoomTypeId uuid.UUID
	ExpiredAt  time.Time
	Date       time.Time
}

func Register(id *InventoryId, quantity int, fee *Fee) (*Inventory, error) {
	if err := validQuantity(quantity); err != nil {
		return nil, err
	}
	return &Inventory{
		id:                id,
		holds:             []Hold{},
		fee:               fee,
		quantityAvailable: quantity,
		isClosed:          false,
	}, nil
}

func Reconstruct(id *InventoryId, quantityAvailable int, fee *Fee, isClosed bool, holds []Hold) *Inventory {
	if holds == nil {
		holds = []Hold{}
	}
	return &Inventory{
		id:                id,
		holds:             holds,
		fee:               fee,
		quantityAvailable: quantityAvailable,
		isClosed:          isClosed,
	}
}

func (inv *Inventory) ChangeQuantity(n int) error {
	if err := validQuantity(n); err != nil {
		return err
	}

	if n < inv.maxSlotNo() {
		return ErrQuantityBelowHolds
	}
	inv.quantityAvailable = n
	return nil
}

func (inv *Inventory) ChangeFee(f *Fee) error {
	inv.fee = f
	return nil
}

func (inv *Inventory) Close() error {
	if inv.isClosed {
		return ErrAlreadyClosed
	}
	inv.isClosed = true
	return nil
}

func (inv *Inventory) Reopen() error {
	if !inv.isClosed {
		return ErrNotClosed
	}
	inv.isClosed = false
	return nil
}

func (inv *Inventory) Hold(input HoldInput) (int, error) {
	if inv.isClosed {
		return 0, ErrClosed
	}

	if !input.ExpiredAt.After(input.Date) {
		return 0, ErrInvalidExpiration
	}
	if _, ok := inv.FindHold(input.BookingId); ok {
		return 0, ErrDuplicatedHold
	}

	slotNo, ok := inv.vacantSlotNo()
	if !ok {
		return 0, ErrSoldOut
	}

	expiredAt := input.ExpiredAt
	inv.holds = append(inv.holds, Hold{
		id:        input.HoldId,
		bookingId: input.BookingId,
		slotNo:    slotNo,
		status:    TemporaryHold,
		expiredAt: &expiredAt,
	})
	return slotNo, nil
}

func (inv *Inventory) StartPayment(bookingId uuid.UUID) error {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return ErrHoldNotFound
	}
	if inv.holds[i].status != TemporaryHold {
		return ErrInvalidTransition
	}
	inv.holds[i].status = ProcessingPayment
	inv.holds[i].expiredAt = nil
	return nil
}

func (inv *Inventory) Confirm(bookingId uuid.UUID) error {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return ErrHoldNotFound
	}
	if inv.holds[i].status != ProcessingPayment {
		return ErrInvalidTransition
	}
	inv.holds[i].status = Confirmed
	inv.holds[i].expiredAt = nil
	return nil
}

func (inv *Inventory) Release(bookingId uuid.UUID) error {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return ErrHoldNotFound
	}
	inv.holds = append(inv.holds[:i], inv.holds[i+1:]...)
	return nil
}

func (inv *Inventory) CollectExpired(now time.Time) int {
	remained := make([]Hold, 0, len(inv.holds))
	collected := 0
	for _, h := range inv.holds {
		if h.status == TemporaryHold && h.expiredAt != nil && h.expiredAt.Before(now) {
			collected++
			continue
		}
		remained = append(remained, h)
	}
	inv.holds = remained
	return collected
}

func (inv *Inventory) Id() *InventoryId {
	return inv.id
}

func (inv *Inventory) Fee() *Fee {
	return inv.fee
}

func (inv *Inventory) QuantityAvailable() int {
	return inv.quantityAvailable
}

func (inv *Inventory) IsClosed() bool {
	return inv.isClosed
}

func (inv *Inventory) HoldCount() int {
	return len(inv.holds)
}

func (inv *Inventory) Available() int {
	return inv.quantityAvailable - len(inv.holds)
}

func (inv *Inventory) Holds() []Hold {
	return append([]Hold{}, inv.holds...)
}

func (inv *Inventory) FindHold(bookingId uuid.UUID) (*Hold, bool) {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return nil, false
	}
	return &inv.holds[i], true
}

func (inv *Inventory) indexOfHold(bookingId uuid.UUID) (int, bool) {
	for i := range inv.holds {
		if inv.holds[i].bookingId == bookingId {
			return i, true
		}
	}
	return 0, false
}

func (inv *Inventory) vacantSlotNo() (int, bool) {
	used := make(map[int]struct{}, len(inv.holds))
	for _, h := range inv.holds {
		used[h.slotNo] = struct{}{}
	}
	for slotNo := 1; slotNo <= inv.quantityAvailable; slotNo++ {
		if _, ok := used[slotNo]; !ok {
			return slotNo, true
		}
	}
	return 0, false
}

func (inv *Inventory) maxSlotNo() int {
	max := 0
	for _, h := range inv.holds {
		if h.slotNo > max {
			max = h.slotNo
		}
	}
	return max
}

func validQuantity(quantity int) error {
	if quantity < 0 {
		return ErrInvalidQuantity
	}
	return nil
}
