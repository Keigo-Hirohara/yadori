package booker

import (
	"errors"
	"net/http"

	bookerdb "github.com/Keigo-Hirohara/yadori/internal/booker/db"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

type Handler struct {
	db bookerdb.DBTX
}

func NewHandler(db bookerdb.DBTX) *Handler {
	return &Handler{db: db}
}

var errorTable = []struct {
	target error
	code   string
	status int
}{
	{ErrBookerNotFound, "BOOKER_NOT_FOUND", http.StatusNotFound},
	{ErrInvalidFirstName, "INVALID_FIRST_NAME", http.StatusBadRequest},
	{ErrInvalidLastName, "INVALID_LAST_NAME", http.StatusBadRequest},
	{ErrInvalidPhoneNumber, "INVALID_PHONE_NUMBER", http.StatusBadRequest},
	{ErrInvalidPostalCode, "INVALID_POSTAL_CODE", http.StatusBadRequest},
	{ErrPrefectureRequired, "PREFECTURE_REQUIRED", http.StatusBadRequest},
	{ErrCityRequired, "CITY_REQUIRED", http.StatusBadRequest},
}

func writeError(w http.ResponseWriter, err error) {
	for _, e := range errorTable {
		if errors.Is(err, e.target) {
			sharedhttp.WriteError(w, e.status, e.code, err.Error())
			return
		}
	}
	sharedhttp.WriteInternal(w, err)
}

type bookerRequest struct {
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	PhoneNumber   string `json:"phoneNumber"`
	PostalCode    string `json:"postalCode"`
	Prefecture    string `json:"prefecture"`
	City          string `json:"city"`
	StreetAddress string `json:"streetAddress"`
	Building      string `json:"building"`
}

type bookerResponse struct {
	BookerId    string `json:"bookerId"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	PhoneNumber string `json:"phoneNumber"`
}

func toResponse(b *Booker) bookerResponse {
	return bookerResponse{
		BookerId:    b.ID().String(),
		FirstName:   b.FirstName(),
		LastName:    b.LastName(),
		PhoneNumber: b.PhoneNumber(),
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req bookerRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	b, err := NewBooker(BookerCreateInput{
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PhoneNumber:   req.PhoneNumber,
		PostalCode:    req.PostalCode,
		Prefecture:    req.Prefecture,
		City:          req.City,
		StreetAddress: req.StreetAddress,
		Building:      req.Building,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if err := b.Save(r.Context(), h.db); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, toResponse(&b))
}

func (h *Handler) Find(w http.ResponseWriter, r *http.Request) {
	id, err := sharedhttp.PathUUID(r, "bookerId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	b, err := FindById(r.Context(), h.db, id)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toResponse(b))
}
