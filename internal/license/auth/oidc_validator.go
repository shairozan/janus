package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCUser represents a user authenticated via OIDC.
type OIDCUser struct {
	Sub    string    // Subject (unique user identifier)
	Email  string    // User's email address (populated by FetchUserInfo, not ValidateToken)
	Name   string    // User's display name (populated by FetchUserInfo)
	Groups []string  // Cognito groups (from the cognito:groups claim) — e.g. customer_admin
	Expiry time.Time // Token expiry (from access token exp claim)
}

// OIDCValidator validates OIDC tokens and fetches user info from the provider.
type OIDCValidator struct {
	verifier *oidc.IDTokenVerifier
	provider *oidc.Provider
	config   *OIDCConfig
}

// NewOIDCValidator creates a new OIDC token validator.
func NewOIDCValidator(ctx context.Context, config *OIDCConfig) (*OIDCValidator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid OIDC config: %w", err)
	}

	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	// Cognito access tokens use `client_id` rather than `aud` for the app
	// identifier. SkipClientIDCheck disables go-oidc's audience check; we
	// verify client_id and token_use manually in ValidateToken below.
	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return &OIDCValidator{
		verifier: verifier,
		provider: provider,
		config:   config,
	}, nil
}

// ValidateToken validates a Cognito access token and returns the subject and
// expiry. It checks: JWKS signature, issuer, client_id == configured ClientID,
// token_use == "access", and expiry.
// Email and Name are not populated here — access tokens do not carry them.
// Use FetchUserInfo to obtain those fields.
func (v *OIDCValidator) ValidateToken(ctx context.Context, tokenString string) (*OIDCUser, error) {
	token, err := v.verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}

	var claims struct {
		Sub      string   `json:"sub"`
		ClientID string   `json:"client_id"`
		TokenUse string   `json:"token_use"`
		Groups   []string `json:"cognito:groups"`
	}

	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract token claims: %w", err)
	}

	if claims.ClientID != v.config.ClientID {
		return nil, fmt.Errorf("token client_id %q does not match expected %q", claims.ClientID, v.config.ClientID)
	}

	if claims.TokenUse != "access" {
		return nil, fmt.Errorf("token_use is %q, expected \"access\"", claims.TokenUse)
	}

	return &OIDCUser{
		Sub:    claims.Sub,
		Groups: claims.Groups,
		Expiry: token.Expiry,
	}, nil
}

// FetchUserInfo calls the OIDC provider's userinfo endpoint using the given
// access token and returns the user's sub, email, and name.
func (v *OIDCValidator) FetchUserInfo(ctx context.Context, accessToken string) (*OIDCUser, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})

	userInfo, err := v.provider.UserInfo(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}

	var claims struct {
		Name string `json:"name"`
	}

	if err := userInfo.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract userinfo claims: %w", err)
	}

	return &OIDCUser{
		Sub:   userInfo.Subject,
		Email: userInfo.Email,
		Name:  claims.Name,
	}, nil
}

// GetIssuer returns the configured OIDC issuer URL.
func (v *OIDCValidator) GetIssuer() string {
	return v.config.IssuerURL
}
