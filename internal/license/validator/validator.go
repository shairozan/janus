package validator

import (
	"crypto/rsa"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/pharmalytica/janus/internal/license/publickey"
	"github.com/pharmalytica/janus/internal/signing"
)

// Claims represents the JWT claims for Janus licensing.
type Claims struct {
	AgreementID    int64    `json:"agreement_id"`
	OrganizationID int64    `json:"organization_id"`
	Tier           string   `json:"tier"`
	Features       []string `json:"features"`
	MaxSeats       int      `json:"max_seats"`
	UserEmail      string   `json:"user_email"`

	// SigningPublicKey is the PEM-encoded RSA public key for run log signature verification.
	// Users generate RSA key pairs externally and submit the public key when requesting
	// a license. Janus validates that the configured private key matches this public key
	// at startup. If empty, run log signing is disabled.
	SigningPublicKey string `json:"signing_public_key,omitempty"`

	jwt.RegisteredClaims
}

// Validator validates JWT tokens using the embedded public key(s).
type Validator struct {
	publicKeys []*rsa.PublicKey // Historical RSA public keys; a token is valid if any verifies it
}

// NewValidator creates a new JWT validator using the embedded public key(s) from assets.
func NewValidator(assets embed.FS) (*Validator, error) {
	publicKeys, err := publickey.GetPublicKeys(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded public key: %w", err)
	}

	return &Validator{
		publicKeys: publicKeys,
	}, nil
}

// ValidateToken validates a JWT token against every embedded public key, returning the
// claims for the first key that verifies it. Validation fails only if no key matches
// (or none are embedded). When a key's signature matches but the token is otherwise
// invalid (e.g. expired), that specific error is surfaced rather than the generic one.
func (v *Validator) ValidateToken(tokenString string) (*Claims, error) {
	// semanticErr captures a signature-valid-but-otherwise-invalid result (e.g. expired
	// or not-yet-valid) so we can give a useful message instead of the generic fallback.
	var semanticErr error

	for _, key := range v.publicKeys {
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Ensure the signing method is RS256
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return key, nil
		})

		if err == nil {
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

		// The signature verified with this key, but the token is expired or not yet
		// valid. Remember it; trying other keys won't change that outcome.
		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			semanticErr = err
		}
	}

	if semanticErr != nil {
		return nil, fmt.Errorf("license token is expired or not yet valid: %w", semanticErr)
	}

	// No embedded key produced a valid signature. Return a generic message to avoid
	// leaking internal details such as key formats or parser internals.
	return nil, fmt.Errorf("token validation failed")
}

// validateClaims performs additional validation on the claims.
func (v *Validator) validateClaims(claims *Claims) error {
	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("token has expired")
	}

	// Check not-before time
	if claims.NotBefore != nil && claims.NotBefore.After(time.Now()) {
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

	return c.ExpiresAt.Before(time.Now())
}

// TimeUntilExpiry returns the duration until the token expires.
func (c *Claims) TimeUntilExpiry() time.Duration {
	if c.ExpiresAt == nil {
		return 0
	}

	return time.Until(c.ExpiresAt.Time)
}

// ValidateSigningKeyPair validates that the configured private key matches
// the public key embedded in the license claims.
// Returns nil if:
//   - No private key is configured (signing is optional)
//   - No public key is in the license claims (signing not required by license)
//   - Both keys are present and match
//
// Returns an error if:
//   - Private key is configured but no public key in license
//   - Public key is in license but no private key configured
//   - Keys don't match
func ValidateSigningKeyPair(claims *Claims, privateKeyPath string) error {
	hasPrivateKey := privateKeyPath != ""
	hasPublicKey := claims.SigningPublicKey != ""

	// If neither is configured, signing is disabled - that's fine
	if !hasPrivateKey && !hasPublicKey {
		return nil
	}

	// If only one is configured, that's an error
	if hasPrivateKey && !hasPublicKey {
		return fmt.Errorf("signing private key is configured but license does not contain a public key; " +
			"please request a new license with your public key or remove the signing configuration")
	}

	if !hasPrivateKey && hasPublicKey {
		return fmt.Errorf("license contains a signing public key but no private key is configured; " +
			"please configure signing.private_key_path in your config file")
	}

	// Both are present - validate they match
	signer, err := signing.NewSigner(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load signing private key: %w", err)
	}

	publicKey, err := signing.LoadPublicKeyFromPEM(claims.SigningPublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key from license: %w", err)
	}

	if err := signer.ValidateKeyPair(publicKey); err != nil {
		return fmt.Errorf("signing key pair mismatch: the configured private key does not match " +
			"the public key in your license; please ensure you are using the correct private key")
	}

	return nil
}
