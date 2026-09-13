package postgres

import (
	"fmt"

	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	bookingdb "github.com/Keigo-Hirohara/yadori/internal/booking/infra/postgres/db"
)

func toBooking(row bookingdb.Booking, guestRows []bookingdb.Guest) (*domain.Booking, error) {
	period, err := domain.NewStayPeriod(row.CheckinDate, row.CheckoutDate)
	if err != nil {
		return nil, err
	}
	totalFee, err := domain.NewTotalFee(int(row.TotalFee))
	if err != nil {
		return nil, err
	}
	status, err := toDomainStatus(row.Status)
	if err != nil {
		return nil, err
	}

	guests := make([]domain.Guest, 0, len(guestRows))
	for _, g := range guestRows {
		name, err := domain.NewName(g.FirstName, g.LastName)
		if err != nil {
			return nil, err
		}
		guests = append(guests, domain.NewGuest(g.ID, name))
	}

	return domain.Reconstruct(
		row.ID,
		row.BookerID,
		row.RoomTypeID,
		period,
		guests,
		totalFee,
		status,
		domain.ReconstructCancellationFee(int(row.CancellationFee)),
	), nil
}

func toUpsertBookingParams(b *domain.Booking) (bookingdb.UpsertBookingParams, error) {
	status, err := toDBStatus(b.Status())
	if err != nil {
		return bookingdb.UpsertBookingParams{}, err
	}
	return bookingdb.UpsertBookingParams{
		ID:              b.Id(),
		BookerID:        b.BookerId(),
		RoomTypeID:      b.RoomTypeId(),
		TotalFee:        int32(b.TotalFee().Amount()),
		CheckinDate:     b.StayPeriod().CheckinDate(),
		CheckoutDate:    b.StayPeriod().CheckoutDate(),
		Status:          status,
		CancellationFee: int32(b.CancellationFee().Amount()),
	}, nil
}

func toDBStatus(s domain.Status) (bookingdb.BookingStatus, error) {
	switch s {
	case domain.TemporaryHold:
		return bookingdb.BookingStatusTemporaryHold, nil
	case domain.ProcessingPayment:
		return bookingdb.BookingStatusProcessingPayment, nil
	case domain.Confirmed:
		return bookingdb.BookingStatusConfirmed, nil
	case domain.Cancelled:
		return bookingdb.BookingStatusCancelled, nil
	default:
		return "", fmt.Errorf("保存できない予約の状態です: %d", s)
	}
}

func toDomainStatus(s bookingdb.BookingStatus) (domain.Status, error) {
	switch s {
	case bookingdb.BookingStatusTemporaryHold:
		return domain.TemporaryHold, nil
	case bookingdb.BookingStatusProcessingPayment:
		return domain.ProcessingPayment, nil
	case bookingdb.BookingStatusConfirmed:
		return domain.Confirmed, nil
	case bookingdb.BookingStatusCancelled:
		return domain.Cancelled, nil
	default:
		return 0, fmt.Errorf("未知の予約の状態です: %s", s)
	}
}
