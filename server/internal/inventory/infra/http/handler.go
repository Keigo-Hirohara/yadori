package inventoryhttp

import (
	"context"
	"net/http"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
	"github.com/google/uuid"
)

type service interface {
	Register(ctx context.Context, in app.RegisterInput) error
	ChangeQuantity(ctx context.Context, roomTypeId uuid.UUID, date time.Time, quantity int) error
	ChangeFee(ctx context.Context, roomTypeId uuid.UUID, date time.Time, feeAmount int) error
	Close(ctx context.Context, roomTypeId uuid.UUID, date time.Time) error
	Reopen(ctx context.Context, roomTypeId uuid.UUID, date time.Time) error
	ListSummaries(ctx context.Context, roomTypeId uuid.UUID, from, to time.Time) ([]app.InventorySummary, error)
}

type Handler struct {
	service service
}

func NewHandler(s service) *Handler {
	return &Handler{service: s}
}

type registerRequest struct {
	Date     string `json:"date"`
	Quantity int    `json:"quantity"`
	Fee      int    `json:"fee"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	roomTypeId, err := sharedhttp.PathUUID(r, "roomTypeId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	var req registerRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}
	date, err := time.Parse(time.DateOnly, req.Date)
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	err = h.service.Register(r.Context(), app.RegisterInput{
		RoomTypeId: roomTypeId,
		Date:       date,
		Quantity:   req.Quantity,
		FeeAmount:  req.Fee,
		Now:        time.Now(),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, nil)
}

type changeQuantityRequest struct {
	Quantity int `json:"quantity"`
}

func (h *Handler) ChangeQuantity(w http.ResponseWriter, r *http.Request) {
	roomTypeId, date, ok := h.target(w, r)
	if !ok {
		return
	}
	var req changeQuantityRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	if err := h.service.ChangeQuantity(r.Context(), roomTypeId, date, req.Quantity); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteNoContent(w)
}

type changeFeeRequest struct {
	Fee int `json:"fee"`
}

func (h *Handler) ChangeFee(w http.ResponseWriter, r *http.Request) {
	roomTypeId, date, ok := h.target(w, r)
	if !ok {
		return
	}
	var req changeFeeRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	if err := h.service.ChangeFee(r.Context(), roomTypeId, date, req.Fee); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteNoContent(w)
}

func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	roomTypeId, date, ok := h.target(w, r)
	if !ok {
		return
	}
	if err := h.service.Close(r.Context(), roomTypeId, date); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteNoContent(w)
}

func (h *Handler) Reopen(w http.ResponseWriter, r *http.Request) {
	roomTypeId, date, ok := h.target(w, r)
	if !ok {
		return
	}
	if err := h.service.Reopen(r.Context(), roomTypeId, date); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteNoContent(w)
}

type inventoryResponse struct {
	Date              string `json:"date"`
	Fee               int    `json:"fee"`
	QuantityAvailable int    `json:"quantityAvailable"`
	HeldCount         int    `json:"heldCount"`
	Available         int    `json:"available"`
	IsClosed          bool   `json:"isClosed"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	roomTypeId, err := sharedhttp.PathUUID(r, "roomTypeId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	from, err := time.Parse(time.DateOnly, r.URL.Query().Get("from"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	to, err := time.Parse(time.DateOnly, r.URL.Query().Get("to"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	summaries, err := h.service.ListSummaries(r.Context(), roomTypeId, from, to)
	if err != nil {
		writeError(w, err)
		return
	}

	body := make([]inventoryResponse, 0, len(summaries))
	for _, s := range summaries {
		body = append(body, inventoryResponse{
			Date:              s.Date.Format(time.DateOnly),
			Fee:               s.Fee,
			QuantityAvailable: s.QuantityAvailable,
			HeldCount:         s.HeldCount,
			Available:         s.QuantityAvailable - s.HeldCount,
			IsClosed:          s.IsClosed,
		})
	}
	sharedhttp.WriteJSON(w, http.StatusOK, body)
}

func (h *Handler) target(w http.ResponseWriter, r *http.Request) (uuid.UUID, time.Time, bool) {
	roomTypeId, err := sharedhttp.PathUUID(r, "roomTypeId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return uuid.Nil, time.Time{}, false
	}
	date, err := sharedhttp.PathDate(r, "date")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return uuid.Nil, time.Time{}, false
	}
	return roomTypeId, date, true
}
