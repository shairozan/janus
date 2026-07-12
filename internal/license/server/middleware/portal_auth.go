package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/models"
)

// OrgUserLookup resolves an OrgUser by their Cognito subject. Returns (nil, nil)
// if no such user exists.
type OrgUserLookup func(ctx context.Context, cognitoSub string) (*models.OrgUser, error)

// ResolveOrgUser requires an authenticated OIDC user (set by Authentication),
// resolves the matching OrgUser, and stores it in the request context for the
// downstream portal handlers/middleware.
//
// Unlike the Janus-staff API, portal routes do NOT apply an email-domain
// allowlist — customers federate from arbitrary domains, so authorization is by
// org membership + Cognito group only.
func ResolveOrgUser(lookup OrgUserLookup, logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			oidcUser, ok := GetOIDCUser(r)
			if !ok || oidcUser == nil || oidcUser.Sub == "" {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")

				return
			}

			orgUser, err := lookup(r.Context(), oidcUser.Sub)
			if err != nil {
				logger.Printf("org user lookup failed for sub %s: %v", oidcUser.Sub, err)
				writeJSONError(w, http.StatusInternalServerError, "failed to resolve user")

				return
			}

			if orgUser == nil || !orgUser.IsActive() {
				writeJSONError(w, http.StatusUnauthorized, "user is not a member of any organization")

				return
			}

			next.ServeHTTP(w, ContextWithOrgUser(r, orgUser))
		})
	}
}

// RequireCustomerAdmin requires the caller to be a customer_admin. Cognito group
// membership (from the token's cognito:groups claim) is authoritative; the
// persisted OrgUser.Role is accepted as a fallback.
func RequireCustomerAdmin() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isCustomerAdmin(r) {
				writeJSONError(w, http.StatusForbidden, "customer admin role required")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOrgScope asserts the resolved OrgUser belongs to the organization
// identified in the route (extracted by orgIDFrom). Prevents a member of org A
// from acting on org B.
func RequireOrgScope(orgIDFrom func(*http.Request) (int64, bool)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgUser, ok := GetOrgUser(r)
			if !ok || orgUser == nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")

				return
			}

			routeOrgID, ok := orgIDFrom(r)
			if !ok {
				writeJSONError(w, http.StatusBadRequest, "missing or invalid organization id")

				return
			}

			if orgUser.OrganizationID != routeOrgID {
				writeJSONError(w, http.StatusForbidden, "access denied: not a member of this organization")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isCustomerAdmin reports whether the request's caller holds the customer_admin
// role, preferring the Cognito group claim and falling back to the persisted role.
func isCustomerAdmin(r *http.Request) bool {
	if oidcUser, ok := GetOIDCUser(r); ok && oidcUser != nil {
		for _, g := range oidcUser.Groups {
			if g == models.OrgUserRoleCustomerAdmin {
				return true
			}
		}
	}

	if orgUser, ok := GetOrgUser(r); ok && orgUser != nil && orgUser.IsAdmin() {
		return true
	}

	return false
}
