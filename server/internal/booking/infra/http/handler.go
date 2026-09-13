package bookinghttp

import (
	"context"
	"net/http"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/booking/app"
	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
	"github.com/google/uuid"
)

type service interface {
	Book(ctx context.Context, in app.BookInput, now time.Time) (*domain.Booking, error)
	Confirm(ctx context.Context, bookingId uuid.UUID) error
	Cancel(ctx context.Context, bookingId uuid.UUID, reason domain.CancellationReason, now time.Time) (domain.CancellationFee, error)
	ChangeGuests(ctx context.Context, bookingId uuid.UUID, in []app.GuestInput) error
	Find(ctx context.Context, bookingId uuid.UUID) (*domain.Booking, error)
	ListByBooker(ctx context.Context, bookerId uuid.UUID) ([]app.BookingSummary, error)
}

type Handler struct {
	service service
}

func NewHandler(s service) *Handler {
	return &Handler{service: s}
}

type guestBody struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type bookingResponse struct {
	BookingId    string      `json:"bookingId"`
	BookerId     string      `json:"bookerId"`
	RoomTypeId   string      `json:"roomTypeId"`
	CheckinDate  string      `json:"checkinDate"`
	CheckoutDate string      `json:"checkoutDate"`
	Nights       int         `json:"nights"`
	TotalFee     int         `json:"totalFee"`
	Status       string      `json:"status"`
	Guests       []guestBody `json:"guests"`
}

func toResponse(b *domain.Booking) bookingResponse {
	guests := make([]guestBody, 0, b.GuestCount())
	for _, g := range b.Guests() {
		guests = append(guests, guestBody{
			FirstName: g.Name().FirstName(),
			LastName:  g.Name().LastName(),
		})
	}
	return bookingResponse{
		BookingId:    b.Id().String(),
		BookerId:     b.BookerId().String(),
		RoomTypeId:   b.RoomTypeId().String(),
		CheckinDate:  b.StayPeriod().CheckinDate().Format(time.DateOnly),
		CheckoutDate: b.StayPeriod().CheckoutDate().Format(time.DateOnly),
		Nights:       b.StayPeriod().Nights(),
		TotalFee:     b.TotalFee().Amount(),
		Status:       statusText(b.Status()),
		Guests:       guests,
	}
}

func statusText(s domain.Status) string {
	switch s {
	case domain.TemporaryHold:
		return "temporary_hold"
	case domain.ProcessingPayment:
		return "processing_payment"
	case domain.Confirmed:
		return "confirmed"
	case domain.Cancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

type bookRequest struct {
	BookerId     string      `json:"bookerId"`
	RoomTypeId   string      `json:"roomTypeId"`
	CheckinDate  string      `json:"checkinDate"`
	CheckoutDate string      `json:"checkoutDate"`
	Guests       []guestBody `json:"guests"`
}

func (h *Handler) Book(w http.ResponseWriter, r *http.Request) {
	var req bookRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	bookerId, err := uuid.Parse(req.BookerId)
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	roomTypeId, err := uuid.Parse(req.RoomTypeId)
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	checkin, err := time.Parse(time.DateOnly, req.CheckinDate)
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	checkout, err := time.Parse(time.DateOnly, req.CheckoutDate)
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	guests := make([]app.GuestInput, 0, len(req.Guests))
	for _, g := range req.Guests {
		guests = append(guests, app.GuestInput{FirstName: g.FirstName, LastName: g.LastName})
	}

	booking, err := h.service.Book(r.Context(), app.BookInput{
		BookerId:     bookerId,
		RoomTypeId:   roomTypeId,
		CheckinDate:  checkin,
		CheckoutDate: checkout,
		Guests:       guests,
	}, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, toResponse(booking))
}

func (h *Handler) Find(w http.ResponseWriter, r *http.Request) {
	bookingId, err := sharedhttp.PathUUID(r, "bookingId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	booking, err := h.service.Find(r.Context(), bookingId)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toResponse(booking))
}

type bookingSummaryResponse struct {
	BookingId       string `json:"bookingId"`
	RoomTypeId      string `json:"roomTypeId"`
	CheckinDate     string `json:"checkinDate"`
	CheckoutDate    string `json:"checkoutDate"`
	TotalFee        int    `json:"totalFee"`
	CancellationFee int    `json:"cancellationFee"`
	Status          string `json:"status"`
}

func (h *Handler) ListByBooker(w http.ResponseWriter, r *http.Request) {
	bookerId, err := sharedhttp.PathUUID(r, "bookerId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	summaries, err := h.service.ListByBooker(r.Context(), bookerId)
	if err != nil {
		writeError(w, err)
		return
	}

	body := make([]bookingSummaryResponse, 0, len(summaries))
	for _, s := range summaries {
		body = append(body, bookingSummaryResponse{
			BookingId:       s.BookingId.String(),
			RoomTypeId:      s.RoomTypeId.String(),
			CheckinDate:     s.CheckinDate.Format(time.DateOnly),
			CheckoutDate:    s.CheckoutDate.Format(time.DateOnly),
			TotalFee:        s.TotalFee,
			CancellationFee: s.CancellationFee,
			Status:          statusText(s.Status),
		})
	}
	sharedhttp.WriteJSON(w, http.StatusOK, body)
}

type paymentRequest struct {
	Result string `json:"result"`
}

func (h *Handler) Payment(w http.ResponseWriter, r *http.Request) {
	bookingId, err := sharedhttp.PathUUID(r, "bookingId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	var req paymentRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	switch req.Result {
	case "success":
		if err := h.service.Confirm(r.Context(), bookingId); err != nil {
			writeError(w, err)
			return
		}
		booking, err := h.service.Find(r.Context(), bookingId)
		if err != nil {
			writeError(w, err)
			return
		}
		sharedhttp.WriteJSON(w, http.StatusOK, toResponse(booking))

	case "failure":

		if _, err := h.service.Cancel(r.Context(), bookingId, domain.PaymentFailed, time.Now()); err != nil {
			writeError(w, err)
			return
		}
		sharedhttp.WriteError(w, http.StatusPaymentRequired, "PAYMENT_FAILED", "決済に失敗しました")

	case "timeout":

		sharedhttp.WriteError(w, http.StatusGatewayTimeout, "PAYMENT_TIMEOUT", "決済サービスから応答がありません")

	default:
		sharedhttp.WriteBadRequest(w)
	}
}

type cancelResponse struct {
	BookingId       string `json:"bookingId"`
	Status          string `json:"status"`
	CancellationFee int    `json:"cancellationFee"`
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	bookingId, err := sharedhttp.PathUUID(r, "bookingId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	fee, err := h.service.Cancel(r.Context(), bookingId, domain.ByGuest, time.Now())
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, cancelResponse{
		BookingId:       bookingId.String(),
		Status:          statusText(domain.Cancelled),
		CancellationFee: fee.Amount(),
	})
}

type changeGuestsRequest struct {
	Guests []guestBody `json:"guests"`
}

func (h *Handler) ChangeGuests(w http.ResponseWriter, r *http.Request) {
	bookingId, err := sharedhttp.PathUUID(r, "bookingId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	var req changeGuestsRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	guests := make([]app.GuestInput, 0, len(req.Guests))
	for _, g := range req.Guests {
		guests = append(guests, app.GuestInput{FirstName: g.FirstName, LastName: g.LastName})
	}

	if err := h.service.ChangeGuests(r.Context(), bookingId, guests); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteNoContent(w)
}
