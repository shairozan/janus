package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModelConfig represents the top-level .janus.config.json structure.
// This separates general model settings from execution-specific configuration.
type ModelConfig struct {
	// CorrelationStrategy controls how parameters are matched across runs during comparison.
	// Valid values: "conservative" (default), "inferred", "positional"
	CorrelationStrategy string `json:"correlation_strategy,omitempty" mapstructure:"correlation_strategy"`

	// Retain lists glob patterns for the model's output files of interest. It is a
	// MODEL-WIDE property (not Hermes-specific): it drives Hermes file collection
	// and run-log embedding for every run, and applies whether or not a Hermes
	// section is present. When set it is authoritative for this model; when
	// empty/absent, resolution falls back to the global hermes.retain, then the
	// category default. See config.ResolveRetain.
	Retain []string `json:"retain,omitempty" mapstructure:"retain"`

	// Hermes contains Hermes-specific execution configuration.
	// This is optional - if not present, Hermes execution is not configured.
	Hermes *HermesExecutionConfig `json:"hermes,omitempty" mapstructure:"hermes"`
}

// HermesExecutionConfig represents Hermes-specific execution settings.
// This is nested under the "hermes" key in .janus.config.json.
type HermesExecutionConfig struct {
	Image                string         `json:"image" mapstructure:"image"`
	Resources            ResourceConfig `json:"resources" mapstructure:"resources"`
	ContainerCommandPath string         `json:"container_command_path,omitempty" mapstructure:"container_command_path"` // Path to modeling tool binary in container (e.g., "/opt/NONMEM/nm75/run/nmfe75", "/opt/monolix/bin/monolix")

	// PsNImage is the container image for staged PsN orchestration stages (the
	// horizontal-bootstrap resample + aggregate pods). It needs PsN but no NONMEM
	// license. Optional; falls back to the global hermes.psn_image. The execution
	// Image (NONMEM) is used for the per-fit pods. See PsNImageOrDefault.
	PsNImage string `json:"psn_image,omitempty" mapstructure:"psn_image"`

	// Deprecated: Use ContainerCommandPath instead
	NonmemPath string `json:"nonmem_path,omitempty" mapstructure:"nonmem_path"` // Backward compatibility - will be removed in future version
}

// PsNImageOrDefault resolves the PsN orchestration image for staged PsN tools
// (bootstrap setup/aggregate): the per-model psn_image if set, otherwise the
// supplied global default (hermes.psn_image). It returns an error when neither
// is configured, since staged PsN tools cannot run without a PsN image.
func (c *HermesExecutionConfig) PsNImageOrDefault(globalDefault string) (string, error) {
	if strings.TrimSpace(c.PsNImage) != "" {
		return c.PsNImage, nil
	}

	if strings.TrimSpace(globalDefault) != "" {
		return globalDefault, nil
	}

	return "", fmt.Errorf(
		"staged PsN execution requires a PsN image: set 'hermes.psn_image' in " +
			".janus.config.json or configure the global hermes.psn_image",
	)
}

// HermesModelConfig is an alias for backward compatibility with existing code.
// New code should use ModelConfig and HermesExecutionConfig directly.
//
// Deprecated: Use ModelConfig with HermesExecutionConfig instead.
type HermesModelConfig = HermesExecutionConfig

// ResourceConfig defines the computational resources for Hermes execution.
type ResourceConfig struct {
	CPUCores int    `json:"cpu_cores" mapstructure:"cpu_cores"`
	Memory   string `json:"memory" mapstructure:"memory"` // Format: "4Gi", "2048Mi", "1G", etc.
}

// memoryFormatRegex validates memory specifications like "4Gi", "2048Mi", "1G", "512M".
var memoryFormatRegex = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(Ki|Mi|Gi|Ti|K|M|G|T)$`)

// LoadModelConfig loads .janus.config.json from the directory containing the model file.
// This returns the full ModelConfig structure including both top-level settings
// (like CorrelationStrategy) and nested Hermes configuration.
//
// The function supports both the new nested format (with "hermes" key) and the legacy
// flat format for backward compatibility. Legacy format is automatically migrated.
//
// Parameters:
//   - modelPath: Full path to the model file (e.g., "/path/to/model.mod")
//
// Returns:
//   - *ModelConfig: The loaded configuration (may have nil Hermes if not configured)
//   - error: If file is missing, malformed, or fails validation
func LoadModelConfig(modelPath string) (*ModelConfig, error) {
	// Get directory containing the model file
	modelDir := filepath.Dir(modelPath)
	configPath := filepath.Join(modelDir, ".janus.config.json")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf(
			".janus.config.json not found in model directory.\n"+
				"Expected location: %s",
			configPath,
		)
	}

	// Read the file directly to determine format
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .janus.config.json at %s: %w", configPath, err)
	}

	// Try to parse as new nested format first
	var modelCfg ModelConfig
	if err := json.Unmarshal(data, &modelCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal .janus.config.json: %w", err)
	}

	// Check if this is the new nested format or old flat format that needs migration
	if modelCfg.Hermes == nil {
		migratedCfg, migrated, err := migrateFromLegacyFormat(data)
		if err != nil {
			return nil, err
		}

		if migrated {
			modelCfg = *migratedCfg
		}
	}

	// Retain is a top-level, model-wide property. Lift a retain that an older
	// config nested under the "hermes" key (as #94 briefly wrote it) so it isn't
	// silently dropped now that the field lives on ModelConfig.
	if len(modelCfg.Retain) == 0 {
		if nested := nestedHermesRetain(data); len(nested) > 0 {
			modelCfg.Retain = nested
		}
	}

	return &modelCfg, nil
}

// nestedHermesRetain extracts a retain list that was written under the "hermes"
// key of .janus.config.json (the transitional #94 shape), for lifting to the
// top-level ModelConfig.Retain. Returns nil when absent.
func nestedHermesRetain(data []byte) []string {
	var raw struct {
		Hermes struct {
			Retain []string `json:"retain"`
		} `json:"hermes"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	return raw.Hermes.Retain
}

// migrateFromLegacyFormat attempts to migrate old flat format config to the new nested format.
// Returns the migrated config, a boolean indicating if migration occurred, and any error.
//
// The legacy format has fields like "image", "resources" at the root level.
// The new format nests these under a "hermes" key.
func migrateFromLegacyFormat(data []byte) (*ModelConfig, bool, error) {
	// Check if this is the old flat format by looking for "image" at root level
	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, false, fmt.Errorf("failed to parse .janus.config.json: %w", err)
	}

	if _, hasImage := rawMap["image"]; !hasImage {
		// Not a legacy format - no migration needed
		return nil, false, nil
	}

	// Old flat format - migrate to new structure
	var legacyCfg legacyHermesConfig
	if err := json.Unmarshal(data, &legacyCfg); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal legacy .janus.config.json: %w", err)
	}

	// Validate that the legacy config has required fields before accepting migration
	if legacyCfg.Image == "" {
		return nil, false, fmt.Errorf("legacy .janus.config.json has 'image' key but value is empty")
	}

	modelCfg := &ModelConfig{
		CorrelationStrategy: legacyCfg.CorrelationStrategy,
		Retain:              legacyCfg.Retain, // top-level retain is a model-wide property
		Hermes: &HermesExecutionConfig{
			Image:                legacyCfg.Image,
			Resources:            legacyCfg.Resources,
			ContainerCommandPath: legacyCfg.ContainerCommandPath,
			NonmemPath:           legacyCfg.NonmemPath,
		},
	}

	return modelCfg, true, nil
}

// legacyHermesConfig represents the old flat .janus.config.json format.
// Used for backward compatibility when reading old config files.
type legacyHermesConfig struct {
	Image                string         `json:"image"`
	Resources            ResourceConfig `json:"resources"`
	ContainerCommandPath string         `json:"container_command_path,omitempty"`
	NonmemPath           string         `json:"nonmem_path,omitempty"`
	CorrelationStrategy  string         `json:"correlation_strategy,omitempty"`
	Retain               []string       `json:"retain,omitempty"`
}

// LoadHermesModelConfig loads the Hermes execution configuration from .janus.config.json.
//
// For Hermes execution, this file is REQUIRED - if it doesn't exist, this returns an error.
// This is intentional: Hermes cannot execute without knowing which container image and
// resources to use.
//
// This function supports both the new nested format (with "hermes" key) and the legacy
// flat format for backward compatibility.
//
// Parameters:
//   - modelPath: Full path to the model file (e.g., "/path/to/model.mod")
//
// Returns:
//   - *HermesModelConfig: The loaded and validated Hermes configuration
//   - error: If file is missing, malformed, or fails validation
func LoadHermesModelConfig(modelPath string) (*HermesModelConfig, error) {
	modelCfg, err := LoadModelConfig(modelPath)
	if err != nil {
		// Check if the error is about missing file and provide Hermes-specific message
		configPath := ConfigPath(modelPath)
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf(
				"hermes execution requires .janus.config.json in model directory.\n"+
					"Expected location: %s\n"+
					"This file must specify the container image and resource requirements.\n"+
					"See documentation for configuration format",
				configPath,
			)
		}

		return nil, err
	}

	// Check if Hermes configuration is present
	if modelCfg.Hermes == nil {
		configPath := ConfigPath(modelPath)

		return nil, fmt.Errorf(
			"hermes execution requires 'hermes' configuration in .janus.config.json.\n"+
				"Config location: %s\n"+
				"Add a 'hermes' section with image and resource settings",
			configPath,
		)
	}

	// Validate the configuration
	if err := modelCfg.Hermes.Validate(); err != nil {
		configPath := ConfigPath(modelPath)

		return nil, fmt.Errorf("invalid .janus.config.json at %s: %w", configPath, err)
	}

	return modelCfg.Hermes, nil
}

// Validate checks that the HermesModelConfig contains valid values.
func (c *HermesModelConfig) Validate() error {
	if c.Image == "" {
		return fmt.Errorf("'image' field is required and cannot be empty")
	}

	if c.Resources.CPUCores <= 0 {
		return fmt.Errorf("'resources.cpu_cores' must be a positive integer, got: %d", c.Resources.CPUCores)
	}

	if c.Resources.Memory == "" {
		return fmt.Errorf("'resources.memory' is required and cannot be empty")
	}

	if !memoryFormatRegex.MatchString(c.Resources.Memory) {
		return fmt.Errorf(
			"'resources.memory' must be in format like '4Gi', '2048Mi', '1G', '512M', got: %s",
			c.Resources.Memory,
		)
	}

	return nil
}

// GetCommandPath returns the container command path, falling back to NonmemPath for backward compatibility.
func (c *HermesModelConfig) GetCommandPath() string {
	if c.ContainerCommandPath != "" {
		return c.ContainerCommandPath
	}

	// Fallback to deprecated NonmemPath field for backward compatibility
	return c.NonmemPath
}

// ConfigPath returns the expected path for .janus.config.json given a model file path.
// This is useful for checking if the config exists or for creating it.
func ConfigPath(modelPath string) string {
	modelDir := filepath.Dir(modelPath)

	return filepath.Join(modelDir, ".janus.config.json")
}

// SaveModelConfig writes the ModelConfig to .janus.config.json in the model's directory.
// This saves in the new nested format with the "hermes" key.
func SaveModelConfig(modelPath string, cfg *ModelConfig) error {
	configPath := ConfigPath(modelPath)

	// Marshal to pretty JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file with restrictive permissions
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
