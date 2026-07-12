package commands

import (
	"fmt"

	"github.com/spf13/viper"
)

// ServeConfig represents the configuration for the serve command.
type ServeConfig struct {
	DatabaseURL          string   `mapstructure:"database-url"`
	Port                 int      `mapstructure:"port"`
	Host                 string   `mapstructure:"host"`
	EncryptionKey        string   `mapstructure:"encryption-key"`
	OIDCIssuer           string   `mapstructure:"oidc-issuer"`
	OIDCClientID         string   `mapstructure:"oidc-client-id"`
	OIDCAllowedDomains   []string `mapstructure:"oidc-allowed-domains"`
}

// Validate checks if the ServeConfig is valid.
func (c *ServeConfig) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required (use --database-url or LICENSING_DATABASE_URL)")
	}

	if c.EncryptionKey == "" {
		return fmt.Errorf("encryption key is required (use --encryption-key or LICENSING_ENCRYPTION_KEY)")
	}

	if len(c.EncryptionKey) != 32 {
		return fmt.Errorf("encryption key must be exactly 32 bytes, got %d", len(c.EncryptionKey))
	}

	return nil
}

// HasOIDC returns true if OIDC configuration is present.
func (c *ServeConfig) HasOIDC() bool {
	return c.OIDCIssuer != "" && c.OIDCClientID != ""
}

// UnmarshalServeConfig creates ServeConfig from Viper configuration.
func UnmarshalServeConfig() (*ServeConfig, error) {
	cfg := &ServeConfig{}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal serve config: %w", err)
	}

	return cfg, nil
}
