package middleware_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

// captureSink records audit events for assertions.
type captureSink struct {
	mu     sync.Mutex
	events []*models.AuditEvent
}

func (s *captureSink) Write(_ context.Context, e *models.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)

	return nil
}

func (s *captureSink) only(t *testing.T) *models.AuditEvent {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.events, 1, "expected exactly one audit record")

	return s.events[0]
}

func TestAudit_RecordsMutationWithActorAndRedaction(t *testing.T) {
	sink := &captureSink{}
	tag := middleware.AuditTag{Resource: models.AuditResourceSSOConfig, Action: models.AuditActionCreate}
	mw := middleware.Audit(tag, sink, log.Default())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// handler can still read the body (proves it was rebuffered by the middleware)
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "knomix-okta")
		middleware.Enrich(r, "provider_name", "knomix-okta")
		middleware.SetAuditResourceID(r, "55")
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/3/sso",
		strings.NewReader(`{"provider_name":"knomix-okta","oidc_client_secret":"shhh","nested":{"password":"p"}}`))
	req = middleware.ContextWithOIDCUser(req, &auth.OIDCUser{Sub: "sub-1", Email: "admin@knomix.io"})
	req = middleware.ContextWithOrgUser(req, &models.OrgUser{ID: 7, OrganizationID: 3, Email: "admin@knomix.io"})
	rec := httptest.NewRecorder()

	mw(handler).ServeHTTP(rec, req)

	ev := sink.only(t)
	assert.Equal(t, models.AuditResourceSSOConfig, ev.Resource)
	assert.Equal(t, models.AuditActionCreate, ev.Action)
	assert.Equal(t, http.MethodPost, ev.RequestMethod)
	assert.Equal(t, http.StatusCreated, ev.ResultStatus)
	assert.Equal(t, "admin@knomix.io", ev.ActorLabel)
	require.NotNil(t, ev.ActorOrgUserID)
	assert.Equal(t, int64(7), *ev.ActorOrgUserID)
	require.NotNil(t, ev.OrganizationID)
	assert.Equal(t, int64(3), *ev.OrganizationID)
	require.NotNil(t, ev.ResourceID)
	assert.Equal(t, "55", *ev.ResourceID)

	// redaction: secret keys replaced, non-secret kept, enrich merged
	assert.Equal(t, "knomix-okta", ev.Body["provider_name"])
	assert.Equal(t, "[redacted]", ev.Body["oidc_client_secret"])
	nested, ok := ev.Body["nested"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "[redacted]", nested["password"])
}

func TestAudit_SkipsPlainGET(t *testing.T) {
	sink := &captureSink{}
	mw := middleware.Audit(middleware.AuditTag{Resource: "license", Action: "list"}, sink, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/keys", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Empty(t, sink.events, "plain GET should not be audited")
}

func TestAudit_AuditsSensitiveRead(t *testing.T) {
	sink := &captureSink{}
	tag := middleware.AuditTag{Resource: models.AuditResourceLicense, Action: "download", AuditReads: true}
	mw := middleware.Audit(tag, sink, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/license", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	ev := sink.only(t)
	assert.Equal(t, "download", ev.Action)
	assert.Equal(t, http.StatusOK, ev.ResultStatus)
}
