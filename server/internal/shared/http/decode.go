package sharedhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidPathValue = errors.New("パスの値が不正です")

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		WriteBadRequest(w)
		return false
	}
	return true
}

func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, ErrInvalidPathValue
	}
	return id, nil
}

func PathDate(r *http.Request, name string) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, r.PathValue(name))
	if err != nil {
		return time.Time{}, ErrInvalidPathValue
	}
	return date, nil
}
