//go:build unit
// +build unit

package execution

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shairozan/janus/internal/config"
)

func TestNONMEMExecutor_buildNONMEMCommand(t *testing.T) {
	// Setup test config
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm76/run",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	tests := []struct {
		name              string
		modelPath         string
		isParallel        bool
		cores             int
		isGrid            bool
		additionalOptions []string
		expectBinaryPath  string
		expectArgs        []string
		expectError       bool
	}{
		{
			name:              "basic synchronous execution",
			modelPath:         "/path/to/model.mod",
			isParallel:        false,
			cores:             1,
			isGrid:            false,
			additionalOptions: nil,
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path/to/model.mod", "/path/to/model.lst"},
			expectError:       false,
		},
		{
			name:              "parallel execution",
			modelPath:         "/path/to/model.mod",
			isParallel:        true,
			cores:             4,
			isGrid:            false,
			additionalOptions: nil,
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path/to/model.mod", "/path/to/model.lst", "-parallel", "/path/to/model.pnm"},
			expectError:       false,
		},
		{
			name:              "synchronous with additional options",
			modelPath:         "/path/to/model.mod",
			isParallel:        false,
			cores:             1,
			isGrid:            false,
			additionalOptions: []string{"-maxeval=9999", "-files=100"},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path/to/model.mod", "/path/to/model.lst", "-maxeval=9999", "-files=100"},
			expectError:       false,
		},
		{
			name:              "parallel with additional options",
			modelPath:         "/path/to/model.mod",
			isParallel:        true,
			cores:             8,
			isGrid:            true, // Should not affect local execution for now
			additionalOptions: []string{"-maxeval=9999", "-files=100", "-sigdigits=4"},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path/to/model.mod", "/path/to/model.lst", "-parallel", "/path/to/model.pnm", "-maxeval=9999", "-files=100", "-sigdigits=4"},
			expectError:       false,
		},
		{
			name:              "different file extensions",
			modelPath:         "/models/pk.ctl",
			isParallel:        false,
			cores:             1,
			isGrid:            false,
			additionalOptions: []string{"-clean=1"},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/models/pk.ctl", "/models/pk.lst", "-clean=1"},
			expectError:       false,
		},
		{
			name:              "model with no extension",
			modelPath:         "/models/run001",
			isParallel:        true,
			cores:             2,
			isGrid:            false,
			additionalOptions: []string{"-background"},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/models/run001", "/models/run001.lst", "-parallel", "/models/run001.pnm", "-background"},
			expectError:       false,
		},
		{
			name:              "empty additional options should be ignored",
			modelPath:         "/path/to/model.mod",
			isParallel:        false,
			cores:             1,
			isGrid:            false,
			additionalOptions: []string{},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path/to/model.mod", "/path/to/model.lst"},
			expectError:       false,
		},
		{
			name:              "complex model path with spaces handled by OS",
			modelPath:         "/path with spaces/model file.mod",
			isParallel:        false,
			cores:             1,
			isGrid:            false,
			additionalOptions: []string{"-option=value"},
			expectBinaryPath:  filepath.Join("/opt/NONMEM/nm76/run", "nmfe76"),
			expectArgs:        []string{"/path with spaces/model file.mod", "/path with spaces/model file.lst", "-option=value"},
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualBinary, actualArgs, err := executor.buildNONMEMCommand(
				tt.modelPath,
				tt.isParallel,
				tt.cores,
				tt.isGrid,
				tt.additionalOptions,
			)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if tt.expectError {
				return // Don't check other values if we expected an error
			}

			// Normalize paths for comparison (handle different OS path separators)
			expectedBinary, err := filepath.Abs(tt.expectBinaryPath)
			if err != nil {
				t.Fatalf("Failed to normalize expected binary path: %v", err)
			}
			actualBinaryNorm, err := filepath.Abs(actualBinary)
			if err != nil {
				t.Fatalf("Failed to normalize actual binary path: %v", err)
			}

			// Check binary path
			if actualBinaryNorm != expectedBinary {
				t.Errorf("Binary path mismatch:\n  Expected: %s\n  Actual:   %s", expectedBinary, actualBinaryNorm)
			}

			// Check arguments
			if !reflect.DeepEqual(actualArgs, tt.expectArgs) {
				t.Errorf("Arguments mismatch:\n  Expected: %v\n  Actual:   %v", tt.expectArgs, actualArgs)
			}

			// Verify argument ordinality (order is crucial for NONMEM)
			if len(actualArgs) >= 2 {
				// First two arguments should always be model file and output file
				if actualArgs[0] != tt.modelPath {
					t.Errorf("First argument should be model path, got: %s", actualArgs[0])
				}
				expectedOutputFile := strings.TrimSuffix(tt.modelPath, filepath.Ext(tt.modelPath)) + ".lst"
				if actualArgs[1] != expectedOutputFile {
					t.Errorf("Second argument should be output file, expected: %s, got: %s", expectedOutputFile, actualArgs[1])
				}
			}

			// If parallel, verify -parallel and .pnm file order
			if tt.isParallel && len(actualArgs) >= 4 {
				if actualArgs[2] != "-parallel" {
					t.Errorf("For parallel execution, third argument should be '-parallel', got: %s", actualArgs[2])
				}
				expectedPnmFile := strings.TrimSuffix(tt.modelPath, filepath.Ext(tt.modelPath)) + ".pnm"
				if actualArgs[3] != expectedPnmFile {
					t.Errorf("For parallel execution, fourth argument should be .pnm file, expected: %s, got: %s", expectedPnmFile, actualArgs[3])
				}
			}

			// Verify additional options are at the end and in order
			if len(tt.additionalOptions) > 0 {
				startIdx := 2 // After model and output file
				if tt.isParallel {
					startIdx = 4 // After model, output, -parallel, pnm file
				}

				if len(actualArgs) < startIdx+len(tt.additionalOptions) {
					t.Errorf("Expected additional options not found in arguments")
				} else {
					actualAdditionalOptions := actualArgs[startIdx : startIdx+len(tt.additionalOptions)]
					if !reflect.DeepEqual(actualAdditionalOptions, tt.additionalOptions) {
						t.Errorf("Additional options mismatch:\n  Expected: %v\n  Actual:   %v", tt.additionalOptions, actualAdditionalOptions)
					}
				}
			}
		})
	}
}

func TestNONMEMExecutor_buildNONMEMCommand_ErrorCases(t *testing.T) {
	// Note: buildNonmemBinaryPath rarely fails since filepath.Abs can handle most inputs
	// The main error cases would be filesystem-related or permission issues
	// For now, we focus on testing that valid configurations work correctly
	// and document that error testing would require more complex filesystem mocking

	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/valid/path",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	// Test that normal operation doesn't cause errors
	t.Run("valid_configuration_no_error", func(t *testing.T) {
		_, _, err := executor.buildNONMEMCommand(
			"/path/to/model.mod",
			false,
			1,
			false,
			nil,
		)

		if err != nil {
			t.Errorf("Unexpected error with valid configuration: %v", err)
		}
	})

	// Test with very long paths (potential edge case)
	t.Run("very_long_model_path", func(t *testing.T) {
		longPath := strings.Repeat("/very/long/directory/structure", 50) + "/model.mod"
		_, _, err := executor.buildNONMEMCommand(
			longPath,
			false,
			1,
			false,
			nil,
		)

		// Should not error - filesystem will handle path length limits
		if err != nil {
			t.Errorf("Unexpected error with long path: %v", err)
		}
	})
}

func TestNONMEMExecutor_CommandArgumentOrder(t *testing.T) {
	// This test specifically focuses on verifying the exact order of arguments
	// which is critical for NONMEM execution
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm76/run",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	modelPath := "/path/to/model.mod"
	additionalOptions := []string{"-maxeval=9999", "-files=100", "-sigdigits=4", "-clean=1"}

	// Test synchronous execution argument order
	t.Run("synchronous_argument_order", func(t *testing.T) {
		_, args, err := executor.buildNONMEMCommand(modelPath, false, 1, false, additionalOptions)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := []string{
			"/path/to/model.mod", // 0: model file
			"/path/to/model.lst", // 1: output file
			"-maxeval=9999",      // 2: first additional option
			"-files=100",         // 3: second additional option
			"-sigdigits=4",       // 4: third additional option
			"-clean=1",           // 5: fourth additional option
		}

		if !reflect.DeepEqual(args, expected) {
			t.Errorf("Synchronous argument order mismatch:\n  Expected: %v\n  Actual:   %v", expected, args)
		}
	})

	// Test parallel execution argument order
	t.Run("parallel_argument_order", func(t *testing.T) {
		_, args, err := executor.buildNONMEMCommand(modelPath, true, 4, false, additionalOptions)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := []string{
			"/path/to/model.mod", // 0: model file
			"/path/to/model.lst", // 1: output file
			"-parallel",          // 2: parallel flag
			"/path/to/model.pnm", // 3: parallel config file
			"-maxeval=9999",      // 4: first additional option
			"-files=100",         // 5: second additional option
			"-sigdigits=4",       // 6: third additional option
			"-clean=1",           // 7: fourth additional option
		}

		if !reflect.DeepEqual(args, expected) {
			t.Errorf("Parallel argument order mismatch:\n  Expected: %v\n  Actual:   %v", expected, args)
		}
	})
}

func TestNONMEMExecutor_AdditionalOptionsEdgeCases(t *testing.T) {
	// Test edge cases for additional options handling
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm76/run",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	tests := []struct {
		name              string
		additionalOptions []string
		expectedSuffix    []string
		description       string
	}{
		{
			name:              "single_option_with_equals",
			additionalOptions: []string{"-maxeval=9999"},
			expectedSuffix:    []string{"-maxeval=9999"},
			description:       "Single option with equals sign",
		},
		{
			name:              "single_option_without_equals",
			additionalOptions: []string{"-clean"},
			expectedSuffix:    []string{"-clean"},
			description:       "Single flag option without value",
		},
		{
			name:              "multiple_mixed_options",
			additionalOptions: []string{"-maxeval=9999", "-clean", "-files=100", "-background"},
			expectedSuffix:    []string{"-maxeval=9999", "-clean", "-files=100", "-background"},
			description:       "Mix of options with and without values",
		},
		{
			name:              "option_with_spaces_in_value",
			additionalOptions: []string{"-option=value with spaces"},
			expectedSuffix:    []string{"-option=value with spaces"},
			description:       "Option with spaces in value",
		},
		{
			name:              "complex_nmtran_options",
			additionalOptions: []string{"-maxeval=9999", "-sigdig=4", "-files=1000", "-clean=3", "-nmtran_skip"},
			expectedSuffix:    []string{"-maxeval=9999", "-sigdig=4", "-files=1000", "-clean=3", "-nmtran_skip"},
			description:       "Real-world NONMEM options",
		},
		{
			name:              "duplicate_options",
			additionalOptions: []string{"-maxeval=9999", "-maxeval=5000"},
			expectedSuffix:    []string{"-maxeval=9999", "-maxeval=5000"},
			description:       "Duplicate options (last one should win in NONMEM)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with synchronous execution
			_, args, err := executor.buildNONMEMCommand(
				"/path/to/model.mod",
				false,
				1,
				false,
				tt.additionalOptions,
			)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify additional options are appended correctly
			if len(args) < 2+len(tt.expectedSuffix) {
				t.Fatalf("Expected at least %d arguments, got %d", 2+len(tt.expectedSuffix), len(args))
			}

			actualSuffix := args[2:] // Skip model file and output file
			if !reflect.DeepEqual(actualSuffix, tt.expectedSuffix) {
				t.Errorf("%s - Additional options mismatch:\n  Expected: %v\n  Actual:   %v",
					tt.description, tt.expectedSuffix, actualSuffix)
			}

			// Test with parallel execution
			_, argsParallel, err := executor.buildNONMEMCommand(
				"/path/to/model.mod",
				true,
				4,
				false,
				tt.additionalOptions,
			)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify additional options are appended after parallel options
			if len(argsParallel) < 4+len(tt.expectedSuffix) {
				t.Fatalf("Expected at least %d arguments for parallel, got %d", 4+len(tt.expectedSuffix), len(argsParallel))
			}

			actualSuffixParallel := argsParallel[4:] // Skip model, output, -parallel, pnm file
			if !reflect.DeepEqual(actualSuffixParallel, tt.expectedSuffix) {
				t.Errorf("%s - Parallel additional options mismatch:\n  Expected: %v\n  Actual:   %v",
					tt.description, tt.expectedSuffix, actualSuffixParallel)
			}
		})
	}
}

func TestNONMEMExecutor_RealWorldScenarios(t *testing.T) {
	// Test real-world NONMEM execution scenarios
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm74/run",
			NonmemBinary: "nmfe74",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	tests := []struct {
		name              string
		modelPath         string
		isParallel        bool
		cores             int
		additionalOptions []string
		expectedCommand   []string
		description       string
	}{
		{
			name:              "typical_popPK_run",
			modelPath:         "/project/models/run001.mod",
			isParallel:        false,
			cores:             1,
			additionalOptions: []string{"-maxeval=9999", "-files=100"},
			expectedCommand:   []string{"/project/models/run001.mod", "/project/models/run001.lst", "-maxeval=9999", "-files=100"},
			description:       "Typical population PK model run",
		},
		{
			name:              "parallel_bootstrap",
			modelPath:         "/analysis/bootstrap/run001.mod",
			isParallel:        true,
			cores:             8,
			additionalOptions: []string{"-maxeval=0", "-clean=2"},
			expectedCommand:   []string{"/analysis/bootstrap/run001.mod", "/analysis/bootstrap/run001.lst", "-parallel", "/analysis/bootstrap/run001.pnm", "-maxeval=0", "-clean=2"},
			description:       "Parallel bootstrap run with evaluation only",
		},
		{
			name:              "final_model_high_precision",
			modelPath:         "/final/model_final.ctl",
			isParallel:        true,
			cores:             4,
			additionalOptions: []string{"-maxeval=9999", "-sigdigits=4", "-files=1000", "-nmtran_skip"},
			expectedCommand:   []string{"/final/model_final.ctl", "/final/model_final.lst", "-parallel", "/final/model_final.pnm", "-maxeval=9999", "-sigdigits=4", "-files=1000", "-nmtran_skip"},
			description:       "Final model with high precision settings",
		},
		{
			name:              "simulation_run",
			modelPath:         "/simulation/sim001.mod",
			isParallel:        false,
			cores:             1,
			additionalOptions: []string{"-maxeval=0", "-seed=12345"},
			expectedCommand:   []string{"/simulation/sim001.mod", "/simulation/sim001.lst", "-maxeval=0", "-seed=12345"},
			description:       "Simulation run with fixed seed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binaryPath, args, err := executor.buildNONMEMCommand(
				tt.modelPath,
				tt.isParallel,
				tt.cores,
				false, // isGrid - not testing grid submission here
				tt.additionalOptions,
			)
			if err != nil {
				t.Fatalf("Unexpected error for %s: %v", tt.description, err)
			}

			// Verify binary path is constructed correctly
			expectedBinaryPath := filepath.Join("/opt/NONMEM/nm74/run", "nmfe74")
			if !strings.HasSuffix(binaryPath, expectedBinaryPath) {
				t.Errorf("Binary path mismatch for %s:\n  Expected suffix: %s\n  Actual: %s",
					tt.description, expectedBinaryPath, binaryPath)
			}

			// Verify command arguments
			if !reflect.DeepEqual(args, tt.expectedCommand) {
				t.Errorf("Command mismatch for %s:\n  Expected: %v\n  Actual:   %v",
					tt.description, tt.expectedCommand, args)
			}
		})
	}
}

func TestNONMEMExecutor_FilePathGeneration(t *testing.T) {
	// Test the generation of output files (.lst) and parallel config files (.pnm)
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm76/run",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	tests := []struct {
		name               string
		modelPath          string
		expectedOutputFile string
		expectedPnmFile    string
	}{
		{
			name:               "mod extension",
			modelPath:          "/models/run001.mod",
			expectedOutputFile: "/models/run001.lst",
			expectedPnmFile:    "/models/run001.pnm",
		},
		{
			name:               "ctl extension",
			modelPath:          "/models/pk.ctl",
			expectedOutputFile: "/models/pk.lst",
			expectedPnmFile:    "/models/pk.pnm",
		},
		{
			name:               "no extension",
			modelPath:          "/models/run001",
			expectedOutputFile: "/models/run001.lst",
			expectedPnmFile:    "/models/run001.pnm",
		},
		{
			name:               "multiple dots in filename",
			modelPath:          "/models/pop.pk.final.mod",
			expectedOutputFile: "/models/pop.pk.final.lst",
			expectedPnmFile:    "/models/pop.pk.final.pnm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test synchronous (only output file)
			_, argsSync, err := executor.buildNONMEMCommand(tt.modelPath, false, 1, false, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(argsSync) < 2 {
				t.Fatalf("Expected at least 2 arguments, got %d", len(argsSync))
			}
			if argsSync[1] != tt.expectedOutputFile {
				t.Errorf("Output file mismatch:\n  Expected: %s\n  Actual:   %s", tt.expectedOutputFile, argsSync[1])
			}

			// Test parallel (output file and pnm file)
			_, argsParallel, err := executor.buildNONMEMCommand(tt.modelPath, true, 4, false, nil)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(argsParallel) < 4 {
				t.Fatalf("Expected at least 4 arguments for parallel, got %d", len(argsParallel))
			}
			if argsParallel[1] != tt.expectedOutputFile {
				t.Errorf("Output file mismatch in parallel:\n  Expected: %s\n  Actual:   %s", tt.expectedOutputFile, argsParallel[1])
			}
			if argsParallel[3] != tt.expectedPnmFile {
				t.Errorf("PNM file mismatch in parallel:\n  Expected: %s\n  Actual:   %s", tt.expectedPnmFile, argsParallel[3])
			}
		})
	}
}

func TestNONMEMExecutor_CommandVerification_ExampleUsage(t *testing.T) {
	// This test demonstrates how to verify EXACT command execution for NONMEM
	// This is the key value - we can test the EXACT binary and arguments without execution

	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/opt/NONMEM/nm76/run",
			NonmemBinary: "nmfe76",
		},
	}

	executor := NewNONMEMExecutor(cfg).(*NONMEMExecutor)

	// Test case: User specifies additional NONMEM options from the UI
	modelPath := "/project/models/final_run.mod"
	userOptions := []string{"-maxeval=9999", "-files=100", "-sigdigits=4"}

	t.Run("verify_exact_command_construction", func(t *testing.T) {
		// This is what gets executed when user clicks "Run Here" with parallel mode
		binaryPath, args, err := executor.buildNONMEMCommand(
			modelPath,
			true,  // parallel execution
			8,     // 8 cores
			false, // local (not grid)
			userOptions,
		)

		if err != nil {
			t.Fatalf("Command construction failed: %v", err)
		}

		// VERIFY EXACT BINARY PATH
		expectedBinary := filepath.Join("/opt/NONMEM/nm76/run", "nmfe76")
		actualBinaryAbs, _ := filepath.Abs(binaryPath)
		expectedBinaryAbs, _ := filepath.Abs(expectedBinary)

		if actualBinaryAbs != expectedBinaryAbs {
			t.Errorf("CRITICAL: Wrong binary will be executed!\n  Expected: %s\n  Actual:   %s",
				expectedBinaryAbs, actualBinaryAbs)
		}

		// VERIFY EXACT ARGUMENT ORDER (critical for NONMEM)
		expectedArgs := []string{
			"/project/models/final_run.mod", // [0] Model file (required first)
			"/project/models/final_run.lst", // [1] Output file (required second)
			"-parallel",                     // [2] Parallel flag
			"/project/models/final_run.pnm", // [3] Parallel config file
			"-maxeval=9999",                 // [4] User option 1
			"-files=100",                    // [5] User option 2
			"-sigdigits=4",                  // [6] User option 3
		}

		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("CRITICAL: Wrong command arguments!\n  Expected: %v\n  Actual:   %v",
				expectedArgs, args)

			// Detailed comparison for debugging
			for i, expected := range expectedArgs {
				if i >= len(args) {
					t.Errorf("  Missing argument at position %d: %s", i, expected)
				} else if args[i] != expected {
					t.Errorf("  Mismatch at position %d: expected %s, got %s", i, expected, args[i])
				}
			}
		}

		// VERIFY ORDINALITY - Order matters for NONMEM!
		if len(args) >= 2 {
			if args[0] != modelPath {
				t.Errorf("CRITICAL: Model file must be first argument, got: %s", args[0])
			}
			if args[1] != "/project/models/final_run.lst" {
				t.Errorf("CRITICAL: Output file must be second argument, got: %s", args[1])
			}
		}

		if len(args) >= 4 {
			if args[2] != "-parallel" {
				t.Errorf("CRITICAL: -parallel flag must be third argument for parallel runs, got: %s", args[2])
			}
			if args[3] != "/project/models/final_run.pnm" {
				t.Errorf("CRITICAL: PNM file must be fourth argument for parallel runs, got: %s", args[3])
			}
		}

		// Verify user options appear AFTER system options in correct order
		if len(args) >= 7 {
			userOptionStart := 4 // After model, output, -parallel, pnm
			for i, expectedOption := range userOptions {
				actualIndex := userOptionStart + i
				if args[actualIndex] != expectedOption {
					t.Errorf("CRITICAL: User option %d wrong: expected %s, got %s",
						i+1, expectedOption, args[actualIndex])
				}
			}
		}

		// OUTPUT THE EXACT COMMAND FOR VERIFICATION
		t.Logf("EXACT COMMAND THAT WOULD BE EXECUTED:")
		t.Logf("  Binary: %s", binaryPath)
		t.Logf("  Args:   %v", args)
		t.Logf("  Full command: %s %s", binaryPath, strings.Join(args, " "))
	})

	t.Run("synchronous_vs_parallel_difference", func(t *testing.T) {
		// Compare synchronous vs parallel to ensure correct differences

		// Synchronous
		_, syncArgs, err := executor.buildNONMEMCommand(modelPath, false, 1, false, userOptions)
		if err != nil {
			t.Fatalf("Sync command failed: %v", err)
		}

		// Parallel
		_, parallelArgs, err := executor.buildNONMEMCommand(modelPath, true, 8, false, userOptions)
		if err != nil {
			t.Fatalf("Parallel command failed: %v", err)
		}

		// Verify the difference is exactly the parallel options
		expectedSyncArgs := []string{
			"/project/models/final_run.mod",
			"/project/models/final_run.lst",
			"-maxeval=9999",
			"-files=100",
			"-sigdigits=4",
		}

		expectedParallelArgs := []string{
			"/project/models/final_run.mod",
			"/project/models/final_run.lst",
			"-parallel",
			"/project/models/final_run.pnm",
			"-maxeval=9999",
			"-files=100",
			"-sigdigits=4",
		}

		if !reflect.DeepEqual(syncArgs, expectedSyncArgs) {
			t.Errorf("Sync args mismatch:\n  Expected: %v\n  Actual: %v", expectedSyncArgs, syncArgs)
		}

		if !reflect.DeepEqual(parallelArgs, expectedParallelArgs) {
			t.Errorf("Parallel args mismatch:\n  Expected: %v\n  Actual: %v", expectedParallelArgs, parallelArgs)
		}

		t.Logf("SYNCHRONOUS COMMAND: %v", syncArgs)
		t.Logf("PARALLEL COMMAND:    %v", parallelArgs)
	})
}
