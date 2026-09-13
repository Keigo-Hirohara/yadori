package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNoGuest           = errors.New("宿泊者は1名以上必要です")
	ErrOverCapacity      = errors.New("宿泊人数が定員を超えています")
	ErrPastStayPeriod    = errors.New("過去の日程は予約できません")
	ErrInvalidTransition = errors.New("この状態からは遷移できません")
	ErrBookingNotFound   = errors.New("予約が見つかりません")
)

type Status int

const (
	TemporaryHold Status = iota
	ProcessingPayment
	Confirmed
	Cancelled
)

var allowedTransitions = map[Status][]Status{
	TemporaryHold:     {ProcessingPayment, Cancelled},
	ProcessingPayment: {Confirmed, Cancelled},
	Confirmed:         {Cancelled},
	Cancelled:         {},
}

type Booking struct {
	id              uuid.UUID
	bookerId        uuid.UUID
	roomTypeId      uuid.UUID
	guests          []Guest
	totalFee        TotalFee
	stayPeriod      StayPeriod
	status          Status
	cancellationFee CancellationFee
}

func Book(
	id, bookerId, roomTypeId uuid.UUID,
	period StayPeriod,
	guests []Guest,
	totalFee TotalFee,
	capacity int,
	now time.Time,
) (*Booking, error) {
	if err := validGuests(guests, capacity); err != nil {
		return nil, err
	}
	if period.CheckinDate().Before(truncateToDate(now)) {
		return nil, ErrPastStayPeriod
	}

	return &Booking{
		id:         id,
		bookerId:   bookerId,
		roomTypeId: roomTypeId,
		guests:     append([]Guest{}, guests...),
		totalFee:   totalFee,
		stayPeriod: period,
		status:     TemporaryHold,
	}, nil
}

func Reconstruct(
	id, bookerId, roomTypeId uuid.UUID,
	period StayPeriod,
	guests []Guest,
	totalFee TotalFee,
	status Status,
	cancellationFee CancellationFee,
) *Booking {
	if guests == nil {
		guests = []Guest{}
	}
	return &Booking{
		id:              id,
		bookerId:        bookerId,
		roomTypeId:      roomTypeId,
		guests:          guests,
		totalFee:        totalFee,
		stayPeriod:      period,
		status:          status,
		cancellationFee: cancellationFee,
	}
}

func (b *Booking) StartPayment() error {
	return b.transition(ProcessingPayment)
}

func (b *Booking) Confirm() error {
	return b.transition(Confirmed)
}

func (b *Booking) Cancel(reason CancellationReason, now time.Time) (CancellationFee, error) {
	if b.status == Cancelled {
		return b.cancellationFee, nil
	}
	if !canTransition(b.status, Cancelled) {
		return CancellationFee{}, ErrInvalidTransition
	}

	fee := CancellationFee{}
	if b.status == Confirmed {
		fee = CalculateCancellationFee(b.stayPeriod, b.totalFee, reason, now)
	}

	b.status = Cancelled
	b.cancellationFee = fee
	return fee, nil
}

func (b *Booking) FixTotalFee(f TotalFee) error {
	if b.status != TemporaryHold {
		return ErrInvalidTransition
	}
	b.totalFee = f
	return nil
}

func (b *Booking) ChangeGuests(guests []Guest, capacity int) error {
	if !b.isChangeable() {
		return ErrInvalidTransition
	}
	if err := validGuests(guests, capacity); err != nil {
		return err
	}
	b.guests = append([]Guest{}, guests...)
	return nil
}

func (b *Booking) Id() uuid.UUID {
	return b.id
}

func (b *Booking) BookerId() uuid.UUID {
	return b.bookerId
}

func (b *Booking) RoomTypeId() uuid.UUID {
	return b.roomTypeId
}

func (b *Booking) StayPeriod() StayPeriod {
	return b.stayPeriod
}

func (b *Booking) TotalFee() TotalFee {
	return b.totalFee
}

func (b *Booking) Status() Status {
	return b.status
}

func (b *Booking) CancellationFee() CancellationFee {
	return b.cancellationFee
}

func (b *Booking) Guests() []Guest {
	return append([]Guest{}, b.guests...)
}

func (b *Booking) GuestCount() int {
	return len(b.guests)
}

func (b *Booking) transition(to Status) error {
	if !canTransition(b.status, to) {
		return ErrInvalidTransition
	}
	b.status = to
	return nil
}

func (b *Booking) isChangeable() bool {
	switch b.status {
	case TemporaryHold, ProcessingPayment, Confirmed:
		return true
	default:
		return false
	}
}

func canTransition(from, to Status) bool {
	for _, s := range allowedTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

func validGuests(guests []Guest, capacity int) error {
	if len(guests) == 0 {
		return ErrNoGuest
	}
	if len(guests) > capacity {
		return ErrOverCapacity
	}
	return nil
}
