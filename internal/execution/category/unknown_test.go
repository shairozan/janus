package category

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
)

func TestUnknownCategory_Name(t *testing.T) {
	cat := NewUnknownCategory()
	assert.Equal(t, "Unknown", cat.Name())
}

func TestUnknownCategory_RequiresLicense(t *testing.T) {
	cat := NewUnknownCategory()
	assert.False(t, cat.RequiresLicense(), "Unknown category should not require license")
}

func TestUnknownCategory_GetLicense(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("returns error (should never be called)", func(t *testing.T) {
		content, err := cat.GetLicense(nil)
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "unknown category does not support license files")
	})

	t.Run("handles nil config", func(t *testing.T) {
		_, err := cat.GetLicense(nil)
		assert.Error(t, err)
	})

	t.Run("handles non-nil config", func(t *testing.T) {
		cfg := &config.Config{}
		_, err := cat.GetLicense(cfg)
		assert.Error(t, err)
	})
}

func TestUnknownCategory_GetModel(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("reads model file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "unknown.xyz")
		modelContent := []byte("SOME UNKNOWN FORMAT CONTENT")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		content, err := cat.GetModel(modelPath)
		assert.NoError(t, err)
		assert.Equal(t, modelContent, content)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		content, err := cat.GetModel("/nonexistent/model.xyz")
		assert.Error(t, err)
		assert.Nil(t, content)
		assert.Contains(t, err.Error(), "failed to read model file")
	})
}

func TestUnknownCategory_GetDataPath(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("returns error (cannot detect data files)", func(t *testing.T) {
		modelContent := []byte("SOME MODEL CONTENT")
		modelPath := "/tmp/model.xyz"

		dataPath, err := cat.GetDataPath(modelContent, modelPath)
		assert.Error(t, err)
		assert.Empty(t, dataPath)
		assert.Contains(t, err.Error(), "unknown category cannot detect data files")
		assert.Contains(t, err.Error(), "platform unknown")
	})
}

func TestUnknownCategory_ContainerStructure(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("includes only model file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.xyz")
		modelContent := []byte("UNKNOWN FORMAT MODEL")
		err := os.WriteFile(modelPath, modelContent, 0644)
		require.NoError(t, err)

		structure, err := cat.ContainerStructure(modelPath, nil)
		assert.NoError(t, err)
		assert.Len(t, structure, 1, "Unknown category should only include model file")

		// Verify only model file is included
		assert.Contains(t, structure, "model.xyz")
		assert.Equal(t, modelContent, structure["model.xyz"])
	})

	t.Run("does not include data or license files", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.xyz")
		err := os.WriteFile(modelPath, []byte("MODEL"), 0644)
		require.NoError(t, err)

		// Create data and license files (should NOT be included)
		err = os.WriteFile(filepath.Join(tmpDir, "data.csv"), []byte("DATA"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tmpDir, "license.lic"), []byte("LICENSE"), 0644)
		require.NoError(t, err)

		structure, err := cat.ContainerStructure(modelPath, nil)
		assert.NoError(t, err)
		assert.Len(t, structure, 1)
		assert.NotContains(t, structure, "data.csv")
		assert.NotContains(t, structure, "license.lic")
	})

	t.Run("handles nil config", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.xyz")
		err := os.WriteFile(modelPath, []byte("MODEL"), 0644)
		require.NoError(t, err)

		// Should not panic with nil config
		structure, err := cat.ContainerStructure(modelPath, nil)
		assert.NoError(t, err)
		assert.NotNil(t, structure)
	})

	t.Run("handles non-nil config", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.xyz")
		err := os.WriteFile(modelPath, []byte("MODEL"), 0644)
		require.NoError(t, err)

		cfg := &config.Config{}
		structure, err := cat.ContainerStructure(modelPath, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, structure)
	})

	t.Run("returns error for non-existent model file", func(t *testing.T) {
		structure, err := cat.ContainerStructure("/nonexistent/model.xyz", nil)
		assert.Error(t, err)
		assert.Nil(t, structure)
	})
}

func TestUnknownCategory_RetentionTargets(t *testing.T) {
	cat := NewUnknownCategory()

	targets := cat.RetentionTargets()
	assert.Len(t, targets, 1, "Unknown category should return single wildcard")
	assert.Equal(t, []string{"*"}, targets, "Unknown category should retain all files (*)")
}

func TestUnknownCategory_CommandOverrides(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("returns empty map", func(t *testing.T) {
		overrides := cat.CommandOverrides(nil)
		assert.NotNil(t, overrides)
		assert.Empty(t, overrides, "Unknown category should have no command overrides")
	})

	t.Run("handles nil config", func(t *testing.T) {
		overrides := cat.CommandOverrides(nil)
		assert.NotNil(t, overrides)
		assert.Empty(t, overrides)
	})

	t.Run("handles non-nil config", func(t *testing.T) {
		cfg := &config.Config{}
		overrides := cat.CommandOverrides(cfg)
		assert.NotNil(t, overrides)
		assert.Empty(t, overrides)
	})
}

// TestUnknownCategory_PassThroughBehavior tests that Unknown category truly acts as a pass-through.
func TestUnknownCategory_PassThroughBehavior(t *testing.T) {
	cat := NewUnknownCategory()

	t.Run("pass-through characteristics", func(t *testing.T) {
		// Should not require license
		assert.False(t, cat.RequiresLicense())

		// Should not provide command overrides
		assert.Empty(t, cat.CommandOverrides(nil))

		// Should retain all files
		assert.Equal(t, []string{"*"}, cat.RetentionTargets())

		// Should return error when trying to get license
		_, err := cat.GetLicense(nil)
		assert.Error(t, err)

		// Should return error when trying to get data path
		_, err = cat.GetDataPath([]byte("content"), "/path/model")
		assert.Error(t, err)
	})
}
