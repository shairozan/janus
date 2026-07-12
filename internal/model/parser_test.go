//go:build unit
// +build unit

package model

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModelDependencies_SingleDataFile(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	dataPath := filepath.Join(tempDir, "data.csv")

	// Create model file with $DATA statement
	modelContent := `$PROBLEM Test Model
$DATA data.csv IGNORE=@

$INPUT ID TIME DV AMT

$PRED
Y = THETA(1) + ETA(1)

$THETA 1
$OMEGA 0.1
$SIGMA 0.1

$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Create data file
	err = os.WriteFile(dataPath, []byte("ID,TIME,DV,AMT\n1,0,0,100\n"), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)
	require.NotNil(t, deps)

	// Verify results
	assert.Equal(t, modelPath, deps.ModelFile)
	assert.Len(t, deps.DataFiles, 1)
	assert.Equal(t, dataPath, deps.DataFiles[0])
	assert.Equal(t, "data.csv", deps.RelativePaths[dataPath])
}

func TestParseModelDependencies_RelativePaths(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "models", "run001")
	dataDir := filepath.Join(tempDir, "data")

	err := os.MkdirAll(modelDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	modelPath := filepath.Join(modelDir, "model.mod")
	dataPath := filepath.Join(dataDir, "dataset.csv")

	// Create model file with relative path to data (../../data/dataset.csv)
	modelContent := `$PROBLEM Test Model
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
	err = os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,0\n"), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, deps.DataFiles, 1)
	assert.Equal(t, dataPath, deps.DataFiles[0])
	assert.Equal(t, "../../data/dataset.csv", deps.RelativePaths[dataPath])
}

func TestParseModelDependencies_AbsolutePath(t *testing.T) {
	// Create temp files
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	dataPath := filepath.Join(tempDir, "absolute_data.csv")

	// Create model file with absolute path
	modelContent := fmt.Sprintf(`$PROBLEM Test Model
$DATA %s IGNORE=@

$INPUT ID TIME DV

$PRED
Y = THETA(1)

$THETA 1
$ESTIMATION METHOD=1
`, dataPath)

	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(dataPath, []byte("ID,TIME,DV\n1,0,0\n"), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)

	// Verify results
	assert.Len(t, deps.DataFiles, 1)
	assert.Equal(t, dataPath, deps.DataFiles[0])
	assert.Equal(t, dataPath, deps.RelativePaths[dataPath])
}

func TestParseModelDependencies_MissingDataFile(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Create model file referencing non-existent data
	modelContent := `$PROBLEM Test Model
$DATA nonexistent.csv IGNORE=@

$INPUT ID TIME DV
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Parse dependencies - should fail
	deps, err := ParseModelDependencies(modelPath)
	assert.Error(t, err)
	assert.Nil(t, deps)
	assert.Contains(t, err.Error(), "data file not found")
	assert.Contains(t, err.Error(), "nonexistent.csv")
}

func TestParseModelDependencies_AltDataDirFallback(t *testing.T) {
	modelDir := t.TempDir()
	altDir := t.TempDir()

	modelPath := filepath.Join(modelDir, "model.mod")
	modelContent := "$PROBLEM Test\n$DATA data.csv IGNORE=@\n"
	require.NoError(t, os.WriteFile(modelPath, []byte(modelContent), 0644))

	// The data file exists only in the alternative directory.
	altDataPath := filepath.Join(altDir, "data.csv")
	require.NoError(t, os.WriteFile(altDataPath, []byte("ID,TIME,DV\n"), 0644))

	// Without the alt dir it fails; with it, the file resolves there.
	_, err := ParseModelDependencies(modelPath)
	require.Error(t, err)

	deps, err := ParseModelDependenciesWithDataDirs(modelPath, []string{altDir})
	require.NoError(t, err)
	require.Len(t, deps.DataFiles, 1)
	assert.Equal(t, altDataPath, deps.DataFiles[0])
}

func TestParseModelDependencies_ModelDirWinsOverAltDir(t *testing.T) {
	modelDir := t.TempDir()
	altDir := t.TempDir()

	modelPath := filepath.Join(modelDir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROBLEM\n$DATA data.csv\n"), 0644))

	localData := filepath.Join(modelDir, "data.csv")
	require.NoError(t, os.WriteFile(localData, []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(altDir, "data.csv"), []byte("y"), 0644))

	deps, err := ParseModelDependenciesWithDataDirs(modelPath, []string{altDir})
	require.NoError(t, err)
	assert.Equal(t, localData, deps.DataFiles[0])
}

func TestParseModelDependencies_MissingModelFile(t *testing.T) {
	deps, err := ParseModelDependencies("/nonexistent/model.mod")
	assert.Error(t, err)
	assert.Nil(t, deps)
	assert.Contains(t, err.Error(), "model file not found")
}

func TestParseModelDependencies_NoDataFiles(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Create model file without $DATA statement
	modelContent := `$PROBLEM Test Model

$INPUT ID TIME DV

$PRED
Y = THETA(1)

$THETA 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)

	// Verify results - no data files found
	assert.Len(t, deps.DataFiles, 0)
	assert.Equal(t, modelPath, deps.ModelFile)
}

func TestParseModelDependencies_CommentsIgnored(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")

	// Create model file with commented $DATA
	modelContent := `$PROBLEM Test Model
; $DATA commented_data.csv IGNORE=@
; This is a comment

$INPUT ID TIME DV
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)

	// Verify no data files found (commented line ignored)
	assert.Len(t, deps.DataFiles, 0)
}

func TestParseModelDependencies_CaseInsensitive(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "model.mod")
	dataPath := filepath.Join(tempDir, "data.csv")

	// Create model file with lowercase $data
	modelContent := `$PROBLEM Test Model
$data data.csv IGNORE=@

$INPUT ID TIME DV
$PRED
Y = THETA(1)
$THETA 1
$ESTIMATION METHOD=1
`
	err := os.WriteFile(modelPath, []byte(modelContent), 0644)
	require.NoError(t, err)

	err = os.WriteFile(dataPath, []byte("ID,TIME,DV\n"), 0644)
	require.NoError(t, err)

	// Parse dependencies
	deps, err := ParseModelDependencies(modelPath)
	require.NoError(t, err)

	// Verify data file found (case-insensitive match)
	assert.Len(t, deps.DataFiles, 1)
	assert.Equal(t, dataPath, deps.DataFiles[0])
}

func TestGetCommonAncestor_ColocatedFiles(t *testing.T) {
	tempDir := t.TempDir()

	deps := &ModelDependencies{
		ModelFile: filepath.Join(tempDir, "model.mod"),
		DataFiles: []string{filepath.Join(tempDir, "data.csv")},
	}

	common := deps.GetCommonAncestor()
	assert.Equal(t, tempDir, common)
}

func TestGetCommonAncestor_NonColocatedFiles(t *testing.T) {
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "models", "run001")
	dataDir := filepath.Join(tempDir, "data")

	deps := &ModelDependencies{
		ModelFile: filepath.Join(modelDir, "model.mod"),
		DataFiles: []string{filepath.Join(dataDir, "data.csv")},
	}

	common := deps.GetCommonAncestor()
	assert.Equal(t, tempDir, common)
}

func TestGetCommonAncestor_NoDataFiles(t *testing.T) {
	tempDir := t.TempDir()
	modelDir := filepath.Join(tempDir, "models")

	deps := &ModelDependencies{
		ModelFile: filepath.Join(modelDir, "model.mod"),
		DataFiles: []string{},
	}

	common := deps.GetCommonAncestor()
	assert.Equal(t, modelDir, common)
}

func TestFindCommonPath(t *testing.T) {
	tests := []struct {
		name     string
		path1    string
		path2    string
		expected string
	}{
		{
			name:     "Same directory",
			path1:    "/home/user/models",
			path2:    "/home/user/models",
			expected: "/home/user/models",
		},
		{
			name:     "Subdirectory",
			path1:    "/home/user/models",
			path2:    "/home/user/data",
			expected: "/home/user",
		},
		{
			name:     "Deep nesting",
			path1:    "/home/user/projects/janus/models/run001",
			path2:    "/home/user/projects/janus/data",
			expected: "/home/user/projects/janus",
		},
		{
			name:     "Root only",
			path1:    "/home/user1",
			path2:    "/var/data",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findCommonPath(tt.path1, tt.path2)
			assert.Equal(t, filepath.Clean(tt.expected), filepath.Clean(result))
		})
	}
}
