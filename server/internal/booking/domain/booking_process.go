package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	PaymentWindow = 30 * time.Minute
	StallAfter    = 1 * time.Minute
)

var ErrPaymentDeadlinePassed = errors.New("支払いの期限を過ぎています")

type ProcessState int

const (
	Holding ProcessState = iota
	AwaitingPayment
	PreparingPayment
	PaymentRequested
	Confirming
	Releasing
	Completed
	Released
)

var processStateNames = map[ProcessState]string{
	Holding:          "確保中",
	AwaitingPayment:  "支払い待ち",
	PreparingPayment: "決済準備中",
	PaymentRequested: "決済依頼済み",
	Confirming:       "確定中",
	Releasing:        "解放中",
	Completed:        "完了",
	Released:         "取り消し済み",
}

func (s ProcessState) String() string {
	return processStateNames[s]
}

type ResumeAction int

const (
	NothingToDo ResumeAction = iota
	ReleaseAsExpired
	InquirePayment
	ContinueConfirming
	ContinueReleasing
)

type BookingProcess struct {
	bookingId     uuid.UUID
	state         ProcessState
	paymentId     *uuid.UUID
	releaseReason *CancellationReason
	startedAt     time.Time
	updatedAt     time.Time
}

func StartBookingProcess(bookingId uuid.UUID, now time.Time) *BookingProcess {
	return &BookingProcess{
		bookingId: bookingId,
		state:     Holding,
		startedAt: now,
		updatedAt: now,
	}
}

func (p *BookingProcess) FinishHolding(now time.Time) error {
	return p.move(Holding, AwaitingPayment, now)
}

func (p *BookingProcess) PreparePayment(now time.Time) error {
	if p.state != AwaitingPayment {
		return ErrInvalidTransition
	}
	if !now.Before(p.paymentDeadline()) {
		return ErrPaymentDeadlinePassed
	}
	return p.move(AwaitingPayment, PreparingPayment, now)
}

func (p *BookingProcess) RequestPayment(paymentId uuid.UUID, now time.Time) error {
	if err := p.move(PreparingPayment, PaymentRequested, now); err != nil {
		return err
	}
	p.paymentId = &paymentId
	return nil
}

func (p *BookingProcess) PaymentSucceeded(now time.Time) error {
	return p.move(PaymentRequested, Confirming, now)
}

func (p *BookingProcess) PaymentFailed(now time.Time) error {
	if err := p.move(PaymentRequested, Releasing, now); err != nil {
		return err
	}
	reason := PaymentFailed
	p.releaseReason = &reason
	return nil
}

func (p *BookingProcess) FinishConfirming(now time.Time) error {
	return p.move(Confirming, Completed, now)
}

func (p *BookingProcess) Release(reason CancellationReason, now time.Time) error {
	switch p.state {
	case Holding, AwaitingPayment, PreparingPayment:
	default:
		return ErrInvalidTransition
	}
	p.state = Releasing
	p.releaseReason = &reason
	p.updatedAt = now
	return nil
}

func (p *BookingProcess) FinishReleasing(now time.Time) error {
	return p.move(Releasing, Released, now)
}

func (p *BookingProcess) NextResumeAction(now time.Time) ResumeAction {
	if p.state == AwaitingPayment {
		if now.Before(p.paymentDeadline()) {
			return NothingToDo
		}
		return ReleaseAsExpired
	}

	if now.Before(p.updatedAt.Add(StallAfter)) {
		return NothingToDo
	}

	switch p.state {
	case Holding, PreparingPayment:
		return ReleaseAsExpired
	case PaymentRequested:
		return InquirePayment
	case Confirming:
		return ContinueConfirming
	case Releasing:
		return ContinueReleasing
	default:
		return NothingToDo
	}
}

func (p *BookingProcess) BookingId() uuid.UUID {
	return p.bookingId
}

func (p *BookingProcess) State() ProcessState {
	return p.state
}

func (p *BookingProcess) PaymentId() *uuid.UUID {
	return p.paymentId
}

func (p *BookingProcess) ReleaseReason() *CancellationReason {
	return p.releaseReason
}

func (p *BookingProcess) StartedAt() time.Time {
	return p.startedAt
}

func (p *BookingProcess) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *BookingProcess) move(from, to ProcessState, now time.Time) error {
	if p.state != from {
		return ErrInvalidTransition
	}
	p.state = to
	p.updatedAt = now
	return nil
}

func (p *BookingProcess) paymentDeadline() time.Time {
	return p.startedAt.Add(PaymentWindow)
}
