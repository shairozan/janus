package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for a Janus license token.
// This structure matches the licensing.md specification.
type Claims struct {
	jwt.RegisteredClaims

	// Required identifiers
	AgreementID    int64 `json:"agreement_id"`    // Agreement ID for tracking
	OrganizationID int64 `json:"organization_id"` // Organization ID

	// License information
	Tier            string   `json:"tier"`              // "basic", "professional", "enterprise"
	Features        []string `json:"features"`          // ["local", "grid", "audit"]
	Seats           int      `json:"seats"`             // Number of licensed seats
	MaxSeats        int      `json:"max_seats"`         // Maximum seats (alias for compatibility)
	OrgName         string   `json:"org_name"`          // Human-readable organization name
	UserEmail       string   `json:"user_email"`        // User email (duplicate of sub for compatibility)

	// License model
	LicenseModel    string `json:"license_model,omitempty"`     // "per-seat" or "concurrent"
	ConcurrentLimit int    `json:"concurrent_limit,omitempty"`  // For concurrent model

	// Optional: License server URL for concurrent model
	LicenseServerURL string `json:"license_server_url,omitempty"`

	// Key rotation support
	KeyID string `json:"-"` // Stored in JWT header, not claims
}

// NewClaims creates JWT claims from license data.
func NewClaims(
	agreementID int64,
	organizationID int64,
	customerID string,
	userEmail string,
	orgName string,
	tier string,
	features []string,
	seats int,
	licenseModel string,
	concurrentLimit int,
	duration time.Duration,
	keyID string,
) *Claims {
	now := time.Now()

	return &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://license.janus.io",
			Subject:   userEmail,
			Audience:  jwt.ClaimStrings{customerID},
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        generateJTI(), // Unique token ID for revocation
		},
		AgreementID:      agreementID,
		OrganizationID:   organizationID,
		Tier:             tier,
		Features:         features,
		Seats:            seats,
		MaxSeats:         seats, // Both seats and max_seats for compatibility
		OrgName:          orgName,
		UserEmail:        userEmail, // Duplicate in both sub and user_email for compatibility
		LicenseModel:     licenseModel,
		ConcurrentLimit:  concurrentLimit,
		KeyID:            keyID,
	}
}

// HasFeature checks if a specific feature is enabled.
func (c *Claims) HasFeature(feature string) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}

	return false
}

// IsExpired checks if the token has expired.
func (c *Claims) IsExpired() bool {
	return c.ExpiresAt.Before(time.Now())
}

// IsInGracePeriod checks if token is expired but within grace period (7 days).
func (c *Claims) IsInGracePeriod() bool {
	if !c.IsExpired() {
		return false
	}

	expiration := c.ExpiresAt.Time
	gracePeriodEnd := expiration.Add(7 * 24 * time.Hour)

	return time.Now().Before(gracePeriodEnd)
}

// TimeSinceExpiration returns how long ago the token expired.
func (c *Claims) TimeSinceExpiration() time.Duration {
	if !c.IsExpired() {
		return 0
	}

	return time.Since(c.ExpiresAt.Time)
}

// GraceDaysRemaining returns days remaining in grace period (0 if not in grace period).
func (c *Claims) GraceDaysRemaining() int {
	if !c.IsInGracePeriod() {
		return 0
	}

	expiration := c.ExpiresAt.Time
	gracePeriodEnd := expiration.Add(7 * 24 * time.Hour)
	remaining := time.Until(gracePeriodEnd)

	return int(remaining.Hours() / 24)
}

// GetCustomerID returns the customer ID from the audience claim.
func (c *Claims) GetCustomerID() string {
	if len(c.Audience) > 0 {
		return c.Audience[0]
	}

	return ""
}

// generateJTI generates a unique JWT ID for token tracking/revocation.
func generateJTI() string {
	// TODO: Implement proper unique ID generation
	// For now, use timestamp + random suffix
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random alphanumeric string of given length.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}

	return string(b)
}