package auth

import (
	"net/http"
	"strings"

	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
)

const (
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodeForbidden       = "FORBIDDEN"
)

func RequireBooker(v Verifier) sharedhttp.Middleware {
	return require(v, Booker)
}

func RequireOperator(v Verifier) sharedhttp.Middleware {
	return require(v, Operator)
}

func require(v Verifier, kind UserKind) sharedhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				sharedhttp.WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, "ログインが必要です")
				return
			}

			p, err := v.Verify(r.Context(), raw)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				sharedhttp.WriteError(w, http.StatusUnauthorized, CodeUnauthenticated, "ログインが必要です")
				return
			}
			if p.Kind != kind {
				sharedhttp.WriteError(w, http.StatusForbidden, CodeForbidden, ErrWrongKind.Error())
				return
			}

			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
