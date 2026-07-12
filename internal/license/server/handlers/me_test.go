package handlers_test

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/server/handlers"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// TestMeReplaceKey_Gates covers the handler-level gates that run BEFORE the
// service is invoked (so no DB is needed): authentication and the mandatory
// rotation acknowledgement.
func TestMeReplaceKey_Gates(t *testing.T) {
	// Service is never called on these paths, so a zero-value service is fine.
	svc := portal.NewService(nil, nil, nil, log.Default())
	h := handlers.MeReplaceKey(svc, log.Default(), writeJSON, writeError)

	t.Run("no org user → 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/key", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		h(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("missing acknowledgement → 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/key",
			strings.NewReader(`{"public_key_pem":"PEM","acknowledged":false}`))
		req = middleware.ContextWithOrgUser(req, &models.OrgUser{ID: 1, CognitoSub: "s"})
		rec := httptest.NewRecorder()
		h(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "acknowledge")
	})

	t.Run("missing pem → 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/key",
			strings.NewReader(`{"acknowledged":true}`))
		req = middleware.ContextWithOrgUser(req, &models.OrgUser{ID: 1, CognitoSub: "s"})
		rec := httptest.NewRecorder()
		h(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
