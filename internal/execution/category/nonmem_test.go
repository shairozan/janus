package category

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestNONMEMCategory_Name(t *testing.T) {
	cat := NewNONMEMCategory()
	assert.Equal(t, "NONMEM", cat.Name())
}

func TestNONMEMCategory_RequiresLicense(t *testing.T) {
	cat := NewNONMEMCategory()
	assert.True(t, cat.RequiresLicense())
}

func TestNONMEMCategory_GetLicense(t *testing.T) {
	cat := NewNONMEMCategory()

	t.Run("loads from config path (highest priority)", func(t *testing.T) {
		tmpDir := t.TempDir()
		configLicPath := filepath.Join(tmpDir, "config_license.lic")
		err := os.WriteFile(configLicPath, []byte("LICENSE_FROM_CONFIG"), 0644)
		require.NoError(t, err)

		// Also create license in home to prove config takes priority
		tmpHome := t.TempDir()
		homeLicPath := filepath.Join(tmpHome, "nonmem.lic")
		err = os.WriteFile(homeLicPath, []byte("LICENSE_FROM_HOME"), 0644)
		require.NoError(t, err)
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		cfg := &config.Config{}
		cfg.NONMEM.License.Path = configLicPath

		content, err := cat.GetLicense(cfg)
		assert.NoError(t, err)
		assert.Equal(t, []byte("LICENSE_FROM_CONFIG"), content)
	})

	t.Run("loads from home directory (default) when config not set", func(t *testing.T) {
		// Create temp home directory
		tmpHome := t.TempDir()
		licPath := filepath.Join(tmpHome, "nonmem.lic")
		err := os.WriteFile(licPath, []byte("LICENSE_FROM_HOME"), 0644)
		require.NoError(t, err)

		// Override both HOME and USERPROFILE for cross-platform compatibility
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

		// Config with no license path set
		cfg := &config.Config{}

		content, err := cat.GetLicense(cfg)
		assert.NoError(t, err)
		assert.Equal(t, []byte("LICENSE_FROM_HOME"), content)
	})

	t.Run("loads from config path with home expansion", func(t *testing.T) {
		// Create temp home directory with license
		tmpHome := t.TempDir()
		licPath := filepath.Join(tmpHome, "custom_license.lic")
		err := os.WriteFile(licPath, []byte("LICENSE_FROM_CONFIG_TILDE"), 0644)
		require.NoError(t, err)

		// Override both HOME and USERPROFILE for cross-platform compatibility
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		// Config with tilde path
		cfg := &config.Config{}
		cfg.NONMEM.License.Path = "~/custom_license.lic"

		content, err := cat.GetLicense(cfg)
		assert.NoError(t, err)
		assert.Equal(t, []byte("LICENSE_FROM_CONFIG_TILDE"), content)
	})

	t.Run("loads from current directory", func(t *testing.T) {
		// Create temp directory and change to it
		tmpDir := t.TempDir()
		origWd, _ := os.Getwd()
		err := os.Chdir(tmpDir)
		require.NoError(t, err)
		defer os.Chdir(origWd)

		// Write license to current directory
		err = os.WriteFile("nonmem.lic", []byte("LICENSE_FROM_CWD"), 0644)
		require.NoError(t, err)

		// Override HOME and USERPROFILE to empty temp dir (no license there)
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

		content, err := cat.GetLicense(nil)
		assert.NoError(t, err)
		assert.Equal(t, []byte("LICENSE_FROM_CWD"), content)
	})

	t.Run("returns error when license not found", func(t *testing.T) {
		// Change to empty temp directory
		tmpDir := t.TempDir()
		origWd, _ := os.Getwd()
		err := os.Chdir(tmpDir)
		require.NoError(t, err)
		defer os.Chdir(origWd)

		// Override HOME and USERPROFILE to empty temp dir (no license there)
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

		content, err := cat.GetLicense(nil)
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "NONMEM license file not found")
		assert.Contains(t, err.Error(), "Attempted locations")
	})

	t.Run("handles nil config", func(t *testing.T) {
		// Create temp home directory with license
		tmpHome := t.TempDir()
		licPath := filepath.Join(tmpHome, "nonmem.lic")
		err := os.WriteFile(licPath, []byte("LICENSE"), 0644)
		require.NoError(t, err)

		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		// Should not panic with nil config
		content, err := cat.GetLicense(nil)
		assert.NoError(t, err)
		assert.NotNil(t, content)
	})
}

func TestNONMEMCategory_GetModel(t *testing.T) {
	cat := NewNONMEMCategory()

	t.Run("reads model file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := []byte("$PROBLEM Test Model\n$DATA data.csv")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		content, err := cat.GetModel(modelPath)
		assert.NoError(t, err)
		assert.Equal(t, modelContent, content)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		content, err := cat.GetModel("/nonexistent/model.mod")
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "failed to read model file")
	})
}

func TestNONMEMCategory_GetDataPath_AltDir(t *testing.T) {
	cat := NewNONMEMCategory()

	modelDir := t.TempDir()
	altDir := t.TempDir()
	modelPath := filepath.Join(modelDir, "model.mod")
	modelContent := []byte("$PROBLEM Test\n$DATA data.csv IGNORE=@\n")

	// The data file exists only in the alternative directory.
	altData := filepath.Join(altDir, "data.csv")
	require.NoError(t, os.WriteFile(altData, []byte("ID,TIME,DV\n"), 0644))

	// Without an alt dir → not found.
	_, err := cat.GetDataPath(modelContent, modelPath)
	require.Error(t, err)

	// With the alt dir → resolves there.
	got, err := cat.GetDataPath(modelContent, modelPath, altDir)
	require.NoError(t, err)
	assert.Equal(t, altData, got)
}

func TestNONMEMCategory_GetDataPath(t *testing.T) {
	cat := NewNONMEMCategory()

	t.Run("extracts data path from $DATA directive", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		dataPath := filepath.Join(tmpDir, "data.csv")

		// Create data file
		err := os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10"), 0644)
		require.NoError(t, err)

		// Create model with $DATA directive
		modelContent := []byte("$PROBLEM Test\n$DATA data.csv IGNORE=@\n$INPUT ID TIME DV")

		extracted, err := cat.GetDataPath(modelContent, modelPath)
		assert.NoError(t, err)
		assert.Equal(t, dataPath, extracted)
	})

	t.Run("extracts data path from $DATA directive", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		dataPath := filepath.Join(tmpDir, "dataset.csv")

		// Create data file
		err := os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10"), 0644)
		require.NoError(t, err)

		// Create model with $DATA directive
		modelContent := []byte("$PROBLEM Test\n$DATA dataset.csv\n$INPUT ID TIME DV\n$PK\nCL=THETA(1)")

		extracted, err := cat.GetDataPath(modelContent, modelPath)
		assert.NoError(t, err)
		assert.Equal(t, dataPath, extracted)
	})

	t.Run("handles relative paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelDir := filepath.Join(tmpDir, "models")
		dataDir := filepath.Join(tmpDir, "data")
		err := os.MkdirAll(modelDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)

		modelPath := filepath.Join(modelDir, "model.mod")
		dataPath := filepath.Join(dataDir, "analysis.csv")

		// Create data file
		err = os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10"), 0644)
		require.NoError(t, err)

		// Model references data with relative path
		modelContent := []byte("$PROBLEM Test\n$DATA ../data/analysis.csv IGNORE=@")

		extracted, err := cat.GetDataPath(modelContent, modelPath)
		assert.NoError(t, err)
		assert.Equal(t, dataPath, extracted)
	})

	t.Run("case insensitive directive matching", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		dataPath := filepath.Join(tmpDir, "data.csv")

		err := os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,10"), 0644)
		require.NoError(t, err)

		// Lowercase $data and $input
		modelContent := []byte("$problem Test\n$data data.csv IGNORE=@")

		extracted, err := cat.GetDataPath(modelContent, modelPath)
		assert.NoError(t, err)
		assert.Equal(t, dataPath, extracted)
	})

	t.Run("returns error when no $DATA found", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := []byte("$PROBLEM Test\n$INPUT ID TIME DV\n$PK\nCL=THETA(1)")

		_, err := cat.GetDataPath(modelContent, modelPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no $DATA directive found")
	})

	t.Run("returns error when data file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := []byte("$PROBLEM Test\n$DATA nonexistent.csv")

		_, err := cat.GetDataPath(modelContent, modelPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "data file not found")
	})
}

func TestNONMEMCategory_ContainerStructure(t *testing.T) {
	cat := NewNONMEMCategory()

	t.Run("includes model, data, and license files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create model file
		modelPath := filepath.Join(tmpDir, "test.mod")
		modelContent := []byte("$PROBLEM Test\n$DATA data.csv IGNORE=@")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		// Create data file
		dataPath := filepath.Join(tmpDir, "data.csv")
		dataContent := []byte("ID,TIME,DV\n1,0,10")
		err = os.WriteFile(dataPath, dataContent, 0644)
		require.NoError(t, err)

		// Create license file
		licPath := filepath.Join(tmpDir, "nonmem.lic")
		licContent := []byte("LICENSE_KEY_123")
		err = os.WriteFile(licPath, licContent, 0644)
		require.NoError(t, err)

		// Use config to specify license path (highest priority)
		cfg := &config.Config{}
		cfg.NONMEM.License.Path = licPath

		structure, err := cat.ContainerStructure(modelPath, cfg)
		assert.NoError(t, err)
		assert.Len(t, structure, 3)

		// Verify model file
		assert.Contains(t, structure, "test.mod")
		assert.Equal(t, modelContent, structure["test.mod"])

		// Verify data file
		assert.Contains(t, structure, "data.csv")
		assert.Equal(t, dataContent, structure["data.csv"])

		// Verify license file
		assert.Contains(t, structure, "nonmem.lic")
		assert.Equal(t, licContent, structure["nonmem.lic"])
	})

	t.Run("includes PNM file when present for parallel execution", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create model file
		modelPath := filepath.Join(tmpDir, "test.mod")
		modelContent := []byte("$PROBLEM Test\n$DATA data.csv IGNORE=@")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		// Create data file
		dataPath := filepath.Join(tmpDir, "data.csv")
		dataContent := []byte("ID,TIME,DV\n1,0,10")
		err = os.WriteFile(dataPath, dataContent, 0644)
		require.NoError(t, err)

		// Create license file
		licPath := filepath.Join(tmpDir, "nonmem.lic")
		licContent := []byte("LICENSE_KEY_123")
		err = os.WriteFile(licPath, licContent, 0644)
		require.NoError(t, err)

		// Create PNM file (simulating parallel execution setup)
		pnmPath := filepath.Join(tmpDir, "test.pnm")
		pnmContent := []byte("$GENERAL\nNODES=4 PARSE_TYPE=2 TIMEOUTI=100 TIMEOUT=2400 PARAPRINT=0 TRANSFER_TYPE=1\n")
		err = os.WriteFile(pnmPath, pnmContent, 0644)
		require.NoError(t, err)

		cfg := &config.Config{}
		cfg.NONMEM.License.Path = licPath

		structure, err := cat.ContainerStructure(modelPath, cfg)
		assert.NoError(t, err)
		assert.Len(t, structure, 4) // model, data, license, pnm

		// Verify PNM file is included
		assert.Contains(t, structure, "test.pnm")
		assert.Equal(t, pnmContent, structure["test.pnm"])
	})

	t.Run("returns error when license not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		modelPath := filepath.Join(tmpDir, "test.mod")
		modelContent := []byte("$PROBLEM Test\n$DATA data.csv")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		dataPath := filepath.Join(tmpDir, "data.csv")
		err = os.WriteFile(dataPath, []byte("ID,TIME,DV"), 0644)
		require.NoError(t, err)

		// Override HOME and USERPROFILE to empty temp dir (no license there)
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

		// Change to empty temp directory (no license in cwd)
		origWd, _ := os.Getwd()
		cwdTemp := t.TempDir()
		err = os.Chdir(cwdTemp)
		require.NoError(t, err)
		defer os.Chdir(origWd)

		structure, err := cat.ContainerStructure(modelPath, nil)
		assert.Error(t, err)
		assert.Nil(t, structure)
		assert.Contains(t, err.Error(), "NONMEM license file not found")
	})

	t.Run("handles nil config", func(t *testing.T) {
		tmpDir := t.TempDir()

		modelPath := filepath.Join(tmpDir, "test.mod")
		err := os.WriteFile(modelPath, []byte("$PROBLEM Test\n$DATA data.csv"), 0644)
		require.NoError(t, err)

		dataPath := filepath.Join(tmpDir, "data.csv")
		err = os.WriteFile(dataPath, []byte("ID,TIME,DV"), 0644)
		require.NoError(t, err)

		// Create license in home directory
		tmpHome := t.TempDir()
		licPath := filepath.Join(tmpHome, "nonmem.lic")
		err = os.WriteFile(licPath, []byte("LICENSE"), 0644)
		require.NoError(t, err)

		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		// Should not panic with nil config
		structure, err := cat.ContainerStructure(modelPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, structure)
	})
}

func TestNONMEMCategory_RetentionTargets(t *testing.T) {
	cat := NewNONMEMCategory()

	targets := cat.RetentionTargets()
	assert.NotEmpty(t, targets)

	// Verify standard NONMEM output patterns are included
	assert.Contains(t, targets, "*.lst")
	assert.Contains(t, targets, "*.ext")
	assert.Contains(t, targets, "*.xml")
	assert.Contains(t, targets, "*.phi")
	assert.Contains(t, targets, "*.cov")
	assert.Contains(t, targets, "*.cor")
	assert.Contains(t, targets, "*.coi")
	assert.Contains(t, targets, "*.shk")

	// The category is a thin delegate over the canonical default (single source of
	// truth shared with the seed written into new .janus.config.json files).
	assert.Equal(t, config.DefaultNONMEMRetain(), targets)
}

func TestNONMEMCategory_CommandOverrides(t *testing.T) {
	cat := NewNONMEMCategory()

	t.Run("returns license flag override", func(t *testing.T) {
		overrides := cat.CommandOverrides(nil)
		assert.NotEmpty(t, overrides)
		assert.Contains(t, overrides, "LICENSE_FLAG")
		assert.Equal(t, "-licfile=${WORKSPACE}/nonmem.lic", overrides["LICENSE_FLAG"])
	})

	t.Run("handles nil config", func(t *testing.T) {
		// Should not panic with nil config
		overrides := cat.CommandOverrides(nil)
		assert.NotNil(t, overrides)
	})

	t.Run("handles non-nil config", func(t *testing.T) {
		cfg := &config.Config{}
		overrides := cat.CommandOverrides(cfg)
		assert.NotNil(t, overrides)
		assert.Contains(t, overrides, "LICENSE_FLAG")
	})
}
