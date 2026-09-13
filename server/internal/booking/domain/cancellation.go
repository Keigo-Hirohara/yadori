package domain

import "time"

type CancellationReason int

const (
	ByGuest CancellationReason = iota
	ByAccommodation
	PaymentFailed
	Expired
)

type CancellationFee struct {
	amount int
}

func (f CancellationFee) Amount() int {
	return f.amount
}

func ReconstructCancellationFee(amount int) CancellationFee {
	return CancellationFee{amount: amount}
}

func CalculateCancellationFee(p StayPeriod, total TotalFee, reason CancellationReason, now time.Time) CancellationFee {
	if reason != ByGuest {
		return CancellationFee{amount: 0}
	}

	daysBeforeCheckin := daysBetween(now, p.CheckinDate())
	switch {
	case daysBeforeCheckin >= 7:
		return CancellationFee{amount: 0}
	case daysBeforeCheckin >= 3:
		return CancellationFee{amount: total.Amount() * 50 / 100}
	default:
		return CancellationFee{amount: total.Amount()}
	}
}
