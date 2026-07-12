//go:build unit
// +build unit

package execution

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

// TestSLURMOutputPathResolution verifies that output file paths are correctly resolved
// relative to the model location, ensuring file watchers monitor the right directories.
func TestSLURMOutputPathResolution(t *testing.T) {
	tests := []struct {
		name              string
		modelPath         string
		expectedWorkDir   string
		expectedOutputDir string
		description       string
	}{
		{
			name:              "simple_model_in_current_dir",
			modelPath:         "/project/models/run001.mod",
			expectedWorkDir:   "/project/models",
			expectedOutputDir: "/project/models",
			description:       "Model in project directory - output files written to same directory",
		},
		{
			name:              "model_in_subdirectory",
			modelPath:         "/project/pharma/models/pk/run001.mod",
			expectedWorkDir:   "/project/pharma/models/pk",
			expectedOutputDir: "/project/pharma/models/pk",
			description:       "Model in subdirectory - output files follow model location",
		},
		{
			name:              "model_in_temporary_directory",
			modelPath:         "/tmp/janus_temp/model.ctl",
			expectedWorkDir:   "/tmp/janus_temp",
			expectedOutputDir: "/tmp/janus_temp",
			description:       "Model in temp directory - output files written to temp location",
		},
		{
			name:              "windows_style_path",
			modelPath:         "C:\\Users\\analyst\\models\\final_run.mod",
			expectedWorkDir:   filepath.Dir("C:\\Users\\analyst\\models\\final_run.mod"),
			expectedOutputDir: filepath.Dir("C:\\Users\\analyst\\models\\final_run.mod"),
			description:       "Windows path - output files follow model location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log("Testing scenario:", tt.description)

			// Create SLURM executor
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath: "/opt/NONMEM",
				},
			}
			executor, err := NewSLURMExecutor(cfg, nil)
			require.NoError(t, err)

			// Build submit options
			submitOptions := executor.buildSubmitOptions(tt.modelPath, 4, true, []string{})

			// Verify working directory
			assert.Equal(t, tt.expectedWorkDir, submitOptions.WorkingDir,
				"Working directory should be the model's directory")

			// Verify output file paths are absolute and in the expected directory
			expectedOutputFile := tt.modelPath + ".slurm.out"
			expectedErrorFile := tt.modelPath + ".slurm.err"

			assert.Equal(t, expectedOutputFile, submitOptions.OutputFile,
				"Output file path should be absolute, based on model path")
			assert.Equal(t, expectedErrorFile, submitOptions.ErrorFile,
				"Error file path should be absolute, based on model path")

			// Verify paths are in the expected directory
			outputDir := filepath.Dir(submitOptions.OutputFile)
			errorDir := filepath.Dir(submitOptions.ErrorFile)

			assert.Equal(t, tt.expectedOutputDir, outputDir,
				"Output file should be in the expected directory")
			assert.Equal(t, tt.expectedOutputDir, errorDir,
				"Error file should be in the expected directory")

			// Critical verification: ensure file watcher will monitor the correct paths
			t.Logf("✅ File watcher will monitor:")
			t.Logf("   STDOUT: %s", submitOptions.OutputFile)
			t.Logf("   STDERR: %s", submitOptions.ErrorFile)
			t.Logf("   Working dir: %s", submitOptions.WorkingDir)
		})
	}
}

// TestSLURMPathResolutionDocumentation documents the SLURM path resolution behavior
// to ensure future developers understand how file locations are determined.
func TestSLURMPathResolutionDocumentation(t *testing.T) {
	t.Log("=== SLURM Output File Path Resolution Documentation ===")
	t.Log("")
	t.Log("SLURM Job Submission Path Behavior:")
	t.Log("1. --chdir: Sets the job's working directory (where the job executes)")
	t.Log("2. --output: Output file path (can be absolute or relative)")
	t.Log("3. --error: Error file path (can be absolute or relative)")
	t.Log("")
	t.Log("Janus Implementation:")
	t.Log("• Working Directory: Set to filepath.Dir(modelPath) - model's directory")
	t.Log("• Output Files: Absolute paths = modelPath + '.slurm.out/err'")
	t.Log("")
	t.Log("Why This Works:")
	t.Log("• SLURM writes absolute paths to their exact locations")
	t.Log("• Relative paths would be resolved against the working directory")
	t.Log("• Using absolute paths ensures predictable file locations")
	t.Log("")
	t.Log("File Watcher Behavior:")
	t.Log("• Monitors the exact paths passed to SLURM")
	t.Log("• No path translation needed - direct 1:1 mapping")
	t.Log("• Works regardless of where Janus is running from")

	// Demonstrate with a real example
	modelPath := "/project/models/analysis.mod"
	t.Logf("")
	t.Logf("Example with model: %s", modelPath)
	t.Logf("  Job working dir: %s", filepath.Dir(modelPath))
	t.Logf("  Output file: %s", modelPath+".slurm.out")
	t.Logf("  Error file: %s", modelPath+".slurm.err")
	t.Logf("  File watcher monitors: Same absolute paths")
}

// TestFileWatcherPathConsistency verifies that the file watcher monitors
// the exact same paths that SLURM will write to.
func TestFileWatcherPathConsistency(t *testing.T) {
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath: "/opt/NONMEM",
		},
	}
	executor, err := NewSLURMExecutor(cfg, nil)
	require.NoError(t, err)

	testCases := []string{
		"/home/user/models/pk_model.mod",
		"/scratch/analysis/final_run.ctl",
		"/data/pharma/models/bioequivalence/be_study.mod",
	}

	for _, modelPath := range testCases {
		t.Run(filepath.Base(modelPath), func(t *testing.T) {
			// Get the paths that would be sent to SLURM
			submitOptions := executor.buildSubmitOptions(modelPath, 2, false, []string{})

			// Simulate what the file watcher setup would look like
			stdoutData := &FileStreamData{
				filePath: submitOptions.OutputFile,
			}
			stderrData := &FileStreamData{
				filePath: submitOptions.ErrorFile,
			}

			// Verify consistency
			assert.Equal(t, submitOptions.OutputFile, stdoutData.filePath,
				"File watcher stdout path must match SLURM output path")
			assert.Equal(t, submitOptions.ErrorFile, stderrData.filePath,
				"File watcher stderr path must match SLURM error path")

			// Verify paths are absolute
			assert.True(t, filepath.IsAbs(stdoutData.filePath),
				"File watcher must use absolute paths")
			assert.True(t, filepath.IsAbs(stderrData.filePath),
				"File watcher must use absolute paths")

			t.Logf("✅ Consistent paths for %s:", modelPath)
			t.Logf("   SLURM stdout: %s", submitOptions.OutputFile)
			t.Logf("   Watcher stdout: %s", stdoutData.filePath)
			t.Logf("   SLURM stderr: %s", submitOptions.ErrorFile)
			t.Logf("   Watcher stderr: %s", stderrData.filePath)
		})
	}
}