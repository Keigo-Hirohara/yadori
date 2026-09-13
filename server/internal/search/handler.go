package search

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	searchdb "github.com/Keigo-Hirohara/yadori/internal/search/db"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

type Handler struct {
	db searchdb.DBTX
}

func NewHandler(db searchdb.DBTX) *Handler {
	return &Handler{db: db}
}

var errorTable = []struct {
	target error
	code   string
	status int
}{
	{ErrInvalidPeriod, "INVALID_STAY_PERIOD", http.StatusBadRequest},
	{ErrInvalidGuests, "INVALID_GUESTS", http.StatusBadRequest},
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

type searchResponse struct {
	RoomTypeId        string `json:"roomTypeId"`
	RoomTypeName      string `json:"roomTypeName"`
	Capacity          int    `json:"capacity"`
	HasPrivateBath    bool   `json:"hasPrivateBath"`
	HasBalcony        bool   `json:"hasBalcony"`
	AccommodationId   string `json:"accommodationId"`
	AccommodationName string `json:"accommodationName"`
	Prefecture        string `json:"prefecture"`
	City              string `json:"city"`
	Nights            int    `json:"nights"`
	TotalFee          int    `json:"totalFee"`
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	checkin, err := time.Parse(time.DateOnly, query.Get("checkin"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	checkout, err := time.Parse(time.DateOnly, query.Get("checkout"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	guests, err := strconv.Atoi(query.Get("guests"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	maxFee, err := optionalInt(query.Get("max_fee"))
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	results, err := Search(r.Context(), h.db, Criteria{
		Prefecture:   query.Get("prefecture"),
		CheckinDate:  checkin,
		CheckoutDate: checkout,
		Guests:       guests,
		MaxFee:       maxFee,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	body := make([]searchResponse, 0, len(results))
	for _, result := range results {
		body = append(body, searchResponse{
			RoomTypeId:        result.RoomTypeId.String(),
			RoomTypeName:      result.RoomTypeName,
			Capacity:          result.Capacity,
			HasPrivateBath:    result.HasPrivateBath,
			HasBalcony:        result.HasBalcony,
			AccommodationId:   result.AccommodationId.String(),
			AccommodationName: result.AccommodationName,
			Prefecture:        result.Prefecture,
			City:              result.City,
			Nights:            result.Nights,
			TotalFee:          result.TotalFee,
		})
	}
	sharedhttp.WriteJSON(w, http.StatusOK, body)
}

func optionalInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}
