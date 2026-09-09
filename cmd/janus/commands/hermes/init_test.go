//go:build unit
// +build unit

package hermes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid positive", "4", false},
		{"Valid large", "128", false},
		{"Valid one", "1", false},
		{"Invalid zero", "0", true},
		{"Invalid negative", "-1", true},
		{"Invalid text", "abc", true},
		{"Invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveInt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMemoryFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid Gibibytes", "8Gi", false},
		{"Valid Mebibytes", "2048Mi", false},
		{"Valid Kibibytes", "1024Ki", false},
		{"Valid Tebibytes", "1Ti", false},
		{"Valid Gigabytes", "8G", false},
		{"Valid Megabytes", "2048M", false},
		{"Valid Kilobytes", "1024K", false},
		{"Valid Terabytes", "1T", false},
		{"Valid decimal", "4.5Gi", false},
		{"Invalid no unit", "8192", true},
		{"Invalid wrong unit", "8Gb", true},
		{"Invalid space", "8 Gi", true},
		{"Invalid empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMemoryFormat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	hermesConfig := &config.HermesExecutionConfig{
		Image: "test/image:v1",
		Resources: config.ResourceConfig{
			CPUCores: 8,
			Memory:   "16Gi",
		},
	}

	err := saveConfig(modelPath, hermesConfig)
	require.NoError(t, err)

	// Verify file was created
	configPath := config.ConfigPath(modelPath)
	assert.FileExists(t, configPath)

	// Read and verify content (should be in new nested format)
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loadedConfig config.ModelConfig
	err = json.Unmarshal(data, &loadedConfig)
	require.NoError(t, err)

	require.NotNil(t, loadedConfig.Hermes)
	assert.Equal(t, hermesConfig.Image, loadedConfig.Hermes.Image)
	assert.Equal(t, hermesConfig.Resources.CPUCores, loadedConfig.Hermes.Resources.CPUCores)
	assert.Equal(t, hermesConfig.Resources.Memory, loadedConfig.Hermes.Resources.Memory)
}

func TestRunInit_MissingModelFile(t *testing.T) {
	err := runInit("/nonexistent/model.mod", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model file not found")
}

func TestRunInit_ExistingConfigWithoutForce(t *testing.T) {
	// Create temp model and existing config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
	require.NoError(t, err)

	// Create existing config
	existingConfig := &config.HermesModelConfig{
		Image: "existing:v1",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}
	data, _ := json.Marshal(existingConfig)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	// Try to init without force - should fail
	err = runInit(modelPath, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration file already exists")
	assert.Contains(t, err.Error(), "--force")
}

func TestInitCommand_Structure(t *testing.T) {
	cmd := initCommand()

	assert.Contains(t, cmd.Use, "init")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, cmd.Flags().HasFlags())

	// Verify force flag exists
	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "f", forceFlag.Shorthand)
}

func TestHermesCommand_Structure(t *testing.T) {
	cmd := Command()

	assert.Equal(t, "hermes", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, cmd.HasSubCommands())

	// Verify init subcommand exists
	initCmd := cmd.Commands()[0]
	assert.Contains(t, initCmd.Use, "init")
}
