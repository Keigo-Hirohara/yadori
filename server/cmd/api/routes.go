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

type guards struct {
	booker             sharedhttp.Middleware
	operator           sharedhttp.Middleware
	accommodationOwner sharedhttp.Middleware
	roomTypeOwner      sharedhttp.Middleware
}

func routes(h handlers, g guards) *http.ServeMux {
	mux := http.NewServeMux()

	handle := func(pattern string, fn http.HandlerFunc, middlewares ...sharedhttp.Middleware) {
		mux.Handle(pattern, sharedhttp.Chain(fn, middlewares...))
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		sharedhttp.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	handle("GET /api/v1/room-types/search", h.search.Search)
	handle("GET /api/v1/room-types/{roomTypeId}", h.accommodation.FindRoomType)

	handle("GET /api/v1/bookers/me", h.booker.Me, g.booker)
	handle("POST /api/v1/bookers/me", h.booker.Create, g.booker)
	handle("GET /api/v1/bookers/me/bookings", h.booking.ListByBooker, g.booker)

	handle("POST /api/v1/bookings", h.booking.Book, g.booker)
	handle("GET /api/v1/bookings/{bookingId}", h.booking.Find, g.booker)
	handle("POST /api/v1/bookings/{bookingId}/payment", h.booking.Payment, g.booker)
	handle("POST /api/v1/bookings/{bookingId}/cancel", h.booking.Cancel, g.booker)
	handle("PUT /api/v1/bookings/{bookingId}/guests", h.booking.ChangeGuests, g.booker)

	handle("GET /api/v1/admin/accommodations", h.accommodation.List, g.operator)
	handle("POST /api/v1/admin/accommodations", h.accommodation.Create, g.operator)
	handle("GET /api/v1/admin/accommodations/{accommodationId}", h.accommodation.Find, g.operator, g.accommodationOwner)
	handle("GET /api/v1/admin/accommodations/{accommodationId}/room-types", h.accommodation.ListRoomTypes, g.operator, g.accommodationOwner)
	handle("POST /api/v1/admin/accommodations/{accommodationId}/room-types", h.accommodation.CreateRoomType, g.operator, g.accommodationOwner)
	handle("GET /api/v1/admin/room-types/{roomTypeId}", h.accommodation.FindRoomType, g.operator, g.roomTypeOwner)

	handle("GET /api/v1/admin/room-types/{roomTypeId}/inventories", h.inventory.List, g.operator, g.roomTypeOwner)
	handle("POST /api/v1/admin/room-types/{roomTypeId}/inventories", h.inventory.Register, g.operator, g.roomTypeOwner)
	handle("PUT /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/quantity", h.inventory.ChangeQuantity, g.operator, g.roomTypeOwner)
	handle("PUT /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/fee", h.inventory.ChangeFee, g.operator, g.roomTypeOwner)
	handle("POST /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/close", h.inventory.Close, g.operator, g.roomTypeOwner)
	handle("POST /api/v1/admin/room-types/{roomTypeId}/inventories/{date}/reopen", h.inventory.Reopen, g.operator, g.roomTypeOwner)

	return mux
}
