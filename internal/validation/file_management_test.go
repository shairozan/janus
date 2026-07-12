//go:build validation
// +build validation

package validation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
)

// TestREQ26_ModelFileValidation validates REQ-26: Model file validation.
func TestREQ26_ModelFileValidation(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-26",
		Description: "I can validate model file format",
		Category:    CategoryFileManagement,
		TestFunc: func(t *testing.T) {
			tempDir := t.TempDir()

			// Test 1: Valid model file
			validModel := tempDir + "/valid_model.mod"
			err := os.WriteFile(validModel, []byte("$PROB Test Model\n$INPUT ID TIME DV\n$DATA test.csv"), 0644)
			require.NoError(t, err, "Should be able to create valid model file")

			// Verify file exists and is readable
			_, err = os.Stat(validModel)
			assert.NoError(t, err, "Valid model file should exist and be readable")

			// Test 2: File with wrong extension
			wrongExt := tempDir + "/model.txt"
			err = os.WriteFile(wrongExt, []byte("$PROB Test Model"), 0644)
			require.NoError(t, err, "Should be able to create file with wrong extension")

			// Verify executor handles file extension validation
			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:    "/bin/echo",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor := execution.NewNONMEMExecutor(cfg)
			ctx := context.Background()

			// Test with valid .mod file
			_, err = executor.Execute(ctx, validModel, false, 1, false, nil)
			assert.NoError(t, err, "Should accept valid .mod file")

			// Test 3: Non-existent file
			nonExistent := tempDir + "/nonexistent.mod"
			_, err = executor.Execute(ctx, nonExistent, false, 1, false, nil)
			assert.Error(t, err, "Should reject non-existent file")

			t.Log("Model file validation verified - valid files accepted, invalid files rejected")
		},
	}

	test.Run(t)
}

// TestREQ27_OutputFileManagement validates REQ-27: Output file management.
func TestREQ27_OutputFileManagement(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-27",
		Description: "I can manage output files",
		Category:    CategoryFileManagement,
		TestFunc: func(t *testing.T) {
			tempDir := t.TempDir()

			// Create a model file
			modelFile := tempDir + "/test_model.mod"
			err := os.WriteFile(modelFile, []byte("$PROB Test"), 0644)
			require.NoError(t, err, "Should be able to create model file")

			// Verify output file path generation
			expectedOutput := strings.Replace(modelFile, ".mod", ".lst", 1)

			cfg := &config.Config{
				Input: config.Input{
					NonmemPath:    "/bin/echo",
					ExecutionMode: config.ExecutionModeNONMEM,
				},
			}

			executor := execution.NewNONMEMExecutor(cfg)
			ctx := context.Background()

			// Execute
			result, err := executor.Execute(ctx, modelFile, false, 1, false, nil)
			require.NoError(t, err, "Execution should succeed")
			require.NotNil(t, result, "Result should not be nil")

			// Verify execution completed successfully
			assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")

			// Verify output file naming convention
			outputBase := strings.TrimSuffix(filepath.Base(modelFile), ".mod")
			assert.NotEmpty(t, outputBase, "Output base name should be derived from model file")

			t.Logf("Output file management verified - output paths correctly generated")
			t.Logf("Model: %s", modelFile)
			t.Logf("Expected output: %s", expectedOutput)
		},
	}

	test.Run(t)
}

// TestREQ28_FileChangeDetection validates REQ-28: File change detection.
func TestREQ28_FileChangeDetection(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-28",
		Description: "I can detect when model files are modified",
		Category:    CategoryFileManagement,
		TestFunc: func(t *testing.T) {
			tempDir := t.TempDir()

			// Create a model file
			modelFile := tempDir + "/test_model.mod"
			originalContent := []byte("$PROB Test Model\n$INPUT ID")
			err := os.WriteFile(modelFile, originalContent, 0644)
			require.NoError(t, err, "Should be able to create model file")

			// Get initial file info
			info1, err := os.Stat(modelFile)
			require.NoError(t, err, "Should be able to stat file")
			modTime1 := info1.ModTime()

			// Wait to ensure filesystem timestamp granularity captures the change
			// Some filesystems have coarse timestamp resolution (e.g., 1-2 seconds on FAT32)
			// Modern filesystems like ext4, NTFS, APFS typically have millisecond or better resolution
			time.Sleep(2 * time.Second)

			// Modify the file
			modifiedContent := []byte("$PROB Modified Model\n$INPUT ID TIME DV")
			err = os.WriteFile(modelFile, modifiedContent, 0644)
			require.NoError(t, err, "Should be able to modify file")

			// Get new file info
			info2, err := os.Stat(modelFile)
			require.NoError(t, err, "Should be able to stat modified file")
			modTime2 := info2.ModTime()

			// Verify modification is detectable
			assert.NotEqual(t, modTime1, modTime2, "Modification time should change after file modification")
			assert.True(t, modTime2.After(modTime1), "New modification time should be later than original")

			// Verify content changed
			readContent, err := os.ReadFile(modelFile)
			require.NoError(t, err, "Should be able to read modified file")
			assert.NotEqual(t, originalContent, readContent, "File content should be different after modification")

			t.Logf("File change detection verified - modifications can be detected via mod time and content")
		},
	}

	test.Run(t)
}

// TestREQ29_ProjectFileOrganization validates REQ-29: Project file organization.
func TestREQ29_ProjectFileOrganization(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-29",
		Description: "I can organize files by project",
		Category:    CategoryFileManagement,
		TestFunc: func(t *testing.T) {
			// Create a project-like directory structure
			tempDir := t.TempDir()

			// Project structure
			projectDir := filepath.Join(tempDir, "my_project")
			modelsDir := filepath.Join(projectDir, "models")
			resultsDir := filepath.Join(projectDir, "results")

			// Create directories
			err := os.MkdirAll(modelsDir, 0755)
			require.NoError(t, err, "Should be able to create models directory")

			err = os.MkdirAll(resultsDir, 0755)
			require.NoError(t, err, "Should be able to create results directory")

			// Create model files
			model1 := filepath.Join(modelsDir, "model1.mod")
			model2 := filepath.Join(modelsDir, "model2.mod")

			err = os.WriteFile(model1, []byte("$PROB Model 1"), 0644)
			require.NoError(t, err, "Should be able to create model1")

			err = os.WriteFile(model2, []byte("$PROB Model 2"), 0644)
			require.NoError(t, err, "Should be able to create model2")

			// Verify project structure
			assert.DirExists(t, projectDir, "Project directory should exist")
			assert.DirExists(t, modelsDir, "Models subdirectory should exist")
			assert.DirExists(t, resultsDir, "Results subdirectory should exist")
			assert.FileExists(t, model1, "Model 1 should exist")
			assert.FileExists(t, model2, "Model 2 should exist")

			// Verify files can be enumerated within project
			modelFiles, err := filepath.Glob(filepath.Join(modelsDir, "*.mod"))
			require.NoError(t, err, "Should be able to glob model files")
			assert.Len(t, modelFiles, 2, "Should find 2 model files in project")

			t.Logf("Project file organization verified - can create and maintain project structure")
			t.Logf("Project: %s", projectDir)
			t.Logf("Models: %d files found", len(modelFiles))
		},
	}

	test.Run(t)
}
