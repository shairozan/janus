//go:build validation
// +build validation

package validation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

// TestREQ18_ConfigurationFileLoading validates REQ-18: I can load configuration from file.
func TestREQ18_ConfigurationFileLoading(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-18",
		Description: "I can load configuration from file",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Create a temporary config file
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test_config.yml")

			configContent := `
organization: "Test Organization"
default-directory: "/tmp/test"
nonmem-path: "/opt/NONMEM/nm76/run/nmfe76"
execution-mode: "NONMEM"
runlog:
  backend: "json"
  path: "/tmp/runlog.jsonl"
projects: false
validation:
  iq: "/tmp/iq_reports"
  oq: "/tmp/oq_reports"
slurm:
  host: "cluster.example.com"
  port: 22
  timeout: "30s"
`

			// Write test config file
			err := os.WriteFile(configFile, []byte(configContent), 0644)
			require.NoError(t, err, "Should be able to write test config file")

			// Reset viper to ensure clean state
			viper.Reset()

			// Set config file and read
			viper.SetConfigFile(configFile)
			err = viper.ReadInConfig()
			assert.NoError(t, err, "Should be able to read configuration from file")

			// Unmarshal input configuration
			input, err := config.UnmarshalInputFromViper()
			assert.NoError(t, err, "Should be able to unmarshal input from viper")

			// Validate that values were loaded correctly
			assert.Equal(t, "Test Organization", input.Organization, "Organization should be loaded from config")
			assert.Equal(t, "/tmp/test", input.DefaultDirectory, "Default directory should be loaded from config")
			assert.Equal(t, "/opt/NONMEM/nm76/run/nmfe76", input.NonmemPath, "NONMEM path should be loaded from config")
			assert.Equal(t, "NONMEM", input.ExecutionMode, "Execution mode should be loaded from config")
			assert.Equal(t, "json", input.RunLog.Backend, "Audit backend should be loaded from config")
			assert.False(t, input.ProjectsEnable, "Projects enable should be loaded from config")

			// Validate nested structures
			assert.Equal(t, "/tmp/iq_reports", input.Validation.IQ, "Validation IQ path should be loaded from config")
			assert.Equal(t, "/tmp/oq_reports", input.Validation.OQ, "Validation OQ path should be loaded from config")
			assert.Equal(t, "cluster.example.com", input.SLURM.Host, "SLURM host should be loaded from config")
			assert.Equal(t, 22, input.SLURM.Port, "SLURM port should be loaded from config")
			assert.Equal(t, "30s", input.SLURM.Timeout, "SLURM timeout should be loaded from config")

			t.Logf("Configuration loaded successfully from: %s", configFile)
		},
	}

	test.Run(t)
}

// TestREQ19_ConfigurationFlagOverrides validates REQ-19: I can override configuration with command-line flags.
func TestREQ19_ConfigurationFlagOverrides(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-19",
		Description: "I can override configuration with command-line flags",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Create a temporary config file
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test_config.yml")

			configContent := `
organization: "Config Organization"
default-directory: "/config/path"
nonmem-path: "/config/nonmem/path"
execution-mode: "NONMEM"
`

			// Write test config file
			err := os.WriteFile(configFile, []byte(configContent), 0644)
			require.NoError(t, err, "Should be able to write test config file")

			// Reset viper to ensure clean state
			viper.Reset()

			// Set config file and read
			viper.SetConfigFile(configFile)
			err = viper.ReadInConfig()
			require.NoError(t, err, "Should be able to read configuration from file")

			// Simulate command-line flag overrides
			viper.Set("organization", "Flag Organization")
			viper.Set("execution-mode", "BBI")
			viper.Set("nonmem-path", "/flag/nonmem/path")

			// Unmarshal input configuration
			input, err := config.UnmarshalInputFromViper()
			assert.NoError(t, err, "Should be able to unmarshal input from viper")

			// Validate that flag values override config file values
			assert.Equal(t, "Flag Organization", input.Organization, "Flag value should override config file value")
			assert.Equal(t, "BBI", input.ExecutionMode, "Flag value should override config file value")
			assert.Equal(t, "/flag/nonmem/path", input.NonmemPath, "Flag value should override config file value")

			// Validate that non-overridden values remain from config file
			assert.Equal(t, "/config/path", input.DefaultDirectory, "Non-overridden value should remain from config file")

			t.Logf("Flag overrides working correctly: org=%s, mode=%s", input.Organization, input.ExecutionMode)
		},
	}

	test.Run(t)
}

// TestREQ20_DefaultConfigurationValues validates REQ-20: I can use default configuration when no config file exists.
func TestREQ20_DefaultConfigurationValues(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-20",
		Description: "I can use default configuration when no config file exists",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Reset viper to ensure clean state
			viper.Reset()

			// Attempt to read from a non-existent file - should not error
			viper.SetConfigFile("/tmp/non_existent_config.yml")
			err := viper.ReadInConfig()

			// This should error because file doesn't exist, but we should handle it gracefully
			assert.Error(t, err, "Reading non-existent config file should error")

			// Set some basic defaults that would typically be set by cobra flags
			viper.SetDefault("execution-mode", "NONMEM")
			viper.SetDefault("organization", "")
			viper.SetDefault("runlog-enabled", false)
			viper.SetDefault("projects", false)

			// Create Input with defaults
			input, err := config.UnmarshalInputFromViper()
			assert.NoError(t, err, "Should be able to unmarshal input even without config file")

			// Validate default values
			assert.Equal(t, "NONMEM", input.ExecutionMode, "Should use default execution mode")
			assert.Equal(t, "", input.Organization, "Should use default organization")
			assert.Empty(t, input.RunLog.Backend, "Should use default run log backend (empty)")
			assert.False(t, input.ProjectsEnable, "Should use default projects setting")

			// Test that we can create a valid config with defaults
			cfg, err := config.NewConfig(input)
			assert.NoError(t, err, "Should be able to create config with defaults")
			assert.NotNil(t, cfg, "Config should not be nil")

			// Validate that runtime fields are populated
			assert.NotEmpty(t, cfg.Version, "Version should be populated")
			assert.NotEmpty(t, cfg.User, "User should be populated")

			t.Logf("Default configuration created successfully: mode=%s, user=%s", cfg.ExecutionMode, cfg.User)
		},
	}

	test.Run(t)
}

// TestREQ21_ConfigurationValidation validates REQ-21: I can validate configuration values.
func TestREQ21_ConfigurationValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-21",
		Description: "I can validate configuration values",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Test NONMEM execution mode (should always pass)
			err := config.ValidateExecutionMode("NONMEM")
			assert.NoError(t, err, "NONMEM execution mode should always be valid")

			// Test that BBI, PSN, and HERMES modes validate syntax but may fail on tool availability
			// The validation function should recognize them as valid mode names, even if tools aren't available
			validModeNames := []string{"NONMEM", "BBI", "PSN", "HERMES"}
			allValidModes := config.GetValidExecutionModes()
			assert.Equal(t, validModeNames, allValidModes, "Should return all valid execution mode names")

			// Test invalid execution mode
			err = config.ValidateExecutionMode("INVALID_MODE")
			assert.Error(t, err, "Invalid execution mode should error")
			assert.Contains(t, err.Error(), "invalid execution mode", "Error should mention invalid execution mode")

			// Test configuration creation with invalid execution mode
			input := &config.Input{
				ExecutionMode: "INVALID_MODE",
			}

			_, err = config.NewConfig(input)
			assert.Error(t, err, "Creating config with invalid execution mode should error")
			assert.Contains(t, err.Error(), "execution mode validation failed", "Error should mention execution mode validation failure")

			// Test configuration creation with valid execution mode (NONMEM)
			input.ExecutionMode = "NONMEM"
			cfg, err := config.NewConfig(input)
			assert.NoError(t, err, "Creating config with valid execution mode should not error")
			assert.NotNil(t, cfg, "Config should be created successfully")

			t.Logf("Configuration validation working correctly for valid modes: %v", allValidModes)
		},
	}

	test.Run(t)
}

// TestREQ21_ExecutionModeToolValidation validates REQ-21: Execution mode tool validation.
func TestREQ21_ExecutionModeToolValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-21",
		Description: "I can validate execution mode tool requirements",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Test NONMEM mode validation (should always pass as it doesn't require tool validation)
			err := config.ValidateExecutionMode("NONMEM")
			assert.NoError(t, err, "NONMEM mode should always be valid")

			// Test BBI mode validation (will likely fail unless 'bbi' is in PATH)
			err = config.ValidateExecutionMode("BBI")
			if err != nil {
				assert.Contains(t, err.Error(), "bbi", "BBI validation error should mention 'bbi' command")
				t.Logf("BBI validation failed as expected: %v", err)
			} else {
				t.Log("BBI validation passed - 'bbi' command found in PATH")
			}

			// Test PSN mode validation (will likely fail unless 'execute' is in PATH)
			err = config.ValidateExecutionMode("PSN")
			if err != nil {
				assert.Contains(t, err.Error(), "execute", "PSN validation error should mention 'execute' command")
				t.Logf("PSN validation failed as expected: %v", err)
			} else {
				t.Log("PSN validation passed - 'execute' command found in PATH")
			}

			// Test that validation errors are properly formatted
			err = config.ValidateExecutionMode("INVALID")
			assert.Error(t, err, "Invalid mode should error")
			assert.Contains(t, err.Error(), "must be one of", "Error should list valid modes")

			t.Log("Execution mode tool validation working correctly")
		},
	}

	test.Run(t)
}

// TestREQ18_ConfigurationProcessing validates REQ-18: Complete configuration processing.
func TestREQ18_ConfigurationProcessing(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-18",
		Description: "I can process complete configuration from viper",
		Category:    CategoryConfiguration,
		TestFunc: func(t *testing.T) {
			// Reset viper to ensure clean state
			viper.Reset()

			// Set some test configuration values
			viper.Set("execution-mode", "NONMEM")
			viper.Set("organization", "Test Org")
			viper.Set("nonmem-path", "/test/nonmem/path")
			viper.Set("runlog.backend", "json")
			viper.Set("runlog.path", "/tmp/runlog.jsonl")

			// Process configuration using the main Process function
			cfg, err := config.Process()
			assert.NoError(t, err, "Configuration processing should not error")
			assert.NotNil(t, cfg, "Processed config should not be nil")

			// Validate that configuration was processed correctly
			assert.Equal(t, "NONMEM", cfg.ExecutionMode, "Execution mode should be processed correctly")
			assert.Equal(t, "Test Org", cfg.Organization, "Organization should be processed correctly")
			assert.Equal(t, "/test/nonmem/path", cfg.NonmemPath, "NONMEM path should be processed correctly")
			assert.Equal(t, "json", cfg.RunLog.Backend, "Audit backend should be processed correctly")

			// Validate that runtime fields are populated
			assert.NotEmpty(t, cfg.Version, "Version should be populated during processing")
			assert.NotEmpty(t, cfg.User, "User should be populated during processing")

			t.Logf("Configuration processing successful: mode=%s, org=%s, version=%s, user=%s",
				cfg.ExecutionMode, cfg.Organization, cfg.Version, cfg.User)
		},
	}

	test.Run(t)
}
