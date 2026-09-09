package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/model"
)

// HermesPayload represents the complete workspace structure for Hermes execution.
type HermesPayload struct {
	WorkspaceStructure map[string][]byte             // Relative path -> file content
	EntryPoint         string                        // Relative path to model within workspace
	Config             *config.HermesExecutionConfig // Hermes configuration
}

// BuildHermesPayload constructs the Hermes workspace structure from a model file.
// This is the SINGLE SOURCE OF TRUTH for building Hermes payloads.
// All interfaces (CLI, GUI, REST API) MUST use this function.
func BuildHermesPayload(modelPath string, hermesConfig *config.HermesExecutionConfig, altDataDirs ...string) (*HermesPayload, error) {
	if hermesConfig == nil {
		return nil, fmt.Errorf("hermes configuration is required")
	}

	// Parse model dependencies, searching any alternative data directories when
	// a data file is not found next to the model.
	deps, err := model.ParseModelDependenciesWithDataDirs(modelPath, altDataDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse model dependencies: %w", err)
	}

	// Find common ancestor directory
	commonAncestor := deps.GetCommonAncestor()

	// Build workspace structure
	workspace := make(map[string][]byte)

	// Add model file
	modelRelPath, err := filepath.Rel(commonAncestor, deps.ModelFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compute relative path for model: %w", err)
	}

	modelContent, err := os.ReadFile(deps.ModelFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read model file: %w", err)
	}

	workspace[modelRelPath] = modelContent

	// Add data files
	for _, dataPath := range deps.DataFiles {
		dataRelPath, err := filepath.Rel(commonAncestor, dataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path for data file %s: %w", dataPath, err)
		}

		dataContent, err := os.ReadFile(dataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read data file %s: %w", dataPath, err)
		}

		workspace[dataRelPath] = dataContent
	}

	// Add external files (if any)
	for _, externalPath := range deps.ExternalFiles {
		externalRelPath, err := filepath.Rel(commonAncestor, externalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path for external file %s: %w", externalPath, err)
		}

		externalContent, err := os.ReadFile(externalPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read external file %s: %w", externalPath, err)
		}

		workspace[externalRelPath] = externalContent
	}

	// Normalize paths to use forward slashes for consistency (Hermes requirement)
	normalizedWorkspace := make(map[string][]byte)
	for path, content := range workspace {
		normalizedPath := filepath.ToSlash(path)
		normalizedWorkspace[normalizedPath] = content
	}

	normalizedEntryPoint := filepath.ToSlash(modelRelPath)

	return &HermesPayload{
		WorkspaceStructure: normalizedWorkspace,
		EntryPoint:         normalizedEntryPoint,
		Config:             hermesConfig,
	}, nil
}

// GetWorkspaceFiles returns a list of all files in the workspace.
func (p *HermesPayload) GetWorkspaceFiles() []string {
	files := make([]string, 0, len(p.WorkspaceStructure))
	for path := range p.WorkspaceStructure {
		files = append(files, path)
	}

	return files
}

// GetWorkspaceSize returns the total size of all files in bytes.
func (p *HermesPayload) GetWorkspaceSize() int64 {
	var totalSize int64
	for _, content := range p.WorkspaceStructure {
		totalSize += int64(len(content))
	}

	return totalSize
}

// ValidateWorkspace checks that the workspace is valid for execution.
func (p *HermesPayload) ValidateWorkspace() error {
	if len(p.WorkspaceStructure) == 0 {
		return fmt.Errorf("workspace is empty")
	}

	if p.EntryPoint == "" {
		return fmt.Errorf("entry point is not set")
	}

	// Verify entry point exists in workspace
	if _, exists := p.WorkspaceStructure[p.EntryPoint]; !exists {
		return fmt.Errorf("entry point %s not found in workspace", p.EntryPoint)
	}

	// Verify config is valid
	if p.Config == nil {
		return fmt.Errorf("hermes configuration is missing")
	}

	if err := p.Config.Validate(); err != nil {
		return fmt.Errorf("invalid Hermes configuration: %w", err)
	}

	return nil
}

// GetWorkspaceTree returns a visual representation of the workspace structure.
func (p *HermesPayload) GetWorkspaceTree() string {
	var sb strings.Builder
	sb.WriteString("workspace/\n")

	files := p.GetWorkspaceFiles()
	for i, file := range files {
		isLast := i == len(files)-1
		prefix := "├── "
		if isLast {
			prefix = "└── "
		}

		// Indent nested files
		depth := strings.Count(file, "/")
		indent := strings.Repeat("│   ", depth)

		sb.WriteString(indent)
		sb.WriteString(prefix)
		sb.WriteString(filepath.Base(file))

		if file == p.EntryPoint {
			sb.WriteString(" [ENTRY POINT]")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
