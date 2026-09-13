package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidStayPeriod = errors.New("チェックアウト日はチェックイン日より後でなければなりません")
)

type StayPeriod struct {
	checkinDate  time.Time
	checkoutDate time.Time
}

func NewStayPeriod(checkin, checkout time.Time) (StayPeriod, error) {
	in := truncateToDate(checkin)
	out := truncateToDate(checkout)
	if !out.After(in) {
		return StayPeriod{}, ErrInvalidStayPeriod
	}
	return StayPeriod{checkinDate: in, checkoutDate: out}, nil
}

func (p StayPeriod) CheckinDate() time.Time {
	return p.checkinDate
}

func (p StayPeriod) CheckoutDate() time.Time {
	return p.checkoutDate
}

func (p StayPeriod) Nights() int {
	return daysBetween(p.checkinDate, p.checkoutDate)
}

func (p StayPeriod) Dates() []time.Time {
	dates := make([]time.Time, 0, p.Nights())
	for d := p.checkinDate; d.Before(p.checkoutDate); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates
}

func (p StayPeriod) Overlaps(other StayPeriod) bool {
	return p.checkinDate.Before(other.checkoutDate) && other.checkinDate.Before(p.checkoutDate)
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func daysBetween(from, to time.Time) int {
	f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(t.Sub(f).Hours() / 24)
}
