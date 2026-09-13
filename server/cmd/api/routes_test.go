package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedhttp "github.com/Keigo-Hirohara/yadori/internal/shared/http"
	"github.com/stretchr/testify/require"
)

func TestRoutes(t *testing.T) {
	mux := routes(handlers{})

	t.Run("ヘルスチェックが応答する", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	})

	t.Run("知らないパスは404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("メソッドが違えば405", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/bookings", nil))

		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestMiddlewareChain(t *testing.T) {
	t.Run("リクエストIDがレスポンスヘッダに載る", func(t *testing.T) {
		handler := sharedhttp.Chain(routes(handlers{}),
			sharedhttp.Recover,
			sharedhttp.RequestID,
			sharedhttp.AccessLog,
			sharedhttp.Timeout(time.Second),
			sharedhttp.MaxBytes(1<<20),
		)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		require.NotEmpty(t, rec.Header().Get("X-Request-Id"))
	})

	t.Run("送られてきたリクエストIDを引き継ぐ", func(t *testing.T) {
		handler := sharedhttp.Chain(routes(handlers{}), sharedhttp.RequestID)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("X-Request-Id", "abc-123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, "abc-123", rec.Header().Get("X-Request-Id"))
	})

	t.Run("許可した生成元にはCORSヘッダを返す", func(t *testing.T) {
		handler := sharedhttp.Chain(routes(handlers{}),
			sharedhttp.CORS([]string{"http://localhost:5173"}))

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))
		require.Contains(t, rec.Header().Get("Vary"), "Origin")
	})

	t.Run("許可していない生成元には許可を出さない", func(t *testing.T) {
		handler := sharedhttp.Chain(routes(handlers{}),
			sharedhttp.CORS([]string{"http://localhost:5173"}))

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("Origin", "http://evil.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("事前確認はハンドラまで届かず204で返る", func(t *testing.T) {
		reached := false
		spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
		handler := sharedhttp.Chain(spy, sharedhttp.CORS([]string{"http://localhost:5173"}))

		req := httptest.NewRequest(http.MethodOptions, "/api/v1/bookings", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		require.False(t, reached, "プリフライトはCORSで完結させる")
	})

	t.Run("パニックしてもプロセスは落ちず500を返す", func(t *testing.T) {
		panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("想定外の失敗")
		})
		handler := sharedhttp.Chain(panicking, sharedhttp.Recover)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

		require.Equal(t, http.StatusInternalServerError, rec.Code)

		var body sharedhttp.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, sharedhttp.CodeInternal, body.Code)
		require.NotContains(t, body.Message, "想定外の失敗", "内部の詳細を漏らさない")
	})
}
