package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
)

func TestLoadHermesModelConfig_NewFormat_Success(t *testing.T) {
	// Create temp directory with model and config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create a valid config file in the NEW nested format
	validConfig := `{
		"correlation_strategy": "label-first",
		"hermes": {
			"image": "ghcr.io/pharmalytica/nonmem:7.5.0",
			"resources": {
				"cpu_cores": 4,
				"memory": "8Gi"
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Load the config
	cfg, err := config.LoadHermesModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "ghcr.io/pharmalytica/nonmem:7.5.0", cfg.Image)
	assert.Equal(t, 4, cfg.Resources.CPUCores)
	assert.Equal(t, "8Gi", cfg.Resources.Memory)
}

func TestLoadHermesModelConfig_LegacyFormat_Success(t *testing.T) {
	// Create temp directory with model and config
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create a valid config file in the OLD flat format (backward compatibility)
	validConfig := `{
		"image": "ghcr.io/pharmalytica/nonmem:7.5.0",
		"resources": {
			"cpu_cores": 4,
			"memory": "8Gi"
		}
	}`

	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	require.NoError(t, err)

	// Load the config - should work with legacy format
	cfg, err := config.LoadHermesModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "ghcr.io/pharmalytica/nonmem:7.5.0", cfg.Image)
	assert.Equal(t, 4, cfg.Resources.CPUCores)
	assert.Equal(t, "8Gi", cfg.Resources.Memory)
}

func TestLoadHermesModelConfig_MissingFile_ReturnsError(t *testing.T) {
	// Create temp directory without config file
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Attempt to load config - should fail because file is required
	cfg, err := config.LoadHermesModelConfig(modelPath)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "hermes execution requires .janus.config.json")
	assert.Contains(t, err.Error(), tempDir)
}

func TestLoadHermesModelConfig_MalformedJSON(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Create malformed JSON
	malformedJSON := `{
		"image": "test-image",
		"resources": {
			"cpu_cores": 4,
			"memory": "8Gi"
		` // Missing closing braces

	err := os.WriteFile(configPath, []byte(malformedJSON), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadHermesModelConfig(modelPath)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to unmarshal .janus.config.json")
}

func TestLoadHermesModelConfig_VariousMemoryFormats(t *testing.T) {
	testCases := []struct {
		name          string
		memoryValue   string
		shouldSucceed bool
	}{
		{"Gibibytes", "8Gi", true},
		{"Mebibytes", "2048Mi", true},
		{"Kibibytes", "1024Ki", true},
		{"Tebibytes", "1Ti", true},
		{"Gigabytes", "8G", true},
		{"Megabytes", "2048M", true},
		{"Kilobytes", "1024K", true},
		{"Terabytes", "1T", true},
		{"Decimal Gibibytes", "4.5Gi", true},
		{"Invalid - no unit", "8192", false},
		{"Invalid - wrong unit", "8Gb", false},
		{"Invalid - space", "8 Gi", false},
		{"Empty string", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			modelPath := filepath.Join(tempDir, "model.mod")
			configPath := filepath.Join(tempDir, ".janus.config.json")

			configContent := `{
				"image": "test-image",
				"resources": {
					"cpu_cores": 4,
					"memory": "` + tc.memoryValue + `"
				}
			}`

			err := os.WriteFile(configPath, []byte(configContent), 0644)
			require.NoError(t, err)

			cfg, err := config.LoadHermesModelConfig(modelPath)

			if tc.shouldSucceed {
				assert.NoError(t, err, "Expected memory format %s to be valid", tc.memoryValue)
				assert.NotNil(t, cfg)
			} else {
				assert.Error(t, err, "Expected memory format %s to be invalid", tc.memoryValue)
				assert.Nil(t, cfg)
			}
		})
	}
}

func TestHermesModelConfig_Validate_MissingImage(t *testing.T) {
	cfg := &config.HermesModelConfig{
		Image: "", // Missing
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image")
	assert.Contains(t, err.Error(), "required")
}

func TestHermesModelConfig_Validate_InvalidCPUCores(t *testing.T) {
	testCases := []struct {
		name      string
		cpuCores  int
		shouldErr bool
	}{
		{"Valid - 1 core", 1, false},
		{"Valid - 4 cores", 4, false},
		{"Valid - 16 cores", 16, false},
		{"Invalid - 0 cores", 0, true},
		{"Invalid - negative", -1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.HermesModelConfig{
				Image: "test-image",
				Resources: config.ResourceConfig{
					CPUCores: tc.cpuCores,
					Memory:   "8Gi",
				},
			}

			err := cfg.Validate()

			if tc.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cpu_cores")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHermesModelConfig_Validate_InvalidMemory(t *testing.T) {
	cfg := &config.HermesModelConfig{
		Image: "test-image",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "invalid",
		},
	}

	err := cfg.Validate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory")
	assert.Contains(t, err.Error(), "format")
}

func TestConfigPath(t *testing.T) {
	// Test with real temp directories to ensure cross-platform compatibility
	tempDir := t.TempDir()

	testCases := []struct {
		name      string
		modelPath string
	}{
		{
			name:      "Absolute path",
			modelPath: filepath.Join(tempDir, "models", "model.mod"),
		},
		{
			name:      "Relative path",
			modelPath: filepath.Join("models", "model.mod"),
		},
		{
			name:      "Nested path",
			modelPath: filepath.Join(tempDir, "project", "run001", "model.mod"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := config.ConfigPath(tc.modelPath)

			// Verify result ends with .janus.config.json
			assert.True(t, filepath.Base(result) == ".janus.config.json",
				"Expected path to end with .janus.config.json, got: %s", result)

			// Verify result is in the same directory as the model
			modelDir := filepath.Dir(tc.modelPath)
			expectedPath := filepath.Join(modelDir, ".janus.config.json")
			assert.Equal(t, expectedPath, result)
		})
	}
}

func TestLoadHermesModelConfig_RealWorldExample(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "run001.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Real-world example config (legacy format - still supported)
	realWorldConfig := `{
		"image": "ghcr.io/metrumresearchgroup/nonmem:7.5.1",
		"resources": {
			"cpu_cores": 8,
			"memory": "16Gi"
		}
	}`

	err := os.WriteFile(configPath, []byte(realWorldConfig), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadHermesModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "ghcr.io/metrumresearchgroup/nonmem:7.5.1", cfg.Image)
	assert.Equal(t, 8, cfg.Resources.CPUCores)
	assert.Equal(t, "16Gi", cfg.Resources.Memory)
}

func TestLoadModelConfig_NewFormat(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// New format with top-level settings and nested hermes
	newFormatConfig := `{
		"correlation_strategy": "label-first",
		"hermes": {
			"image": "ghcr.io/pharmalytica/nonmem:7.5.0",
			"container_command_path": "/opt/NONMEM/nm75/run/nmfe75",
			"resources": {
				"cpu_cores": 4,
				"memory": "8Gi"
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(newFormatConfig), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "label-first", cfg.CorrelationStrategy)
	require.NotNil(t, cfg.Hermes)
	assert.Equal(t, "ghcr.io/pharmalytica/nonmem:7.5.0", cfg.Hermes.Image)
	assert.Equal(t, "/opt/NONMEM/nm75/run/nmfe75", cfg.Hermes.ContainerCommandPath)
	assert.Equal(t, 4, cfg.Hermes.Resources.CPUCores)
	assert.Equal(t, "8Gi", cfg.Hermes.Resources.Memory)
}

func TestLoadModelConfig_LegacyFormat_Migrated(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Old flat format - should be automatically migrated
	legacyConfig := `{
		"correlation_strategy": "conservative",
		"image": "ghcr.io/pharmalytica/nonmem:7.5.0",
		"resources": {
			"cpu_cores": 4,
			"memory": "8Gi"
		}
	}`

	err := os.WriteFile(configPath, []byte(legacyConfig), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	// Correlation strategy should be preserved
	assert.Equal(t, "conservative", cfg.CorrelationStrategy)
	// Hermes config should be nested under hermes key
	require.NotNil(t, cfg.Hermes)
	assert.Equal(t, "ghcr.io/pharmalytica/nonmem:7.5.0", cfg.Hermes.Image)
	assert.Equal(t, 4, cfg.Hermes.Resources.CPUCores)
	assert.Equal(t, "8Gi", cfg.Hermes.Resources.Memory)
}

func TestSaveModelConfig(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Create a ModelConfig to save
	modelCfg := &config.ModelConfig{
		CorrelationStrategy: "label-first",
		Hermes: &config.HermesExecutionConfig{
			Image:                "ghcr.io/test/image:v1",
			ContainerCommandPath: "/opt/nonmem/nmfe75",
			Resources: config.ResourceConfig{
				CPUCores: 8,
				Memory:   "16Gi",
			},
		},
	}

	// Save the config
	err := config.SaveModelConfig(modelPath, modelCfg)
	require.NoError(t, err)

	// Verify file was created
	configPath := config.ConfigPath(modelPath)
	assert.FileExists(t, configPath)

	// Load it back and verify
	loadedCfg, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	require.NotNil(t, loadedCfg)
	assert.Equal(t, "label-first", loadedCfg.CorrelationStrategy)
	require.NotNil(t, loadedCfg.Hermes)
	assert.Equal(t, "ghcr.io/test/image:v1", loadedCfg.Hermes.Image)
	assert.Equal(t, "/opt/nonmem/nmfe75", loadedCfg.Hermes.ContainerCommandPath)
	assert.Equal(t, 8, loadedCfg.Hermes.Resources.CPUCores)
	assert.Equal(t, "16Gi", loadedCfg.Hermes.Resources.Memory)
}

func TestSaveModelConfig_RetainRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	retain := []string{"*.lst", "*.ext", "sdtab*"}
	require.NoError(t, config.SaveModelConfig(modelPath, &config.ModelConfig{
		Retain: retain,
		Hermes: &config.HermesExecutionConfig{
			Image:     "ghcr.io/test/image:v1",
			Resources: config.ResourceConfig{CPUCores: 4, Memory: "8Gi"},
		},
	}))

	loaded, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	require.NotNil(t, loaded.Hermes)
	assert.Equal(t, retain, loaded.Retain)

	// retain is a model-wide property: it must be written at the top level, not
	// nested under the hermes key.
	raw, err := os.ReadFile(config.ConfigPath(modelPath))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Contains(t, m, "retain", "retain must be written at the top level")
	hermes, ok := m["hermes"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, hermes, "retain", "retain must not be nested under the hermes key")
}

func TestSaveModelConfig_AbsentRetain(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	require.NoError(t, config.SaveModelConfig(modelPath, &config.ModelConfig{
		Hermes: &config.HermesExecutionConfig{
			Image:     "ghcr.io/test/image:v1",
			Resources: config.ResourceConfig{CPUCores: 4, Memory: "8Gi"},
		},
	}))

	loaded, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	assert.Empty(t, loaded.Retain, "absent retain should load as empty (inherit)")
}

func TestLoadModelConfig_LegacyTopLevelRetainMigrates(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Old flat format with a top-level "retain" (as the pre-#94 docs showed).
	legacy := `{
	  "image": "ghcr.io/test/image:v1",
	  "resources": {"cpu_cores": 4, "memory": "8Gi"},
	  "retain": ["*.lst", "patab*"]
	}`
	require.NoError(t, os.WriteFile(config.ConfigPath(modelPath), []byte(legacy), 0o600))

	loaded, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	require.NotNil(t, loaded.Hermes)
	assert.Equal(t, []string{"*.lst", "patab*"}, loaded.Retain, "top-level legacy retain should stay at the top level")
}

func TestLoadModelConfig_WithoutHermes(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Config with only top-level settings (no hermes)
	configWithoutHermes := `{
		"correlation_strategy": "positional"
	}`

	err := os.WriteFile(configPath, []byte(configWithoutHermes), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadModelConfig(modelPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "positional", cfg.CorrelationStrategy)
	assert.Nil(t, cfg.Hermes) // No Hermes config
}

func TestLoadHermesModelConfig_MissingHermesSection(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	configPath := filepath.Join(tempDir, ".janus.config.json")

	// Config with only top-level settings (no hermes)
	configWithoutHermes := `{
		"correlation_strategy": "positional"
	}`

	err := os.WriteFile(configPath, []byte(configWithoutHermes), 0644)
	require.NoError(t, err)

	// LoadHermesModelConfig should fail because there's no hermes section
	cfg, err := config.LoadHermesModelConfig(modelPath)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "hermes execution requires 'hermes' configuration")
}
