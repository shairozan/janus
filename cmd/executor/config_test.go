package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindHermesConfig_ExplicitPath(t *testing.T) {
	// Create temp directory with config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom-config.json")

	// Create config file
	err := os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Test explicit config path
	foundConfig, foundModel, err := findHermesConfig([]string{"model.mod"}, configPath)
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, "", foundModel) // Model path not determined from explicit config
}

func TestFindHermesConfig_ExplicitPathNotFound(t *testing.T) {
	// Test with non-existent explicit config
	_, _, err := findHermesConfig([]string{"model.mod"}, "/nonexistent/config.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "explicit config not found")
}

func TestFindHermesConfig_FromModelPath(t *testing.T) {
	// Create temp directory with model and config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
	require.NoError(t, err)

	// Create config file
	err = os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Test discovery from model path
	foundConfig, foundModel, err := findHermesConfig([]string{modelPath}, "")
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	absModelPath, _ := filepath.Abs(modelPath)

	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, absModelPath, foundModel)
}

func TestFindHermesConfig_FromRelativePath(t *testing.T) {
	// Create temp directory with nested structure
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "models", "run1")
	err := os.MkdirAll(modelDir, 0755)
	require.NoError(t, err)

	modelPath := filepath.Join(modelDir, "model.mod")
	configPath := filepath.Join(modelDir, ".janus.config.json")

	// Create files
	err = os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Change to temp directory to test relative paths
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Test with relative path
	foundConfig, foundModel, err := findHermesConfig([]string{"models/run1/model.mod"}, "")
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	absModelPath, _ := filepath.Abs("models/run1/model.mod")

	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, absModelPath, foundModel)
}

func TestFindHermesConfig_FromDirectory(t *testing.T) {
	// Create temp directory with config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create config file
	err := os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Test discovery from directory argument
	foundConfig, foundModel, err := findHermesConfig([]string{tempDir}, "")
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	absTempDir, _ := filepath.Abs(tempDir)

	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, absTempDir, foundModel)
}

func TestFindHermesConfig_FallbackToCurrentDirectory(t *testing.T) {
	// Create temp directory and change to it
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create config in temp directory
	err := os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Change to temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Test fallback to current directory when args don't contain paths
	foundConfig, foundModel, err := findHermesConfig([]string{"--some-flag", "value"}, "")
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)

	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, "", foundModel)
}

func TestFindHermesConfig_NotFound(t *testing.T) {
	// Test with no config anywhere
	tempDir := t.TempDir()

	// Change to temp directory (no config)
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	err := os.Chdir(tempDir)
	require.NoError(t, err)

	// Should error when no config found
	_, _, err = findHermesConfig([]string{"nonexistent.mod"}, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no .janus.config.json found")
	assert.Contains(t, err.Error(), "Hint:")
}

func TestFindHermesConfig_MultipleArgs(t *testing.T) {
	// Create temp directory with model and config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create files
	err := os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Test with multiple arguments (typical NONMEM style)
	foundConfig, foundModel, err := findHermesConfig(
		[]string{modelPath, "model.lst", "-maxeval=9999"},
		"",
	)
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	absModelPath, _ := filepath.Abs(modelPath)

	assert.Equal(t, absConfigPath, foundConfig)
	assert.Equal(t, absModelPath, foundModel)
}

func TestFindHermesConfig_SkipsExecutorFlags(t *testing.T) {
	// Create temp directory with config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create files
	err := os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(configPath, []byte(`{"image": "test:latest"}`), 0644)
	require.NoError(t, err)

	// Test that executor flags are skipped during discovery
	// (This is defensive - args should already be filtered by parseExecutorFlags)
	foundConfig, _, err := findHermesConfig(
		[]string{"--executor-quiet", modelPath},
		"",
	)
	require.NoError(t, err)

	absConfigPath, _ := filepath.Abs(configPath)
	assert.Equal(t, absConfigPath, foundConfig)
}

