package inventoryhttp

import (
	"errors"
	"net/http"

	"github.com/Keigo-Hirohara/yadori/internal/inventory/app"
	"github.com/Keigo-Hirohara/yadori/internal/inventory/domain"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

var errorTable = []struct {
	target error
	code   string
	status int
}{
	{domain.ErrInventoryNotFound, "INVENTORY_NOT_FOUND", http.StatusNotFound},
	{domain.ErrHoldNotFound, "HOLD_NOT_FOUND", http.StatusNotFound},

	{domain.ErrAlreadyRegistered, "INVENTORY_ALREADY_REGISTERED", http.StatusConflict},
	{domain.ErrSoldOut, "INVENTORY_SOLD_OUT", http.StatusConflict},
	{domain.ErrClosed, "INVENTORY_CLOSED", http.StatusConflict},
	{domain.ErrAlreadyClosed, "INVENTORY_ALREADY_CLOSED", http.StatusConflict},
	{domain.ErrNotClosed, "INVENTORY_NOT_CLOSED", http.StatusConflict},
	{domain.ErrDuplicatedHold, "HOLD_DUPLICATED", http.StatusConflict},
	{domain.ErrInvalidTransition, "INVALID_TRANSITION", http.StatusConflict},
	{domain.ErrQuantityBelowHolds, "QUANTITY_BELOW_HOLDS", http.StatusConflict},

	{domain.ErrInvalidQuantity, "INVALID_QUANTITY", http.StatusBadRequest},
	{domain.ErrMinusFee, "INVALID_FEE", http.StatusBadRequest},
	{domain.ErrPast, "PAST_DATE", http.StatusBadRequest},
	{domain.ErrInvalidExpiration, "INVALID_EXPIRATION", http.StatusBadRequest},

	{app.ErrBusy, "RESOURCE_BUSY", http.StatusServiceUnavailable},
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
