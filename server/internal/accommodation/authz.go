package accommodation

import (
	"net/http"

	accommodationdb "github.com/Keigo-Hirohara/yadori/internal/accommodation/db"
	"github.com/Keigo-Hirohara/yadori/internal/shared/auth"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

func RequireAccommodationOwner(db accommodationdb.DBTX) sharedhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, ok := auth.OperatorFrom(r.Context())
			if !ok {
				sharedhttp.WriteError(w, http.StatusUnauthorized, auth.CodeUnauthenticated, "ログインが必要です")
				return
			}
			id, err := sharedhttp.PathUUID(r, "accommodationId")
			if err != nil {
				sharedhttp.WriteBadRequest(w)
				return
			}
			a, err := FindAccommodationById(r.Context(), db, id)
			if err != nil || a.OperatorSubject() != subject {
				writeError(w, ErrAccommodationNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRoomTypeOwner(db accommodationdb.DBTX) sharedhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, ok := auth.OperatorFrom(r.Context())
			if !ok {
				sharedhttp.WriteError(w, http.StatusUnauthorized, auth.CodeUnauthenticated, "ログインが必要です")
				return
			}
			id, err := sharedhttp.PathUUID(r, "roomTypeId")
			if err != nil {
				sharedhttp.WriteBadRequest(w)
				return
			}
			owns, err := OwnsRoomType(r.Context(), db, subject, id)
			if err != nil || !owns {
				writeError(w, ErrRoomTypeNotFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
