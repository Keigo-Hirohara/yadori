package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Keigo-Hirohara/yadori/internal/shared/auth"
	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

const audience = "yadori-api"

type fakeIdP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	idp := &fakeIdP{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                idp.server.URL,
			"jwks_uri":                              idp.server.URL + "/keys",
			"authorization_endpoint":                idp.server.URL + "/auth",
			"token_endpoint":                        idp.server.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}},
		})
	})
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *fakeIdP) issuer() string { return idp.server.URL }

func (idp *fakeIdP) token(t *testing.T, subject string, expiresIn time.Duration, signWith *rsa.PrivateKey) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: signWith},
		(&jose.SignerOptions{}).WithHeader("kid", "test"),
	)
	require.NoError(t, err)
	claims := jwt.Claims{
		Issuer:   idp.server.URL,
		Subject:  subject,
		Audience: jwt.Audience{audience},
		Expiry:   jwt.NewNumericDate(time.Now().Add(expiresIn)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func newVerifier(t *testing.T, idp *fakeIdP, kind auth.UserKind) auth.Verifier {
	t.Helper()
	v, err := auth.NewOIDCVerifier(context.Background(), kind, idp.issuer(), audience)
	require.NoError(t, err)
	return v
}

func call(handler http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func codeOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body sharedhttp.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Code
}

func TestRequireBooker(t *testing.T) {
	idp := newFakeIdP(t)
	verifier := newVerifier(t, idp, auth.Booker)

	var seen string
	protected := auth.RequireBooker(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = auth.BookerFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("正しいトークンなら通り、利用者IDが取り出せる", func(t *testing.T) {
		rec := call(protected, idp.token(t, "user-1", time.Hour, idp.key))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "user-1", seen)
	})

	t.Run("トークンが無ければ401", func(t *testing.T) {
		rec := call(protected, "")

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.Equal(t, auth.CodeUnauthenticated, codeOf(t, rec))
		require.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
	})

	t.Run("別の鍵で署名されたトークンは401", func(t *testing.T) {
		other, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		rec := call(protected, idp.token(t, "user-1", time.Hour, other))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("期限切れのトークンは401", func(t *testing.T) {
		rec := call(protected, idp.token(t, "user-1", -time.Minute, idp.key))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("壊れたトークンは401", func(t *testing.T) {
		rec := call(protected, "not.a.jwt")

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Bearer 以外の方式は受け付けない", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rec := httptest.NewRecorder()
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestUserKinds(t *testing.T) {
	bookerIdP := newFakeIdP(t)
	operatorIdP := newFakeIdP(t)
	bookerVerifier := newVerifier(t, bookerIdP, auth.Booker)
	operatorVerifier := newVerifier(t, operatorIdP, auth.Operator)

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	bookerOnly := auth.RequireBooker(bookerVerifier)(ok)
	operatorOnly := auth.RequireOperator(operatorVerifier)(ok)

	t.Run("運営者のトークンで予約者向けの操作はできない", func(t *testing.T) {
		rec := call(bookerOnly, operatorIdP.token(t, "op-1", time.Hour, operatorIdP.key))

		require.Equal(t, http.StatusUnauthorized, rec.Code, "発行者が違うので検証の段階で通らない")
	})

	t.Run("予約者のトークンで運営者向けの操作はできない", func(t *testing.T) {
		rec := call(operatorOnly, bookerIdP.token(t, "user-1", time.Hour, bookerIdP.key))

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("運営者は運営者向けの操作ができる", func(t *testing.T) {
		rec := call(operatorOnly, operatorIdP.token(t, "op-1", time.Hour, operatorIdP.key))

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("予約者の context から運営者IDは取り出せない", func(t *testing.T) {
		var operator string
		var found bool
		handler := auth.RequireBooker(bookerVerifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			operator, found = auth.OperatorFrom(r.Context())
		}))
		call(handler, bookerIdP.token(t, "user-1", time.Hour, bookerIdP.key))

		require.False(t, found)
		require.Empty(t, operator)
	})
}
