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

// TestREQ34_GracefulErrorHandling validates REQ-34: Graceful error handling.
func TestREQ34_GracefulErrorHandling(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-34",
		Description: "I can handle errors gracefully",
		Category:    CategoryErrorHandling,
		TestFunc: func(t *testing.T) {
			// Test 1: Invalid NONMEM path
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:    "/nonexistent/path/to/nmfe",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "Executor should be created even with invalid path")

			// Attempt execution with invalid path
			ctx := context.Background()
			result, err := executor.Execute(ctx, "/tmp/test.mod", false, 1, false, nil)

			// Should return error without crashing
			assert.Error(t, err, "Should return error for invalid NONMEM path")
			assert.Nil(t, result, "Result should be nil on error")

			// Test 2: Missing model file
			cfg2 := &config.Config{
				Input: config.Input{
					NonmemPath:    "/bin/echo", // Use echo as placeholder
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor2 := execution.NewNONMEMExecutor(cfg2)
			result2, err2 := executor2.Execute(ctx, "/nonexistent/model.mod", false, 1, false, nil)

			// Should handle missing file gracefully
			assert.Error(t, err2, "Should return error for missing model file")
			assert.Nil(t, result2, "Result should be nil on error")

			t.Log("Graceful error handling verified - application handles errors without crashing")
		},
	}

	test.Run(t)
}

// TestREQ35_ErrorMessageClarity validates REQ-35: Error message clarity.
func TestREQ35_ErrorMessageClarity(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-35",
		Description: "I can understand error messages",
		Category:    CategoryErrorHandling,
		TestFunc: func(t *testing.T) {
			// Test that error messages contain actionable information
			tempDir := t.TempDir()
			modelFile := tempDir + "/test.mod"

			// Create the model file so file existence check passes
			err := os.WriteFile(modelFile, []byte("$PROB Test Model"), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:    "/nonexistent/nmfe",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor := execution.NewNONMEMExecutor(cfg)
			ctx := context.Background()
			_, err = executor.Execute(ctx, modelFile, false, 1, false, nil)

			require.Error(t, err, "Should return error for invalid path")

			// Error message should contain relevant information
			errorMsg := err.Error()
			assert.NotEmpty(t, errorMsg, "Error message should not be empty")

			// Error should mention the failure
			assert.True(t,
				strings.Contains(strings.ToLower(errorMsg), "failed") ||
					strings.Contains(strings.ToLower(errorMsg), "error") ||
					strings.Contains(strings.ToLower(errorMsg), "not found") ||
					strings.Contains(strings.ToLower(errorMsg), "no such") ||
					strings.Contains(strings.ToLower(errorMsg), "exist"),
				"Error message should indicate failure: %s", errorMsg)

			t.Logf("Error message clarity verified: %s", errorMsg)
		},
	}

	test.Run(t)
}

// TestREQ36_RecoveryFromFailures validates REQ-36: Recovery from failures.
func TestREQ36_RecoveryFromFailures(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-36",
		Description: "I can recover from failures",
		Category:    CategoryErrorHandling,
		TestFunc: func(t *testing.T) {
			tempDir := t.TempDir()
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("test model"), 0644)
			require.NoError(t, err, "Should be able to create test model")

			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:    "/bin/echo", // Use echo as working executable
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor := execution.NewNONMEMExecutor(cfg)
			ctx := context.Background()

			// First execution: fails with bad path
			cfg.NonmemPath = "/nonexistent/path"
			result1, err1 := executor.Execute(ctx, modelFile, false, 1, false, nil)
			assert.Error(t, err1, "First execution should fail")
			assert.Nil(t, result1, "Result should be nil on failure")

			// Second execution: succeeds after fixing the configuration
			executor2 := execution.NewNONMEMExecutor(&config.Config{
				Input: config.Input{
					NonmemPath:    "/bin/echo",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			})
			result2, err2 := executor2.Execute(ctx, modelFile, false, 1, false, nil)

			// Should recover and execute successfully
			assert.NoError(t, err2, "Second execution should succeed after recovery")
			assert.NotNil(t, result2, "Result should not be nil on success")

			t.Log("Recovery from failures verified - application can continue after error conditions")
		},
	}

	test.Run(t)
}
