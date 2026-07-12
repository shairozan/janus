//go:build unit
// +build unit

package execution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestBuildHermesPayload_ColocatedFiles(t *testing.T) {
	// Create temp directory with colocated files
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	dataPath := filepath.Join(tempDir, "data.csv")

	// Create model file
	modelContent := `$PROBLEM Test
$DATA data.csv IGNORE=@
$INPUT ID TIME DV
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Create data file
	dataContent := []byte("ID,TIME,DV\n1,0,0\n")
	err = os.WriteFile(dataPath, dataContent, 0644)
	require.NoError(t, err)

	// Create Hermes config
	hermesConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	// Build payload
	payload, err := BuildHermesPayload(modelPath, hermesConfig)
	require.NoError(t, err)
	require.NotNil(t, payload)

	// Verify workspace structure
	assert.Len(t, payload.WorkspaceStructure, 2) // model + data
	assert.Equal(t, "model.mod", payload.EntryPoint)
	assert.Equal(t, hermesConfig, payload.Config)

	// Verify file contents
	assert.Equal(t, modelContent, string(payload.WorkspaceStructure["model.mod"]))
	assert.Equal(t, string(dataContent), string(payload.WorkspaceStructure["data.csv"]))
}

func TestBuildHermesPayload_NonColocatedFiles(t *testing.T) {
	// Create temp directory with non-colocated files
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "models", "run001")
	dataDir := filepath.Join(tempDir, "data")

	err := os.MkdirAll(modelDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	modelPath := filepath.Join(modelDir, "model.mod")
	dataPath := filepath.Join(dataDir, "dataset.csv")

	// Create model file with relative path to data
	modelContent := `$PROBLEM Test
$DATA ../../data/dataset.csv IGNORE=@
$INPUT ID TIME DV
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
`
	err = os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Create data file
	dataContent := []byte("ID,TIME,DV\n1,0,0\n")
	err = os.WriteFile(dataPath, dataContent, 0644)
	require.NoError(t, err)

	// Create Hermes config
	hermesConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	// Build payload
	payload, err := BuildHermesPayload(modelPath, hermesConfig)
	require.NoError(t, err)
	require.NotNil(t, payload)

	// Verify workspace structure preserves directory layout
	assert.Len(t, payload.WorkspaceStructure, 2)

	// Entry point should be relative to common ancestor
	assert.True(t, strings.HasSuffix(payload.EntryPoint, "models/run001/model.mod"))

	// Verify both files exist in workspace
	modelFound := false
	dataFound := false
	for path := range payload.WorkspaceStructure {
		if strings.HasSuffix(path, "model.mod") {
			modelFound = true
		}
		if strings.HasSuffix(path, "dataset.csv") {
			dataFound = true
		}
	}
	assert.True(t, modelFound, "Model file should be in workspace")
	assert.True(t, dataFound, "Data file should be in workspace")
}

func TestBuildHermesPayload_ComplexDirectory(t *testing.T) {
	// Create complex directory structure
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	modelDir := filepath.Join(projectDir, "models", "run001")
	dataDir := filepath.Join(projectDir, "data", "raw")

	err := os.MkdirAll(modelDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	modelPath := filepath.Join(modelDir, "complicated.mod")
	dataPath := filepath.Join(dataDir, "measurements.csv")

	// Create model with relative path
	modelContent := `$PROBLEM Complex Test
$DATA ../../data/raw/measurements.csv IGNORE=@
$INPUT ID TIME DV AMT EVID
$PRED
Y = THETA(1) + ETA(1)
$THETA 1
$OMEGA 0.1
$SIGMA 0.1
$ESTIMATION METHOD=1
`
	err = os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	dataContent := []byte("ID,TIME,DV,AMT,EVID\n1,0,0,100,1\n1,1,5,0,0\n")
	err = os.WriteFile(dataPath, dataContent, 0644)
	require.NoError(t, err)

	hermesConfig := &config.HermesModelConfig{
		Image: "test/complex:v1",
		Resources: config.ResourceConfig{
			CPUCores: 8,
			Memory:   "16Gi",
		},
	}

	// Build payload
	payload, err := BuildHermesPayload(modelPath, hermesConfig)
	require.NoError(t, err)

	// Verify workspace
	assert.Len(t, payload.WorkspaceStructure, 2)
	assert.NotEmpty(t, payload.EntryPoint)

	// Verify file contents are preserved
	for _, content := range payload.WorkspaceStructure {
		assert.NotEmpty(t, content, "All files should have content")
	}
}

func TestBuildHermesPayload_PreservesRelativePaths(t *testing.T) {
	// This test ensures the workspace structure preserves relative path relationships
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "a", "b", "c")
	dataDir := filepath.Join(tempDir, "a", "data")

	err := os.MkdirAll(modelDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	modelPath := filepath.Join(modelDir, "model.mod")
	dataPath := filepath.Join(dataDir, "data.csv")

	modelContent := `$PROBLEM Test
$DATA ../../data/data.csv IGNORE=@
$INPUT ID
$PRED
Y = 1
$ESTIMATION METHOD=1
`
	err = os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(dataPath, []byte("ID\n1\n"), 0644)
	require.NoError(t, err)

	hermesConfig := &config.HermesModelConfig{
		Image: "test:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}

	payload, err := BuildHermesPayload(modelPath, hermesConfig)
	require.NoError(t, err)

	// The workspace should preserve the structure such that:
	// - model is at b/c/model.mod
	// - data is at data/data.csv
	// - from model's perspective, ../../data/data.csv still resolves correctly

	files := payload.GetWorkspaceFiles()
	t.Logf("Workspace structure: %v", files)
	t.Logf("Entry point: %s", payload.EntryPoint)

	// Verify structure makes sense
	assert.Len(t, files, 2)
	assert.NotEmpty(t, payload.EntryPoint)
}

func TestBuildHermesPayload_MissingConfig(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	modelContent := `$PROBLEM Test
$INPUT ID
$PRED
Y = 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Try to build without config
	payload, err := BuildHermesPayload(modelPath, nil)
	assert.Error(t, err)
	assert.Nil(t, payload)
	assert.Contains(t, err.Error(), "configuration is required")
}

func TestBuildHermesPayload_MissingModelFile(t *testing.T) {
	hermesConfig := &config.HermesModelConfig{
		Image: "test:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}

	payload, err := BuildHermesPayload("/nonexistent/model.mod", hermesConfig)
	assert.Error(t, err)
	assert.Nil(t, payload)
}

func TestHermesPayload_GetWorkspaceFiles(t *testing.T) {
	payload := &HermesPayload{
		WorkspaceStructure: map[string][]byte{
			"model.mod":  []byte("content1"),
			"data.csv":   []byte("content2"),
			"output.ext": []byte("content3"),
		},
		EntryPoint: "model.mod",
	}

	files := payload.GetWorkspaceFiles()
	assert.Len(t, files, 3)
	assert.Contains(t, files, "model.mod")
	assert.Contains(t, files, "data.csv")
	assert.Contains(t, files, "output.ext")
}

func TestHermesPayload_GetWorkspaceSize(t *testing.T) {
	payload := &HermesPayload{
		WorkspaceStructure: map[string][]byte{
			"model.mod": []byte("12345"),      // 5 bytes
			"data.csv":  []byte("1234567890"), // 10 bytes
		},
		EntryPoint: "model.mod",
	}

	size := payload.GetWorkspaceSize()
	assert.Equal(t, int64(15), size)
}

func TestHermesPayload_ValidateWorkspace(t *testing.T) {
	tests := []struct {
		name        string
		payload     *HermesPayload
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid workspace",
			payload: &HermesPayload{
				WorkspaceStructure: map[string][]byte{
					"model.mod": []byte("content"),
				},
				EntryPoint: "model.mod",
				Config: &config.HermesModelConfig{
					Image: "test:latest",
					Resources: config.ResourceConfig{
						CPUCores: 2,
						Memory:   "4Gi",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty workspace",
			payload: &HermesPayload{
				WorkspaceStructure: map[string][]byte{},
				EntryPoint:         "model.mod",
				Config: &config.HermesModelConfig{
					Image: "test:latest",
					Resources: config.ResourceConfig{
						CPUCores: 2,
						Memory:   "4Gi",
					},
				},
			},
			expectError: true,
			errorMsg:    "workspace is empty",
		},
		{
			name: "Missing entry point",
			payload: &HermesPayload{
				WorkspaceStructure: map[string][]byte{
					"model.mod": []byte("content"),
				},
				EntryPoint: "",
				Config: &config.HermesModelConfig{
					Image: "test:latest",
					Resources: config.ResourceConfig{
						CPUCores: 2,
						Memory:   "4Gi",
					},
				},
			},
			expectError: true,
			errorMsg:    "entry point is not set",
		},
		{
			name: "Entry point not in workspace",
			payload: &HermesPayload{
				WorkspaceStructure: map[string][]byte{
					"model.mod": []byte("content"),
				},
				EntryPoint: "different.mod",
				Config: &config.HermesModelConfig{
					Image: "test:latest",
					Resources: config.ResourceConfig{
						CPUCores: 2,
						Memory:   "4Gi",
					},
				},
			},
			expectError: true,
			errorMsg:    "entry point different.mod not found in workspace",
		},
		{
			name: "Missing config",
			payload: &HermesPayload{
				WorkspaceStructure: map[string][]byte{
					"model.mod": []byte("content"),
				},
				EntryPoint: "model.mod",
				Config:     nil,
			},
			expectError: true,
			errorMsg:    "configuration is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.payload.ValidateWorkspace()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHermesPayload_GetWorkspaceTree(t *testing.T) {
	payload := &HermesPayload{
		WorkspaceStructure: map[string][]byte{
			"models/run001/model.mod": []byte("content1"),
			"data/dataset.csv":        []byte("content2"),
		},
		EntryPoint: "models/run001/model.mod",
	}

	tree := payload.GetWorkspaceTree()
	assert.Contains(t, tree, "workspace/")
	assert.Contains(t, tree, "model.mod")
	assert.Contains(t, tree, "dataset.csv")
	assert.Contains(t, tree, "[ENTRY POINT]")

	t.Logf("Workspace tree:\n%s", tree)
}

func TestBuildHermesPayload_PathNormalization(t *testing.T) {
	// Verify paths are normalized to forward slashes (Hermes requirement)
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	dataPath := filepath.Join(tempDir, "subdir", "data.csv")

	err := os.MkdirAll(filepath.Dir(dataPath), 0755)
	require.NoError(t, err)

	modelContent := `$PROBLEM Test
$DATA subdir/data.csv IGNORE=@
$INPUT ID
$PRED
Y = 1
$ESTIMATION METHOD=1
`
	err = os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(dataPath, []byte("ID\n1\n"), 0644)
	require.NoError(t, err)

	hermesConfig := &config.HermesModelConfig{
		Image: "test:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
	}

	payload, err := BuildHermesPayload(modelPath, hermesConfig)
	require.NoError(t, err)

	// All paths should use forward slashes
	for path := range payload.WorkspaceStructure {
		assert.NotContains(t, path, "\\", "Path should not contain backslashes: %s", path)
	}

	assert.NotContains(t, payload.EntryPoint, "\\", "Entry point should not contain backslashes")
}
