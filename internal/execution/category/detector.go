package category

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CategoryType represents the detected model platform type.
// This is determined by analyzing the model file's extension and contents.
type CategoryType string

const (
	// CategoryNONMEM represents NONMEM control files (.mod, .ctl, .nmctl).
	// Detected by: $PROBLEM and $DATA directives.
	CategoryNONMEM CategoryType = "NONMEM"

	// CategoryMonolix represents Monolix project files (.mlxtran).
	// Detected by: <DATAFILE>, <MODEL>, <FIT>, <PARAMETER>, <MONOLIX> tags.
	CategoryMonolix CategoryType = "Monolix"

	// CategoryStan represents generic Stan model files (.stan).
	// Detected by: data {}, parameters {}, and model {} blocks.
	// Note: Torsten is checked first (Torsten is a Stan superset).
	CategoryStan CategoryType = "Stan"

	// CategoryTorsten represents Torsten-enhanced Stan files (.stan).
	// Detected by: Stan blocks + Torsten-specific PK functions.
	// Must be detected BEFORE generic Stan (Torsten is a Stan superset).
	CategoryTorsten CategoryType = "Torsten"

	// CategoryUnknown represents unrecognized model files
	// Used when no detection rules match. Execution proceeds in
	// "pass-through" mode with minimal assumptions.
	CategoryUnknown CategoryType = "Unknown"
)

// DetectionRule represents a single detection check with priority.
//
// Detection rules are evaluated in priority order (lower number = higher priority).
// The first rule that returns true determines the category.
type DetectionRule struct {
	// Priority determines the order of rule evaluation (lower = earlier)
	// Standard priorities:
	//   1: Monolix (by extension + content)
	//   2: NONMEM (by extension + content)
	//   3: Torsten (MUST be before Stan)
	//   4: Stan (generic)
	Priority int

	// Check is the detection function that analyzes the file
	// Parameters:
	//   - filename: Name of the file (for extension checking)
	//   - content: File contents as byte array
	// Returns:
	//   - bool: true if this rule matches
	Check func(filename string, content []byte) bool

	// Category is the category type to assign if this rule matches
	Category CategoryType

	// Name is a human-readable description of this rule (for debugging)
	Name string
}

// Detector analyzes model files and determines their category.
//
// The detector uses a two-phase approach:
//  1. Fast path: Extension-based detection (.mlxtran, .mod/.ctl, .stan)
//  2. Fallback: Content-based detection using detection rules
//
// If no rules match, the category is set to CategoryUnknown (not an error).
type Detector struct {
	// rules contains all detection rules in priority order
	rules []DetectionRule
}

// NewDetector creates a new detector with standard detection rules.
//
// The detector is pre-configured with rules for all supported platforms:
//   - Monolix (priority 1)
//   - NONMEM (priority 2)
//   - Torsten (priority 3) - checked before Stan
//   - Stan (priority 4)
//
// Example:
//
//	detector := NewDetector()
//	category, err := detector.Detect("/path/to/model.mod")
//	if err != nil {
//	    // Handle read error
//	}
//	switch category {
//	case CategoryNONMEM:
//	    // Use NONMEM category
//	case CategoryUnknown:
//	    // Use pass-through category
//	}
func NewDetector() *Detector {
	return &Detector{
		rules: []DetectionRule{
			// Priority 1: Monolix (extension + content)
			{
				Priority: 1,
				Check:    isMonolixFile,
				Category: CategoryMonolix,
				Name:     "Monolix detection",
			},
			// Priority 2: NONMEM (extension + content)
			{
				Priority: 2,
				Check:    isNONMEMFile,
				Category: CategoryNONMEM,
				Name:     "NONMEM detection",
			},
			// Priority 3: Torsten (MUST be before Stan - Torsten is a Stan superset)
			{
				Priority: 3,
				Check:    isTorstenFile,
				Category: CategoryTorsten,
				Name:     "Torsten detection",
			},
			// Priority 4: Stan (generic Stan)
			{
				Priority: 4,
				Check:    isStanFile,
				Category: CategoryStan,
				Name:     "Stan detection",
			},
		},
	}
}

// Detect analyzes a model file and returns its category.
//
// Detection uses a two-phase approach:
//  1. Extension-based detection (fast path)
//  2. Content-based detection (fallback)
//
// If no detection rules match, CategoryUnknown is returned (not an error).
// This allows execution to proceed in pass-through mode.
//
// Parameters:
//   - modelPath: Absolute path to the model file
//
// Returns:
//   - CategoryType: The detected category (or CategoryUnknown)
//   - error: Only if the file cannot be read (not if detection fails)
//
// Example:
//
//	detector := NewDetector()
//	category, err := detector.Detect("/path/to/model.mod")
//	if err != nil {
//	    return fmt.Errorf("failed to read model: %w", err)
//	}
//
//	if category == CategoryUnknown {
//	    log.Warn("Could not categorize model, using pass-through mode")
//	}
func (d *Detector) Detect(modelPath string) (CategoryType, error) {
	log.WithField("model_path", modelPath).Debug("Starting model categorization")

	// Read file content
	content, err := os.ReadFile(modelPath)
	if err != nil {
		log.WithError(err).WithField("model_path", modelPath).Debug("Failed to read model file")

		return CategoryUnknown, fmt.Errorf("failed to read model file: %w", err)
	}

	// Try extension-based detection first (fast path)
	ext := strings.ToLower(filepath.Ext(modelPath))
	log.WithField("extension", ext).Debug("Checking file extension")

	switch ext {
	case ".mlxtran":
		if isMonolixFile(modelPath, content) {
			log.Debug("Detected as Monolix by extension")

			return CategoryMonolix, nil
		}
	case ".mod", ".ctl", ".nmctl":
		if isNONMEMFile(modelPath, content) {
			log.Debug("Detected as NONMEM by extension")

			return CategoryNONMEM, nil
		}
	case ".stan":
		// For .stan files, we need content analysis to distinguish Torsten from Stan
		// IMPORTANT: Check Torsten first (it's a Stan superset)
		if isTorstenFile(modelPath, content) {
			log.Debug("Detected as Torsten by extension and content")

			return CategoryTorsten, nil
		}
		if isStanFile(modelPath, content) {
			log.Debug("Detected as Stan by extension and content")

			return CategoryStan, nil
		}
	}

	// Try content-based detection (fallback)
	log.Debug("Extension-based detection failed, trying content-based detection")
	for _, rule := range d.rules {
		if rule.Check(modelPath, content) {
			log.WithFields(map[string]interface{}{
				"rule":     rule.Name,
				"category": rule.Category,
			}).Debug("Content-based detection matched")

			return rule.Category, nil
		}
	}

	// If no detection rules matched, return Unknown (not an error)
	// Unknown category means "pass through as-is, don't try to be smart"
	log.Debug("No detection rules matched, returning Unknown category")

	return CategoryUnknown, nil
}

// Detection Functions
// These functions implement the platform-specific detection logic.
// Each function checks for required markers/patterns in the file content.

// isMonolixFile detects Monolix .mlxtran project files.
//
// Detection criteria:
//   - Must have at least 2 of these XML-like tags:
//     <DATAFILE>, <MODEL>, <FIT>, <PARAMETER>, <MONOLIX>
//
// Monolix files use a plaintext format with XML-like section markers.
func isMonolixFile(filename string, content []byte) bool {
	text := string(content)

	// Count required markers
	markers := []string{"<DATAFILE>", "<MODEL>", "<FIT>", "<PARAMETER>", "<MONOLIX>"}
	matchCount := 0

	for _, marker := range markers {
		if strings.Contains(text, marker) {
			matchCount++
		}
	}

	// Require at least 2 markers to reduce false positives
	matched := matchCount >= 2
	log.WithFields(map[string]interface{}{
		"filename":    filepath.Base(filename),
		"match_count": matchCount,
		"matched":     matched,
	}).Debug("Monolix detection")

	return matched
}

// isNONMEMFile detects NONMEM control files (.mod, .ctl, .nmctl).
//
// Detection criteria:
//   - Must have $PROBLEM or $PROB directive (case-insensitive)
//   - Must have $DATA or $INPUT directive (case-insensitive)
//
// NONMEM uses Fortran-style directives starting with $.
func isNONMEMFile(filename string, content []byte) bool {
	text := strings.ToUpper(string(content))

	// Check for required directives (case-insensitive)
	hasProblem := strings.Contains(text, "$PROBLEM") || strings.Contains(text, "$PROB")
	hasData := strings.Contains(text, "$DATA") || strings.Contains(text, "$INPUT")

	matched := hasProblem && hasData
	log.WithFields(map[string]interface{}{
		"filename":    filepath.Base(filename),
		"has_problem": hasProblem,
		"has_data":    hasData,
		"matched":     matched,
	}).Debug("NONMEM detection")

	return matched
}

// isStanFile detects generic Stan model files (.stan).
//
// Detection criteria:
//   - Must have data { block
//   - Must have parameters { block
//   - Must have model { block
//
// Stan uses C++-like syntax with required block structures.
// Note: This should be checked AFTER isTorstenFile, as Torsten is a Stan superset.
// Whitespace between keyword and opening brace is flexible.
func isStanFile(filename string, content []byte) bool {
	text := string(content)

	// Check for required blocks (flexible whitespace with regex)
	dataRegex := regexp.MustCompile(`data\s+\{`)
	parametersRegex := regexp.MustCompile(`parameters\s+\{`)
	modelRegex := regexp.MustCompile(`model\s+\{`)

	hasData := dataRegex.MatchString(text)
	hasParameters := parametersRegex.MatchString(text)
	hasModel := modelRegex.MatchString(text)

	matched := hasData && hasParameters && hasModel
	log.WithFields(map[string]interface{}{
		"filename":       filepath.Base(filename),
		"has_data":       hasData,
		"has_parameters": hasParameters,
		"has_model":      hasModel,
		"matched":        matched,
	}).Debug("Stan detection")

	return matched
}

// isTorstenFile detects Torsten-enhanced Stan files (.stan).
//
// Detection criteria:
//   - Must be a valid Stan file (has data/parameters/model blocks)
//   - Must contain at least one Torsten-specific PK function
//
// Torsten extends Stan with pharmacometric functions for PK/PD modeling.
// This MUST be checked before isStanFile, as Torsten is a Stan superset.
//
// Torsten-specific functions include:
//   - PKModelOneCpt, PKModelTwoCpt, linCmtModel (built-in PK models)
//   - pmx_solve_* (ODE solvers: bdf, rk45, adams, onecpt, twocpt, group_bdf, group_rk45)
//   - generalOdeModel, generalCptModel, mixOde*CptModel (general models)
func isTorstenFile(filename string, content []byte) bool {
	// Must be a valid Stan file first
	if !isStanFile(filename, content) {
		return false
	}

	text := string(content)

	// Check for Torsten-specific functions
	torstenFunctions := []string{
		// Built-in PK models
		"PKModelOneCpt",
		"PKModelTwoCpt",
		"linCmtModel",

		// Torsten ODE solvers
		"pmx_solve_bdf",
		"pmx_solve_rk45",
		"pmx_solve_adams",
		"pmx_solve_onecpt",
		"pmx_solve_twocpt",
		"pmx_solve_group_bdf",
		"pmx_solve_group_rk45",

		// General models
		"generalOdeModel",
		"generalCptModel",
		"mixOde1CptModel",
		"mixOde2CptModel",
	}

	for _, fn := range torstenFunctions {
		if strings.Contains(text, fn) {
			log.WithFields(map[string]interface{}{
				"filename": filepath.Base(filename),
				"function": fn,
			}).Debug("Torsten detection - found PK function")

			return true
		}
	}

	log.WithField("filename", filepath.Base(filename)).Debug("Torsten detection - no PK functions found")

	return false
}
