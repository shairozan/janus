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
	RedisURL             string   `mapstructure:"redis-url"`
	AWSRegion            string   `mapstructure:"aws-region"`
	AWSAccessKeyID       string   `mapstructure:"aws-access-key-id"`
	AWSSecretAccessKey   string   `mapstructure:"aws-secret-access-key"`
	AWSRoleARN           string   `mapstructure:"aws-role-arn"`
	CognitoUserPoolID    string   `mapstructure:"cognito-user-pool-id"`
	CognitoHostedDomain  string   `mapstructure:"cognito-hosted-domain"`
	MailgunDomain        string   `mapstructure:"mailgun-domain"`
	MailgunAPIKey        string   `mapstructure:"mailgun-api-key"`
	MailgunRegion        string   `mapstructure:"mailgun-region"`
	MailgunFrom          string   `mapstructure:"mailgun-from"`
	StripeAPIKey         string   `mapstructure:"stripe-api-key"`
	StripeWebhookSecret  string   `mapstructure:"stripe-webhook-secret"`
	SignupAllowedOrigins []string `mapstructure:"signup-allowed-origins"`
}

// HasStripe returns true if Stripe billing is configured.
func (c *ServeConfig) HasStripe() bool {
	return c.StripeAPIKey != ""
}

// HasCognitoAdmin returns true if Cognito admin provisioning is configured
// (region + user pool id present).
func (c *ServeConfig) HasCognitoAdmin() bool {
	return c.AWSRegion != "" && c.CognitoUserPoolID != ""
}

// HasMailgun returns true if Mailgun notifications are configured.
func (c *ServeConfig) HasMailgun() bool {
	return c.MailgunDomain != "" && c.MailgunAPIKey != "" && c.MailgunFrom != ""
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

// HasRedis returns true if a Redis URL is configured.
func (c *ServeConfig) HasRedis() bool {
	return c.RedisURL != ""
}

// UnmarshalServeConfig creates ServeConfig from Viper configuration.
func UnmarshalServeConfig() (*ServeConfig, error) {
	cfg := &ServeConfig{}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal serve config: %w", err)
	}

	return cfg, nil
}
