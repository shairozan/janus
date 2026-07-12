//go:build validation
// +build validation

package validation

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
)

// TestREQ01_BasicNONMEMExecution validates REQ-01: I can run NONMEM locally.
func TestREQ01_BasicNONMEMExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-01",
		Description: "I can run NONMEM locally",
		Category:    CategoryNONMEMExecution,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for test files
			tempDir := t.TempDir()
			modelFile := tempDir + "/test_model.mod"

			// Create test model file
			modelContent := `$PROBLEM Test Model
$INPUT ID TIME DV
$DATA test_data.csv IGNORE=@
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
$COVARIANCE`
			err := os.WriteFile(modelFile, []byte(modelContent), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Create test configuration using echo for cross-platform testing
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:   "/bin", // Use /bin directory for testing
					NonmemBinary: "echo", // Use echo binary for testing
				},
			}

			// Create NONMEM executor
			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Execute the model (this will use echo instead of real NONMEM)
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, nil)

			// Validate execution results
			assert.NoError(t, err, "NONMEM execution should not error")
			require.NotNil(t, result, "Execution result should not be nil")
			assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")
			assert.NotEmpty(t, result.Stdout, "STDOUT should not be empty")

			// Verify that the output contains expected elements from echo command
			stdout := string(result.Stdout)
			assert.Contains(t, stdout, modelFile, "STDOUT should contain the model file path")
			assert.Contains(t, stdout, "test_model.lst", "STDOUT should contain expected output file")

			t.Logf("NONMEM execution completed successfully with exit code: %d", result.ExitCode)
			t.Logf("STDOUT: %s", stdout)
		},
	}

	test.Run(t)
}

// TestREQ02_NONMEMWithAdditionalOptions validates REQ-02: I can run NONMEM with additional options.
func TestREQ02_NONMEMWithAdditionalOptions(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-02",
		Description: "I can run NONMEM with additional options",
		Category:    CategoryNONMEMExecution,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for test files
			tempDir := t.TempDir()
			modelFile := tempDir + "/advanced_model.mod"

			// Create test model file with more complex content
			modelContent := `$PROBLEM Advanced NONMEM Model with Options
$INPUT ID TIME DV AMT EVID CMT
$DATA advanced_data.csv IGNORE=@
$PRED
CL = THETA(1) * EXP(ETA(1))
V  = THETA(2) * EXP(ETA(2))
Y  = F + ERR(1)
$THETA 1 10
$OMEGA 0.1 0.1
$SIGMA 0.1
$ESTIMATION METHOD=1 MAXEVAL=9999 PRINT=1
$COVARIANCE`
			err := os.WriteFile(modelFile, []byte(modelContent), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Create test configuration using echo for cross-platform testing
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:   "/bin",
					NonmemBinary: "echo",
				},
			}

			// Create NONMEM executor
			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Execute the model with additional options
			additionalOptions := []string{"-maxeval=9999", "-files=100"}
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, false, 1, false, additionalOptions)

			// Validate execution results
			assert.NoError(t, err, "NONMEM execution with options should not error")
			require.NotNil(t, result, "Execution result should not be nil")
			assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")
			assert.NotEmpty(t, result.Stdout, "STDOUT should not be empty")

			// Verify that the output contains expected elements
			stdout := string(result.Stdout)
			assert.Contains(t, stdout, modelFile, "STDOUT should contain the model file path")
			assert.Contains(t, stdout, "advanced_model.lst", "STDOUT should contain expected output file")
			assert.Contains(t, stdout, "-maxeval=9999", "STDOUT should contain additional option -maxeval=9999")
			assert.Contains(t, stdout, "-files=100", "STDOUT should contain additional option -files=100")

			t.Logf("NONMEM execution with options completed successfully")
			t.Logf("Additional options verified: %v", additionalOptions)
			t.Logf("STDOUT: %s", stdout)
		},
	}

	test.Run(t)
}

// TestREQ03_NONMEMParallelExecution validates REQ-03: I can run NONMEM in parallel mode.
func TestREQ03_NONMEMParallelExecution(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-03",
		Description: "I can run NONMEM in parallel mode",
		Category:    CategoryNONMEMExecution,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for test files
			tempDir := t.TempDir()
			modelFile := tempDir + "/parallel_model.mod"

			// Create test model file for parallel execution
			modelContent := `$PROBLEM Parallel NONMEM Model
$INPUT ID TIME DV WT AGE
$DATA parallel_data.csv IGNORE=@
$PRED
CL = THETA(1) * (WT/70)**0.75 * EXP(ETA(1))
V  = THETA(2) * (WT/70) * EXP(ETA(2))
K  = CL/V
Y  = F + ERR(1)
$THETA 1.5 50
$OMEGA 0.1 0.1
$SIGMA 0.2
$ESTIMATION METHOD=1 INTERACTION PRINT=1
$COVARIANCE PRINT=E`
			err := os.WriteFile(modelFile, []byte(modelContent), 0644)
			require.NoError(t, err, "Should be able to create test model file")

			// Create test configuration using echo for cross-platform testing
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:   "/bin",
					NonmemBinary: "echo",
				},
			}

			// Create NONMEM executor
			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Execute the model in parallel mode with 4 cores
			ctx := context.Background()
			result, err := executor.Execute(ctx, modelFile, true, 4, false, nil)

			// Validate execution results
			assert.NoError(t, err, "NONMEM parallel execution should not error")
			require.NotNil(t, result, "Execution result should not be nil")
			assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")
			assert.NotEmpty(t, result.Stdout, "STDOUT should not be empty")

			// Verify that the output contains expected elements for parallel execution
			stdout := string(result.Stdout)
			assert.Contains(t, stdout, modelFile, "STDOUT should contain the model file path")
			assert.Contains(t, stdout, "parallel_model.lst", "STDOUT should contain expected output file")
			assert.Contains(t, stdout, "-parallel", "STDOUT should contain parallel flag")
			assert.Contains(t, stdout, "parallel_model.pnm", "STDOUT should contain PNM file for parallel execution")

			t.Logf("NONMEM parallel execution completed successfully")
			t.Logf("Parallel mode: true, Cores: 4")
			t.Logf("STDOUT: %s", stdout)
		},
	}

	test.Run(t)
}

// TestREQ04_NONMEMOutputFileHandling validates REQ-04: I can specify NONMEM output file locations.
func TestREQ04_NONMEMOutputFileHandling(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-04",
		Description: "I can specify NONMEM output file locations",
		Category:    CategoryNONMEMExecution,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for test files
			tempDir := t.TempDir()

			// Create test configuration using echo for cross-platform testing
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:   "/bin",
					NonmemBinary: "echo",
				},
			}

			// Create NONMEM executor
			executor := execution.NewNONMEMExecutor(cfg)
			require.NotNil(t, executor, "NONMEM executor should be created")

			// Test cases for different file extensions and paths
			testCases := []struct {
				inputFileName string
				expectedOut   string
				description   string
			}{
				{
					inputFileName: "model.mod",
					expectedOut:   "model.lst",
					description:   "Standard .mod file should generate .lst output",
				},
				{
					inputFileName: "complex_model.mod",
					expectedOut:   "complex_model.lst",
					description:   "Complex model name should preserve name in output",
				},
				{
					inputFileName: "model_v2.mod",
					expectedOut:   "model_v2.lst",
					description:   "Underscore in filename should be preserved",
				},
				{
					inputFileName: "population_pk.mod",
					expectedOut:   "population_pk.lst",
					description:   "Descriptive model name should preserve name in output",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.description, func(t *testing.T) {
					// Create test model file
					modelFile := tempDir + "/" + tc.inputFileName
					modelContent := `$PROBLEM Output File Test Model
$INPUT ID TIME DV
$DATA test.csv IGNORE=@
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
$COVARIANCE`
					err := os.WriteFile(modelFile, []byte(modelContent), 0644)
					require.NoError(t, err, "Should be able to create test model file")

					// Execute the model
					ctx := context.Background()
					result, err := executor.Execute(ctx, modelFile, false, 1, false, nil)

					// Validate execution results
					assert.NoError(t, err, "NONMEM execution should not error for %s", tc.inputFileName)
					require.NotNil(t, result, "Execution result should not be nil")
					assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")
					assert.NotEmpty(t, result.Stdout, "STDOUT should not be empty")

					// Verify that the output contains expected elements
					stdout := string(result.Stdout)
					assert.Contains(t, stdout, modelFile, "STDOUT should contain the input model file path")
					assert.Contains(t, stdout, tc.expectedOut, "STDOUT should contain expected output file: %s", tc.expectedOut)

					t.Logf("Input: %s -> Expected Output: %s", tc.inputFileName, tc.expectedOut)
					t.Logf("STDOUT: %s", stdout)
				})
			}
		},
	}

	test.Run(t)
}

// TestNONMEMValidationSuite runs all NONMEM validation tests as a suite.
func TestNONMEMValidationSuite(t *testing.T) {
	t.Log("=== STARTING NONMEM VALIDATION SUITE ===")

	// Reset the global reporter for this suite
	ResetGlobalReporter()

	// Run all NONMEM validation tests
	t.Run("REQ-01", TestREQ01_BasicNONMEMExecution)
	t.Run("REQ-02", TestREQ02_NONMEMWithAdditionalOptions)
	t.Run("REQ-03", TestREQ03_NONMEMParallelExecution)
	t.Run("REQ-04", TestREQ04_NONMEMOutputFileHandling)

	// Generate validation report
	reporter := GetGlobalReporter()
	reporter.PrintSummary()

	// Generate comprehensive run log-ready report
	if err := reporter.GenerateAuditorReport("nonmem_run log_validation_report.json", "1.0.0", "Automated Test System"); err != nil {
		t.Logf("Warning: Could not generate comprehensive run log report: %v", err)
	} else {
		t.Log("Comprehensive run log report saved to: nonmem_run log_validation_report.json")
	}

	// Optionally save basic technical report to file
	if err := reporter.GenerateReport("nonmem_validation_report.json"); err != nil {
		t.Logf("Warning: Could not generate basic validation report: %v", err)
	} else {
		t.Log("Basic validation report saved to: nonmem_validation_report.json")
	}

	t.Log("=== NONMEM VALIDATION SUITE COMPLETE ===")
}

// TestExecutionValidationSuite runs all execution-related validation tests as a comprehensive suite.
func TestExecutionValidationSuite(t *testing.T) {
	t.Log("=== STARTING EXECUTION VALIDATION SUITE ===")

	// Reset the global reporter for this suite
	ResetGlobalReporter()

	// Run NONMEM validation tests (REQ-01 through REQ-04)
	t.Run("REQ-01", TestREQ01_BasicNONMEMExecution)
	t.Run("REQ-02", TestREQ02_NONMEMWithAdditionalOptions)
	t.Run("REQ-03", TestREQ03_NONMEMParallelExecution)
	t.Run("REQ-04", TestREQ04_NONMEMOutputFileHandling)

	// Run PSN validation tests (REQ-05 through REQ-06)
	t.Run("REQ-05", TestREQ05_PSNExecuteCommandGeneration)
	t.Run("REQ-06", TestREQ06_PSNAdditionalOptions)
	t.Run("REQ-05-Parallel", TestREQ05_PSNParallelExecution)
	t.Run("REQ-06-Grid", TestREQ06_PSNGridExecution)

	// Run BBI validation tests (REQ-07 through REQ-08)
	t.Run("REQ-07", TestREQ07_BBICommandGeneration)
	t.Run("REQ-08", TestREQ08_BBIConfigurationOptions)
	t.Run("REQ-07-Parallel", TestREQ07_BBIParallelExecution)
	t.Run("REQ-08-Grid", TestREQ08_BBIGridExecution)
	t.Run("REQ-07-Combined", TestREQ07_BBICombinedOptions)

	// Run run log compliance validation tests (REQ-41 through REQ-46)
	t.Run("REQ-41", TestREQ41_JSONRunLogEnablement)
	t.Run("REQ-42", TestREQ42_JobIDRunLogging)
	t.Run("REQ-43", TestREQ43_STDOUTCapture)
	t.Run("REQ-44", TestREQ44_STDERRCapture)
	t.Run("REQ-45", TestREQ45_BinaryPathLogging)
	t.Run("REQ-46", TestREQ46_CommandArgumentsLogging)
	t.Run("REQ-41-FileWrite", TestRunLogFileWrite)

	// Generate validation report
	reporter := GetGlobalReporter()
	reporter.PrintSummary()

	// Generate comprehensive run log-ready report
	if err := reporter.GenerateAuditorReport("execution_run log_validation_report.json", "1.0.0", "Automated Test System"); err != nil {
		t.Logf("Warning: Could not generate comprehensive run log report: %v", err)
	} else {
		t.Log("Comprehensive run log report saved to: execution_run log_validation_report.json")
	}

	// Save detailed technical report to file
	if err := reporter.GenerateReport("execution_validation_report.json"); err != nil {
		t.Logf("Warning: Could not generate basic validation report: %v", err)
	} else {
		t.Log("Basic validation report saved to: execution_validation_report.json")
	}

	t.Log("=== EXECUTION VALIDATION SUITE COMPLETE ===")
}

// TestConfigurationValidationSuite runs all configuration-related validation tests as a suite.
func TestConfigurationValidationSuite(t *testing.T) {
	t.Log("=== STARTING CONFIGURATION VALIDATION SUITE ===")

	// Reset the global reporter for this suite
	ResetGlobalReporter()

	// Run configuration validation tests (REQ-18 through REQ-21)
	t.Run("REQ-18", TestREQ18_ConfigurationFileLoading)
	t.Run("REQ-19", TestREQ19_ConfigurationFlagOverrides)
	t.Run("REQ-20", TestREQ20_DefaultConfigurationValues)
	t.Run("REQ-21", TestREQ21_ConfigurationValidation)
	t.Run("REQ-21-Tools", TestREQ21_ExecutionModeToolValidation)
	t.Run("REQ-18-Processing", TestREQ18_ConfigurationProcessing)

	// Generate validation report
	reporter := GetGlobalReporter()
	reporter.PrintSummary()

	// Save detailed report to file
	if err := reporter.GenerateReport("configuration_validation_report.json"); err != nil {
		t.Logf("Warning: Could not generate validation report: %v", err)
	} else {
		t.Log("Validation report saved to: configuration_validation_report.json")
	}

	t.Log("=== CONFIGURATION VALIDATION SUITE COMPLETE ===")
}

// TestRunLogComplianceValidationSuite runs all run log compliance validation tests as a suite.
func TestRunLogComplianceValidationSuite(t *testing.T) {
	t.Log("=== STARTING AUDIT COMPLIANCE VALIDATION SUITE ===")

	// Reset the global reporter for this suite
	ResetGlobalReporter()

	// Run run log compliance validation tests (REQ-41 through REQ-46)
	t.Run("REQ-41", TestREQ41_JSONRunLogEnablement)
	t.Run("REQ-42", TestREQ42_JobIDRunLogging)
	t.Run("REQ-43", TestREQ43_STDOUTCapture)
	t.Run("REQ-44", TestREQ44_STDERRCapture)
	t.Run("REQ-45", TestREQ45_BinaryPathLogging)
	t.Run("REQ-46", TestREQ46_CommandArgumentsLogging)
	t.Run("REQ-41-FileWrite", TestRunLogFileWrite)

	// Generate validation report
	reporter := GetGlobalReporter()
	reporter.PrintSummary()

	// Save detailed report to file
	if err := reporter.GenerateReport("runlog_compliance_validation_report.json"); err != nil {
		t.Logf("Warning: Could not generate validation report: %v", err)
	} else {
		t.Log("Validation report saved to: runlog_compliance_validation_report.json")
	}

	t.Log("=== AUDIT COMPLIANCE VALIDATION SUITE COMPLETE ===")
}
