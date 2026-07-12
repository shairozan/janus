package middleware

import (
	"context"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Context keys for request-scoped data.
type contextKey string

const (
	contextKeyAgreement    contextKey = "agreement"
	contextKeyOrganization contextKey = "organization"
	contextKeyClaims       contextKey = "jwt_claims"
	contextKeyOIDCUser     contextKey = "oidc_user"
	contextKeyOrgUser      contextKey = "org_user"
)

// GetClaims retrieves JWT claims from request context.
func GetClaims(r *http.Request) (*jwt.Claims, bool) {
	claims, ok := r.Context().Value(contextKeyClaims).(*jwt.Claims)

	return claims, ok
}

// GetOIDCUser retrieves OIDC user information from request context.
func GetOIDCUser(r *http.Request) (*auth.OIDCUser, bool) {
	user, ok := r.Context().Value(contextKeyOIDCUser).(*auth.OIDCUser)

	return user, ok
}

// ContextWithOIDCUser returns a copy of r with user stored in its context.
// Useful in tests and middleware that needs to inject a known user.
func ContextWithOIDCUser(r *http.Request, user *auth.OIDCUser) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextKeyOIDCUser, user))
}

// GetOrgUser retrieves the resolved portal OrgUser from request context.
func GetOrgUser(r *http.Request) (*models.OrgUser, bool) {
	u, ok := r.Context().Value(contextKeyOrgUser).(*models.OrgUser)

	return u, ok
}

// ContextWithOrgUser returns a copy of r with the OrgUser stored in its context.
func ContextWithOrgUser(r *http.Request, u *models.OrgUser) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextKeyOrgUser, u))
}

// GetAgreementContext retrieves agreement data from request context.
func GetAgreementContext(r *http.Request) (*models.Agreement, bool) {
	agreement, ok := r.Context().Value(contextKeyAgreement).(*models.Agreement)

	return agreement, ok
}

// GetOrganizationContext retrieves organization data from request context.
func GetOrganizationContext(r *http.Request) (*models.Organization, bool) {
	org, ok := r.Context().Value(contextKeyOrganization).(*models.Organization)

	return org, ok
}
