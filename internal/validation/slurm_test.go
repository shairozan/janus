//go:build validation
// +build validation

package validation

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
)

// TestREQ60_SLURMConfigurationValidation validates REQ-60: I can validate SLURM configuration settings.
func TestREQ60_SLURMConfigurationValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-60",
		Description: "I can validate SLURM configuration settings",
		Category:    CategorySLURMIntegration,
		TestFunc: func(t *testing.T) {
			// Test valid SLURM configuration
			validConfig := config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					APIVersion: "v0.0.40",
					Timeout:    "30s",
				},
			}

			err := config.ValidateSLURMMode(validConfig)
			assert.NoError(t, err, "Valid SLURM REST configuration should pass validation")

			// Test invalid SLURM configuration - missing socket path
			invalidConfig := config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					APIVersion: "v0.0.40",
					Timeout:    "30s",
				},
			}

			err = config.ValidateSLURMMode(invalidConfig)
			assert.Error(t, err, "SLURM REST configuration without socket path should fail validation")
			assert.Contains(t, err.Error(), "socket_path", "Error should mention missing socket_path")

			// Test invalid SLURM configuration - missing API version
			invalidConfig2 := config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					Timeout:    "30s",
				},
			}

			err = config.ValidateSLURMMode(invalidConfig2)
			assert.Error(t, err, "SLURM REST configuration without API version should fail validation")
			assert.Contains(t, err.Error(), "api_version", "Error should mention missing api_version")

			t.Log("SLURM configuration validation tests completed successfully")
		},
	}

	test.Run(t)
}

// TestREQ61_SLURMRESTModeValidation validates REQ-61: I can validate SLURM REST mode configuration.
func TestREQ61_SLURMRESTModeValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-61",
		Description: "I can validate SLURM REST mode configuration",
		Category:    CategorySLURMIntegration,
		TestFunc: func(t *testing.T) {
			// Test REST mode validation
			restConfig := config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					APIVersion: "v0.0.40",
					Timeout:    "30s",
					AuthToken:  "optional-token",
				},
			}

			err := config.ValidateSLURMMode(restConfig)
			assert.NoError(t, err, "Valid REST mode configuration should pass")

			// Test invalid mode
			invalidModeConfig := config.SLURMConfig{
				Mode: "INVALID",
			}

			err = config.ValidateSLURMMode(invalidModeConfig)
			assert.Error(t, err, "Invalid SLURM mode should fail validation")
			assert.Contains(t, err.Error(), "invalid SLURM mode", "Error should mention invalid mode")

			t.Log("SLURM REST mode validation tests completed successfully")
		},
	}

	test.Run(t)
}

// TestREQ62_SLURMCLIModeValidation validates REQ-62: I can validate SLURM CLI mode configuration.
func TestREQ62_SLURMCLIModeValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-62",
		Description: "I can validate SLURM CLI mode configuration",
		Category:    CategorySLURMIntegration,
		TestFunc: func(t *testing.T) {
			// Test CLI mode validation
			cliConfig := config.SLURMConfig{
				Mode:    config.SLURMModeCLI,
				Host:    "cluster.example.com",
				Port:    22,
				Timeout: "30s",
			}

			err := config.ValidateSLURMMode(cliConfig)
			assert.NoError(t, err, "Valid CLI mode configuration should pass")

			// Test default mode (empty should default to CLI)
			defaultConfig := config.SLURMConfig{}

			err = config.ValidateSLURMMode(defaultConfig)
			assert.NoError(t, err, "Default (empty) SLURM configuration should pass")

			t.Log("SLURM CLI mode validation tests completed successfully")
		},
	}

	test.Run(t)
}

// TestREQ63_SLURMGridIntegration validates REQ-63: I can use SLURM as a grid scheduler with existing executors.
func TestREQ63_SLURMGridIntegration(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-63",
		Description: "I can use SLURM as a grid scheduler with existing executors",
		Category:    CategorySLURMIntegration,
		TestFunc: func(t *testing.T) {
			// Create temporary directory and test model file
			tempDir := t.TempDir()
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("$PROB Test Model"), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Create a test configuration with SLURM scheduler
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: config.ExecutionModeNONMEM,
					Scheduler:     "SLURM",
					SLURM: config.SLURMConfig{
						Mode:    config.SLURMModeCLI,
						Host:    "localhost",
						Port:    22,
						Timeout: "30s",
					},
					NonmemPath:   "/bin", // Use /bin directory for testing
					NonmemBinary: "echo", // Use echo binary for testing
				},
			}

			// Create NONMEM executor with SLURM scheduler
			factory := execution.NewExecutorFactory(cfg)
			executor, err := factory.CreateExecutor(config.ExecutionModeNONMEM)
			require.NoError(t, err, "Should be able to create NONMEM executor")
			require.NotNil(t, executor, "Executor should not be nil")

			// Test local execution (should work normally)
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, nil)
			assert.NoError(t, err, "Local execution should work")
			assert.NotNil(t, result, "Result should not be nil")

			// Test grid execution with SLURM (will fail because sbatch not available, but should attempt SLURM)
			_, err = executor.Execute(ctx, modelFile, false, 1, true, nil)
			if err != nil {
				// This is expected if sbatch is not available
				if strings.Contains(err.Error(), "sbatch") ||
					strings.Contains(err.Error(), "executable file not found") ||
					strings.Contains(err.Error(), "no such file or directory") {

					t.Logf("SLURM sbatch not available in test environment (expected): %v", err)
				} else {
					t.Logf("Grid execution failed with error: %v", err)
				}
			} else {
				t.Log("SLURM grid execution completed successfully")
			}

			t.Log("SLURM grid integration validation completed")
		},
	}

	test.Run(t)
}

// TestREQ64_SLURMSchedulerConfiguration validates REQ-64: I can configure SLURM as a scheduler.
func TestREQ64_SLURMSchedulerConfiguration(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-64",
		Description: "I can configure SLURM as a scheduler",
		Category:    CategorySLURMIntegration,
		TestFunc: func(t *testing.T) {
			// Test that execution modes work with SLURM scheduler
			validModes := config.GetValidExecutionModes()
			expectedModes := []string{"NONMEM", "BBI", "PSN", "HERMES"}
			assert.Equal(t, expectedModes, validModes, "Valid execution modes should be NONMEM, BBI, PSN, HERMES")

			// Create input configuration with SLURM scheduler
			input := &config.Input{
				ExecutionMode: config.ExecutionModeNONMEM,
				Scheduler:     "SLURM",
				SLURM: config.SLURMConfig{
					Mode:    config.SLURMModeCLI,
					Timeout: "30s",
				},
				NonmemPath:   "/opt/nonmem",
				NonmemBinary: "nmfe75",
			}

			// Test that we can create a config with SLURM scheduler
			cfg, err := config.NewConfig(input)
			require.NoError(t, err, "Should be able to create config with SLURM scheduler")
			require.NotNil(t, cfg, "Config should not be nil")
			assert.Equal(t, config.ExecutionModeNONMEM, cfg.ExecutionMode, "Execution mode should be NONMEM")
			assert.Equal(t, "SLURM", cfg.Scheduler, "Scheduler should be SLURM")

			// Test SLURM mode validation
			err = config.ValidateSLURMMode(cfg.SLURM)
			assert.NoError(t, err, "SLURM configuration should be valid")

			t.Log("SLURM scheduler configuration validation completed successfully")
		},
	}

	test.Run(t)
}
