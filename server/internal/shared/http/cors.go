package sharedhttp

import (
	"net/http"
	"slices"
	"strings"
)

func CORS(allowedOrigins []string) Middleware {
	allowedMethods := strings.Join([]string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions,
	}, ", ")
	allowedHeaders := strings.Join([]string{"Content-Type", "X-Request-Id"}, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
				w.Header().Set("Access-Control-Max-Age", "600")

				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
