package accommodation

import (
	"errors"
	"net/http"

	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

type Handler struct {
	db accommodationdb.DBTX
}

func NewHandler(db accommodationdb.DBTX) *Handler {
	return &Handler{db: db}
}

var errorTable = []struct {
	target error
	code   string
	status int
}{
	{ErrAccommodationNotFound, "ACCOMMODATION_NOT_FOUND", http.StatusNotFound},
	{ErrRoomTypeNotFound, "ROOM_TYPE_NOT_FOUND", http.StatusNotFound},
	{ErrInvalidAccommodationId, "ACCOMMODATION_NOT_FOUND", http.StatusNotFound},
	{ErrInvalidName, "INVALID_NAME", http.StatusBadRequest},
	{ErrInvalidPhoneNumber, "INVALID_PHONE_NUMBER", http.StatusBadRequest},
	{ErrInvalidPostalCode, "INVALID_POSTAL_CODE", http.StatusBadRequest},
	{ErrPrefectureRequired, "PREFECTURE_REQUIRED", http.StatusBadRequest},
	{ErrCityRequired, "CITY_REQUIRED", http.StatusBadRequest},
	{ErrInvalidRoomTypeName, "INVALID_ROOM_TYPE_NAME", http.StatusBadRequest},
	{ErrInvalidCapacity, "INVALID_CAPACITY", http.StatusBadRequest},
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

type accommodationRequest struct {
	Name          string `json:"name"`
	PhoneNumber   string `json:"phoneNumber"`
	PostalCode    string `json:"postalCode"`
	Prefecture    string `json:"prefecture"`
	City          string `json:"city"`
	StreetAddress string `json:"streetAddress"`
	Building      string `json:"building"`
}

type accommodationResponse struct {
	AccommodationId string `json:"accommodationId"`
	Name            string `json:"name"`
	PhoneNumber     string `json:"phoneNumber"`
	PostalCode      string `json:"postalCode"`
	Prefecture      string `json:"prefecture"`
	City            string `json:"city"`
	StreetAddress   string `json:"streetAddress"`
	Building        string `json:"building"`
}

func toAccommodationResponse(a *Accommodation) accommodationResponse {
	return accommodationResponse{
		AccommodationId: a.ID().String(),
		Name:            a.Name(),
		PhoneNumber:     a.PhoneNumber(),
		PostalCode:      a.PostalCode(),
		Prefecture:      a.Prefecture(),
		City:            a.City(),
		StreetAddress:   a.StreetAddress(),
		Building:        a.Building(),
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req accommodationRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	a, err := NewAccommodation(AccommodationCreateInput{
		Name:          req.Name,
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
	if err := a.Save(r.Context(), h.db); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, toAccommodationResponse(&a))
}

func (h *Handler) Find(w http.ResponseWriter, r *http.Request) {
	id, err := sharedhttp.PathUUID(r, "accommodationId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	a, err := FindAccommodationById(r.Context(), h.db, id)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toAccommodationResponse(a))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	accommodations, err := ListAccommodations(r.Context(), h.db)
	if err != nil {
		writeError(w, err)
		return
	}

	body := make([]accommodationResponse, 0, len(accommodations))
	for i := range accommodations {
		body = append(body, toAccommodationResponse(&accommodations[i]))
	}
	sharedhttp.WriteJSON(w, http.StatusOK, body)
}

type roomTypeRequest struct {
	Name           string `json:"name"`
	Capacity       int    `json:"capacity"`
	HasPrivateBath bool   `json:"hasPrivateBath"`
	HasBalcony     bool   `json:"hasBalcony"`
}

type roomTypeResponse struct {
	RoomTypeId      string `json:"roomTypeId"`
	AccommodationId string `json:"accommodationId"`
	Name            string `json:"name"`
	Capacity        int    `json:"capacity"`
	HasPrivateBath  bool   `json:"hasPrivateBath"`
	HasBalcony      bool   `json:"hasBalcony"`
}

func toRoomTypeResponse(rt *RoomType) roomTypeResponse {
	return roomTypeResponse{
		RoomTypeId:      rt.ID().String(),
		AccommodationId: rt.AccommodationId().String(),
		Name:            rt.Name(),
		Capacity:        rt.Capacity(),
		HasPrivateBath:  rt.HasPrivateBath(),
		HasBalcony:      rt.HasBalcony(),
	}
}

func (h *Handler) CreateRoomType(w http.ResponseWriter, r *http.Request) {
	accommodationId, err := sharedhttp.PathUUID(r, "accommodationId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}
	var req roomTypeRequest
	if !sharedhttp.DecodeJSON(w, r, &req) {
		return
	}

	rt, err := NewRoomType(RoomTypeCreateInput{
		AccommodationId: accommodationId,
		Name:            req.Name,
		Capacity:        req.Capacity,
		HasPrivateBath:  req.HasPrivateBath,
		HasBalcony:      req.HasBalcony,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	if err := rt.Save(r.Context(), h.db); err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusCreated, toRoomTypeResponse(rt))
}

func (h *Handler) ListRoomTypes(w http.ResponseWriter, r *http.Request) {
	accommodationId, err := sharedhttp.PathUUID(r, "accommodationId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	roomTypes, err := ListRoomTypesByAccommodationId(r.Context(), h.db, accommodationId)
	if err != nil {
		writeError(w, err)
		return
	}

	body := make([]roomTypeResponse, 0, len(roomTypes))
	for i := range roomTypes {
		body = append(body, toRoomTypeResponse(&roomTypes[i]))
	}
	sharedhttp.WriteJSON(w, http.StatusOK, body)
}

func (h *Handler) FindRoomType(w http.ResponseWriter, r *http.Request) {
	id, err := sharedhttp.PathUUID(r, "roomTypeId")
	if err != nil {
		sharedhttp.WriteBadRequest(w)
		return
	}

	rt, err := FindRoomTypeById(r.Context(), h.db, id)
	if err != nil {
		writeError(w, err)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, toRoomTypeResponse(rt))
}
