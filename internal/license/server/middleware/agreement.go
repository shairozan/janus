package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Agreement loads agreement data from JWT claims into the request context.
// This middleware requires Authentication to have run first to populate claims.
// It looks up the agreement based on the organization from the JWT claims.
func Agreement(database *db.DB, writeError func(http.ResponseWriter, int, string)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get JWT claims (requires Authentication middleware)
			claims, ok := GetClaims(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "valid JWT required for this endpoint")
				return
			}

			// Get organization from claims (customer_id in subject)
			var organization models.Organization
			if err := database.DB.Where("customer_id = ?", claims.Subject).First(&organization).Error; err != nil {
				writeError(w, http.StatusNotFound, fmt.Sprintf("organization not found: %v", err))
				return
			}

			// Check if organization is active
			if organization.DeactivatedAt != nil {
				writeError(w, http.StatusForbidden, "organization is deactivated")
				return
			}

			// Find active agreement for this organization
			// Note: An organization may have multiple agreements, we'll take the most recent active one
			var agreement models.Agreement
			if err := database.DB.
				Where("organization_id = ?", organization.ID).
				Where("deactivated_at IS NULL").
				Order("created_at DESC").
				First(&agreement).Error; err != nil {
				writeError(w, http.StatusNotFound, fmt.Sprintf("no active agreement found: %v", err))
				return
			}

			// Get effective tier for agreement (with fallback to license)
			var tier string
			if agreement.Tier != nil && *agreement.Tier != "" {
				tier = *agreement.Tier
			} else {
				// Load license to get tier
				var license models.License
				if err := database.DB.First(&license, agreement.LicenseID).Error; err != nil {
					writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load license: %v", err))

					return
				}
				tier = license.Tier
			}

			// Verify the agreement's tier matches what's in the JWT
			// This ensures the JWT hasn't been tampered with or is outdated
			if tier != claims.Tier {
				writeError(w, http.StatusForbidden, "JWT tier does not match current agreement")
				return
			}

			// Create a new context with both agreement and organization data
			ctx := r.Context()
			ctx = context.WithValue(ctx, contextKeyAgreement, &agreement)
			ctx = context.WithValue(ctx, contextKeyOrganization, &organization)

			// Pass the enriched request to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeError is a helper for writing JSON error responses - passed in to avoid circular dependencies.