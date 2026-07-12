package auth

import "fmt"

// OIDCConfig holds the configuration for OIDC authentication.
type OIDCConfig struct {
	// IssuerURL is the OIDC provider's issuer URL (e.g., https://dev-xxx.us.auth0.com)
	IssuerURL string

	// ClientID is the OAuth2 client ID for this application
	ClientID string

	// Audience is the expected audience for ID tokens (optional, defaults to ClientID)
	Audience string
}

// Validate checks if the OIDC configuration is valid.
func (c *OIDCConfig) Validate() error {
	if c.IssuerURL == "" {
		return fmt.Errorf("OIDC issuer URL is required")
	}
	if c.ClientID == "" {
		return fmt.Errorf("OIDC client ID is required")
	}

	// Default audience to client ID if not specified
	if c.Audience == "" {
		c.Audience = c.ClientID
	}

	return nil
}

// IsConfigured returns true if OIDC configuration is present.
func (c *OIDCConfig) IsConfigured() bool {
	return c.IssuerURL != "" && c.ClientID != ""
}
