//go:build validation
// +build validation

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution"
)

// TestREQ07_BBICommandGeneration validates REQ-07: I can run BBI commands.
func TestREQ07_BBICommandGeneration(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-07",
		Description: "I can run BBI commands",
		Category:    CategoryBBIIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "BBI",
				},
			}

			// Create BBI executor
			executor := execution.NewBBIExecutor(cfg)
			require.NotNil(t, executor, "BBI executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			bbiExecutor, ok := executor.(*execution.BBIExecutor)
			require.True(t, ok, "Executor should be a BBIExecutor")

			// Test basic command generation
			binary, args, err := bbiExecutor.BuildCommand("test_model.mod", false, 1, false, nil)

			// Validate results
			assert.NoError(t, err, "Basic BBI command generation should not error")
			assert.Equal(t, "bbi", binary, "Binary should be 'bbi' for BBI")
			assert.Equal(t, []string{"nonmem", "run", "local", "test_model.mod"}, args, "Arguments should include BBI subcommands and input file")

			t.Logf("Generated BBI command: %s %v", binary, args)
		},
	}

	test.Run(t)
}

// TestREQ08_BBIConfigurationOptions validates REQ-08: I can configure BBI execution parameters.
func TestREQ08_BBIConfigurationOptions(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-08",
		Description: "I can configure BBI execution parameters",
		Category:    CategoryBBIIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "BBI",
				},
			}

			// Create BBI executor
			executor := execution.NewBBIExecutor(cfg)
			require.NotNil(t, executor, "BBI executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			bbiExecutor, ok := executor.(*execution.BBIExecutor)
			require.True(t, ok, "Executor should be a BBIExecutor")

			// Test command generation with additional BBI-specific options
			additionalOptions := []string{"--nm_version", "nm75", "--clean_lvl", "2"}
			binary, args, err := bbiExecutor.BuildCommand("test_model.mod", false, 1, false, additionalOptions)

			// Validate results
			assert.NoError(t, err, "BBI command generation with options should not error")
			assert.Equal(t, "bbi", binary, "Binary should be 'bbi' for BBI")

			// Check that base arguments are present
			assert.Contains(t, args, "nonmem", "BBI subcommand should be in arguments")
			assert.Contains(t, args, "run", "BBI run subcommand should be in arguments")
			assert.Contains(t, args, "local", "BBI local execution should be in arguments")
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that additional options are included
			assert.Contains(t, args, "--nm_version", "Additional option --nm_version should be included")
			assert.Contains(t, args, "nm75", "Additional option value nm75 should be included")
			assert.Contains(t, args, "--clean_lvl", "Additional option --clean_lvl should be included")
			assert.Contains(t, args, "2", "Additional option value 2 should be included")

			t.Logf("Generated BBI command with options: %s %v", binary, args)
		},
	}

	test.Run(t)
}

// TestREQ07_BBIParallelExecution validates REQ-07: BBI parallel execution options.
func TestREQ07_BBIParallelExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-07",
		Description: "I can run BBI commands with parallel options",
		Category:    CategoryBBIIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "BBI",
				},
			}

			// Create BBI executor
			executor := execution.NewBBIExecutor(cfg)
			require.NotNil(t, executor, "BBI executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			bbiExecutor, ok := executor.(*execution.BBIExecutor)
			require.True(t, ok, "Executor should be a BBIExecutor")

			// Test parallel execution with 4 cores
			binary, args, err := bbiExecutor.BuildCommand("test_model.mod", true, 4, false, nil)

			// Validate results
			assert.NoError(t, err, "BBI parallel command generation should not error")
			assert.Equal(t, "bbi", binary, "Binary should be 'bbi' for BBI")

			// Check that base arguments are present
			assert.Contains(t, args, "nonmem", "BBI subcommand should be in arguments")
			assert.Contains(t, args, "run", "BBI run subcommand should be in arguments")
			assert.Contains(t, args, "local", "BBI local execution should be in arguments")
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that parallel flags are added
			assert.Contains(t, args, "--parallel", "Parallel flag should be included for parallel execution")
			assert.Contains(t, args, "--threads=4", "Threads option should be included for parallel execution")

			t.Logf("Generated BBI parallel command: %s %v", binary, args)
			t.Logf("Parallel mode: true, Cores: 4")
		},
	}

	test.Run(t)
}

// TestREQ08_BBIGridExecution validates REQ-08: BBI grid execution options.
func TestREQ08_BBIGridExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-08",
		Description: "I can configure BBI execution parameters for grid execution",
		Category:    CategoryBBIIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "BBI",
				},
			}

			// Create BBI executor
			executor := execution.NewBBIExecutor(cfg)
			require.NotNil(t, executor, "BBI executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			bbiExecutor, ok := executor.(*execution.BBIExecutor)
			require.True(t, ok, "Executor should be a BBIExecutor")

			// Test grid execution
			binary, args, err := bbiExecutor.BuildCommand("test_model.mod", false, 1, true, nil)

			// Validate results
			assert.NoError(t, err, "BBI grid command generation should not error")
			assert.Equal(t, "bbi", binary, "Binary should be 'bbi' for BBI")

			// Check that base arguments are present
			assert.Contains(t, args, "nonmem", "BBI subcommand should be in arguments")
			assert.Contains(t, args, "run", "BBI run subcommand should be in arguments")
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that grid execution type is set correctly
			assert.Contains(t, args, "sge", "SGE execution should be in arguments for grid mode")
			assert.NotContains(t, args, "local", "Local execution should NOT be in arguments for grid mode")

			t.Logf("Generated BBI grid command: %s %v", binary, args)
			t.Logf("Grid mode: true")
		},
	}

	test.Run(t)
}

// TestREQ07_BBICombinedOptions validates REQ-07: BBI with combined parallel and grid options.
func TestREQ07_BBICombinedOptions(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-07",
		Description: "I can run BBI commands with combined parallel and grid options",
		Category:    CategoryBBIIntegration,
		TestFunc: func(t *testing.T) {
			// Create test configuration
			cfg := &config.Config{
				Input: config.Input{
					ExecutionMode: "BBI",
				},
			}

			// Create BBI executor
			executor := execution.NewBBIExecutor(cfg)
			require.NotNil(t, executor, "BBI executor should be created")

			// Cast to concrete type to access BuildCommand method for validation testing
			bbiExecutor, ok := executor.(*execution.BBIExecutor)
			require.True(t, ok, "Executor should be a BBIExecutor")

			// Test combined parallel and grid execution with additional options
			additionalOptions := []string{"--output_dir", "/tmp/results"}
			binary, args, err := bbiExecutor.BuildCommand("test_model.mod", true, 8, true, additionalOptions)

			// Validate results
			assert.NoError(t, err, "BBI combined command generation should not error")
			assert.Equal(t, "bbi", binary, "Binary should be 'bbi' for BBI")

			// Check that base arguments are present
			assert.Contains(t, args, "nonmem", "BBI subcommand should be in arguments")
			assert.Contains(t, args, "run", "BBI run subcommand should be in arguments")
			assert.Contains(t, args, "test_model.mod", "Input file should be in arguments")

			// Check that grid execution type is set correctly
			assert.Contains(t, args, "sge", "SGE execution should be in arguments for grid mode")

			// Check that parallel flags are added
			assert.Contains(t, args, "--parallel", "Parallel flag should be included for parallel execution")
			assert.Contains(t, args, "--threads=8", "Threads option should be included for parallel execution")

			// Check that additional options are included
			assert.Contains(t, args, "--output_dir", "Additional option --output_dir should be included")
			assert.Contains(t, args, "/tmp/results", "Additional option value /tmp/results should be included")

			t.Logf("Generated BBI combined command: %s %v", binary, args)
			t.Logf("Parallel mode: true, Cores: 8, Grid mode: true")
		},
	}

	test.Run(t)
}
