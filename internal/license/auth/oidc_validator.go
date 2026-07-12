package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCUser represents a user authenticated via OIDC.
type OIDCUser struct {
	Sub   string // Subject (unique user identifier)
	Email string // User's email address
	Name  string // User's display name
}

// OIDCValidator validates OIDC ID tokens.
type OIDCValidator struct {
	verifier *oidc.IDTokenVerifier
	config   *OIDCConfig
}

// NewOIDCValidator creates a new OIDC token validator.
func NewOIDCValidator(ctx context.Context, config *OIDCConfig) (*OIDCValidator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid OIDC config: %w", err)
	}

	// Initialize OIDC provider
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	// Create ID token verifier with audience validation
	// The verifier will automatically validate:
	// - Token signature using provider's public keys (JWKS)
	// - Issuer claim matches the provider's issuer URL
	// - Audience claim contains the specified ClientID
	// - Token expiration (exp claim)
	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &OIDCValidator{
		verifier: verifier,
		config:   config,
	}, nil
}

// ValidateIDToken validates an OIDC ID token and extracts user information.
func (v *OIDCValidator) ValidateIDToken(ctx context.Context, tokenString string) (*OIDCUser, error) {
	// Verify the ID token
	idToken, err := v.verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return &OIDCUser{
		Sub:   claims.Sub,
		Email: claims.Email,
		Name:  claims.Name,
	}, nil
}

// GetIssuer returns the configured OIDC issuer URL.
func (v *OIDCValidator) GetIssuer() string {
	return v.config.IssuerURL
}
