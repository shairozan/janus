package middleware

import (
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