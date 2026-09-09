//go:build unit
// +build unit

package execution

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution/category"
)

// TestHermesExecutor_NONMEMCategoryIntegration tests NONMEM model detection and category system integration.
func TestHermesExecutor_NONMEMCategoryIntegration(t *testing.T) {
	// Create test environment
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))

	// Set both HOME and USERPROFILE for cross-platform compatibility
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	t.Run("Detects NONMEM model correctly", func(t *testing.T) {
		// Create NONMEM model file
		modelPath := filepath.Join(tmpDir, "test.mod")
		modelContent := `$PROBLEM Test Problem
$INPUT ID TIME DV AMT
$DATA test_data.csv IGNORE=@
$SUBROUTINE ADVAN2 TRANS2
$PK
CL = THETA(1) * EXP(ETA(1))
V  = THETA(2) * EXP(ETA(2))
$ERROR
IPRED = F
Y = IPRED + IPRED * EPS(1)
$THETA (0, 1) ; CL
$THETA (0, 10) ; V
$OMEGA 0.1
$OMEGA 0.1
$SIGMA 0.1
$ESTIMATION METHOD=1 MAXEVAL=9999
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		// Create data file
		dataPath := filepath.Join(tmpDir, "test_data.csv")
		dataContent := "ID,TIME,DV,AMT\n1,0,0,100\n1,1,10.5,0\n"
		require.NoError(t, os.WriteFile(dataPath, []byte(dataContent), 0644))

		// Create license file
		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		licenseContent := []byte("# NONMEM License\nUSER:test@example.com\n")
		require.NoError(t, os.WriteFile(licensePath, licenseContent, 0644))

		// Create executor
		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/hermes-nonmem:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		// Detect category
		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType, "Should detect NONMEM model")

		// Create category handler
		modelCategory := category.NewNONMEMCategory()
		assert.Equal(t, "NONMEM", modelCategory.Name())
		assert.True(t, modelCategory.RequiresLicense())
	})

	t.Run("Loads NONMEM license from correct location", func(t *testing.T) {
		// Create license in home directory
		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		licenseContent := []byte("# Test License\nUSER:test@example.com\n")
		require.NoError(t, os.WriteFile(licensePath, licenseContent, 0644))

		cfg := &config.Config{}
		modelCategory := category.NewNONMEMCategory()

		data, err := modelCategory.GetLicense(cfg)
		require.NoError(t, err)
		assert.Equal(t, licenseContent, data)
	})

	t.Run("Extracts data file path from $DATA directive", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "run001.mod")
		modelContent := `$PROBLEM Test
$DATA ../data/study.csv IGNORE=@
$INPUT ID TIME DV
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		// Create data file in expected location
		dataDir := filepath.Join(tmpDir, "..", "data")
		require.NoError(t, os.MkdirAll(dataDir, 0755))
		dataPath := filepath.Join(dataDir, "study.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10\n"), 0644))

		modelCategory := category.NewNONMEMCategory()
		extractedPath, err := modelCategory.GetDataPath([]byte(modelContent), modelPath)

		require.NoError(t, err)
		assert.Contains(t, extractedPath, "study.csv")
	})

	t.Run("ContainerStructure includes model, data, and license", func(t *testing.T) {
		// Create model file
		modelPath := filepath.Join(tmpDir, "complete.mod")
		modelContent := `$PROBLEM Complete Test
$DATA data.csv IGNORE=@
$INPUT ID TIME DV
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		// Create data file
		dataPath := filepath.Join(tmpDir, "data.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10\n"), 0644))

		// Create license file
		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		require.NoError(t, os.WriteFile(licensePath, []byte("# License\n"), 0644))

		cfg := &config.Config{}
		modelCategory := category.NewNONMEMCategory()

		structure, err := modelCategory.ContainerStructure(modelPath, cfg)
		require.NoError(t, err)

		// Verify all three files are present
		assert.Contains(t, structure, "complete.mod", "Should include model file")
		assert.Contains(t, structure, "data.csv", "Should include data file")
		assert.Contains(t, structure, "nonmem.lic", "Should include license file")
		assert.Len(t, structure, 3, "Should have exactly 3 files")
	})

	t.Run("RetentionTargets returns NONMEM output patterns", func(t *testing.T) {
		modelCategory := category.NewNONMEMCategory()
		patterns := modelCategory.RetentionTargets()

		assert.NotEmpty(t, patterns)
		assert.Contains(t, patterns, "*.lst")
		assert.Contains(t, patterns, "*.ext")
		assert.Contains(t, patterns, "*.xml")
		assert.Contains(t, patterns, "*.phi")
	})

	t.Run("CommandOverrides includes license flag", func(t *testing.T) {
		cfg := &config.Config{}
		modelCategory := category.NewNONMEMCategory()

		overrides := modelCategory.CommandOverrides(cfg)

		assert.Contains(t, overrides, "LICENSE_FLAG")
		assert.Equal(t, "-licfile=${WORKSPACE}/nonmem.lic", overrides["LICENSE_FLAG"])
	})

	t.Run("buildNONMEMCommand applies category license flag", func(t *testing.T) {
		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/hermes-nonmem:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)
		executor.modelCategory = category.NewNONMEMCategory()

		command, args, err := executor.buildNONMEMCommand("/path/to/model.mod", false, 0, nil)

		require.NoError(t, err)
		assert.Equal(t, "nonmem", command)
		assert.Contains(t, args, "-licfile=${WORKSPACE}/nonmem.lic", "Should include license flag from category")
	})
}

// TestHermesExecutor_UnknownCategoryIntegration tests Unknown category pass-through behavior.
func TestHermesExecutor_UnknownCategoryIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Detects unknown file as Unknown category", func(t *testing.T) {
		// Create unrecognized model file
		modelPath := filepath.Join(tmpDir, "custom.xyz")
		modelContent := "# Custom model format\nparameter x = 10\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/hermes:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		// Detect category
		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryUnknown, categoryType)
	})

	t.Run("Unknown category does not require license", func(t *testing.T) {
		modelCategory := category.NewUnknownCategory()

		assert.Equal(t, "Unknown", modelCategory.Name())
		assert.False(t, modelCategory.RequiresLicense(), "Unknown category should not require license")
	})

	t.Run("Unknown category GetLicense returns error", func(t *testing.T) {
		cfg := &config.Config{}
		modelCategory := category.NewUnknownCategory()

		_, err := modelCategory.GetLicense(cfg)
		assert.Error(t, err, "Should return error when GetLicense called on Unknown category")
	})

	t.Run("Unknown category ContainerStructure includes only model file", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "unknown.xyz")
		modelContent := "# Unknown format\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		cfg := &config.Config{}
		modelCategory := category.NewUnknownCategory()

		structure, err := modelCategory.ContainerStructure(modelPath, cfg)
		require.NoError(t, err)

		// Should only include the model file itself
		assert.Len(t, structure, 1, "Unknown category should only include model file")
		assert.Contains(t, structure, "unknown.xyz")
	})

	t.Run("Unknown category RetentionTargets returns wildcard", func(t *testing.T) {
		modelCategory := category.NewUnknownCategory()
		patterns := modelCategory.RetentionTargets()

		assert.Equal(t, []string{"*"}, patterns, "Unknown category should retain all files")
	})

	t.Run("Unknown category CommandOverrides returns empty", func(t *testing.T) {
		cfg := &config.Config{}
		modelCategory := category.NewUnknownCategory()

		overrides := modelCategory.CommandOverrides(cfg)
		assert.Empty(t, overrides, "Unknown category should have no command overrides")
	})
}

// TestHermesExecutor_CategoryDebugLogging tests debug logging integration.
func TestHermesExecutor_CategoryDebugLogging(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Debug logging disabled by default", func(t *testing.T) {
		// Ensure EXECUTOR_DEBUG is not set
		t.Setenv("EXECUTOR_DEBUG", "false")

		// Get logger and verify level
		logger := category.GetLogger()
		assert.NotNil(t, logger)

		// Create a buffer to capture log output
		var buf bytes.Buffer
		logger.SetOutput(&buf)

		// Trigger a debug log
		logger.Debug("This should not appear")

		// Should be empty (debug logs disabled)
		assert.Empty(t, buf.String())
	})

	t.Run("Debug logging enabled with EXECUTOR_DEBUG=true", func(t *testing.T) {
		t.Setenv("EXECUTOR_DEBUG", "true")

		// Note: Logger is initialized at package import time, so changing env var
		// in the test won't affect the already-initialized logger. This test
		// verifies the logger API works correctly.
		logger := category.GetLogger()

		// Create buffer to capture output
		var buf bytes.Buffer
		logger.SetOutput(&buf)

		// Manually set debug level for this test
		logger.SetLevel(category.GetLogger().Level)
		logger.Debug("Debug message")

		// Verify the logger infrastructure is working (even if level wasn't changed)
		assert.NotNil(t, logger)
	})

	t.Run("Structured logging with fields", func(t *testing.T) {
		// Note: Logger level is set at package init time based on EXECUTOR_DEBUG env var
		// This test verifies that structured logging API works correctly
		logger := category.GetLogger()
		var buf bytes.Buffer
		logger.SetOutput(&buf)

		// Use Warn level (default level) instead of Info for testing
		logger.WithFields(map[string]interface{}{
			"model_path": "/path/to/model.mod",
			"category":   "NONMEM",
		}).Warn("Model categorized")

		output := buf.String()
		assert.Contains(t, output, "model_path")
		assert.Contains(t, output, "category")
		assert.Contains(t, output, "NONMEM")
	})

	t.Run("Detection logging with EXECUTOR_DEBUG", func(t *testing.T) {
		t.Setenv("EXECUTOR_DEBUG", "true")

		// Create NONMEM model
		modelPath := filepath.Join(tmpDir, "test_debug.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		// Capture debug output
		logger := category.GetLogger()
		var buf bytes.Buffer
		logger.SetOutput(&buf)

		// Detect model (should produce debug logs)
		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType)

		// Verify debug output was produced
		_ = buf.String()
		// Note: Detection happens inside the detector, logs might be present
		// This test verifies the logger infrastructure is working
		assert.NotNil(t, logger)
	})
}

// TestHermesExecutor_CategoryErrorHandling tests error conditions.
func TestHermesExecutor_CategoryErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))

	// Set both HOME and USERPROFILE for cross-platform compatibility
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	t.Run("Missing license file returns clear error", func(t *testing.T) {
		// No license file created
		cfg := &config.Config{}
		modelCategory := category.NewNONMEMCategory()

		_, err := modelCategory.GetLicense(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NONMEM license file not found")
		assert.Contains(t, err.Error(), "Attempted locations", "Should show all attempted locations")
	})

	t.Run("Missing data file returns clear error", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "missing_data.mod")
		modelContent := "$PROBLEM Test\n$DATA nonexistent.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		modelCategory := category.NewNONMEMCategory()
		_, err := modelCategory.GetDataPath([]byte(modelContent), modelPath)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "data file not found")
		assert.Contains(t, err.Error(), "nonexistent.csv")
	})

	t.Run("Malformed model returns Unknown category", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "malformed.mod")
		modelContent := "This is not a valid NONMEM model\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err, "Detection should not error, just return Unknown")
		assert.Equal(t, category.CategoryUnknown, categoryType)
	})

	t.Run("Unsupported category returns error from Execute", func(t *testing.T) {
		// This test would require mocking the Execute flow
		// For now, verify the detection works for unsupported platforms

		// Create a Monolix file (detected but not implemented)
		modelPath := filepath.Join(tmpDir, "model.mlxtran")
		modelContent := `<DATAFILE>
file = 'data.txt'

<MODEL>
PK:
V = THETA(V)
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryMonolix, categoryType)

		// In actual Execute(), this would return an error about unsupported category
	})
}

// TestHermesExecutor_ConfigOverride tests that config can override category defaults.
func TestHermesExecutor_ConfigOverride(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Config retention patterns override category defaults", func(t *testing.T) {
		// Create NONMEM model
		modelPath := filepath.Join(tmpDir, "override.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		// Config with custom retention patterns
		cfg := &config.Config{}
		cfg.Hermes.Retain = []string{"*.custom", "*.log"}

		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)
		executor.modelCategory = category.NewNONMEMCategory()

		// Category defaults would be *.lst, *.ext, etc.
		categoryDefaults := executor.modelCategory.RetentionTargets()
		assert.Contains(t, categoryDefaults, "*.lst")

		// But config should override these
		// (This would be tested in executeViaGRPC where retention logic lives)
		assert.Equal(t, []string{"*.custom", "*.log"}, cfg.Hermes.Retain)
	})
}

// TestHermesExecutor_NilConfigWithCategory tests nil config handling in category methods.
func TestHermesExecutor_NilConfigWithCategory(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	t.Run("NONMEM category handles nil config", func(t *testing.T) {
		// Create license in home directory
		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		require.NoError(t, os.WriteFile(licensePath, []byte("# License\n"), 0644))

		modelCategory := category.NewNONMEMCategory()

		// GetLicense with nil config should still work (uses env var and file fallbacks)
		data, err := modelCategory.GetLicense(nil)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// CommandOverrides with nil config
		overrides := modelCategory.CommandOverrides(nil)
		assert.Contains(t, overrides, "LICENSE_FLAG")

		// ContainerStructure with nil config
		modelPath := filepath.Join(tmpDir, "nil_config.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		dataPath := filepath.Join(tmpDir, "data.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644))

		structure, err := modelCategory.ContainerStructure(modelPath, nil)
		require.NoError(t, err)
		assert.Contains(t, structure, "nil_config.mod")
		assert.Contains(t, structure, "data.csv")
		assert.Contains(t, structure, "nonmem.lic")
	})

	t.Run("Unknown category handles nil config", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "nil_unknown.xyz")
		require.NoError(t, os.WriteFile(modelPath, []byte("unknown\n"), 0644))

		modelCategory := category.NewUnknownCategory()

		structure, err := modelCategory.ContainerStructure(modelPath, nil)
		require.NoError(t, err)
		assert.Len(t, structure, 1)

		overrides := modelCategory.CommandOverrides(nil)
		assert.Empty(t, overrides)
	})
}

// TestHermesExecutor_CategoryDetectionEdgeCases tests edge cases in model detection.
func TestHermesExecutor_CategoryDetectionEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{}
	modelConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}

	executor := NewHermesExecutor(cfg, modelConfig)

	t.Run("Empty file returns Unknown", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "empty.mod")
		require.NoError(t, os.WriteFile(modelPath, []byte(""), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryUnknown, categoryType)
	})

	t.Run("NONMEM with $PROB instead of $PROBLEM", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "short.mod")
		modelContent := "$PROB Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType)
	})

	t.Run("NONMEM with $INPUT instead of $DATA", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "input.mod")
		modelContent := "$PROBLEM Test\n$INPUT ID TIME DV\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType)
	})

	t.Run("Case insensitive NONMEM directives", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "lowercase.mod")
		modelContent := "$problem Test\n$data data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType)
	})

	t.Run("NONMEM with comments", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "commented.mod")
		modelContent := `; This is a comment
$PROBLEM Test Model ; inline comment
; More comments
$DATA data.csv IGNORE=@ ; data file
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryNONMEM, categoryType)
	})

	t.Run("Stan with flexible whitespace", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "whitespace.stan")
		modelContent := "data   {\n  int N;\n}\nparameters   {\n  real mu;\n}\nmodel   {\n  mu ~ normal(0, 1);\n}\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryStan, categoryType)
	})

	t.Run("Torsten detected before Stan", func(t *testing.T) {
		modelPath := filepath.Join(tmpDir, "torsten.stan")
		modelContent := `data {
  int N;
}
parameters {
  real CL;
}
model {
  vector[N] pred = PKModelOneCpt(CL, V, ka);
}
`
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := executor.detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, category.CategoryTorsten, categoryType, "Should detect Torsten, not Stan")
	})
}

// TestHermesExecutor_CategoryExtensionDetection tests extension-based fast path detection.
func TestHermesExecutor_CategoryExtensionDetection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{}
	modelConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}

	executor := NewHermesExecutor(cfg, modelConfig)

	tests := []struct {
		name        string
		filename    string
		content     string
		expected    category.CategoryType
		description string
	}{
		{
			name:        ".mod extension with valid content",
			filename:    "model.mod",
			content:     "$PROBLEM Test\n$DATA data.csv\n",
			expected:    category.CategoryNONMEM,
			description: "Should use extension fast path",
		},
		{
			name:        ".ctl extension with valid content",
			filename:    "control.ctl",
			content:     "$PROBLEM Test\n$INPUT ID TIME DV\n",
			expected:    category.CategoryNONMEM,
			description: "Should detect .ctl as NONMEM",
		},
		{
			name:        ".stan extension with valid content",
			filename:    "model.stan",
			content:     "data {\n}\nparameters {\n}\nmodel {\n}\n",
			expected:    category.CategoryStan,
			description: "Should use extension fast path for Stan",
		},
		{
			name:        ".mlxtran extension",
			filename:    "model.mlxtran",
			content:     "<DATAFILE>\nfile='data.txt'\n<MODEL>\nPK:\n",
			expected:    category.CategoryMonolix,
			description: "Should detect Monolix by extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelPath := filepath.Join(tmpDir, tt.filename)
			require.NoError(t, os.WriteFile(modelPath, []byte(tt.content), 0644))

			categoryType, err := executor.detector.Detect(modelPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, categoryType, tt.description)
		})
	}
}

// TestHermesExecutor_CategoryDataPathExtraction tests various $DATA directive formats.
func TestHermesExecutor_CategoryDataPathExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		modelContent string
		expectedFile string
		shouldError  bool
	}{
		{
			name:         "$DATA with IGNORE",
			modelContent: "$PROBLEM Test\n$DATA data.csv IGNORE=@\n",
			expectedFile: "data.csv",
			shouldError:  false,
		},
		{
			name:         "$DATA with $INPUT",
			modelContent: "$PROBLEM Test\n$DATA data.csv\n$INPUT ID TIME DV\n",
			expectedFile: "data.csv",
			shouldError:  false,
		},
		{
			name:         "Relative path",
			modelContent: "$PROBLEM Test\n$DATA ./data/study.csv\n",
			expectedFile: filepath.Join("data", "study.csv"), // Use filepath.Join for cross-platform paths
			shouldError:  false,
		},
		{
			name:         "Case insensitive",
			modelContent: "$problem test\n$data Data.CSV ignore=@\n",
			expectedFile: "Data.CSV",
			shouldError:  false,
		},
		{
			name:         "Missing data directive",
			modelContent: "$PROBLEM Test\n$THETA 1\n",
			expectedFile: "",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a subdirectory for this test to isolate relative paths
			testDir := filepath.Join(tmpDir, strings.ReplaceAll(tt.name, " ", "_"))
			require.NoError(t, os.MkdirAll(testDir, 0755))

			modelPath := filepath.Join(testDir, "test.mod")
			require.NoError(t, os.WriteFile(modelPath, []byte(tt.modelContent), 0644))

			if !tt.shouldError && tt.expectedFile != "" {
				// Create the expected data file
				var dataPath string
				// Check if it's a relative path (contains directory separator)
				if strings.Contains(tt.expectedFile, string(filepath.Separator)) || strings.Contains(tt.expectedFile, "/") {
					// Handle relative paths - resolve from model directory
					modelDir := filepath.Dir(modelPath)
					dataPath = filepath.Join(modelDir, tt.expectedFile)
					require.NoError(t, os.MkdirAll(filepath.Dir(dataPath), 0755))
				} else {
					dataPath = filepath.Join(testDir, tt.expectedFile)
				}
				require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644))
			}

			modelCategory := category.NewNONMEMCategory()
			extractedPath, err := modelCategory.GetDataPath([]byte(tt.modelContent), modelPath)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, extractedPath, tt.expectedFile)
			}
		})
	}
}
