package jwt

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pharmalytica/janus/internal/license/models"
	"gorm.io/gorm"
)

// KeyProvider provides access to signing keys.
type KeyProvider interface {
	GetActiveSigningKey(orgID *int64) (*models.SigningKey, error)
	GetPrivateKey(keyID string) (*rsa.PrivateKey, error)
	GetPublicKey(keyID string) (*rsa.PublicKey, error)
}

// DBProvider provides database access for loading relationships
type DBProvider interface {
	GetDB() *gorm.DB
}

// Generator handles JWT token generation and validation.
type Generator struct {
	keyProvider KeyProvider
	db          DBProvider
}

// NewGenerator creates a new JWT generator with the provided key provider and database.
func NewGenerator(keyProvider KeyProvider, db DBProvider) *Generator {
	return &Generator{
		keyProvider: keyProvider,
		db:          db,
	}
}

// GenerateTokenFromAgreement generates a JWT token for a user based on an agreement.
// This method loads the required relationships (organization, license) automatically.
func (g *Generator) GenerateTokenFromAgreement(
	agr *models.Agreement,
	userEmail string,
	duration time.Duration,
) (string, error) {
	// Load organization and license using GORM
	db := g.db.GetDB()
	var org models.Organization
	if err := db.First(&org, agr.OrganizationID).Error; err != nil {
		return "", fmt.Errorf("failed to load organization: %w", err)
	}

	var lic models.License
	if err := db.First(&lic, agr.LicenseID).Error; err != nil {
		return "", fmt.Errorf("failed to load license: %w", err)
	}

	// Get effective tier (agreement override or license tier)
	tier := lic.Tier
	if agr.Tier != nil && *agr.Tier != "" {
		tier = *agr.Tier
	}

	// Get effective features (agreement override or license features)
	features := lic.ResolveFeatures()
	if agr.Features != nil && len(agr.Features) > 0 {
		features = []string(agr.Features)
	}

	// Get active signing key - try organization-specific first, then fall back to master key
	signingKey, err := g.keyProvider.GetActiveSigningKey(&agr.OrganizationID)
	if err != nil {
		// Fall back to master signing key if organization-specific key not found
		signingKey, err = g.keyProvider.GetActiveSigningKey(nil)
		if err != nil {
			return "", fmt.Errorf("failed to get signing key: %w", err)
		}
	}

	// Get private key
	privateKey, err := g.keyProvider.GetPrivateKey(signingKey.KeyID)
	if err != nil {
		return "", fmt.Errorf("failed to get private key: %w", err)
	}

	// Create claims from agreement data
	claims := NewClaims(
		agr.ID,             // agreement_id
		agr.OrganizationID, // organization_id
		org.CustomerID,
		userEmail,
		org.Name,
		tier,
		features,
		agr.Seats,
		agr.LicenseModel,
		valueOrZero(agr.ConcurrentLimit),
		duration,
		signingKey.KeyID,
	)

	// Generate the token
	return g.generateToken(claims, privateKey, signingKey.KeyID)
}

// generateToken generates a JWT token from the provided claims.
func (g *Generator) generateToken(claims *Claims, privateKey *rsa.PrivateKey, keyID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Add key ID to header for key rotation support
	token.Header["kid"] = keyID

	// Sign the token
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (g *Generator) ValidateToken(tokenString string) (*Claims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get key ID from header
		keyID, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid kid in token header")
		}

		// Get public key for validation
		publicKey, err := g.keyProvider.GetPublicKey(keyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// valueOrZero returns the value if pointer is not nil, otherwise returns zero value.
func valueOrZero(ptr *int) int {
	if ptr == nil {
		return 0
	}

	return *ptr
}

// Example usage showing how features flow from DB to JWT:
//
// 1. License table has: features = ["local", "grid", "audit"] (JSONB)
// 2. Agreement links Organization to License
// 3. GenerateTokenFromAgreement loads relationships and pulls features via GetFeaturesWithFallback
// 4. Features go into JWT claims
// 5. Client validates JWT and checks claims.Features
//
// Query example to find all agreements with "audit" feature:
//   SELECT a.* FROM agreements a
//   JOIN licenses l ON a.license_id = l.id
//   WHERE l.features @> '["audit"]'::jsonb
//   AND a.deactivated_at IS NULL;
