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
)

type Inventory struct {
	id                *InventoryId
	holds             []Hold
	fee               *Fee
	quantityAvailable int
	isClosed          bool
}

type HoldInput struct {
	BookingId  uuid.UUID
	RoomTypeId uuid.UUID
	ExpiredAt  time.Time
	date       time.Time // 判定の基準となる現在時刻
}

// Publish は販売枠を公開する。
func Publish(id *InventoryId, quantity int, fee *Fee) (*Inventory, error) {
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

// ---- 宿側の操作 ----

// ChangeQuantity は販売可能数を変更する。確保済みの枠を割り込む変更は拒否する。
func (inv *Inventory) ChangeQuantity(n int) error {
	if err := validQuantity(n); err != nil {
		return err
	}
	// 解放によって枠番号が飛んでいる場合があるため、件数ではなく最大の枠番号で判定する
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

// Close はクローズアウトする。枠数と既存の確保はそのまま保持する。
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

// ---- 確保のライフサイクル ----

// Hold は空いている最小の枠番号を確保し、その枠番号を返す。
func (inv *Inventory) Hold(input HoldInput) (int, error) {
	if inv.isClosed {
		return 0, ErrClosed
	}
	if !input.ExpiredAt.After(input.date) {
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
		id:        uuid.New(),
		bookingId: input.BookingId,
		slotNo:    slotNo,
		status:    TemporaryHold,
		expiredAt: &expiredAt,
	})
	return slotNo, nil
}

// StartPayment は仮確保を決済中にし、期限切れ回収の対象から外す。
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

// Confirm は決済中の確保を確定させる。
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

// Release は確保を解放する。枠番号は再利用される。
func (inv *Inventory) Release(bookingId uuid.UUID) error {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return ErrHoldNotFound
	}
	inv.holds = append(inv.holds[:i], inv.holds[i+1:]...)
	return nil
}

// CollectExpired は期限切れの仮確保を回収し、回収した件数を返す。
// 決済中・確定済みは対象外とする。
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

// ---- 参照 ----

func (inv *Inventory) ID() *InventoryId {
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

// Available は確保できる残りの枠数を返す。
func (inv *Inventory) Available() int {
	return inv.quantityAvailable - len(inv.holds)
}

func (inv *Inventory) FindHold(bookingId uuid.UUID) (*Hold, bool) {
	i, ok := inv.indexOfHold(bookingId)
	if !ok {
		return nil, false
	}
	return &inv.holds[i], true
}

// ---- 内部 ----

func (inv *Inventory) indexOfHold(bookingId uuid.UUID) (int, bool) {
	for i := range inv.holds {
		if inv.holds[i].bookingId == bookingId {
			return i, true
		}
	}
	return 0, false
}

// vacantSlotNo は使われていない最小の枠番号を返す。
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
