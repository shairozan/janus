package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
	"gorm.io/gorm"
)

// KeyManager is an interface for managing signing keys.
type KeyManager interface {
	GenerateMasterKey(keyID string, validityPeriod time.Duration) (*models.SigningKey, error)
	GenerateCustomerKey(keyID string, orgID int64, validityPeriod time.Duration) (*models.SigningKey, error)
	RotateKey(newKeyID string, oldKeyID string, orgID *int64, validityPeriod time.Duration) error
}

// SigningKeyResponse represents a signing key response
type SigningKeyResponse struct {
	KeyID          string     `json:"key_id"`
	OrganizationID *int64     `json:"organization_id,omitempty"`
	PublicKeyPEM   string     `json:"public_key_pem"`
	Active         bool       `json:"active"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
}

// ListKeys retrieves all signing keys (optionally filtered by organization)
func ListKeys(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		orgIDStr := r.URL.Query().Get("organization_id")

		var keys []models.SigningKey
		query := db.Order("created_at DESC")

		if orgIDStr != "" {
			orgID, parseErr := strconv.ParseInt(orgIDStr, 10, 64)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "Invalid organization_id")

				return
			}
			query = query.Where("organization_id = ?", orgID)
		}

		if err := query.Find(&keys).Error; err != nil {
			logger.Printf("Failed to list signing keys: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to list signing keys")

			return
		}

		response := make([]SigningKeyResponse, len(keys))
		for i, key := range keys {
			response[i] = SigningKeyResponse{
				KeyID:          key.KeyID,
				OrganizationID: key.OrganizationID,
				PublicKeyPEM:   key.PublicKeyPEM,
				Active:         key.IsActive(),
				CreatedAt:      key.CreatedAt,
				ExpiresAt:      key.ExpiresAt,
				RevokedAt:      key.RevokedAt,
			}
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// GetPublicKey retrieves a public key by key_id
func GetPublicKey(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		keyID := r.URL.Query().Get("key_id")
		if keyID == "" {
			writeError(w, http.StatusBadRequest, "key_id is required")

			return
		}

		var key models.SigningKey
		if err := db.Where("key_id = ?", keyID).First(&key).Error; err != nil {
			logger.Printf("Failed to get signing key %s: %v", keyID, err)
			writeError(w, http.StatusNotFound, "Signing key not found")

			return
		}

		response := SigningKeyResponse{
			KeyID:          key.KeyID,
			OrganizationID: key.OrganizationID,
			PublicKeyPEM:   key.PublicKeyPEM,
			Active:         key.IsActive(),
			CreatedAt:      key.CreatedAt,
			ExpiresAt:      key.ExpiresAt,
			RevokedAt:      key.RevokedAt,
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// RotateKey handles POST /api/v1/keys/rotate requests.
func RotateKey(database *db.DB, keyMgr KeyManager, logger *log.Logger, writeJSON func(http.ResponseWriter, int, interface{}), writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req RotateKeyRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		// Set default expiration if not provided (1 year)
		if req.ExpiresInDays == 0 {
			req.ExpiresInDays = 365
		}

		validityPeriod := time.Duration(req.ExpiresInDays) * 24 * time.Hour

		// Get current key (if any)
		var oldKeyID string
		currentKey, err := database.GetActiveSigningKey(req.OrganizationID)
		if err == nil {
			oldKeyID = currentKey.KeyID
		}

		// Generate new key ID
		newKeyID := fmt.Sprintf("key-%d", time.Now().Unix())

		// Perform key rotation (creates new key and handles old key if present)
		if err := keyMgr.RotateKey(newKeyID, oldKeyID, req.OrganizationID, validityPeriod); err != nil {
			logger.Printf("Error rotating key: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to rotate key")
			return
		}

		// Retrieve the newly created key for response
		newKey, err := database.GetSigningKeyByID(newKeyID)
		if err != nil {
			logger.Printf("Error retrieving new key: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to retrieve new key")
			return
		}

		writeJSON(w, http.StatusOK, RotateKeyResponse{
			NewKeyID:  newKey.KeyID,
			OldKeyID:  oldKeyID,
			ExpiresAt: newKey.ExpiresAt,
			PublicKey: newKey.PublicKeyPEM,
		})
	}
}