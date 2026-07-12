package runlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRunLogStore(t *testing.T) {
	store := NewRunLogStore("/tmp/test", "model.mod")
	assert.NotNil(t, store)
	assert.Equal(t, "/tmp/test", store.baseDir)
	assert.Equal(t, "model.mod", store.modelFile)
}

func TestRunLogStore_Load(t *testing.T) {
	t.Run("creates_directory_if_missing", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")

		err := store.Load()
		require.NoError(t, err)

		// Directory should exist
		_, err = os.Stat(filepath.Join(baseDir, ".janus", "runlog"))
		assert.NoError(t, err)
	})

	t.Run("empty_store_has_zero_count", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")

		err := store.Load()
		require.NoError(t, err)
		assert.Equal(t, 0, store.Count())
	})
}

func TestRunLogStore_AddRun(t *testing.T) {
	t.Run("generates_uuid_if_empty", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")
		require.NoError(t, store.Load())

		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "running",
		}

		err := store.AddRun(record)
		require.NoError(t, err)

		// ID should be set
		assert.NotEmpty(t, record.ID)
		assert.Len(t, record.ID, 36) // UUID format
	})

	t.Run("sets_timestamp_if_zero", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")
		require.NoError(t, store.Load())

		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "running",
		}

		before := time.Now()
		err := store.AddRun(record)
		require.NoError(t, err)
		after := time.Now()

		assert.True(t, record.Timestamp.After(before) || record.Timestamp.Equal(before))
		assert.True(t, record.Timestamp.Before(after) || record.Timestamp.Equal(after))
	})

	t.Run("writes_file_to_disk", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")
		require.NoError(t, store.Load())

		record := &RunRecord{
			ID:        "test-uuid-123",
			ModelFile: "model.mod",
			Status:    "running",
			Timestamp: time.Now(),
		}

		err := store.AddRun(record)
		require.NoError(t, err)

		// File should exist
		filename := filepath.Join(baseDir, ".janus", "runlog", "test-uuid-123.json")
		_, err = os.Stat(filename)
		assert.NoError(t, err)
	})

	t.Run("updates_index", func(t *testing.T) {
		baseDir := t.TempDir()
		store := NewRunLogStore(baseDir, "model.mod")
		require.NoError(t, store.Load())

		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "running",
		}

		err := store.AddRun(record)
		require.NoError(t, err)

		assert.Equal(t, 1, store.Count())
	})
}

func TestRunLogStore_GetRun(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add a run
	original := &RunRecord{
		ID:        "test-get-run",
		ModelFile: "model.mod",
		Command:   "nmfe75 model.mod",
		Status:    "completed",
		ExitCode:  0,
		Timestamp: time.Now(),
	}
	require.NoError(t, store.AddRun(original))

	// Retrieve it
	retrieved, err := store.GetRun("test-get-run")
	require.NoError(t, err)

	assert.Equal(t, original.ID, retrieved.ID)
	assert.Equal(t, original.ModelFile, retrieved.ModelFile)
	assert.Equal(t, original.Command, retrieved.Command)
	assert.Equal(t, original.Status, retrieved.Status)
}

func TestRunLogStore_GetRuns(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add multiple runs
	for i := 0; i < 5; i++ {
		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "completed",
		}
		require.NoError(t, store.AddRun(record))
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	t.Run("returns_all_runs", func(t *testing.T) {
		runs, err := store.GetRuns(100, 0)
		require.NoError(t, err)
		assert.Len(t, runs, 5)
	})

	t.Run("respects_limit", func(t *testing.T) {
		runs, err := store.GetRuns(2, 0)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
	})

	t.Run("respects_offset", func(t *testing.T) {
		runs, err := store.GetRuns(100, 3)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
	})

	t.Run("returns_newest_first", func(t *testing.T) {
		runs, err := store.GetRuns(100, 0)
		require.NoError(t, err)

		for i := 1; i < len(runs); i++ {
			assert.True(t, runs[i-1].Timestamp.After(runs[i].Timestamp) ||
				runs[i-1].Timestamp.Equal(runs[i].Timestamp))
		}
	})
}

func TestRunLogStore_IndexRebuild(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add runs
	for i := 0; i < 3; i++ {
		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "completed",
		}
		require.NoError(t, store.AddRun(record))
	}

	// Delete the index file
	indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
	require.NoError(t, os.Remove(indexPath))

	// Create new store and load - should rebuild index
	newStore := NewRunLogStore(baseDir, "model.mod")
	err := newStore.Load()
	require.NoError(t, err)

	// All runs should be accessible
	assert.Equal(t, 3, newStore.Count())

	runs, err := newStore.GetRuns(100, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 3)
}

func TestRunLogStore_CorruptedIndexRecovery(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add runs
	for i := 0; i < 3; i++ {
		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "completed",
		}
		require.NoError(t, store.AddRun(record))
	}

	// Corrupt the index file (simulate merge conflict)
	indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
	err := os.WriteFile(indexPath, []byte("<<<<<<< HEAD\ninvalid json\n=======\nalso invalid\n>>>>>>>"), 0600)
	require.NoError(t, err)

	// Create new store and load - should rebuild from individual files
	newStore := NewRunLogStore(baseDir, "model.mod")
	err = newStore.Load()
	require.NoError(t, err)

	// All runs should still be accessible
	assert.Equal(t, 3, newStore.Count())
}

func TestRunLogStore_MergeScenario(t *testing.T) {
	// Simulate two branches adding runs independently
	baseDir := t.TempDir()

	// Branch A adds runs
	storeA := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, storeA.Load())

	runA1 := &RunRecord{
		ModelFile: "model.mod",
		Status:    "completed",
		Command:   "Branch A - Run 1",
	}
	runA2 := &RunRecord{
		ModelFile: "model.mod",
		Status:    "completed",
		Command:   "Branch A - Run 2",
	}
	require.NoError(t, storeA.AddRun(runA1))
	require.NoError(t, storeA.AddRun(runA2))

	// Branch B adds runs (simulated by creating new store - index may conflict)
	storeB := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, storeB.Load())

	runB1 := &RunRecord{
		ModelFile: "model.mod",
		Status:    "completed",
		Command:   "Branch B - Run 1",
	}
	require.NoError(t, storeB.AddRun(runB1))

	// After "merge" - delete index to simulate conflict resolution
	indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
	os.Remove(indexPath)

	// Reload store - should rebuild and find all runs
	mergedStore := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, mergedStore.Load())

	// Verify ALL runs present (no conflicts, no data loss)
	assert.Equal(t, 3, mergedStore.Count())

	runs, err := mergedStore.GetRuns(100, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 3)
}

func TestRunLogStore_UpdateRun(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add a run
	record := &RunRecord{
		ID:        "test-update",
		ModelFile: "model.mod",
		Status:    "running",
		Timestamp: time.Now(),
	}
	require.NoError(t, store.AddRun(record))

	// Update it
	record.Status = "completed"
	record.ExitCode = 0
	require.NoError(t, record.SetStdout("NONMEM output here"))
	require.NoError(t, store.UpdateRun(record))

	// Retrieve and verify
	updated, err := store.GetRun("test-update")
	require.NoError(t, err)
	assert.Equal(t, "completed", updated.Status)
	assert.Equal(t, 0, updated.ExitCode)
}

func TestRunLogStore_GetLatestRun(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	// Add runs with delays
	for i := 0; i < 3; i++ {
		record := &RunRecord{
			ModelFile: "model.mod",
			Status:    "completed",
			Command:   "run-" + string(rune('A'+i)),
		}
		require.NoError(t, store.AddRun(record))
		time.Sleep(time.Millisecond * 10)
	}

	// Latest should be the last one added
	latest, err := store.GetLatestRun()
	require.NoError(t, err)
	assert.Equal(t, "run-C", latest.Command)
}

func TestRunLogStore_AtomicWrite(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "model.mod")
	require.NoError(t, store.Load())

	record := &RunRecord{
		ID:        "test-atomic",
		ModelFile: "model.mod",
		Status:    "running",
		Timestamp: time.Now(),
	}

	err := store.AddRun(record)
	require.NoError(t, err)

	// Temp file should not exist
	tmpFile := filepath.Join(baseDir, ".janus", "runlog", "test-atomic.json.tmp")
	_, err = os.Stat(tmpFile)
	assert.True(t, os.IsNotExist(err))

	// Final file should exist
	finalFile := filepath.Join(baseDir, ".janus", "runlog", "test-atomic.json")
	_, err = os.Stat(finalFile)
	assert.NoError(t, err)
}
