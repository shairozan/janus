//go:build validation
// +build validation

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
)

// TestREQ05_PSNExecuteCommandGeneration validates REQ-05: I can run PsN execute commands.
func TestREQ05_PSNExecuteCommandGeneration(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-05",
		Description: "I can run PsN execute commands",
		Category:    CategoryPSNIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "PSN",
				},
			}

			// Create PSN executor
			executor := execution.NewPSNExecutor(cfg)
			require.NotNil(t, executor, "PSN executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			psnExecutor, ok := executor.(*execution.PSNExecutor)
			require.True(t, ok, "Executor should be a PSNExecutor")

			// Test basic command generation
			binary, args, err := psnExecutor.BuildCommand("test_model.mod", false, 1, false, nil)

			// Validate results
			assert.NoError(t, err, "Basic PSN command generation should not error")
			assert.Equal(t, "execute", binary, "Binary should be 'execute' for PSN")
			assert.Equal(t, []string{"test_model.mod"}, args, "Arguments should include input file")

			t.Logf("Generated PSN command: %s %v", binary, args)
		},
	}

	test.Run(t)
}

// TestREQ06_PSNAdditionalOptions validates REQ-06: I can run PsN with additional options.
func TestREQ06_PSNAdditionalOptions(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-06",
		Description: "I can run PsN with additional options",
		Category:    CategoryPSNIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "PSN",
				},
			}

			// Create PSN executor
			executor := execution.NewPSNExecutor(cfg)
			require.NotNil(t, executor, "PSN executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			psnExecutor, ok := executor.(*execution.PSNExecutor)
			require.True(t, ok, "Executor should be a PSNExecutor")

			// Test command generation with additional options
			additionalOptions := []string{"-clean=3", "-min_retries=1"}
			binary, args, err := psnExecutor.BuildCommand("test_model.mod", false, 1, false, additionalOptions)

			// Validate results
			assert.NoError(t, err, "PSN command generation with options should not error")
			assert.Equal(t, "execute", binary, "Binary should be 'execute' for PSN")

			// Check that base arguments are present
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that additional options are included
			assert.Contains(t, args, "-clean=3", "Additional option -clean=3 should be included")
			assert.Contains(t, args, "-min_retries=1", "Additional option -min_retries=1 should be included")

			t.Logf("Generated PSN command with options: %s %v", binary, args)
		},
	}

	test.Run(t)
}

// TestREQ05_PSNParallelExecution validates REQ-05: PSN parallel execution options.
func TestREQ05_PSNParallelExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-05",
		Description: "I can run PsN execute commands with parallel options",
		Category:    CategoryPSNIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "PSN",
				},
			}

			// Create PSN executor
			executor := execution.NewPSNExecutor(cfg)
			require.NotNil(t, executor, "PSN executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			psnExecutor, ok := executor.(*execution.PSNExecutor)
			require.True(t, ok, "Executor should be a PSNExecutor")

			// Test parallel execution with 4 cores
			binary, args, err := psnExecutor.BuildCommand("test_model.mod", true, 4, false, nil)

			// Validate results
			assert.NoError(t, err, "PSN parallel command generation should not error")
			assert.Equal(t, "execute", binary, "Binary should be 'execute' for PSN")

			// Check that base arguments are present
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that parallel flags are added
			assert.Contains(t, args, "-threads=4", "Threads option should be included for parallel execution")

			t.Logf("Generated PSN parallel command: %s %v", binary, args)
			t.Logf("Parallel mode: true, Cores: 4")
		},
	}

	test.Run(t)
}

// TestREQ06_PSNGridExecution validates REQ-06: PSN grid execution options.
func TestREQ06_PSNGridExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-06",
		Description: "I can run PsN with grid execution options",
		Category:    CategoryPSNIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "PSN",
				},
			}

			// Create PSN executor
			executor := execution.NewPSNExecutor(cfg)
			require.NotNil(t, executor, "PSN executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			psnExecutor, ok := executor.(*execution.PSNExecutor)
			require.True(t, ok, "Executor should be a PSNExecutor")

			// Test grid execution
			binary, args, err := psnExecutor.BuildCommand("test_model.mod", false, 1, true, nil)

			// Validate results
			assert.NoError(t, err, "PSN grid command generation should not error")
			assert.Equal(t, "execute", binary, "Binary should be 'execute' for PSN")

			// Check that base arguments are present
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that grid flags are added
			assert.Contains(t, args, "-sge", "SGE flag should be included for grid execution")

			t.Logf("Generated PSN grid command: %s %v", binary, args)
			t.Logf("Grid mode: true")
		},
	}

	test.Run(t)
}
