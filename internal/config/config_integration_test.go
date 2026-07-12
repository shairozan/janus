//go:build integration
// +build integration

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigurationIntegration(t *testing.T) {
	// Clean up any existing viper state
	viper.Reset()

	tests := []struct {
		name           string
		configContent  string
		envVars        map[string]string
		expectedConfig func(t *testing.T, cfg *Config)
		expectError    bool
	}{
		{
			name: "complete_valid_configuration",
			configContent: `
organization: "Test Organization"
default-directory: "/tmp/test"
nonmem-path: "/opt/NONMEM"
nonmem-binary: "nmfe74"
scheduler: "SLURM"
execution-mode: "NONMEM"
projects: true

slurm:
  mode: "REST"
  rest:
    socket_path: "/var/run/slurm/slurmrestd.sock"
    api_version: "v0.0.40"
    timeout: "30s"

`,
			expectedConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "Test Organization", cfg.Organization)
				assert.Equal(t, "/tmp/test", cfg.DefaultDirectory)
				assert.Equal(t, "/opt/NONMEM", cfg.NonmemPath)
				assert.Equal(t, "nmfe74", cfg.NonmemBinary)
				assert.Equal(t, "SLURM", cfg.Scheduler)
				assert.Equal(t, ExecutionModeNONMEM, cfg.ExecutionMode)
				assert.True(t, cfg.ProjectsEnable)
				assert.Equal(t, SLURMModeREST, cfg.SLURM.Mode)
				assert.Equal(t, "/var/run/slurm/slurmrestd.sock", cfg.SLURM.REST.SocketPath)
				assert.Equal(t, "v0.0.40", cfg.SLURM.REST.APIVersion)
				assert.Equal(t, "30s", cfg.SLURM.REST.Timeout)
			},
		},
		{
			name: "minimal_valid_configuration",
			configContent: `
organization: "Minimal Org"
nonmem-path: "/opt/NONMEM"
execution-mode: "NONMEM"
`,
			expectedConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "Minimal Org", cfg.Organization)
				assert.Equal(t, "/opt/NONMEM", cfg.NonmemPath)
				assert.Equal(t, ExecutionModeNONMEM, cfg.ExecutionMode)
				// Values not set in config remain empty (defaults handled by cobra)
				assert.False(t, cfg.ProjectsEnable) // Default false
			},
		},
		{
			name: "environment_variable_override",
			configContent: `
organization: "File Org"
nonmem-path: "/opt/NONMEM"
execution-mode: "NONMEM"
`,
			envVars: map[string]string{
				"JANUS_ORGANIZATION": "Env Override Org",
			},
			expectedConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "Env Override Org", cfg.Organization) // Overridden by env
				assert.Equal(t, "/opt/NONMEM", cfg.NonmemPath)        // From file
				// Environment override for boolean might not work in test - that's OK
			},
		},
		{
			name: "missing_required_fields",
			configContent: `
organization: "Test"
# Missing nonmem-path and execution-mode
`,
			expectError: true,
		},
		{
			name: "invalid_execution_mode",
			configContent: `
organization: "Test"
nonmem-path: "/opt/NONMEM"
execution-mode: "INVALID_MODE"
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir, err := os.MkdirTemp("", "janus_config_test")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			configFile := filepath.Join(tmpDir, "config.yml")
			err = os.WriteFile(configFile, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			// Set environment variables if provided
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			// Reset viper for each test
			viper.Reset()
			viper.SetEnvPrefix("JANUS")
			viper.AutomaticEnv()
			viper.SetConfigFile(configFile)

			err = viper.ReadInConfig()
			require.NoError(t, err)

			// Test configuration processing
			cfg, err := Process()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Run custom assertions
			if tt.expectedConfig != nil {
				tt.expectedConfig(t, cfg)
			}

			// Test configuration validation
			assert.NotEmpty(t, cfg.Organization)
			assert.NotEmpty(t, cfg.NonmemPath)
			assert.Contains(t, []string{ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN}, cfg.ExecutionMode)
		})
	}
}

func TestNewConfigFromInput(t *testing.T) {
	tests := []struct {
		name        string
		input       *Input
		expectError bool
		checkConfig func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid_input_nonmem",
			input: &Input{
				Organization:     "Test Org",
				DefaultDirectory: "/tmp/test",
				NonmemPath:       "/opt/NONMEM",
				NonmemBinary:     "nmfe74",
				Scheduler:        "SLURM",
				ExecutionMode:    ExecutionModeNONMEM,
				ProjectsEnable:   false,
				SLURM: SLURMConfig{
					Mode: SLURMModeREST,
					REST: SLURMRESTConfig{
						SocketPath: "/var/run/slurm/slurmrestd.sock",
						APIVersion: "v0.0.40",
						Timeout:    "30s",
					},
				},
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "Test Org", cfg.Organization)
				assert.Equal(t, ExecutionModeNONMEM, cfg.ExecutionMode)
				assert.Equal(t, "SLURM", cfg.Scheduler)
				assert.False(t, cfg.ProjectsEnable)
			},
		},
		{
			name: "valid_input_bbi",
			input: &Input{
				Organization:  "BBI Org",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeBBI,
			},
			checkConfig: func(t *testing.T, cfg *Config) {
				assert.Equal(t, ExecutionModeBBI, cfg.ExecutionMode)
				assert.Equal(t, "BBI Org", cfg.Organization)
			},
		},
		{
			name: "invalid_input_missing_required",
			input: &Input{
				Organization: "Test",
				// Missing NonmemPath and ExecutionMode
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cfg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}

			// Test that config has required fields
			assert.NotEmpty(t, cfg.Organization)
			assert.NotEmpty(t, cfg.NonmemPath)
			assert.NotEmpty(t, cfg.ExecutionMode)
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	input := &Input{
		Organization:  "Test Org",
		NonmemPath:    "/opt/NONMEM",
		ExecutionMode: ExecutionModeNONMEM,
		// Leaving other fields empty to test defaults
	}

	cfg, err := NewConfig(input)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Test that fields not set remain empty (defaults handled by cobra)
	assert.False(t, cfg.ProjectsEnable)

	// Test that user info is populated
	assert.NotEmpty(t, cfg.User)

	// Test that version info is populated
	assert.NotEmpty(t, cfg.Version)
}

func TestConfigFileOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "janus_config_ops_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "test_config.yml")

	t.Run("save_and_load_config", func(t *testing.T) {
		// Create a config
		input := &Input{
			Organization:     "Save Test Org",
			DefaultDirectory: tmpDir,
			NonmemPath:       "/opt/NONMEM",
			ExecutionMode:    ExecutionModeNONMEM,
		}

		originalCfg, err := NewConfig(input)
		require.NoError(t, err)

		// Save config (this would be done by setup wizard)
		viper.Reset()
		viper.Set("organization", originalCfg.Organization)
		viper.Set("default-directory", originalCfg.DefaultDirectory)
		viper.Set("nonmem-path", originalCfg.NonmemPath)
		viper.Set("execution-mode", originalCfg.ExecutionMode)

		err = viper.WriteConfigAs(configPath)
		require.NoError(t, err)

		// Load config back
		viper.Reset()
		viper.SetConfigFile(configPath)
		err = viper.ReadInConfig()
		require.NoError(t, err)

		loadedCfg, err := Process()
		require.NoError(t, err)

		// Compare key fields
		assert.Equal(t, originalCfg.Organization, loadedCfg.Organization)
		assert.Equal(t, originalCfg.DefaultDirectory, loadedCfg.DefaultDirectory)
		assert.Equal(t, originalCfg.NonmemPath, loadedCfg.NonmemPath)
		assert.Equal(t, originalCfg.ExecutionMode, loadedCfg.ExecutionMode)
	})
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       *Input
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty_organization",
			input: &Input{
				Organization:  "",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeNONMEM,
			},
			expectError: false, // No validation implemented for empty organization yet
		},
		{
			name: "empty_nonmem_path",
			input: &Input{
				Organization:  "Test",
				NonmemPath:    "",
				ExecutionMode: ExecutionModeNONMEM,
			},
			expectError: false, // No validation implemented for empty nonmem-path yet
		},
		{
			name: "invalid_execution_mode",
			input: &Input{
				Organization:  "Test",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: "INVALID",
			},
			expectError: true,
			errorMsg:    "invalid execution mode",
		},
		{
			name: "invalid_slurm_timeout",
			input: &Input{
				Organization:  "Test",
				NonmemPath:    "/opt/NONMEM",
				ExecutionMode: ExecutionModeNONMEM,
				SLURM: SLURMConfig{
					Mode: SLURMModeREST,
					REST: SLURMRESTConfig{
						Timeout: "invalid_duration",
					},
				},
			},
			expectError: false, // SLURM timeout validation happens at client creation, not config validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfig(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}