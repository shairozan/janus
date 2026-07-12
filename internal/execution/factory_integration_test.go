//go:build integration
// +build integration

package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestExecutorFactoryIntegration(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		executionMode string
		expectError   bool
		expectedType  string
	}{
		{
			name: "create_nonmem_executor",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Test Org",
					NonmemPath:    "/opt/NONMEM",
					NonmemBinary:  "nmfe75",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			},
			executionMode: config.ExecutionModeNONMEM,
			expectedType:  "NONMEMExecutor",
		},
		{
			name: "create_bbi_executor",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Test Org",
					NonmemPath:    "/opt/NONMEM",
					ExecutionMode: config.ExecutionModeBBI,
				},
			},
			executionMode: config.ExecutionModeBBI,
			expectedType:  "BBIExecutor",
		},
		{
			name: "create_psn_executor",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Test Org",
					NonmemPath:    "/opt/NONMEM",
					ExecutionMode: config.ExecutionModePSN,
				},
			},
			executionMode: config.ExecutionModePSN,
			expectedType:  "PSNExecutor",
		},
		{
			name: "invalid_execution_mode",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Test Org",
					NonmemPath:    "/opt/NONMEM",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			},
			executionMode: "INVALID_MODE",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewExecutorFactory(tt.config)
			require.NotNil(t, factory)

			executor, err := factory.CreateExecutor(tt.executionMode)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, executor)
				assert.Contains(t, err.Error(), "unsupported execution mode")
				return
			}

			require.NoError(t, err)
			require.NotNil(t, executor)

			// Check executor type
			assert.Equal(t, tt.expectedType, getTypeName(executor))

			// Test that executor can be used (interface compliance)
			ctx := context.Background()
			_, err = executor.Execute(ctx, "test_model.ctl", false, 1, false, nil)
			// We expect an error here since we don't have actual files/binaries,
			// but we're testing that the executor implements the interface correctly
			assert.Error(t, err) // Should error due to missing model file
		})
	}
}

func TestExecutorFactoryConfiguration(t *testing.T) {
	t.Run("factory_inherits_config", func(t *testing.T) {
		cfg := &config.Config{
			Input: config.Input{
				Organization:  "Test Organization",
				NonmemPath:    "/custom/nonmem/path",
				NonmemBinary:  "nmfe74",
				ExecutionMode: config.ExecutionModeNONMEM,
			},
		}

		factory := NewExecutorFactory(cfg)
		executor, err := factory.CreateExecutor(config.ExecutionModeNONMEM)
		require.NoError(t, err)

		// Test that executor has access to the configuration
		nonmemExecutor := executor.(*NONMEMExecutor)
		assert.Equal(t, "/custom/nonmem/path", nonmemExecutor.config.NonmemPath)
		assert.Equal(t, "nmfe74", nonmemExecutor.config.NonmemBinary)
		assert.Equal(t, "Test Organization", nonmemExecutor.config.Organization)
	})
}

func TestExecutorIntegrationWithConfig(t *testing.T) {
	tests := []struct {
		name         string
		setupConfig  func() *config.Config
		testExecutor func(t *testing.T, executor Executor, cfg *config.Config)
	}{
		{
			name: "nonmem_executor_with_slurm_config",
			setupConfig: func() *config.Config {
				return &config.Config{
					Input: config.Input{
						Organization:  "SLURM Org",
						NonmemPath:    "/opt/NONMEM",
						NonmemBinary:  "nmfe75",
						ExecutionMode: config.ExecutionModeNONMEM,
						Scheduler:     "SLURM",
						SLURM: config.SLURMConfig{
							Mode: config.SLURMModeREST,
							REST: config.SLURMRESTConfig{
								SocketPath: "/var/run/slurm/slurmrestd.sock",
								APIVersion: "v0.0.40",
								Timeout:    "30s",
							},
						},
					},
				}
			},
			testExecutor: func(t *testing.T, executor Executor, cfg *config.Config) {
				assert.Equal(t, "SLURM", cfg.Scheduler)
				assert.Equal(t, config.SLURMModeREST, cfg.SLURM.Mode)

				// Test that executor can access SLURM configuration
				nonmemExecutor := executor.(*NONMEMExecutor)
				assert.NotNil(t, nonmemExecutor.config)
				assert.Equal(t, "SLURM", nonmemExecutor.config.Scheduler)
			},
		},
		{
			name: "bbi_executor_with_configuration",
			setupConfig: func() *config.Config {
				return &config.Config{
					Input: config.Input{
						Organization:  "BBI Org",
						NonmemPath:    "/opt/NONMEM",
						ExecutionMode: config.ExecutionModeBBI,
					},
				}
			},
			testExecutor: func(t *testing.T, executor Executor, cfg *config.Config) {
				assert.Equal(t, config.ExecutionModeBBI, cfg.ExecutionMode)

				// Test that BBI executor has configuration
				bbiExecutor := executor.(*BBIExecutor)
				assert.NotNil(t, bbiExecutor.config)
				assert.Equal(t, "BBI Org", bbiExecutor.config.Organization)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			require.NotNil(t, cfg)

			factory := NewExecutorFactory(cfg)
			executor, err := factory.CreateExecutor(cfg.ExecutionMode)
			require.NoError(t, err)
			require.NotNil(t, executor)

			tt.testExecutor(t, executor, cfg)
		})
	}
}

func TestExecutorCommandGeneration(t *testing.T) {
	// Test that executors can generate commands without actually executing them
	cfg := &config.Config{
		Input: config.Input{
			Organization:     "Command Test Org",
			NonmemPath:       "/opt/NONMEM",
			NonmemBinary:     "nmfe75",
			ExecutionMode:    config.ExecutionModeNONMEM,
			DefaultDirectory: "/tmp/test",
		},
	}

	factory := NewExecutorFactory(cfg)

	tests := []struct {
		name           string
		executionMode  string
		isParallel     bool
		cores          int
		expectedInPath string
		expectError    bool
	}{
		{
			name:           "nonmem_synchronous_command",
			executionMode:  config.ExecutionModeNONMEM,
			isParallel:     false,
			cores:          1,
			expectedInPath: "/opt/NONMEM",
		},
		{
			name:           "nonmem_parallel_command",
			executionMode:  config.ExecutionModeNONMEM,
			isParallel:     true,
			cores:          4,
			expectedInPath: "/opt/NONMEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := factory.CreateExecutor(tt.executionMode)
			require.NoError(t, err)

			// For NONMEM executor, we can test command building directly
			if nonmemExecutor, ok := executor.(*NONMEMExecutor); ok {
				binary, args, err := nonmemExecutor.buildNONMEMCommand("test_model.ctl", tt.isParallel, tt.cores, false, nil)

				if tt.expectError {
					assert.Error(t, err)
					return
				}

				require.NoError(t, err)
				assert.Contains(t, binary, tt.expectedInPath)
				assert.NotEmpty(t, args)

				// Verify parallel options are reflected in command
				if tt.isParallel {
					// For parallel execution, expect .pnm file reference
					foundPnmRef := false
					for _, arg := range args {
						if arg == "test_model.pnm" {
							foundPnmRef = true
							break
						}
					}
					assert.True(t, foundPnmRef, "Parallel execution should reference .pnm file")
				}
			}
		})
	}
}

func TestExecutorValidation(t *testing.T) {
	// Test that executors validate configuration properly
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_nonmem_config",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Valid Org",
					NonmemPath:    "/opt/NONMEM",
					NonmemBinary:  "nmfe75",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			},
		},
		{
			name: "missing_nonmem_path",
			config: &config.Config{
				Input: config.Input{
					Organization:  "Test Org",
					NonmemPath:    "",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			},
			expectError: true,
			errorMsg:    "failed to execute NONMEM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewExecutorFactory(tt.config)
			executor, err := factory.CreateExecutor(tt.config.ExecutionMode)

			if tt.expectError {
				// Check if validation occurs during execution
				ctx := context.Background()
				_, execErr := executor.Execute(ctx, "test.ctl", false, 1, false, nil)
				assert.Error(t, execErr)
				assert.Contains(t, execErr.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, executor)
			}
		})
	}
}

// Helper function to get type name for assertions.
func getTypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	switch v.(type) {
	case *NONMEMExecutor:
		return "NONMEMExecutor"
	case *BBIExecutor:
		return "BBIExecutor"
	case *PSNExecutor:
		return "PSNExecutor"
	default:
		return "unknown"
	}
}
