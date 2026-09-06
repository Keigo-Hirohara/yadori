package domain

import (
	"time"

	"github.com/google/uuid"
)

type HoldStatus int

const (
	TemporaryHold HoldStatus = iota
	Confirmed
	ProcessingPayment
)

type Hold struct {
	id        uuid.UUID
	bookingId uuid.UUID
	slotNo    int
	status    HoldStatus
	expiredAt *time.Time
}

func (h *Hold) Id() uuid.UUID {
	return h.id
}
func (h *Hold) BookingId() uuid.UUID {
	return h.bookingId
}
func (h *Hold) SlotNo() int {
	return h.slotNo
}
func (h *Hold) Status() HoldStatus {
	return h.status
}
func (h *Hold) ExpiredAt() *time.Time {
	return h.expiredAt
}
