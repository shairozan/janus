package handlers

import (
	"time"
)

// TokenRequest represents a request to generate a JWT token.
type TokenRequest struct {
	AgreementID int64  `json:"agreement_id"`
	UserEmail   string `json:"user_email"`
	Duration    int64  `json:"duration_seconds"` // Optional, defaults to 365 days
}

// TokenResponse represents a JWT token response.
type TokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ValidateTokenRequest represents a request to validate a JWT token.
type ValidateTokenRequest struct {
	Token string `json:"token"`
}

// ValidateTokenResponse represents a token validation response.
type ValidateTokenResponse struct {
	Valid  bool                   `json:"valid"`
	Claims map[string]interface{} `json:"claims,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

// RotateKeyRequest represents a request to rotate signing keys.
type RotateKeyRequest struct {
	OrganizationID *int64 `json:"organization_id,omitempty"` // Optional, for org-specific keys
	ExpiresInDays  int    `json:"expires_in_days"`           // How long until the new key expires
}

// RotateKeyResponse represents a key rotation response.
type RotateKeyResponse struct {
	NewKeyID  string    `json:"new_key_id"`
	OldKeyID  string    `json:"old_key_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	PublicKey string    `json:"public_key"`
}