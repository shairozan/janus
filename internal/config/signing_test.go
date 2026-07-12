package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandSigningPrivateKeyPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name         string
		path         string
		expectedPath string
		expectError  bool
	}{
		{
			name:         "empty_path",
			path:         "",
			expectedPath: "",
			expectError:  false,
		},
		{
			name:         "absolute_path",
			path:         "/opt/janus/signing-key.pem",
			expectedPath: "/opt/janus/signing-key.pem",
			expectError:  false,
		},
		{
			name:         "home_expansion",
			path:         "~/signing-key.pem",
			expectedPath: filepath.Join(home, "signing-key.pem"),
			expectError:  false,
		},
		{
			name:         "home_expansion_with_subdir",
			path:         "~/.config/janus/signing-key.pem",
			expectedPath: filepath.Join(home, ".config/janus/signing-key.pem"),
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandSigningPrivateKeyPath(tt.path)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPath, result)
			}
		})
	}
}

func TestValidateSigningConfig(t *testing.T) {
	t.Run("empty_config_allowed", func(t *testing.T) {
		cfg := SigningConfig{
			PrivateKeyPath: "",
		}

		err := ValidateSigningConfig(cfg)
		assert.NoError(t, err, "empty signing config should be allowed (optional feature)")
	})

	t.Run("nonexistent_file_fails", func(t *testing.T) {
		cfg := SigningConfig{
			PrivateKeyPath: "/nonexistent/path/to/signing-key.pem",
		}

		err := ValidateSigningConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signing private key file not found")
	})

	t.Run("existing_file_passes", func(t *testing.T) {
		// Create a temporary key file
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "signing-key.pem")
		err := os.WriteFile(keyPath, []byte("test key content"), 0600)
		require.NoError(t, err)

		cfg := SigningConfig{
			PrivateKeyPath: keyPath,
		}

		err = ValidateSigningConfig(cfg)
		assert.NoError(t, err)
	})

	t.Run("tilde_expansion_nonexistent", func(t *testing.T) {
		cfg := SigningConfig{
			PrivateKeyPath: "~/nonexistent_signing_key_12345.pem",
		}

		err := ValidateSigningConfig(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signing private key file not found")
	})
}

func TestSigningConfigInInput(t *testing.T) {
	// Test that SigningConfig is properly included in Input struct
	input := &Input{
		Organization:  "Test Org",
		ExecutionMode: ExecutionModeNONMEM,
		Signing: SigningConfig{
			PrivateKeyPath: "/path/to/key.pem",
		},
	}

	assert.Equal(t, "/path/to/key.pem", input.Signing.PrivateKeyPath)
}
