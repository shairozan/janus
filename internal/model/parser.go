package model

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModelDependencies represents all file dependencies for a NONMEM model.
type ModelDependencies struct {
	ModelFile     string            // Absolute path to model
	DataFiles     []string          // Absolute paths to data files
	ExternalFiles []string          // Other dependencies
	RelativePaths map[string]string // Original relative path -> absolute path
}

// ParseModelDependencies analyzes a .mod file and returns all file dependencies.
func ParseModelDependencies(modelPath string) (*ModelDependencies, error) {
	return ParseModelDependenciesWithDataDirs(modelPath, nil)
}

// ParseModelDependenciesWithDataDirs is like ParseModelDependencies but, when a
// $DATA file is not found relative to the model, it also searches the provided
// alternative data-file directories (Pirana's "alternative data-file
// directory" preference) before reporting the file missing.
func ParseModelDependenciesWithDataDirs(modelPath string, altDataDirs []string) (*ModelDependencies, error) {
	// Convert to absolute path
	absModelPath, err := filepath.Abs(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Verify model file exists
	if _, err := os.Stat(absModelPath); err != nil {
		return nil, fmt.Errorf("model file not found: %s", absModelPath)
	}

	deps := &ModelDependencies{
		ModelFile:     absModelPath,
		DataFiles:     []string{},
		ExternalFiles: []string{},
		RelativePaths: make(map[string]string),
	}

	// Open and parse model file
	file, err := os.Open(absModelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open model file: %w", err)
	}
	defer file.Close()

	modelDir := filepath.Dir(absModelPath)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Regex patterns for NONMEM syntax
	// Matches: $DATA filename or $DATA "filename with spaces"
	dataPattern := regexp.MustCompile(`(?i)^\s*\$DATA\s+["']?([^"'\s]+)["']?`)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip comments
		if strings.HasPrefix(strings.TrimSpace(line), ";") {
			continue
		}

		// Parse $DATA statements
		if matches := dataPattern.FindStringSubmatch(line); matches != nil {
			dataPath := matches[1]

			// Resolve relative path to absolute, falling back to the alternative
			// data-file directories when not found next to the model.
			absDataPath, ok := resolveDataFile(dataPath, modelDir, altDataDirs)
			if !ok {
				return nil, fmt.Errorf("data file not found at line %d: %s (searched model dir %s and %d alternative dir(s))",
					lineNum, dataPath, modelDir, len(altDataDirs))
			}

			deps.DataFiles = append(deps.DataFiles, absDataPath)
			deps.RelativePaths[absDataPath] = dataPath
		}

		// TODO: Parse other file references ($MSFI, $TABLE, etc.) in future iterations
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading model file: %w", err)
	}

	return deps, nil
}

// resolveDataFile locates a $DATA file. An absolute path is used as-is; a
// relative path is first resolved against the model directory, then against
// each alternative data directory (trying both the relative path and the bare
// file name). It returns the absolute path and whether the file was found.
func resolveDataFile(dataPath, modelDir string, altDataDirs []string) (string, bool) {
	if filepath.IsAbs(dataPath) {
		if _, err := os.Stat(dataPath); err == nil {
			return dataPath, true
		}

		return "", false
	}

	primary := filepath.Clean(filepath.Join(modelDir, dataPath))
	if _, err := os.Stat(primary); err == nil {
		return primary, true
	}

	for _, dir := range altDataDirs {
		if dir == "" {
			continue
		}

		for _, candidate := range []string{
			filepath.Clean(filepath.Join(dir, dataPath)),
			filepath.Clean(filepath.Join(dir, filepath.Base(dataPath))),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, true
			}
		}
	}

	return "", false
}

// GetCommonAncestor finds the common ancestor directory for all files in dependencies.
func (d *ModelDependencies) GetCommonAncestor() string {
	if len(d.DataFiles) == 0 {
		// No data files, just use model directory
		return filepath.Dir(d.ModelFile)
	}

	// Start with model directory
	allPaths := []string{d.ModelFile}
	allPaths = append(allPaths, d.DataFiles...)
	allPaths = append(allPaths, d.ExternalFiles...)

	// Find common ancestor by comparing path components
	commonPath := filepath.Dir(allPaths[0])

	for _, path := range allPaths[1:] {
		commonPath = findCommonPath(commonPath, filepath.Dir(path))
	}

	return commonPath
}

// findCommonPath finds the common path between two absolute paths.
func findCommonPath(path1, path2 string) string {
	// Normalize paths
	path1 = filepath.Clean(path1)
	path2 = filepath.Clean(path2)

	// Convert to volume name and path for Windows compatibility
	vol1 := filepath.VolumeName(path1)
	vol2 := filepath.VolumeName(path2)

	// If volumes differ (Windows), no common path
	if vol1 != vol2 {
		return vol1 + string(filepath.Separator)
	}

	// Remove volume from paths for splitting
	path1 = path1[len(vol1):]
	path2 = path2[len(vol2):]

	// Split into components
	parts1 := strings.Split(path1, string(filepath.Separator))
	parts2 := strings.Split(path2, string(filepath.Separator))

	// Find common prefix
	var common []string
	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	for i := 0; i < minLen; i++ {
		// Skip empty parts
		if parts1[i] == "" || parts2[i] == "" {
			continue
		}
		if parts1[i] == parts2[i] {
			common = append(common, parts1[i])
		} else {
			break
		}
	}

	if len(common) == 0 {
		return vol1 + string(filepath.Separator)
	}

	// Rebuild path with volume
	return vol1 + string(filepath.Separator) + filepath.Join(common...)
}
