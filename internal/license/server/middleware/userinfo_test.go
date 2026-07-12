package middleware_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

var allowedDomains = []string{"januspk.com"}

func newUserInfoCache(t *testing.T) *auth.UserInfoCache {
	t.Helper()

	mr := miniredis.RunT(t)
	cache, err := auth.NewUserInfoCache("redis://" + mr.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { _ = cache.Close() })

	return cache
}

// requestWithUser builds an http.Request pre-loaded with an OIDCUser in context
// (as Authentication middleware would produce) and an optional Bearer token.
func requestWithUser(user *auth.OIDCUser, token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if user != nil {
		req = middleware.ContextWithOIDCUser(req, user)
	}

	return req
}

func TestRequireUserInfo_NoUserInContext_Returns401(t *testing.T) {
	fetchCalled := false
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		fetchCalled = true
		return &auth.OIDCUser{Email: "alice@januspk.com"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, nil, allowedDomains, log.Default())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, fetchCalled)
}

func TestRequireUserInfo_AllowedDomain_PassesThrough(t *testing.T) {
	user := &auth.OIDCUser{Sub: "sub1", Expiry: time.Now().Add(time.Hour)}
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		return &auth.OIDCUser{Sub: "sub1", Email: "alice@januspk.com", Name: "Alice"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, nil, allowedDomains, log.Default())

	req := requestWithUser(user, "valid-token")
	rec := httptest.NewRecorder()

	var enriched *auth.OIDCUser
	capture := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		u, _ := middleware.GetOIDCUser(r)
		enriched = u
		rec.WriteHeader(http.StatusOK)
	})

	mw(capture).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, enriched)
	assert.Equal(t, "alice@januspk.com", enriched.Email)
	assert.Equal(t, "Alice", enriched.Name)
}

func TestRequireUserInfo_DisallowedDomain_Returns403(t *testing.T) {
	user := &auth.OIDCUser{Sub: "sub2", Expiry: time.Now().Add(time.Hour)}
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		return &auth.OIDCUser{Sub: "sub2", Email: "eve@evil.com"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, nil, allowedDomains, log.Default())

	req := requestWithUser(user, "valid-token")
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireUserInfo_FetchError_Returns401(t *testing.T) {
	user := &auth.OIDCUser{Sub: "sub3", Expiry: time.Now().Add(time.Hour)}
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		return nil, errors.New("cognito unavailable")
	})

	mw := middleware.RequireUserInfo(fetch, nil, allowedDomains, log.Default())

	req := requestWithUser(user, "valid-token")
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireUserInfo_CacheHit_FetchNotCalled(t *testing.T) {
	cache := newUserInfoCache(t)
	user := &auth.OIDCUser{Sub: "sub4", Expiry: time.Now().Add(time.Hour)}
	token := "cached-token"

	cached := &auth.OIDCUser{Sub: "sub4", Email: "alice@januspk.com", Name: "Cached Alice"}
	require.NoError(t, cache.Set(context.Background(), token, cached, time.Minute))

	fetchCount := 0
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		fetchCount++
		return nil, errors.New("should not be called")
	})

	mw := middleware.RequireUserInfo(fetch, cache, allowedDomains, log.Default())

	req := requestWithUser(user, token)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 0, fetchCount, "fetch should not be called on a cache hit")
}

func TestRequireUserInfo_CacheMiss_WritesToCache(t *testing.T) {
	cache := newUserInfoCache(t)
	user := &auth.OIDCUser{Sub: "sub5", Expiry: time.Now().Add(time.Hour)}
	token := "miss-token"

	fetchCount := 0
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		fetchCount++
		return &auth.OIDCUser{Sub: "sub5", Email: "bob@januspk.com", Name: "Bob"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, cache, allowedDomains, log.Default())

	req := requestWithUser(user, token)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, fetchCount)

	// Second request with the same token should hit the cache.
	req2 := requestWithUser(user, token)
	rec2 := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, 1, fetchCount, "fetch should not be called on the second request (cache hit)")
}

func TestRequireUserInfo_NilCache_StillWorks(t *testing.T) {
	user := &auth.OIDCUser{Sub: "sub6", Expiry: time.Now().Add(time.Hour)}
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		return &auth.OIDCUser{Sub: "sub6", Email: "charlie@januspk.com"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, nil, allowedDomains, log.Default())

	req := requestWithUser(user, "some-token")
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireUserInfo_NoAllowedDomains_AllowsAny(t *testing.T) {
	user := &auth.OIDCUser{Sub: "sub7", Expiry: time.Now().Add(time.Hour)}
	fetch := middleware.UserInfoFunc(func(_ context.Context, _ string) (*auth.OIDCUser, error) {
		return &auth.OIDCUser{Sub: "sub7", Email: "anyone@random.org"}, nil
	})

	mw := middleware.RequireUserInfo(fetch, nil, nil, log.Default())

	req := requestWithUser(user, "some-token")
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestIsAllowedDomain covers the exported staff email-domain check that both the
// requireUserInfo gate and the /me is_staff flag rely on.
func TestIsAllowedDomain(t *testing.T) {
	domains := []string{"knomix.io", "JanusPK.com"}

	cases := map[string]struct {
		email string
		want  bool
	}{
		"exact match":            {"dev@knomix.io", true},
		"case-insensitive email": {"Dev@KNOMIX.io", true},
		"case-insensitive list":  {"ops@januspk.com", true},
		"wrong domain":           {"user@gmail.com", false},
		"subdomain not allowed":  {"user@mail.knomix.io", false},
		"no at-sign":             {"notanemail", false},
		"empty domain":           {"user@", false},
		"empty email":            {"", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := middleware.IsAllowedDomain(tc.email, domains); got != tc.want {
				t.Fatalf("IsAllowedDomain(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}

	if middleware.IsAllowedDomain("dev@knomix.io", nil) {
		t.Fatal("empty allowlist must deny")
	}
}
