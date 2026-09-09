package category

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shairozan/janus/internal/config"
)

// UnknownCategory implements ModelCategory for unrecognized model formats.
//
// This is a **pass-through category** that makes minimal assumptions about the model.
// It serves as a fail-safe when we cannot detect the model platform, allowing users
// to attempt execution with experimental or unsupported formats.
//
// Behavior:
//   - Does NOT require a license file
//   - Cannot detect data files (only includes model file)
//   - Retains ALL output files (*)
//   - No command-line overrides
//
// Users should be warned that execution may fail if the model requires:
//   - Platform-specific license files
//   - Data files not included in the container
//   - Specific command-line arguments
type UnknownCategory struct{}

// NewUnknownCategory creates a new Unknown category instance.
func NewUnknownCategory() *UnknownCategory {
	return &UnknownCategory{}
}

// Name returns the category name.
func (u *UnknownCategory) Name() string {
	return "Unknown"
}

// RequiresLicense returns false since we don't know what license (if any) is needed.
func (u *UnknownCategory) RequiresLicense() bool {
	return false
}

// GetLicense returns an error since Unknown category does not require a license.
// This method should never be called if RequiresLicense() returns false.
func (u *UnknownCategory) GetLicense(cfg *config.Config) ([]byte, error) {
	log.Warn("GetLicense called on Unknown category (should not happen)")

	return nil, fmt.Errorf("unknown category does not support license files")
}

// GetModel reads and returns the model file contents.
func (u *UnknownCategory) GetModel(modelPath string) ([]byte, error) {
	log.WithFields(map[string]interface{}{
		"path": modelPath,
	}).Debug("Reading model file (Unknown category)")

	content, err := os.ReadFile(modelPath)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"path":  modelPath,
			"error": err.Error(),
		}).Error("Failed to read model file")

		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"path":       modelPath,
		"size_bytes": len(content),
	}).Debug("Model file read successfully")

	return content, nil
}

// GetDataPath returns an error since we cannot detect data files for unknown formats.
//
// Users must manually ensure any required data files are accessible to their
// execution environment.
func (u *UnknownCategory) GetDataPath(_ []byte, modelPath string, _ ...string) (string, error) {
	log.WithFields(map[string]interface{}{
		"model_path": modelPath,
	}).Warn("Cannot detect data file for Unknown category")

	return "", fmt.Errorf("unknown category cannot detect data files - platform unknown")
}

// ContainerStructure builds the file structure for the Hermes container.
//
// For Unknown category, we only include the model file itself.
// This is a minimal, "best effort" approach that may not work if the model
// requires additional files (data, libraries, etc.).
//
// Users are warned via logging that this is pass-through mode.
func (u *UnknownCategory) ContainerStructure(modelPath string, _ *config.Config) (map[string][]byte, error) {
	structure := make(map[string][]byte)

	log.WithFields(map[string]interface{}{
		"model_path": modelPath,
	}).Warn("Building container structure for Unknown category (pass-through mode)")

	// Include only the model file itself
	modelContent, err := u.GetModel(modelPath)
	if err != nil {
		return nil, err
	}
	modelBasename := filepath.Base(modelPath)
	structure[modelBasename] = modelContent

	log.WithFields(map[string]interface{}{
		"container_path": modelBasename,
		"size_bytes":     len(modelContent),
		"warning":        "Only model file included - data/license files may be missing",
	}).Warn("Unknown category container structure built (minimal)")

	return structure, nil
}

// RetentionTargets returns a wildcard pattern to retain all output files.
//
// Since we don't know what outputs the unknown platform generates, we keep everything.
// This may result in retaining unnecessary files, but ensures no important outputs are lost.
func (u *UnknownCategory) RetentionTargets() []string {
	log.Debug("Unknown category retains all files (*)")

	return []string{"*"}
}

// CommandOverrides returns an empty map since we have no platform-specific overrides.
func (u *UnknownCategory) CommandOverrides(cfg *config.Config) map[string]string {
	log.Debug("Unknown category has no command overrides")

	return map[string]string{}
}
