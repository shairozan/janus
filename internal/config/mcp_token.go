package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/spf13/viper"
)

// MCPTokenBytes is the size of a generated MCP bearer token before base64url
// encoding (32 bytes -> ~43 url-safe characters).
const MCPTokenBytes = 32

// GenerateMCPToken returns a cryptographically random, url-safe bearer token for
// the MCP server. It is the single source of truth for token generation, used by
// both the GUI and the `janus mcp token` command.
func GenerateMCPToken() (string, error) {
	buf := make([]byte, MCPTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// PersistMCPToken writes the token to the config file via viper, creating the
// file if necessary. viper must already have a config file configured (the
// command initializers and the GUI both arrange this).
func PersistMCPToken(token string) error {
	viper.Set("mcp.auth_token", token)

	if err := viper.WriteConfig(); err != nil {
		// WriteConfig fails when no config file exists yet; fall back to creating one.
		if safeErr := viper.SafeWriteConfig(); safeErr != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	return nil
}

// ProvisionMCPToken returns cfg.AuthToken, generating and persisting a fresh
// token when none is set. The returned bool reports whether a new token was
// generated. It mutates cfg.AuthToken in place so callers see the provisioned
// value. A persistence failure is returned as an error, since the whole point of
// provisioning is a token that survives restarts.
func ProvisionMCPToken(cfg *MCPConfig) (token string, generated bool, err error) {
	if cfg.AuthToken != "" {
		return cfg.AuthToken, false, nil
	}

	token, err = GenerateMCPToken()
	if err != nil {
		return "", false, err
	}

	if err := PersistMCPToken(token); err != nil {
		return "", false, fmt.Errorf("generated token but failed to persist it: %w", err)
	}

	cfg.AuthToken = token

	return token, true, nil
}
