package runpolicy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeModel creates a model dir with run1.mod and the given extra files.
func writeModel(t *testing.T, extra ...string) (dir, modelPath string) {
	t.Helper()

	dir = t.TempDir()
	modelPath = filepath.Join(dir, "run1.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROBLEM"), 0o600))

	for _, name := range extra {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}

	return dir, modelPath
}

func TestZeroPolicyIsNoOp(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst", "run1.ext")

	var p Policy
	archive, err := p.BeforeRun(modelPath)
	require.NoError(t, err)
	assert.Empty(t, archive)
	require.NoError(t, p.AfterRun(modelPath))

	// Nothing added or removed.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestBeforeRunSequentialArchivesPriorResults(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst", "run1.ext")

	p := Policy{Overwrite: OverwriteSequential}
	archive, err := p.BeforeRun(modelPath)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "modelfit_dir1"), archive)

	// Prior results moved; the model file stays put.
	assert.FileExists(t, filepath.Join(archive, "run1.lst"))
	assert.FileExists(t, filepath.Join(archive, "run1.ext"))
	assert.FileExists(t, modelPath)
	assert.NoFileExists(t, filepath.Join(dir, "run1.lst"))
}

func TestBeforeRunSequentialNoPriorResults(t *testing.T) {
	// Only the model exists — nothing to archive.
	_, modelPath := writeModel(t)

	p := Policy{Overwrite: OverwriteSequential}
	archive, err := p.BeforeRun(modelPath)
	require.NoError(t, err)
	assert.Empty(t, archive)
}

func TestBeforeRunSequentialIncrementsDir(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "modelfit_dir1"), 0o755))

	p := Policy{Overwrite: OverwriteSequential}
	archive, err := p.BeforeRun(modelPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "modelfit_dir2"), archive)
}

func TestAfterRunBackup(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst", "run1.ext")

	p := Policy{AutoBackup: true}
	require.NoError(t, p.AfterRun(modelPath))

	backup := filepath.Join(dir, "backup", "run1")
	assert.FileExists(t, filepath.Join(backup, "run1.mod"))
	assert.FileExists(t, filepath.Join(backup, "run1.lst"))
	assert.FileExists(t, filepath.Join(backup, "run1.ext"))

	// Originals are copied, not moved.
	assert.FileExists(t, filepath.Join(dir, "run1.lst"))
}

func TestAfterRunBackupIncrementsDir(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "backup", "run1"), 0o755))

	p := Policy{AutoBackup: true}
	require.NoError(t, p.AfterRun(modelPath))

	assert.DirExists(t, filepath.Join(dir, "backup", "run1-1"))
}

func TestAfterRunCleanup(t *testing.T) {
	dir, modelPath := writeModel(t, "run1.lst", "run1.cov", "run1.cor", "FCON")

	p := Policy{CleanupGlobs: []string{"*.cov", "*.cor", "FCON"}}
	require.NoError(t, p.AfterRun(modelPath))

	assert.NoFileExists(t, filepath.Join(dir, "run1.cov"))
	assert.NoFileExists(t, filepath.Join(dir, "run1.cor"))
	assert.NoFileExists(t, filepath.Join(dir, "FCON"))
	// Results we keep are untouched.
	assert.FileExists(t, filepath.Join(dir, "run1.lst"))
	assert.FileExists(t, modelPath)
}

func TestCleanupSkipsDirectories(t *testing.T) {
	dir, modelPath := writeModel(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "keep_dir"), 0o755))

	p := Policy{CleanupGlobs: []string{"keep_*"}}
	require.NoError(t, p.AfterRun(modelPath))

	assert.DirExists(t, filepath.Join(dir, "keep_dir"))
}

func TestNextSequentialDir(t *testing.T) {
	dir := t.TempDir()

	d1, err := NextSequentialDir(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "modelfit_dir1"), d1)

	require.NoError(t, os.Mkdir(d1, 0o755))
	d2, err := NextSequentialDir(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "modelfit_dir2"), d2)
}
