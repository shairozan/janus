package middleware_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

func lookupReturning(u *models.OrgUser, err error) middleware.OrgUserLookup {
	return func(_ context.Context, _ string) (*models.OrgUser, error) {
		return u, err
	}
}

func TestResolveOrgUser(t *testing.T) {
	activeUser := &models.OrgUser{ID: 7, OrganizationID: 3, CognitoSub: "sub-x", Role: models.OrgUserRoleMember}

	t.Run("no authenticated user → 401", func(t *testing.T) {
		mw := middleware.ResolveOrgUser(lookupReturning(activeUser, nil), log.Default())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("no matching org user → 401", func(t *testing.T) {
		mw := middleware.ResolveOrgUser(lookupReturning(nil, nil), log.Default())
		req := middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), &auth.OIDCUser{Sub: "sub-x"})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("deactivated user → 401", func(t *testing.T) {
		now := time.Now()
		deactivated := &models.OrgUser{ID: 9, OrganizationID: 3, CognitoSub: "sub-x", Role: models.OrgUserRoleMember, DeactivatedAt: &now}
		mw := middleware.ResolveOrgUser(lookupReturning(deactivated, nil), log.Default())
		req := middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), &auth.OIDCUser{Sub: "sub-x"})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("lookup error → 500", func(t *testing.T) {
		mw := middleware.ResolveOrgUser(lookupReturning(nil, errors.New("db down")), log.Default())
		req := middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), &auth.OIDCUser{Sub: "sub-x"})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("active user → resolved into context", func(t *testing.T) {
		mw := middleware.ResolveOrgUser(lookupReturning(activeUser, nil), log.Default())
		req := middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), &auth.OIDCUser{Sub: "sub-x"})
		rec := httptest.NewRecorder()

		var got *models.OrgUser
		capture := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got, _ = middleware.GetOrgUser(r)
		})
		mw(capture).ServeHTTP(rec, req)

		require.NotNil(t, got)
		assert.Equal(t, int64(7), got.ID)
	})
}

func TestRequireCustomerAdmin(t *testing.T) {
	t.Run("admin via cognito group → pass", func(t *testing.T) {
		mw := middleware.RequireCustomerAdmin()
		req := middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/", nil), &auth.OIDCUser{Sub: "s", Groups: []string{"customer_admin"}})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("admin via persisted role → pass", func(t *testing.T) {
		mw := middleware.RequireCustomerAdmin()
		req := middleware.ContextWithOrgUser(httptest.NewRequest(http.MethodGet, "/", nil), &models.OrgUser{Role: models.OrgUserRoleCustomerAdmin})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("member → 403", func(t *testing.T) {
		mw := middleware.RequireCustomerAdmin()
		req := middleware.ContextWithOrgUser(
			middleware.ContextWithOIDCUser(httptest.NewRequest(http.MethodGet, "/", nil), &auth.OIDCUser{Sub: "s", Groups: []string{}}),
			&models.OrgUser{Role: models.OrgUserRoleMember},
		)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestRequireOrgScope(t *testing.T) {
	const routeOrg int64 = 42
	extractor := func(_ *http.Request) (int64, bool) { return routeOrg, true }

	t.Run("matching org → pass", func(t *testing.T) {
		mw := middleware.RequireOrgScope(extractor)
		req := middleware.ContextWithOrgUser(httptest.NewRequest(http.MethodGet, "/", nil), &models.OrgUser{OrganizationID: routeOrg})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("mismatched org → 403", func(t *testing.T) {
		mw := middleware.RequireOrgScope(extractor)
		req := middleware.ContextWithOrgUser(httptest.NewRequest(http.MethodGet, "/", nil), &models.OrgUser{OrganizationID: 999})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("no org user → 401", func(t *testing.T) {
		mw := middleware.RequireOrgScope(extractor)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("unparseable org id → 400", func(t *testing.T) {
		mw := middleware.RequireOrgScope(func(_ *http.Request) (int64, bool) { return 0, false })
		req := middleware.ContextWithOrgUser(httptest.NewRequest(http.MethodGet, "/", nil), &models.OrgUser{OrganizationID: routeOrg})
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
