package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/requests"
)

// TokenGenerator is an interface for generating JWT tokens.
type TokenGenerator interface {
	GenerateTokenFromAgreement(agreement *models.Agreement, userEmail string, duration time.Duration, signingPublicKey string) (string, error)
}

// GenerateToken handles POST /api/v1/tokens requests.
// This is the token GENERATION endpoint - it takes agreement_id in the request body.
func GenerateToken(database *db.DB, jwtGen TokenGenerator, logger *log.Logger, writeJSON func(http.ResponseWriter, int, interface{}), writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")

			return
		}

		// Parse request body
		var req TokenRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))

			return
		}

		// Validate required fields
		if req.AgreementID == 0 {
			writeError(w, http.StatusBadRequest, "agreement_id is required")

			return
		}
		if req.UserEmail == "" {
			writeError(w, http.StatusBadRequest, "user_email is required")

			return
		}

		// Retrieve the agreement
		var agreement models.Agreement
		if err := database.DB.
			First(&agreement, req.AgreementID).Error; err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("agreement not found: %v", err))

			return
		}

		// Check if agreement is active
		if agreement.DeactivatedAt != nil {
			writeError(w, http.StatusBadRequest, "agreement is deactivated")

			return
		}

		// License lifetime derives from the negotiated agreement term (A10); an
		// explicit duration in the request overrides it.
		duration := time.Duration(req.Duration) * time.Second
		if req.Duration == 0 {
			duration = jwt.LicenseValidity(&agreement, time.Now())
		}

		if duration <= 0 {
			writeError(w, http.StatusBadRequest, "agreement term has expired; cannot issue a license")

			return
		}

		// Generate token
		token, err := jwtGen.GenerateTokenFromAgreement(&agreement, req.UserEmail, duration, req.SigningPublicKey)
		if err != nil {
			logger.Printf("Error generating token: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to generate token")

			return
		}

		expiresAt := time.Now().Add(duration)

		writeJSON(w, http.StatusOK, TokenResponse{
			Token:     token,
			ExpiresAt: expiresAt,
		})
	}
}

// TokenValidator is an interface for validating JWT tokens.
type TokenValidator interface {
	ValidateToken(tokenString string) (*jwt.Claims, error)
}

// ValidateToken handles POST /api/v1/tokens/validate requests. Beyond verifying
// the JWT signature/claims, it performs the online revocation check (A10):
// consulting issued_tokens.revoked_at by JTI so an offboarded/non-paying user is
// cut off near-immediately when online.
func ValidateToken(database *db.DB, jwtGen TokenValidator, writeJSON func(http.ResponseWriter, int, interface{}), writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")

			return
		}

		var req ValidateTokenRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))

			return
		}

		if req.Token == "" {
			writeError(w, http.StatusBadRequest, "token is required")

			return
		}

		// Validate token
		claims, err := jwtGen.ValidateToken(req.Token)
		if err != nil {
			writeJSON(w, http.StatusOK, ValidateTokenResponse{
				Valid: false,
				Error: err.Error(),
			})

			return
		}

		// Online revocation check (A10).
		revoked, err := requests.IsTokenRevoked(r.Context(), database, claims.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check revocation status")

			return
		}

		if revoked {
			writeJSON(w, http.StatusOK, ValidateTokenResponse{
				Valid: false,
				Error: "token has been revoked",
			})

			return
		}

		// Convert claims to map for JSON response
		claimsMap := map[string]interface{}{
			"sub":                claims.Subject,
			"iss":                claims.Issuer,
			"exp":                claims.ExpiresAt.Unix(),
			"iat":                claims.IssuedAt.Unix(),
			"tier":               claims.Tier,
			"features":           claims.Features,
			"seats":              claims.Seats,
			"license_model":      claims.LicenseModel,
			"concurrent_limit":   claims.ConcurrentLimit,
			"signing_public_key": claims.SigningPublicKey,
		}

		writeJSON(w, http.StatusOK, ValidateTokenResponse{
			Valid:  true,
			Claims: claimsMap,
		})
	}
}
