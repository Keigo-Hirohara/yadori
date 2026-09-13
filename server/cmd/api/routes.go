package main

import (
	"net/http"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	"github.com/Keigo-Hirohara/yadori/internal/booker"
	bookinghttp "github.com/Keigo-Hirohara/yadori/internal/booking/infra/http"
	inventoryhttp "github.com/Keigo-Hirohara/yadori/internal/inventory/infra/http"
	"github.com/Keigo-Hirohara/yadori/internal/search"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

type handlers struct {
	accommodation *accommodation.Handler
	booker        *booker.Handler
	inventory     *inventoryhttp.Handler
	booking       *bookinghttp.Handler
	search        *search.Handler
}

func routes(h handlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		sharedhttp.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/v1/admin/accommodations", h.accommodation.List)
	mux.HandleFunc("POST /api/v1/admin/accommodations", h.accommodation.Create)
	mux.HandleFunc("GET /api/v1/admin/accommodations/{accommodationId}", h.accommodation.Find)
	mux.HandleFunc("GET /api/v1/admin/accommodations/{accommodationId}/room-types", h.accommodation.ListRoomTypes)
	mux.HandleFunc("POST /api/v1/admin/accommodations/{accommodationId}/room-types", h.accommodation.CreateRoomType)
	mux.HandleFunc("GET /api/v1/admin/room-types/{roomTypeId}", h.accommodation.FindRoomType)

	mux.HandleFunc("GET /api/v1/admin/room-types/{roomTypeId}/inventories", h.inventory.List)
	mux.HandleFunc("POST /api/v1/admin/room-types/{roomTypeId}/inventories", h.inventory.Register)
	mux.HandleFunc("PUT /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/quantity", h.inventory.ChangeQuantity)
	mux.HandleFunc("PUT /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/fee", h.inventory.ChangeFee)
	mux.HandleFunc("POST /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/close", h.inventory.Close)
	mux.HandleFunc("POST /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/reopen", h.inventory.Reopen)

	mux.HandleFunc("GET /api/v1/room-types/search", h.search.Search)
	mux.HandleFunc("GET /api/v1/room-types/{roomTypeId}", h.accommodation.FindRoomType)

	mux.HandleFunc("POST /api/v1/bookers", h.booker.Create)
	mux.HandleFunc("GET /api/v1/bookers/{bookerId}", h.booker.Find)
	mux.HandleFunc("GET /api/v1/bookers/{bookerId}/bookings", h.booking.ListByBooker)

	mux.HandleFunc("POST /api/v1/bookings", h.booking.Book)
	mux.HandleFunc("GET /api/v1/bookings/{bookingId}", h.booking.Find)
	mux.HandleFunc("POST /api/v1/bookings/{bookingId}/payment", h.booking.Payment)
	mux.HandleFunc("POST /api/v1/bookings/{bookingId}/cancel", h.booking.Cancel)
	mux.HandleFunc("PUT /api/v1/bookings/{bookingId}/guests", h.booking.ChangeGuests)

	return mux
}
