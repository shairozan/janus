package category

import "github.com/pharmalytica/janus/internal/config"

// ModelCategory represents a specific modeling platform (NONMEM, Monolix, Stan, Torsten).
//
// Each category encapsulates platform-specific behaviors such as:
//   - License file location and loading
//   - Data file path extraction from model files
//   - Container file structure for Hermes execution
//   - Output retention patterns
//   - Platform-specific command modifications
//
// This interface allows Janus to support multiple modeling platforms through a
// unified execution flow, with platform-specific logic isolated to category implementations.
//
// Example usage:
//
//	detector := NewDetector()
//	categoryType, _ := detector.Detect("model.mod")
//
//	var category ModelCategory
//	switch categoryType {
//	case CategoryNONMEM:
//	    category = NewNONMEMCategory()
//	case CategoryUnknown:
//	    category = NewUnknownCategory()
//	}
//
//	// Use category methods to build execution payload
//	if category.RequiresLicense() {
//	    license, _ := category.GetLicense(cfg)
//	    // Include license in payload
//	}
type ModelCategory interface {
	// Name returns the human-readable category name.
	// Examples: "NONMEM", "Monolix", "Stan", "Torsten", "Unknown"
	Name() string

	// RequiresLicense returns true if this platform requires a license file
	// for execution.
	//
	// For platforms that require licenses (like NONMEM), GetLicense() will be
	// called to locate and read the license file. For platforms that don't
	// require licenses, GetLicense() will not be called.
	//
	// Example:
	//   - NONMEM: true (requires nonmem.lic)
	//   - Monolix: true (requires monolix.lic)
	//   - Stan: false (no license required)
	//   - Unknown: false (don't attempt license lookup)
	RequiresLicense() bool

	// GetLicense attempts to locate and read the platform's license file.
	//
	// This method should implement the platform's license location logic,
	// typically checking multiple locations in priority order:
	//   1. Explicit configuration (if cfg is not nil)
	//   2. Home directory (e.g., ~/nonmem.lic)
	//   3. Current directory (e.g., ./nonmem.lic)
	//
	// Parameters:
	//   - cfg: Configuration object (may be nil in standalone executor mode)
	//
	// Returns:
	//   - []byte: License file contents
	//   - error: If license is required but not found, or if read fails
	//
	// Note: This method will only be called if RequiresLicense() returns true.
	//
	// Example implementation (NONMEM):
	//   func (n *NONMEMCategory) GetLicense(cfg *config.Config) ([]byte, error) {
	//       var path string
	//       if cfg != nil && cfg.NONMEM.License.Path != "" {
	//           path = cfg.NONMEM.License.Path
	//       } else {
	//           path = filepath.Join(os.UserHomeDir(), "nonmem.lic") // default
	//       }
	//       // Additional fallback: ./nonmem.lic
	//       return os.ReadFile(path)
	//   }
	GetLicense(cfg *config.Config) ([]byte, error)

	// GetModel reads and returns the model file contents.
	//
	// This is typically a simple file read operation, but implementations
	// may add platform-specific preprocessing if needed.
	//
	// Parameters:
	//   - modelPath: Absolute path to the model file
	//
	// Returns:
	//   - []byte: Model file contents
	//   - error: If file cannot be read
	GetModel(modelPath string) ([]byte, error)

	// GetDataPath extracts the data file path from the model file contents.
	//
	// Different platforms specify data files in different ways:
	//   - NONMEM: $DATA directive in control file
	//   - Monolix: file= directive in <DATAFILE> section
	//   - Stan: External JSON file (not embedded in model)
	//
	// Implementations should parse the model content and extract the data
	// file reference, then resolve it relative to the model file's directory.
	//
	// Parameters:
	//   - modelContent: Contents of the model file (from GetModel)
	//   - modelPath: Absolute path to the model file (for resolving relative paths)
	//   - altDataDirs: optional alternative directories to search when the data
	//     file is not found next to the model (the "alternative data-file
	//     directory" preference)
	//
	// Returns:
	//   - string: Absolute path to the data file
	//   - error: If data file reference cannot be found or resolved
	//
	// Example (NONMEM):
	//   Input: "$DATA ../data/warfarin.csv IGNORE=@"
	//   Output: "/path/to/model/../data/warfarin.csv" → "/path/to/data/warfarin.csv"
	GetDataPath(modelContent []byte, modelPath string, altDataDirs ...string) (string, error)

	// ContainerStructure builds the complete map of files to send to Hermes.
	//
	// This method orchestrates all the file collection logic:
	//   1. Read the model file (via GetModel)
	//   2. Find and read the data file (via GetDataPath)
	//   3. If RequiresLicense(), read the license file (via GetLicense)
	//   4. Add any additional platform-specific files
	//
	// The returned map has:
	//   - Keys: Relative paths inside the Hermes container workspace
	//   - Values: File contents as byte arrays
	//
	// Parameters:
	//   - modelPath: Absolute path to the model file
	//   - cfg: Configuration object (may be nil)
	//
	// Returns:
	//   - map[string][]byte: Map of filename → file contents
	//   - error: If any required file cannot be loaded
	//
	// Example return value (NONMEM):
	//   {
	//     "model.mod": <model file bytes>,
	//     "data.csv": <data file bytes>,
	//     "nonmem.lic": <license file bytes>
	//   }
	ContainerStructure(modelPath string, cfg *config.Config) (map[string][]byte, error)

	// RetentionTargets returns glob patterns for output files to keep after execution.
	//
	// Hermes will use these patterns to determine which files to return from
	// the container workspace. Different platforms produce different outputs:
	//   - NONMEM: *.lst, *.ext, *.xml, *.phi, *.cov, etc.
	//   - Monolix: predictions.txt, parameters.txt, etc.
	//   - Stan: *.csv (MCMC samples)
	//   - Unknown: * (keep everything)
	//
	// Returns:
	//   - []string: Array of glob patterns
	//
	// Example (NONMEM):
	//   []string{"*.lst", "*.ext", "*.xml", "*.phi", "*.cov", "*.cor"}
	RetentionTargets() []string

	// CommandOverrides returns platform-specific command modifications.
	//
	// This method returns a map of macro replacements to apply to the
	// Hermes execution command. This allows categories to inject platform-specific
	// flags without modifying the core execution logic.
	//
	// Common use cases:
	//   - License file flags (e.g., NONMEM's -licfile)
	//   - Parallelization flags
	//   - Platform-specific options
	//
	// The returned map uses placeholder syntax:
	//   - Key: Macro name (e.g., "LICENSE_FLAG")
	//   - Value: Replacement string (e.g., "-licfile=${WORKSPACE}/nonmem.lic")
	//
	// Parameters:
	//   - cfg: Configuration object (may be nil)
	//
	// Returns:
	//   - map[string]string: Map of macro → replacement string
	//
	// Example (NONMEM):
	//   {
	//     "LICENSE_FLAG": "-licfile=${WORKSPACE}/nonmem.lic"
	//   }
	//
	// Usage in command construction:
	//   command := "nonmem ${LICENSE_FLAG} model.mod"
	//   // After applying overrides: "nonmem -licfile=${WORKSPACE}/nonmem.lic model.mod"
	CommandOverrides(cfg *config.Config) map[string]string
}
