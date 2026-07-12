package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMCPConfig(t *testing.T) {
	tests := []struct {
		name         string
		input        MCPConfig
		expectError  bool
		expectedHost string
		expectedPort int
	}{
		{
			name:        "disabled is a no-op",
			input:       MCPConfig{Enabled: false, Host: "0.0.0.0", Port: 1},
			expectError: false,
			// Disabled config is not normalized, so values pass through untouched.
			expectedHost: "0.0.0.0",
			expectedPort: 1,
		},
		{
			name:         "enabled defaults host and port",
			input:        MCPConfig{Enabled: true},
			expectError:  false,
			expectedHost: DefaultMCPHost,
			expectedPort: DefaultMCPPort,
		},
		{
			name:         "loopback 127.0.0.1 accepted",
			input:        MCPConfig{Enabled: true, Host: "127.0.0.1", Port: 9000},
			expectError:  false,
			expectedHost: "127.0.0.1",
			expectedPort: 9000,
		},
		{
			name:         "loopback localhost accepted",
			input:        MCPConfig{Enabled: true, Host: "localhost", Port: 9000},
			expectError:  false,
			expectedHost: "localhost",
			expectedPort: 9000,
		},
		{
			name:         "loopback ::1 accepted",
			input:        MCPConfig{Enabled: true, Host: "::1", Port: 9000},
			expectError:  false,
			expectedHost: "::1",
			expectedPort: 9000,
		},
		{
			name:        "non-loopback host rejected",
			input:       MCPConfig{Enabled: true, Host: "0.0.0.0", Port: 8731},
			expectError: true,
		},
		{
			name:        "public host rejected",
			input:       MCPConfig{Enabled: true, Host: "192.168.1.5", Port: 8731},
			expectError: true,
		},
		{
			name:        "port below range rejected",
			input:       MCPConfig{Enabled: true, Host: "127.0.0.1", Port: 1023},
			expectError: true,
		},
		{
			name:        "port above range rejected",
			input:       MCPConfig{Enabled: true, Host: "127.0.0.1", Port: 70000},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.input
			err := ValidateMCPConfig(&cfg)

			if tc.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedHost, cfg.Host)
			assert.Equal(t, tc.expectedPort, cfg.Port)
		})
	}
}

func TestValidateMCPConfigNil(t *testing.T) {
	assert.NoError(t, ValidateMCPConfig(nil))
}

func TestGenerateMCPTokenIsRandomAndURLSafe(t *testing.T) {
	first, err := GenerateMCPToken()
	require.NoError(t, err)

	second, err := GenerateMCPToken()
	require.NoError(t, err)

	assert.NotEmpty(t, first)
	assert.NotEqual(t, first, second, "tokens must be random")

	decoded, err := base64.RawURLEncoding.DecodeString(first)
	require.NoError(t, err)
	assert.Len(t, decoded, MCPTokenBytes)
}

// withTempViperConfig points viper at a fresh temp config file for the duration
// of a test, restoring viper afterward so tests stay isolated.
func withTempViperConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte("mcp:\n  enabled: true\n"), 0600))

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(path)
	require.NoError(t, viper.ReadInConfig())

	return path
}

func TestProvisionMCPTokenGeneratesAndPersists(t *testing.T) {
	path := withTempViperConfig(t)

	cfg := &MCPConfig{Enabled: true}
	token, generated, err := ProvisionMCPToken(cfg)
	require.NoError(t, err)
	assert.True(t, generated)
	assert.NotEmpty(t, token)
	assert.Equal(t, token, cfg.AuthToken, "cfg must be updated in place")

	// The token must have been written to the config file.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), token)
}

func TestProvisionMCPTokenReturnsExisting(t *testing.T) {
	withTempViperConfig(t)

	cfg := &MCPConfig{Enabled: true, AuthToken: "preset"}
	token, generated, err := ProvisionMCPToken(cfg)
	require.NoError(t, err)
	assert.False(t, generated)
	assert.Equal(t, "preset", token)
}
