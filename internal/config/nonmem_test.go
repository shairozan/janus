package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNONMEMConfig(t *testing.T) {
	// ValidateNONMEMConfig no longer validates file existence at startup.
	// License validation is deferred to model load/execution time.
	// This allows the application to start without requiring a NONMEM license.

	tests := []struct {
		name   string
		config NONMEMConfig
	}{
		{
			name: "empty_path_allowed",
			config: NONMEMConfig{
				License: NONMEMLicenseConfig{
					Path: "",
				},
			},
		},
		{
			name: "any_path_allowed_at_startup",
			config: NONMEMConfig{
				License: NONMEMLicenseConfig{
					Path: "/some/path/to/license.lic",
				},
			},
		},
		{
			name: "nonexistent_path_allowed_at_startup",
			config: NONMEMConfig{
				License: NONMEMLicenseConfig{
					Path: "/nonexistent/path/to/license.lic",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All configurations should pass at startup - validation is deferred
			err := ValidateNONMEMConfig(tt.config)
			assert.NoError(t, err, "ValidateNONMEMConfig should not validate file existence at startup")
		})
	}
}

func TestValidateNONMEMConfigHomeExpansion(t *testing.T) {
	// ValidateNONMEMConfig no longer validates file existence, so home expansion
	// is not tested here. Home expansion is tested via ExpandNONMEMLicensePath.
	// This test verifies that paths with ~ are accepted without error.

	config := NONMEMConfig{
		License: NONMEMLicenseConfig{
			Path: "~/nonmem.lic",
		},
	}

	err := ValidateNONMEMConfig(config)
	assert.NoError(t, err, "paths with ~ should be accepted at startup")
}

func TestExpandNONMEMLicensePath(t *testing.T) {
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
			path:         "/opt/nonmem/license.lic",
			expectedPath: "/opt/nonmem/license.lic",
			expectError:  false,
		},
		{
			name:         "home_expansion",
			path:         "~/nonmem.lic",
			expectedPath: filepath.Join(home, "nonmem.lic"),
			expectError:  false,
		},
		{
			name:         "home_expansion_with_subdir",
			path:         "~/.config/janus/nonmem.lic",
			expectedPath: filepath.Join(home, ".config/janus/nonmem.lic"),
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandNONMEMLicensePath(tt.path)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPath, result)
			}
		})
	}
}

func TestNewConfigWithNONMEMConfig(t *testing.T) {
	// NewConfig no longer validates that license files exist at startup.
	// This allows the application to start without requiring a NONMEM license.
	// License existence is validated when a model is loaded and execution is requested.

	tests := []struct {
		name  string
		input *Input
	}{
		{
			name: "with_nonmem_license_path",
			input: &Input{
				Organization:  "Test Org",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeNONMEM,
				NONMEM: NONMEMConfig{
					License: NONMEMLicenseConfig{
						Path: "/some/path/to/license.lic",
					},
				},
			},
		},
		{
			name: "without_nonmem_license_path",
			input: &Input{
				Organization:  "Test Org",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeNONMEM,
				// NONMEM config not specified - should use defaults at execution time
			},
		},
		{
			name: "nonexistent_license_path_allowed_at_startup",
			input: &Input{
				Organization:  "Test Org",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeNONMEM,
				NONMEM: NONMEMConfig{
					License: NONMEMLicenseConfig{
						Path: "/nonexistent/license.lic",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(tt.input)
			require.NoError(t, err, "NewConfig should not fail due to license path at startup")
			require.NotNil(t, cfg)
		})
	}
}

func TestBackwardCompatibilityHermesLicensePath(t *testing.T) {
	// Test backward compatibility for hermes.license.path migration
	// No need to create actual files since license validation is deferred

	t.Run("migrates hermes.license.path to nonmem.license.path", func(t *testing.T) {
		input := &Input{
			Organization:  "Test Org",
			NonmemPath:    "/opt/NONMEM",
			ExecutionMode: ExecutionModeNONMEM,
			Hermes: HermesConfig{
				License: HermesLicenseConfig{
					Path: "/some/hermes/license.lic",
				},
			},
			// NONMEM.License.Path is empty
		}

		cfg, err := NewConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify migration occurred
		assert.Equal(t, "/some/hermes/license.lic", cfg.NONMEM.License.Path)
	})

	t.Run("new config takes precedence over deprecated", func(t *testing.T) {
		input := &Input{
			Organization:  "Test Org",
			NonmemPath:    "/opt/NONMEM",
			ExecutionMode: ExecutionModeNONMEM,
			NONMEM: NONMEMConfig{
				License: NONMEMLicenseConfig{
					Path: "/new/license.lic",
				},
			},
			Hermes: HermesConfig{
				License: HermesLicenseConfig{
					Path: "/old/deprecated/license.lic",
				},
			},
		}

		cfg, err := NewConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// New config should take precedence
		assert.Equal(t, "/new/license.lic", cfg.NONMEM.License.Path)
	})

	t.Run("hermes mode works without hermes.license.path", func(t *testing.T) {
		input := &Input{
			Organization:  "Test Org",
			NonmemPath:    "/opt/NONMEM",
			ExecutionMode: ExecutionModeHERMES,
			// No license paths specified - will use defaults at execution time
		}

		cfg, err := NewConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)
	})
}

func TestRequiresNONMEMLicense(t *testing.T) {
	tests := []struct {
		mode     string
		expected bool
	}{
		{ExecutionModeNONMEM, true},
		{ExecutionModeHERMES, true},
		{ExecutionModeBBI, true},
		{ExecutionModePSN, true},
		{"SomeOtherMode", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			result := RequiresNONMEMLicense(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateNONMEMLicenseForExecution(t *testing.T) {
	t.Run("configured_path_not_found", func(t *testing.T) {
		cfg := &Config{
			Input: Input{
				ExecutionMode: ExecutionModeNONMEM,
				NONMEM: NONMEMConfig{
					License: NONMEMLicenseConfig{
						Path: "/nonexistent/path/to/license.lic",
					},
				},
			},
		}

		_, err := ValidateNONMEMLicenseForExecution(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NONMEM license file not found at configured path")
	})

	t.Run("configured_path_with_tilde_not_found", func(t *testing.T) {
		cfg := &Config{
			Input: Input{
				ExecutionMode: ExecutionModeNONMEM,
				NONMEM: NONMEMConfig{
					License: NONMEMLicenseConfig{
						Path: "~/nonexistent_license_file.lic",
					},
				},
			},
		}

		_, err := ValidateNONMEMLicenseForExecution(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NONMEM license file not found at configured path")
	})

	t.Run("no_configured_path_falls_back_to_defaults", func(t *testing.T) {
		cfg := &Config{
			Input: Input{
				ExecutionMode: ExecutionModeNONMEM,
				// No license path configured - should try default locations
			},
		}

		// This will fail unless ~/nonmem.lic or ./nonmem.lic exists
		_, err := ValidateNONMEMLicenseForExecution(cfg)
		// We expect an error since the default locations likely don't exist in test env
		if err != nil {
			assert.Contains(t, err.Error(), "NONMEM license file not found")
		}
		// If no error, a license was found at a default location (which is fine)
	})

	t.Run("configured_path_exists", func(t *testing.T) {
		// Create a temporary license file
		tmpDir := t.TempDir()
		licensePath := filepath.Join(tmpDir, "nonmem.lic")
		err := os.WriteFile(licensePath, []byte("test license content"), 0644)
		require.NoError(t, err)

		cfg := &Config{
			Input: Input{
				ExecutionMode: ExecutionModeNONMEM,
				NONMEM: NONMEMConfig{
					License: NONMEMLicenseConfig{
						Path: licensePath,
					},
				},
			},
		}

		result, err := ValidateNONMEMLicenseForExecution(cfg)
		require.NoError(t, err)
		assert.Equal(t, licensePath, result)
	})
}
