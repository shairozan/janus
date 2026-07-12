package handlers_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/license/server/handlers"
	"github.com/pharmalytica/janus/internal/license/signup"
)

// TestSignup_Gates covers the handler gates that run BEFORE the service is
// invoked, so no DB/Cognito/Stripe is needed. A zero-value service is safe here
// because none of these paths reach svc.SignUp.
func TestSignup_Gates(t *testing.T) {
	h := handlers.Signup(&signup.Service{}, log.Default(), writeJSON, writeError)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h(rec, req)

		return rec
	}

	t.Run("wrong method → 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/signup", nil)
		rec := httptest.NewRecorder()
		h(rec, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})

	t.Run("malformed JSON → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, post(`{not json`).Code)
	})

	t.Run("missing org_name → 400", func(t *testing.T) {
		rec := post(`{"admin_email":"owner@acme.io"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "org_name")
	})

	t.Run("blank org_name → 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, post(`{"org_name":"   ","admin_email":"owner@acme.io"}`).Code)
	})

	t.Run("invalid email → 400", func(t *testing.T) {
		rec := post(`{"org_name":"Acme","admin_email":"not-an-email"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "admin_email")
	})

	t.Run("oversized body → 400 (bounded read truncates)", func(t *testing.T) {
		huge := strings.Repeat("a", 2<<20) // 2 MiB, past the 1 MiB LimitReader cap
		rec := post(`{"org_name":"` + huge + `","admin_email":"owner@acme.io"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
