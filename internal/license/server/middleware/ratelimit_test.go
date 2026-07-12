package middleware_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := middleware.NewRateLimiter(2, time.Hour)

	assert.True(t, rl.Allow("a"), "1st under limit")
	assert.True(t, rl.Allow("a"), "2nd at limit")
	assert.False(t, rl.Allow("a"), "3rd over limit")

	assert.True(t, rl.Allow("b"), "different key is independent")
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 20*time.Millisecond)

	assert.True(t, rl.Allow("a"))
	assert.False(t, rl.Allow("a"))

	time.Sleep(30 * time.Millisecond)
	assert.True(t, rl.Allow("a"), "window elapsed → allowed again")
}

func newSignupLimiter(emailLimit, ipLimit int) func(http.Handler) http.Handler {
	return middleware.SignupRateLimit(
		middleware.NewRateLimiter(emailLimit, time.Hour),
		middleware.NewRateLimiter(ipLimit, time.Hour),
		log.Default(),
	)
}

func TestSignupRateLimit_PerIP(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	h := newSignupLimiter(100, 2)(ok)

	send := func(email string) int {
		// Distinct emails so only the IP limiter can trip. Same RemoteAddr.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(`{"admin_email":"`+email+`"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	assert.Equal(t, http.StatusCreated, send("a@x.io"))
	assert.Equal(t, http.StatusCreated, send("b@x.io"))
	assert.Equal(t, http.StatusTooManyRequests, send("c@x.io"), "3rd from same IP throttled")
}

func TestSignupRateLimit_PerEmail(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	h := newSignupLimiter(2, 100)(ok)

	send := func(ip string) int {
		// Same email, distinct forwarded IPs so only the email limiter can trip.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(`{"admin_email":"dup@x.io"}`))
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	assert.Equal(t, http.StatusCreated, send("203.0.113.1"))
	assert.Equal(t, http.StatusCreated, send("203.0.113.2"))
	assert.Equal(t, http.StatusTooManyRequests, send("203.0.113.3"), "3rd for same email throttled")
}

func TestSignupRateLimit_RestoresBody(t *testing.T) {
	var seen string
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = string(b)
		w.WriteHeader(http.StatusCreated)
	})
	h := newSignupLimiter(100, 100)(echo)

	body := `{"org_name":"Acme","admin_email":"owner@acme.io"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, body, seen, "handler must see the full body the limiter peeked")
}
