package bookinghttp

import (
	"errors"
	"net/http"

	"github.com/Keigo-Hirohara/yadori/internal/accommodation"
	"github.com/Keigo-Hirohara/yadori/internal/booking/app"
	"github.com/Keigo-Hirohara/yadori/internal/booking/domain"
	inventoryapp "github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	inventorydomain "github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

var errorTable = []struct {
	target error
	code   string
	status int
}{
	{domain.ErrBookingNotFound, "BOOKING_NOT_FOUND", http.StatusNotFound},
	{accommodation.ErrRoomTypeNotFound, "ROOM_TYPE_NOT_FOUND", http.StatusNotFound},
	{inventorydomain.ErrInventoryNotFound, "INVENTORY_NOT_FOUND", http.StatusNotFound},

	{inventorydomain.ErrSoldOut, "INVENTORY_SOLD_OUT", http.StatusConflict},
	{inventorydomain.ErrClosed, "INVENTORY_CLOSED", http.StatusConflict},
	{inventorydomain.ErrDuplicatedHold, "HOLD_DUPLICATED", http.StatusConflict},

	{domain.ErrInvalidTransition, "INVALID_TRANSITION", http.StatusConflict},

	{domain.ErrOverCapacity, "OVER_CAPACITY", http.StatusUnprocessableEntity},
	{domain.ErrNoGuest, "NO_GUEST", http.StatusUnprocessableEntity},

	{domain.ErrInvalidStayPeriod, "INVALID_STAY_PERIOD", http.StatusBadRequest},
	{domain.ErrPastStayPeriod, "PAST_STAY_PERIOD", http.StatusBadRequest},
	{domain.ErrInvalidName, "INVALID_GUEST_NAME", http.StatusBadRequest},
	{domain.ErrInvalidTotalFee, "INVALID_TOTAL_FEE", http.StatusBadRequest},

	{inventoryapp.ErrBusy, "RESOURCE_BUSY", http.StatusServiceUnavailable},

	{app.ErrCompensationFailed, "COMPENSATION_FAILED", http.StatusInternalServerError},
}

func writeError(w http.ResponseWriter, err error) {
	for _, e := range errorTable {
		if errors.Is(err, e.target) {
			if e.status == http.StatusServiceUnavailable {
				w.Header().Set("Retry-After", "1")
			}
			sharedhttp.WriteError(w, e.status, e.code, err.Error())
			return
		}
	}
	sharedhttp.WriteInternal(w, err)
}
