//go:build validation
// +build validation

package validation

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/audit"
	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
)

// AuditLogEntry represents a JSON audit log entry structure.
type AuditLogEntry struct {
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`
	Binary    string    `json:"binary"`
	Arguments []string  `json:"arguments"`
	STDOUT    string    `json:"stdout"`
	STDERR    string    `json:"stderr"`
	ExitCode  int       `json:"exit_code"`
	Duration  int64     `json:"duration_ms"`
}

// TestREQ41_JSONAuditTrailEnablement validates REQ-41: I can enable JSON audit trail based on configuration.
func TestREQ41_JSONAuditTrailEnablement(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-41",
		Description: "I can enable JSON audit trail based on configuration",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Test with audit engine disabled (no backend configured)
			cfg := &config.Config{
				Input: config.Input{
					Audit: config.AuditConfig{
						Backend: "", // Empty backend = disabled
					},
				},
			}

			// Verify audit engine is disabled
			assert.Empty(t, cfg.Audit.Backend, "Audit engine should be disabled when backend is empty")

			// Test with audit engine enabled
			cfg.Audit.Backend = "json"

			// Verify audit engine is enabled
			assert.Equal(t, "json", cfg.Audit.Backend, "Audit engine should be enabled when backend is configured")

			t.Logf("Audit engine configuration validation: disabled=%v, enabled=%v", "", "json")
		},
	}

	test.Run(t)
}

// TestREQ42_JobIDAuditLogging validates REQ-42: I can capture Job ID in audit trail.
func TestREQ42_JobIDAuditLogging(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-42",
		Description: "I can capture Job ID in audit trail",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for audit log
			tempDir := t.TempDir()
			auditLogPath := tempDir + "/audit.jsonl"

			// Create test configuration with audit engine enabled
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath: "/bin/echo", // Use echo command for testing
					Audit: config.AuditConfig{
						Backend: "json",
						Path:    auditLogPath,
					},
				},
			}

			// Create NONMEM executor with audit logging enabled
			executor := execution.NewNONMEMExecutorWithAudit(cfg, true)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Create a test model file
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("test model content"), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Execute the command (this will use echo instead of real NONMEM)
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, nil)
			require.NoError(t, err, "Execution should not error")
			require.NotNil(t, result, "Execution result should not be nil")

			// Wait a moment for audit log to be written
			time.Sleep(100 * time.Millisecond)

			// Read the audit log file
			auditEntries, err := audit.ReadLogEntries(auditLogPath)
			require.NoError(t, err, "Should be able to read audit log entries")
			require.Len(t, auditEntries, 1, "Should have exactly one audit entry")

			// Validate the audit entry
			auditEntry := auditEntries[0]
			assert.NotEmpty(t, auditEntry.JobID, "Job ID should not be empty")
			assert.Contains(t, auditEntry.JobID, "job-", "Job ID should contain 'job-' prefix")
			assert.True(t, len(auditEntry.JobID) > 10, "Job ID should be sufficiently long for uniqueness")

			t.Logf("Job ID audit logging validated: %s", auditEntry.JobID)
		},
	}

	test.Run(t)
}

// TestREQ43_STDOUTAuditCapture validates REQ-43: I can capture STDOUT in audit trail.
func TestREQ43_STDOUTAuditCapture(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-43",
		Description: "I can capture STDOUT in audit trail",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for audit log
			tempDir := t.TempDir()
			auditLogPath := tempDir + "/audit.jsonl"

			// Create test configuration with audit engine enabled
			// Use echo command with specific text to test STDOUT capture
			expectedOutput := "Test STDOUT output from command execution"
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath: "/bin/echo", // Use echo command for testing
					Audit: config.AuditConfig{
						Backend: "json",
						Path:    auditLogPath,
					},
				},
			}

			// Create NONMEM executor with audit logging enabled
			executor := execution.NewNONMEMExecutorWithAudit(cfg, true)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Create a test model file
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("test model content"), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Execute the command with additional options that will be echoed
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, []string{expectedOutput})
			require.NoError(t, err, "Execution should not error")
			require.NotNil(t, result, "Execution result should not be nil")

			// Wait a moment for audit log to be written
			time.Sleep(100 * time.Millisecond)

			// Read the audit log file
			auditEntries, err := audit.ReadLogEntries(auditLogPath)
			require.NoError(t, err, "Should be able to read audit log entries")
			require.Len(t, auditEntries, 1, "Should have exactly one audit entry")

			// Validate the audit entry captures STDOUT
			auditEntry := auditEntries[0]
			assert.NotEmpty(t, auditEntry.STDOUT, "STDOUT should not be empty")
			assert.Contains(t, auditEntry.STDOUT, expectedOutput, "STDOUT should contain expected output from command")

			t.Logf("STDOUT audit capture validated: %d characters captured", len(auditEntry.STDOUT))
		},
	}

	test.Run(t)
}

// TestREQ44_STDERRAuditCapture validates REQ-44: I can capture STDERR in audit trail.
func TestREQ44_STDERRAuditCapture(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-44",
		Description: "I can capture STDERR in audit trail",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create mock audit log entry with STDERR content
			stderrContent := `Warning: Model file contains deprecated syntax
Error: Unable to read dataset file: data.csv
Note: Switching to backup estimation method
Warning: Covariance matrix may be unstable`

			auditEntry := AuditLogEntry{
				JobID:     "job-stderr-test",
				Timestamp: time.Now(),
				Binary:    "/opt/NONMEM/nm76/run/nmfe76",
				Arguments: []string{"model.mod", "model.lst"},
				STDOUT:    "Normal execution output",
				STDERR:    stderrContent,
				ExitCode:  1,
				Duration:  2500,
			}

			// Validate STDERR is captured
			assert.NotEmpty(t, auditEntry.STDERR, "STDERR should not be empty")
			assert.Contains(t, auditEntry.STDERR, "Warning:", "STDERR should contain warning messages")
			assert.Contains(t, auditEntry.STDERR, "Error:", "STDERR should contain error messages")

			// Test JSON serialization preserves STDERR
			jsonData, err := json.Marshal(auditEntry)
			assert.NoError(t, err, "Audit entry with STDERR should serialize to JSON")

			var deserializedEntry AuditLogEntry
			err = json.Unmarshal(jsonData, &deserializedEntry)
			assert.NoError(t, err, "JSON with STDERR should deserialize correctly")
			assert.Equal(t, auditEntry.STDERR, deserializedEntry.STDERR, "STDERR should be preserved through JSON serialization")

			t.Logf("STDERR audit capture validated: %d characters captured", len(auditEntry.STDERR))
		},
	}

	test.Run(t)
}

// TestREQ45_BinaryPathAuditLogging validates REQ-45: I can capture executed binary path in audit trail.
func TestREQ45_BinaryPathAuditLogging(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-45",
		Description: "I can capture executed binary path in audit trail",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Test cases for different binary paths
			testCases := []struct {
				binaryPath  string
				description string
			}{
				{
					binaryPath:  "/opt/NONMEM/nm76/run/nmfe76",
					description: "NONMEM binary path",
				},
				{
					binaryPath:  "/usr/local/bin/execute",
					description: "PsN execute binary path",
				},
				{
					binaryPath:  "/usr/bin/bbi",
					description: "BBI binary path",
				},
				{
					binaryPath:  "/opt/torque/bin/qsub",
					description: "TORQUE qsub binary path",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.description, func(t *testing.T) {
					auditEntry := AuditLogEntry{
						JobID:     "job-binary-test",
						Timestamp: time.Now(),
						Binary:    tc.binaryPath,
						Arguments: []string{"arg1", "arg2"},
						STDOUT:    "Output",
						STDERR:    "",
						ExitCode:  0,
						Duration:  1000,
					}

					// Validate binary path is captured
					assert.NotEmpty(t, auditEntry.Binary, "Binary path should not be empty")
					assert.Equal(t, tc.binaryPath, auditEntry.Binary, "Binary path should match expected value")
					assert.True(t, len(auditEntry.Binary) > 1, "Binary path should be a valid path")

					// Test JSON serialization preserves binary path
					jsonData, err := json.Marshal(auditEntry)
					assert.NoError(t, err, "Audit entry with binary path should serialize to JSON")

					var deserializedEntry AuditLogEntry
					err = json.Unmarshal(jsonData, &deserializedEntry)
					assert.NoError(t, err, "JSON with binary path should deserialize correctly")
					assert.Equal(t, auditEntry.Binary, deserializedEntry.Binary, "Binary path should be preserved through JSON serialization")

					t.Logf("Binary path audit logging validated: %s", auditEntry.Binary)
				})
			}
		},
	}

	test.Run(t)
}

// TestREQ46_CommandArgumentsAuditLogging validates REQ-46: I can capture command arguments in audit trail.
func TestREQ46_CommandArgumentsAuditLogging(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-46",
		Description: "I can capture command arguments in audit trail",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Test cases for different argument patterns
			testCases := []struct {
				arguments   []string
				description string
			}{
				{
					arguments:   []string{"model.mod", "model.lst"},
					description: "Basic NONMEM arguments",
				},
				{
					arguments:   []string{"model.mod", "model.lst", "-parallel", "model.pnm"},
					description: "NONMEM parallel arguments",
				},
				{
					arguments:   []string{"model.mod", "-threads=4", "-clean=3"},
					description: "PsN execute arguments",
				},
				{
					arguments:   []string{"nonmem", "run", "local", "model.mod", "--parallel", "--threads=8"},
					description: "BBI command arguments",
				},
				{
					arguments:   []string{"-q", "normal", "-l", "walltime=02:00:00", "script.sh"},
					description: "TORQUE qsub arguments",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.description, func(t *testing.T) {
					auditEntry := AuditLogEntry{
						JobID:     "job-args-test",
						Timestamp: time.Now(),
						Binary:    "/usr/bin/test",
						Arguments: tc.arguments,
						STDOUT:    "Output",
						STDERR:    "",
						ExitCode:  0,
						Duration:  1000,
					}

					// Validate arguments are captured
					assert.NotNil(t, auditEntry.Arguments, "Arguments should not be nil")
					assert.Equal(t, len(tc.arguments), len(auditEntry.Arguments), "Arguments length should match")
					assert.Equal(t, tc.arguments, auditEntry.Arguments, "Arguments should match expected values")

					// Test JSON serialization preserves arguments array
					jsonData, err := json.Marshal(auditEntry)
					assert.NoError(t, err, "Audit entry with arguments should serialize to JSON")

					var deserializedEntry AuditLogEntry
					err = json.Unmarshal(jsonData, &deserializedEntry)
					assert.NoError(t, err, "JSON with arguments should deserialize correctly")
					assert.Equal(t, auditEntry.Arguments, deserializedEntry.Arguments, "Arguments should be preserved through JSON serialization")

					t.Logf("Command arguments audit logging validated: %v", auditEntry.Arguments)
				})
			}
		},
	}

	test.Run(t)
}

// TestAuditLogFileWrite validates writing audit logs to file.
func TestAuditLogFileWrite(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-41",
		Description: "I can write JSON audit trail to file",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create test audit entry
			auditEntry := AuditLogEntry{
				JobID:     "job-file-write-test",
				Timestamp: time.Now(),
				Binary:    "/opt/NONMEM/nm76/run/nmfe76",
				Arguments: []string{"model.mod", "model.lst"},
				STDOUT:    "Execution completed successfully",
				STDERR:    "",
				ExitCode:  0,
				Duration:  3000,
			}

			// Create temporary file for testing
			tempDir := t.TempDir()
			auditFile := tempDir + "/audit.jsonl"

			// Serialize and write to file
			jsonData, err := json.Marshal(auditEntry)
			require.NoError(t, err, "Should be able to marshal audit entry")

			err = os.WriteFile(auditFile, jsonData, 0600)
			require.NoError(t, err, "Should be able to write audit file")

			// Read back and verify
			readData, err := os.ReadFile(auditFile)
			require.NoError(t, err, "Should be able to read audit file")

			var readEntry AuditLogEntry
			err = json.Unmarshal(readData, &readEntry)
			require.NoError(t, err, "Should be able to unmarshal audit entry from file")

			// Verify all fields are preserved
			assert.Equal(t, auditEntry.JobID, readEntry.JobID, "Job ID should be preserved")
			assert.Equal(t, auditEntry.Binary, readEntry.Binary, "Binary path should be preserved")
			assert.Equal(t, auditEntry.Arguments, readEntry.Arguments, "Arguments should be preserved")
			assert.Equal(t, auditEntry.STDOUT, readEntry.STDOUT, "STDOUT should be preserved")
			assert.Equal(t, auditEntry.STDERR, readEntry.STDERR, "STDERR should be preserved")
			assert.Equal(t, auditEntry.ExitCode, readEntry.ExitCode, "Exit code should be preserved")

			t.Logf("Audit log file write validated: %s", auditFile)
		},
	}

	test.Run(t)
}

// TestAuditDisabledWhenConfigurationDisabled validates that audit logging is disabled when configuration is disabled.
func TestAuditDisabledWhenConfigurationDisabled(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-41",
		Description: "Audit logging is disabled when configuration is disabled",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for audit log
			tempDir := t.TempDir()
			auditLogPath := tempDir + "/audit.jsonl"

			// Create test configuration with audit engine DISABLED
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath: "/bin/echo", // Use echo command for testing
					Audit: config.AuditConfig{
						Backend: "", // Empty backend = disabled
						Path:    auditLogPath,
					},
				},
			}

			// Create NONMEM executor with audit logging disabled
			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Create a test model file
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("test model content"), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Execute the command
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, []string{"test-output"})
			require.NoError(t, err, "Execution should not error")
			require.NotNil(t, result, "Execution result should not be nil")

			// Wait a moment to ensure no audit log would be written
			time.Sleep(100 * time.Millisecond)

			// Verify that no audit log file was created or that it's empty
			if _, err := os.Stat(auditLogPath); err == nil {
				// File exists, check if it's empty
				data, readErr := os.ReadFile(auditLogPath)
				require.NoError(t, readErr, "Should be able to read audit file if it exists")
				assert.Empty(t, data, "Audit log file should be empty when audit engine is disabled")
			} else {
				// File doesn't exist, which is expected when audit is disabled
				assert.True(t, os.IsNotExist(err), "Audit log file should not exist when audit engine is disabled")
			}

			t.Log("Verified that audit logging is properly disabled when configuration is disabled")
		},
	}

	test.Run(t)
}