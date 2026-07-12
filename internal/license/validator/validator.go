package validator

import (
	"embed"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pharmalytica/janus/internal/license/publickey"
)

// Claims represents the JWT claims for Janus licensing.
type Claims struct {
	AgreementID    int64    `json:"agreement_id"`
	OrganizationID int64    `json:"organization_id"`
	Tier           string   `json:"tier"`
	Features       []string `json:"features"`
	MaxSeats       int      `json:"max_seats"`
	UserEmail      string   `json:"user_email"`
	jwt.RegisteredClaims
}

// Validator validates JWT tokens using the embedded public key.
type Validator struct {
	publicKey interface{} // RSA public key for verification
}

// NewValidator creates a new JWT validator using the embedded public key from assets.
func NewValidator(assets embed.FS) (*Validator, error) {
	publicKey, err := publickey.GetPublicKey(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded public key: %w", err)
	}

	return &Validator{
		publicKey: publicKey,
	}, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (v *Validator) ValidateToken(tokenString string) (*Claims, error) {
	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is RS256
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Additional validation
	if err := v.validateClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateClaims performs additional validation on the claims.
func (v *Validator) validateClaims(claims *Claims) error {
	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return fmt.Errorf("token has expired")
	}

	// Check not-before time
	if claims.NotBefore != nil && claims.NotBefore.Time.After(time.Now()) {
		return fmt.Errorf("token not yet valid")
	}

	// Validate required fields
	if claims.AgreementID == 0 {
		return fmt.Errorf("missing agreement_id in token")
	}

	if claims.OrganizationID == 0 {
		return fmt.Errorf("missing organization_id in token")
	}

	if claims.Tier == "" {
		return fmt.Errorf("missing tier in token")
	}

	if claims.MaxSeats <= 0 {
		return fmt.Errorf("invalid max_seats in token")
	}

	return nil
}

// HasFeature checks if the token has a specific feature enabled.
func (c *Claims) HasFeature(feature string) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// IsExpired returns true if the token is expired.
func (c *Claims) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false
	}
	return c.ExpiresAt.Time.Before(time.Now())
}

// TimeUntilExpiry returns the duration until the token expires.
func (c *Claims) TimeUntilExpiry() time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}
	return time.Until(c.ExpiresAt.Time)
}
