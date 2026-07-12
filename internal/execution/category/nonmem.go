package category

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pharmalytica/janus/internal/config"
)

// NONMEMCategory implements ModelCategory for NONMEM models (.mod, .ctl, .nmctl).
//
// NONMEM-specific behaviors:
//   - Requires nonmem.lic license file
//   - Parses $DATA directive to locate data files
//   - Retains standard NONMEM outputs (.lst, .ext, .xml, .phi, etc.)
//   - Provides license flag command override
type NONMEMCategory struct{}

// NewNONMEMCategory creates a new NONMEM category instance.
func NewNONMEMCategory() *NONMEMCategory {
	return &NONMEMCategory{}
}

// Name returns the category name.
func (n *NONMEMCategory) Name() string {
	return "NONMEM"
}

// RequiresLicense returns true since NONMEM requires a license file.
func (n *NONMEMCategory) RequiresLicense() bool {
	return true
}

// GetLicense attempts to locate the NONMEM license file using the following priority:
//  1. cfg.NONMEM.License.Path (explicit configuration for non-default locations)
//  2. ~/nonmem.lic (default location)
//  3. ./nonmem.lic (current working directory)
//
// Returns the license file contents if found, or an error listing all attempted locations.
// Handles nil config gracefully for standalone executor mode.
func (n *NONMEMCategory) GetLicense(cfg *config.Config) ([]byte, error) {
	var attemptedLocations []string

	// Priority 1: Explicit config path (for non-default locations)
	if cfg != nil && cfg.NONMEM.License.Path != "" {
		configPath, err := config.ExpandNONMEMLicensePath(cfg.NONMEM.License.Path)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"path":  cfg.NONMEM.License.Path,
				"error": err.Error(),
			}).Error("Failed to expand NONMEM license path from config")

			return nil, fmt.Errorf("failed to expand license path: %w", err)
		}

		log.WithFields(map[string]interface{}{
			"source": "config",
			"path":   configPath,
		}).Debug("Attempting to load NONMEM license from config")

		attemptedLocations = append(attemptedLocations, fmt.Sprintf("config (nonmem.license.path): %s", configPath))
		if content, err := os.ReadFile(configPath); err == nil {
			log.WithFields(map[string]interface{}{
				"path":       configPath,
				"size_bytes": len(content),
			}).Info("NONMEM license loaded from config")

			return content, nil
		}
	}

	// Priority 2: ~/nonmem.lic (default location)
	if homeDir, err := os.UserHomeDir(); err == nil {
		homePath := filepath.Join(homeDir, "nonmem.lic")
		log.WithFields(map[string]interface{}{
			"source": "home",
			"path":   homePath,
		}).Debug("Attempting to load NONMEM license from home directory (default)")

		attemptedLocations = append(attemptedLocations, fmt.Sprintf("default: %s", homePath))
		if content, err := os.ReadFile(homePath); err == nil {
			log.WithFields(map[string]interface{}{
				"path":       homePath,
				"size_bytes": len(content),
			}).Info("NONMEM license loaded from home directory (default)")

			return content, nil
		}
	}

	// Priority 3: ./nonmem.lic (current working directory)
	cwdPath := "nonmem.lic"
	log.WithFields(map[string]interface{}{
		"source": "cwd",
		"path":   cwdPath,
	}).Debug("Attempting to load NONMEM license from current directory")

	attemptedLocations = append(attemptedLocations, fmt.Sprintf("cwd: %s", cwdPath))
	if content, err := os.ReadFile(cwdPath); err == nil {
		log.WithFields(map[string]interface{}{
			"path":       cwdPath,
			"size_bytes": len(content),
		}).Info("NONMEM license loaded from current directory")

		return content, nil
	}

	// License not found in any location
	log.WithFields(map[string]interface{}{
		"attempted_locations": attemptedLocations,
	}).Error("NONMEM license not found")

	return nil, fmt.Errorf("NONMEM license file not found. Attempted locations:\n  - %s",
		strings.Join(attemptedLocations, "\n  - "))
}

// GetModel reads and returns the model file contents.
func (n *NONMEMCategory) GetModel(modelPath string) ([]byte, error) {
	log.WithFields(map[string]interface{}{
		"path": modelPath,
	}).Debug("Reading NONMEM model file")

	content, err := os.ReadFile(modelPath)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"path":  modelPath,
			"error": err.Error(),
		}).Error("Failed to read NONMEM model file")

		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"path":       modelPath,
		"size_bytes": len(content),
	}).Debug("NONMEM model file read successfully")

	return content, nil
}

// GetDataPath extracts the data file path from the NONMEM model file.
//
// Parses the $DATA directive (or $INPUT) to locate the data file.
// Supports various NONMEM data directive formats:
//   - $DATA data.csv
//   - $DATA data.csv IGNORE=@
//   - $DATA ../data/analysis.csv IGNORE=@ IGNORE=C
//   - $INPUT data.csv  (alternative to $DATA)
//
// Resolves relative paths relative to the model file's directory.
// Returns absolute path to the data file.
func (n *NONMEMCategory) GetDataPath(modelContent []byte, modelPath string, altDataDirs ...string) (string, error) {
	text := string(modelContent)

	log.WithFields(map[string]interface{}{
		"model_path": modelPath,
	}).Debug("Parsing NONMEM $DATA directive")

	// Look for $DATA directive (NOT $INPUT - that defines column names, not the data file)
	// Pattern: $DATA followed by whitespace and filename
	// Capture everything up to next whitespace, $, or semicolon
	dataRegex := regexp.MustCompile(`(?i)\$DATA\s+([^\s;$]+)`)
	matches := dataRegex.FindStringSubmatch(text)

	if len(matches) < 2 {
		log.WithFields(map[string]interface{}{
			"model_path": modelPath,
		}).Error("No $DATA directive found in NONMEM model")

		return "", fmt.Errorf("no $DATA directive found in model file")
	}

	dataFile := strings.TrimSpace(matches[1])

	log.WithFields(map[string]interface{}{
		"model_path": modelPath,
		"data_file":  dataFile,
	}).Debug("Found $DATA directive")

	// Resolve relative paths relative to model directory
	var dataPath string
	if filepath.IsAbs(dataFile) {
		dataPath = dataFile
	} else {
		modelDir := filepath.Dir(modelPath)
		dataPath = filepath.Join(modelDir, dataFile)
	}

	// Verify data file exists, falling back to the alternative data directories
	// when it is not found next to the model.
	if _, err := os.Stat(dataPath); err != nil {
		alt := findInAltDirs(dataFile, altDataDirs)
		if alt == "" {
			log.WithFields(map[string]interface{}{
				"data_path":  dataPath,
				"model_path": modelPath,
				"error":      err.Error(),
			}).Error("Data file not found")

			return "", fmt.Errorf("data file not found: %s (referenced in %s)", dataPath, modelPath)
		}

		dataPath = alt
	}

	log.WithFields(map[string]interface{}{
		"data_path":  dataPath,
		"model_path": modelPath,
	}).Info("NONMEM data file located")

	return dataPath, nil
}

// findInAltDirs looks for a data file in the alternative data directories,
// trying both the referenced path and its bare file name. Returns the first
// existing match, or "" if none.
func findInAltDirs(dataFile string, altDataDirs []string) string {
	for _, dir := range altDataDirs {
		if dir == "" {
			continue
		}

		for _, candidate := range []string{
			filepath.Join(dir, dataFile),
			filepath.Join(dir, filepath.Base(dataFile)),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	return ""
}

// ContainerStructure builds the file structure for the Hermes container.
//
// Includes:
//  1. Model file (as basename in container root)
//  2. Data file (as basename in container root)
//  3. License file (as "nonmem.lic" in container root)
//
// Returns a map of container paths to file contents.
func (n *NONMEMCategory) ContainerStructure(modelPath string, cfg *config.Config) (map[string][]byte, error) {
	structure := make(map[string][]byte)

	log.WithFields(map[string]interface{}{
		"model_path": modelPath,
	}).Debug("Building NONMEM container structure")

	// 1. Include model file
	modelContent, err := n.GetModel(modelPath)
	if err != nil {
		return nil, err
	}
	modelBasename := filepath.Base(modelPath)
	structure[modelBasename] = modelContent

	log.WithFields(map[string]interface{}{
		"container_path": modelBasename,
		"size_bytes":     len(modelContent),
	}).Debug("Added model file to container structure")

	// 2. Include data file (searching the alternative data directory if set)
	var altDataDirs []string
	if cfg != nil && cfg.AltDataDirectory != "" {
		altDataDirs = []string{cfg.AltDataDirectory}
	}

	dataPath, err := n.GetDataPath(modelContent, modelPath, altDataDirs...)
	if err != nil {
		return nil, err
	}

	dataContent, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}
	dataBasename := filepath.Base(dataPath)
	structure[dataBasename] = dataContent

	log.WithFields(map[string]interface{}{
		"container_path": dataBasename,
		"size_bytes":     len(dataContent),
	}).Debug("Added data file to container structure")

	// 3. Include license file
	licenseContent, err := n.GetLicense(cfg)
	if err != nil {
		return nil, err
	}
	structure["nonmem.lic"] = licenseContent

	log.WithFields(map[string]interface{}{
		"container_path": "nonmem.lic",
		"size_bytes":     len(licenseContent),
	}).Debug("Added license file to container structure")

	// 4. Include PNM file if it exists (for parallel execution)
	// The PNM file is generated by the GUI before execution starts
	modelDir := filepath.Dir(modelPath)
	modelName := strings.TrimSuffix(modelBasename, filepath.Ext(modelBasename))
	pnmPath := filepath.Join(modelDir, modelName+".pnm")

	if pnmContent, err := os.ReadFile(pnmPath); err == nil {
		pnmBasename := modelName + ".pnm"
		structure[pnmBasename] = pnmContent

		log.WithFields(map[string]interface{}{
			"container_path": pnmBasename,
			"size_bytes":     len(pnmContent),
		}).Debug("Added PNM file to container structure (parallel execution)")
	}

	log.WithFields(map[string]interface{}{
		"total_files": len(structure),
	}).Info("NONMEM container structure built successfully")

	return structure, nil
}

// RetentionTargets returns the file patterns to retain after NONMEM execution.
//
// Standard NONMEM output files:
//   - *.lst (list file - primary output)
//   - *.ext (parameter estimates)
//   - *.xml (run metadata)
//   - *.phi (individual parameters)
//   - *.cov (covariance matrix)
//   - *.cor (correlation matrix)
//   - *.coi (inverse covariance)
//   - *.shk (shrinkage estimates)
//
// The concrete list is the canonical config.DefaultNONMEMRetain, so this runtime
// fallback and the defaults seeded into new .janus.config.json files stay
// identical (single source of truth).
func (n *NONMEMCategory) RetentionTargets() []string {
	return config.DefaultNONMEMRetain()
}

// CommandOverrides returns NONMEM-specific command-line overrides.
//
// Provides the license file flag for NONMEM execution:
//
//	LICENSE_FLAG: "-licfile=${WORKSPACE}/nonmem.lic"
//
// The ${WORKSPACE} placeholder should be replaced by the executor with the
// actual container workspace path.
func (n *NONMEMCategory) CommandOverrides(_ *config.Config) map[string]string {
	overrides := map[string]string{
		"LICENSE_FLAG": "-licfile=${WORKSPACE}/nonmem.lic",
	}

	log.WithFields(map[string]interface{}{
		"overrides": overrides,
	}).Debug("NONMEM command overrides")

	return overrides
}
