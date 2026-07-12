package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

func TestCORS(t *testing.T) {
	allowed := []string{"https://janus.example.com"}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	h := middleware.CORS(allowed)(next)

	t.Run("allowed origin gets ACAO headers on POST", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", nil)
		req.Header.Set("Origin", "https://janus.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code, "POST should reach next handler")
		assert.Equal(t, "https://janus.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("OPTIONS preflight short-circuits with 204", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/signup", nil)
		req.Header.Set("Origin", "https://janus.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://janus.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("disallowed origin gets no ACAO", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("empty allowlist sets no ACAO (portal-only mode)", func(t *testing.T) {
		h := middleware.CORS(nil)(next)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", nil)
		req.Header.Set("Origin", "https://janus.example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, http.StatusCreated, rec.Code)
	})
}
