package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Organization loads organization data into the request context.
// This can be used by any endpoint that needs organization information.
// Priority order for finding organization:
//  1. JWT claims (from Authorization header - requires Authentication middleware)
//  2. Query parameter: ?organization_id=123
//  3. JSON body field: {"organization_id": 123}
func Organization(database *db.DB, writeError func(http.ResponseWriter, int, string)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var customerID string
			var orgID int64

			// Priority 1: Try to get customer ID from JWT claims
			if claims, ok := GetClaims(r); ok {
				customerID = claims.Subject // The 'sub' field contains the customer_id
			}

			// Priority 2: Try to get organization_id from query parameters
			if customerID == "" {
				if orgIDStr := r.URL.Query().Get("organization_id"); orgIDStr != "" {
					if _, err := fmt.Sscanf(orgIDStr, "%d", &orgID); err != nil {
						writeError(w, http.StatusBadRequest, "invalid organization_id in query parameter")

						return
					}
				}
			}

			// Priority 3: Try to get from JSON body
			if customerID == "" && orgID == 0 {
				var body struct {
					OrganizationID int64 `json:"organization_id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))

					return
				}
				orgID = body.OrganizationID
			}

			// Retrieve organization from database
			var organization models.Organization
			var err error

			switch {
			case customerID != "":
				// Look up by customer_id from JWT
				err = database.DB.Where("customer_id = ?", customerID).First(&organization).Error
			case orgID != 0:
				// Look up by ID
				err = database.DB.First(&organization, orgID).Error
			default:
				writeError(w, http.StatusBadRequest, "organization_id or valid JWT required")

				return
			}

			if err != nil {
				writeError(w, http.StatusNotFound, fmt.Sprintf("organization not found: %v", err))

				return
			}

			// Check if organization is active
			if organization.DeactivatedAt != nil {
				writeError(w, http.StatusBadRequest, "organization is deactivated")

				return
			}

			// Add organization to context
			ctx := context.WithValue(r.Context(), contextKeyOrganization, &organization)

			// Pass the enriched request to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
