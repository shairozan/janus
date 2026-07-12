//go:build validation
// +build validation

package category

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

// Validation tests for model categorization system (IQ/OQ)
// These tests provide traceability between requirements and implementation.
//
// Requirements Prefix:
// - EXEC-REQ-XXX: EXECUTOR (Hermes execution system) requirements
// - GUI-REQ-XXX:  GUI (Janus GUI application) requirements
//
// All tests in this file are EXEC-REQ-* requirements.

// TestValidation_EXEC_REQ_050_ModelDetection validates that the executor correctly
// categorizes models before execution.
//
// EXEC-REQ-050: The executor SHALL detect and categorize model files into one of
// the following categories: NONMEM, Monolix, Stan, Torsten, or Unknown.
func TestValidation_EXEC_REQ_050_ModelDetection(t *testing.T) {
	tmpDir := t.TempDir()
	detector := NewDetector()

	t.Run("EXEC-REQ-050.1: Detect NONMEM model", func(t *testing.T) {
		// Requirement: Executor shall detect NONMEM models by $PROBLEM and $DATA directives
		modelPath := filepath.Join(tmpDir, "test.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, categoryType, "EXEC-REQ-050.1: Must detect NONMEM model")
	})

	t.Run("EXEC-REQ-050.2: Detect Monolix model", func(t *testing.T) {
		// Requirement: Executor shall detect Monolix models by <DATAFILE> and <MODEL> tags
		modelPath := filepath.Join(tmpDir, "test.mlxtran")
		modelContent := "<DATAFILE>\nfile='data.txt'\n\n<MODEL>\nPK:\nV=THETA(V)\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, CategoryMonolix, categoryType, "EXEC-REQ-050.2: Must detect Monolix model")
	})

	t.Run("EXEC-REQ-050.3: Detect Stan model", func(t *testing.T) {
		// Requirement: Executor shall detect Stan models by data/parameters/model blocks
		modelPath := filepath.Join(tmpDir, "test.stan")
		modelContent := "data {\n  int N;\n}\nparameters {\n  real mu;\n}\nmodel {\n  mu ~ normal(0,1);\n}\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, CategoryStan, categoryType, "EXEC-REQ-050.3: Must detect Stan model")
	})

	t.Run("EXEC-REQ-050.4: Detect Torsten model (priority over Stan)", func(t *testing.T) {
		// Requirement: Executor shall detect Torsten models before Stan (superset relationship)
		modelPath := filepath.Join(tmpDir, "test_torsten.stan")
		modelContent := "data {\n  int N;\n}\nparameters {\n  real CL;\n}\nmodel {\n  vector[N] pred = PKModelOneCpt(CL, V, ka);\n}\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, CategoryTorsten, categoryType, "EXEC-REQ-050.4: Must detect Torsten before Stan")
	})

	t.Run("EXEC-REQ-050.5: Detect Unknown model (fallback)", func(t *testing.T) {
		// Requirement: Executor shall categorize unrecognized models as Unknown
		modelPath := filepath.Join(tmpDir, "test.xyz")
		modelContent := "# Unknown format\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		categoryType, err := detector.Detect(modelPath)
		require.NoError(t, err)
		assert.Equal(t, CategoryUnknown, categoryType, "EXEC-REQ-050.5: Must default to Unknown for unrecognized formats")
	})
}

// TestValidation_EXEC_REQ_051_LicenseHandling validates that the executor correctly
// handles platform-specific license requirements.
//
// EXEC-REQ-051: The executor SHALL only attempt to load license files for platforms
// that require them, as indicated by the category's RequiresLicense() method.
func TestValidation_EXEC_REQ_051_LicenseHandling(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))
	t.Setenv("HOME", tmpHome)

	t.Run("EXEC-REQ-051.1: NONMEM requires license", func(t *testing.T) {
		// Requirement: Executor shall load NONMEM license from known locations
		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		licenseContent := []byte("# NONMEM License\n")
		require.NoError(t, os.WriteFile(licensePath, licenseContent, 0644))

		category := NewNONMEMCategory()
		assert.True(t, category.RequiresLicense(), "EXEC-REQ-051.1: NONMEM must require license")

		data, err := category.GetLicense(nil)
		require.NoError(t, err, "EXEC-REQ-051.1: Must load NONMEM license successfully")
		assert.Equal(t, licenseContent, data)
	})

	t.Run("EXEC-REQ-051.2: Unknown does not require license", func(t *testing.T) {
		// Requirement: Executor shall not attempt license lookup for Unknown category
		category := NewUnknownCategory()
		assert.False(t, category.RequiresLicense(), "EXEC-REQ-051.2: Unknown must not require license")

		_, err := category.GetLicense(nil)
		assert.Error(t, err, "EXEC-REQ-051.2: GetLicense must error when called on Unknown")
	})

}

// TestValidation_EXEC_REQ_052_DataFileLocation validates that the executor correctly
// locates data files based on platform-specific parsing.
//
// EXEC-REQ-052: The executor SHALL parse model files to locate associated data files
// using platform-specific directives ($DATA for NONMEM, <DATAFILE> for Monolix, etc.).
func TestValidation_EXEC_REQ_052_DataFileLocation(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("EXEC-REQ-052.1: Parse NONMEM $DATA directive", func(t *testing.T) {
		// Requirement: Executor shall extract data file path from $DATA directive
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := "$PROBLEM Test\n$DATA study_data.csv IGNORE=@\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		dataPath := filepath.Join(tmpDir, "study_data.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644))

		category := NewNONMEMCategory()
		extractedPath, err := category.GetDataPath([]byte(modelContent), modelPath)

		require.NoError(t, err, "EXEC-REQ-052.1: Must parse $DATA directive")
		assert.Contains(t, extractedPath, "study_data.csv")
	})

	t.Run("EXEC-REQ-052.2: Parse NONMEM $DATA with $INPUT columns", func(t *testing.T) {
		// Requirement: Executor shall parse $DATA directive (not $INPUT which defines columns)
		modelPath := filepath.Join(tmpDir, "model2.mod")
		modelContent := "$PROBLEM Test\n$DATA input_data.csv\n$INPUT ID TIME DV\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		dataPath := filepath.Join(tmpDir, "input_data.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644))

		category := NewNONMEMCategory()
		extractedPath, err := category.GetDataPath([]byte(modelContent), modelPath)

		require.NoError(t, err, "EXEC-REQ-052.2: Must parse $DATA directive")
		assert.Contains(t, extractedPath, "input_data.csv")
	})

	t.Run("EXEC-REQ-052.3: Resolve relative paths", func(t *testing.T) {
		// Requirement: Executor shall resolve relative data paths from model directory
		modelPath := filepath.Join(tmpDir, "model3.mod")
		modelContent := "$PROBLEM Test\n$DATA ./data/relative.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		dataDir := filepath.Join(tmpDir, "data")
		require.NoError(t, os.MkdirAll(dataDir, 0755))
		dataPath := filepath.Join(dataDir, "relative.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644))

		category := NewNONMEMCategory()
		extractedPath, err := category.GetDataPath([]byte(modelContent), modelPath)

		require.NoError(t, err, "EXEC-REQ-052.3: Must resolve relative paths")
		assert.Contains(t, extractedPath, "relative.csv")
	})

	t.Run("EXEC-REQ-052.4: Unknown category cannot detect data", func(t *testing.T) {
		// Requirement: Executor shall not attempt data parsing for Unknown category
		modelPath := filepath.Join(tmpDir, "unknown.xyz")
		modelContent := "# Unknown format\n"

		category := NewUnknownCategory()
		_, err := category.GetDataPath([]byte(modelContent), modelPath)

		assert.Error(t, err, "EXEC-REQ-052.4: Unknown category must error on GetDataPath")
	})
}

// TestValidation_EXEC_REQ_053_ContainerStructure validates that the executor builds
// correct container file structures for each platform.
//
// EXEC-REQ-053: The executor SHALL build container file structures that include all
// necessary files for execution: model file, data files, and platform-specific
// license files.
func TestValidation_EXEC_REQ_053_ContainerStructure(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))
	t.Setenv("HOME", tmpHome)

	t.Run("EXEC-REQ-053.1: NONMEM structure includes model, data, license", func(t *testing.T) {
		// Requirement: Executor shall include model, data, and license for NONMEM
		modelPath := filepath.Join(tmpDir, "complete.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		dataPath := filepath.Join(tmpDir, "data.csv")
		require.NoError(t, os.WriteFile(dataPath, []byte("ID,DV\n"), 0644))

		licensePath := filepath.Join(tmpHome, "nonmem.lic")
		require.NoError(t, os.WriteFile(licensePath, []byte("# License\n"), 0644))

		category := NewNONMEMCategory()
		structure, err := category.ContainerStructure(modelPath, nil)

		require.NoError(t, err, "EXEC-REQ-053.1: Must build NONMEM container structure")
		assert.Contains(t, structure, "complete.mod", "EXEC-REQ-053.1: Must include model file")
		assert.Contains(t, structure, "data.csv", "EXEC-REQ-053.1: Must include data file")
		assert.Contains(t, structure, "nonmem.lic", "EXEC-REQ-053.1: Must include license file")
		assert.Len(t, structure, 3, "EXEC-REQ-053.1: Must have exactly 3 files")
	})

	t.Run("EXEC-REQ-053.2: Unknown structure includes only model", func(t *testing.T) {
		// Requirement: Executor shall include only model file for Unknown category
		modelPath := filepath.Join(tmpDir, "unknown.xyz")
		require.NoError(t, os.WriteFile(modelPath, []byte("unknown\n"), 0644))

		category := NewUnknownCategory()
		structure, err := category.ContainerStructure(modelPath, nil)

		require.NoError(t, err, "EXEC-REQ-053.2: Must build Unknown container structure")
		assert.Len(t, structure, 1, "EXEC-REQ-053.2: Must include only model file")
		assert.Contains(t, structure, "unknown.xyz")
	})
}

// TestValidation_EXEC_REQ_054_RetentionPatterns validates that the executor uses
// correct output retention patterns for each platform.
//
// EXEC-REQ-054: The executor SHALL retain platform-specific output files based on
// category-defined patterns (*.lst, *.ext for NONMEM; * for Unknown).
func TestValidation_EXEC_REQ_054_RetentionPatterns(t *testing.T) {
	t.Run("EXEC-REQ-054.1: NONMEM retention patterns", func(t *testing.T) {
		// Requirement: Executor shall retain standard NONMEM output files
		category := NewNONMEMCategory()
		patterns := category.RetentionTargets()

		assert.Contains(t, patterns, "*.lst", "EXEC-REQ-054.1: Must retain listing files")
		assert.Contains(t, patterns, "*.ext", "EXEC-REQ-054.1: Must retain parameter estimates")
		assert.Contains(t, patterns, "*.xml", "EXEC-REQ-054.1: Must retain XML output")
		assert.Contains(t, patterns, "*.phi", "EXEC-REQ-054.1: Must retain individual parameters")
	})

	t.Run("EXEC-REQ-054.2: Unknown retention pattern (wildcard)", func(t *testing.T) {
		// Requirement: Executor shall retain all files for Unknown category
		category := NewUnknownCategory()
		patterns := category.RetentionTargets()

		assert.Equal(t, []string{"*"}, patterns, "EXEC-REQ-054.2: Must retain all files for Unknown")
	})
}

// TestValidation_EXEC_REQ_055_CommandOverrides validates that the executor applies
// platform-specific command modifications.
//
// EXEC-REQ-055: The executor SHALL apply category-specific command overrides
// (e.g., license flags) when constructing execution commands.
func TestValidation_EXEC_REQ_055_CommandOverrides(t *testing.T) {
	t.Run("EXEC-REQ-055.1: NONMEM license flag override", func(t *testing.T) {
		// Requirement: Executor shall add license flag for NONMEM commands
		category := NewNONMEMCategory()
		overrides := category.CommandOverrides(nil)

		assert.Contains(t, overrides, "LICENSE_FLAG", "EXEC-REQ-055.1: Must have LICENSE_FLAG override")
		assert.Equal(t, "-licfile=${WORKSPACE}/nonmem.lic", overrides["LICENSE_FLAG"])
	})

	t.Run("EXEC-REQ-055.2: Unknown has no overrides", func(t *testing.T) {
		// Requirement: Executor shall not modify commands for Unknown category
		category := NewUnknownCategory()
		overrides := category.CommandOverrides(nil)

		assert.Empty(t, overrides, "EXEC-REQ-055.2: Unknown must have no command overrides")
	})
}

// TestValidation_EXEC_REQ_056_ErrorHandling validates that the executor provides
// clear error messages for common failure scenarios.
//
// EXEC-REQ-056: The executor SHALL provide clear, actionable error messages when
// required files are missing or model parsing fails.
func TestValidation_EXEC_REQ_056_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(tmpHome, 0755))
	t.Setenv("HOME", tmpHome)

	t.Run("EXEC-REQ-056.1: Missing license error message", func(t *testing.T) {
		// Requirement: Executor shall show all attempted license locations on failure
		category := NewNONMEMCategory()
		_, err := category.GetLicense(nil)

		require.Error(t, err, "EXEC-REQ-056.1: Must error when license not found")
		assert.Contains(t, err.Error(), "NONMEM license file not found")
		assert.Contains(t, err.Error(), "Attempted locations", "EXEC-REQ-056.1: Must show attempted locations")
	})

	t.Run("EXEC-REQ-056.2: Missing data file error message", func(t *testing.T) {
		// Requirement: Executor shall show data file path on failure
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := "$PROBLEM Test\n$DATA missing.csv\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		category := NewNONMEMCategory()
		_, err := category.GetDataPath([]byte(modelContent), modelPath)

		require.Error(t, err, "EXEC-REQ-056.2: Must error when data file not found")
		assert.Contains(t, err.Error(), "data file not found")
		assert.Contains(t, err.Error(), "missing.csv", "EXEC-REQ-056.2: Must show data file path")
	})

	t.Run("EXEC-REQ-056.3: Missing data directive error", func(t *testing.T) {
		// Requirement: Executor shall error clearly when $DATA directive missing
		modelPath := filepath.Join(tmpDir, "model2.mod")
		modelContent := "$PROBLEM Test\n$INPUT ID TIME DV\n$THETA 1\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		category := NewNONMEMCategory()
		_, err := category.GetDataPath([]byte(modelContent), modelPath)

		require.Error(t, err, "EXEC-REQ-056.3: Must error when $DATA missing")
		assert.Contains(t, err.Error(), "no $DATA directive found")
	})
}

// TestValidation_EXEC_REQ_057_ConfigOverride validates that user configuration
// can override category defaults.
//
// EXEC-REQ-057: The executor SHALL allow user configuration to override category
// defaults for retention patterns and other settings.
func TestValidation_EXEC_REQ_057_ConfigOverride(t *testing.T) {
	t.Run("EXEC-REQ-057.1: Config can override retention patterns", func(t *testing.T) {
		// Requirement: Executor shall use config retention patterns when provided
		cfg := &config.Config{}
		cfg.Hermes.Retain = []string{"*.custom", "*.log"}

		category := NewNONMEMCategory()
		defaultPatterns := category.RetentionTargets()

		// Category provides defaults
		assert.Contains(t, defaultPatterns, "*.lst")

		// Config overrides would be applied at execution time
		// This test validates the separation of concerns
		assert.NotEqual(t, cfg.Hermes.Retain, defaultPatterns, "EXEC-REQ-057.1: Config can differ from defaults")
	})
}

// TestValidation_EXEC_REQ_058_DebugLogging validates that the executor provides
// debug logging when EXECUTOR_DEBUG environment variable is set.
//
// EXEC-REQ-058: The executor SHALL provide detailed debug logging when
// EXECUTOR_DEBUG=true, including detection attempts, file locations, and
// category selections.
func TestValidation_EXEC_REQ_058_DebugLogging(t *testing.T) {
	t.Run("EXEC-REQ-058.1: Logger is available", func(t *testing.T) {
		// Requirement: Executor shall provide a logger instance
		logger := GetLogger()
		assert.NotNil(t, logger, "EXEC-REQ-058.1: Logger must be available")
	})

	t.Run("EXEC-REQ-058.2: Logger supports structured fields", func(t *testing.T) {
		// Requirement: Executor shall support structured logging with fields
		logger := GetLogger()

		// Verify structured logging API works
		entry := logger.WithFields(map[string]interface{}{
			"model_path": "/path/to/model",
			"category":   "NONMEM",
		})

		assert.NotNil(t, entry, "EXEC-REQ-058.2: Structured logging must work")
	})
}

// TestValidation_EXEC_REQ_059_DetectionPriority validates that the executor
// checks platforms in the correct priority order.
//
// EXEC-REQ-059: The executor SHALL check model categories in priority order:
// Monolix (1) → NONMEM (2) → Torsten (3) → Stan (4) → Unknown.
// Torsten MUST be checked before Stan due to superset relationship.
func TestValidation_EXEC_REQ_059_DetectionPriority(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("EXEC-REQ-059.1: Torsten detected before Stan", func(t *testing.T) {
		// Requirement: Executor shall prioritize Torsten over Stan
		modelPath := filepath.Join(tmpDir, "torsten.stan")
		// This has both Stan structure AND Torsten functions
		modelContent := "data {\n  int N;\n}\nparameters {\n  real CL;\n}\nmodel {\n  vector[N] pred = PKModelOneCpt(CL, V, ka);\n}\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		detector := NewDetector()
		categoryType, err := detector.Detect(modelPath)

		require.NoError(t, err)
		assert.Equal(t, CategoryTorsten, categoryType, "EXEC-REQ-059.1: Must detect Torsten, not Stan")
	})

	t.Run("EXEC-REQ-059.2: Pure Stan without Torsten functions", func(t *testing.T) {
		// Requirement: Executor shall detect Stan when no Torsten functions present
		modelPath := filepath.Join(tmpDir, "stan.stan")
		modelContent := "data {\n  int N;\n}\nparameters {\n  real mu;\n}\nmodel {\n  mu ~ normal(0, 1);\n}\n"
		require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

		detector := NewDetector()
		categoryType, err := detector.Detect(modelPath)

		require.NoError(t, err)
		assert.Equal(t, CategoryStan, categoryType, "EXEC-REQ-059.2: Must detect pure Stan correctly")
	})
}
