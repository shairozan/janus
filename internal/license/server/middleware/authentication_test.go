package middleware_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

// stubOIDCValidator implements middleware.OIDCValidator for testing.
type stubOIDCValidator struct {
	user *auth.OIDCUser
	err  error
	// called tracks whether ValidateToken was invoked.
	called bool
}

func (s *stubOIDCValidator) ValidateToken(_ context.Context, _ string) (*auth.OIDCUser, error) {
	s.called = true
	return s.user, s.err
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthentication_NoAuthorizationHeader(t *testing.T) {
	stub := &stubOIDCValidator{}
	mw := middleware.Authentication(stub, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, stub.called, "ValidateToken should not be called when no header present")

	_, hasUser := middleware.GetOIDCUser(req)
	assert.False(t, hasUser, "no OIDCUser should be set in context")
}

func TestAuthentication_MalformedAuthorizationHeader(t *testing.T) {
	stub := &stubOIDCValidator{}
	mw := middleware.Authentication(stub, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, stub.called, "ValidateToken should not be called for non-Bearer auth")
}

func TestAuthentication_ValidToken_SetsUserInContext(t *testing.T) {
	want := &auth.OIDCUser{Sub: "user-123"}
	stub := &stubOIDCValidator{user: want}
	mw := middleware.Authentication(stub, log.Default())

	var capturedReq *http.Request
	capture := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedReq = r
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	mw(capture).ServeHTTP(rec, req)

	require.NotNil(t, capturedReq, "handler should have been called")
	user, ok := middleware.GetOIDCUser(capturedReq)
	require.True(t, ok)
	assert.Equal(t, "user-123", user.Sub)
}

func TestAuthentication_InvalidToken_PassesThrough(t *testing.T) {
	stub := &stubOIDCValidator{err: errors.New("bad token")}
	mw := middleware.Authentication(stub, log.Default())

	var handlerCalled bool
	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		_, hasUser := middleware.GetOIDCUser(r)
		assert.False(t, hasUser, "invalid token should not set OIDCUser")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	mw(handler).ServeHTTP(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthentication_NilValidator_PassesThrough(t *testing.T) {
	mw := middleware.Authentication(nil, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
