package runlog_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/runlog"
)

// TestValidation_DirectoryStructure validates the IQ requirement that Janus creates
// the correct directory structure for run log storage.
func TestValidation_DirectoryStructure(t *testing.T) {
	t.Run("creates_janus_runlog_directory", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "test_model")
		require.NoError(t, store.Load())

		// Validate directory structure exists
		runlogDir := filepath.Join(baseDir, ".janus", "runlog")
		info, err := os.Stat(runlogDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir(), "runlog should be a directory")
	})

	t.Run("stores_runs_as_individual_files", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "test_model")
		require.NoError(t, store.Load())

		// Add a run
		record := &runlog.RunRecord{
			ID:        "validation-uuid-001",
			ModelFile: "test_model.mod",
			Status:    "completed",
			Timestamp: time.Now(),
		}
		require.NoError(t, store.AddRun(record))

		// Validate individual file exists
		runFile := filepath.Join(baseDir, ".janus", "runlog", "validation-uuid-001.json")
		_, err := os.Stat(runFile)
		assert.NoError(t, err, "individual run file should exist")
	})
}

// TestValidation_MergeConflictPrevention validates the OQ requirement that
// concurrent branch modifications don't cause merge conflicts.
func TestValidation_MergeConflictPrevention(t *testing.T) {
	t.Run("independent_runs_have_unique_files", func(t *testing.T) {
		baseDir := t.TempDir()

		// Simulate Branch A adding runs
		storeA := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, storeA.Load())

		runA := &runlog.RunRecord{
			ID:        "branch-a-uuid-001",
			ModelFile: "model.mod",
			Command:   "nmfe75 model.mod",
			Status:    "completed",
			Timestamp: time.Now(),
		}
		require.NoError(t, storeA.AddRun(runA))

		// Simulate Branch B adding runs (separate store instance, same directory)
		storeB := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, storeB.Load())

		runB := &runlog.RunRecord{
			ID:        "branch-b-uuid-001",
			ModelFile: "model.mod",
			Command:   "nmfe75 model.mod",
			Status:    "completed",
			Timestamp: time.Now(),
		}
		require.NoError(t, storeB.AddRun(runB))

		// Validate both files exist independently (no conflict possible)
		fileA := filepath.Join(baseDir, ".janus", "runlog", "branch-a-uuid-001.json")
		fileB := filepath.Join(baseDir, ".janus", "runlog", "branch-b-uuid-001.json")

		_, errA := os.Stat(fileA)
		_, errB := os.Stat(fileB)

		assert.NoError(t, errA, "Branch A run file should exist")
		assert.NoError(t, errB, "Branch B run file should exist")
	})

	t.Run("post_merge_index_rebuild_finds_all_runs", func(t *testing.T) {
		baseDir := t.TempDir()

		// Create runs from "Branch A"
		storeA := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, storeA.Load())

		for i := 0; i < 3; i++ {
			require.NoError(t, storeA.AddRun(&runlog.RunRecord{
				ModelFile: "model.mod",
				Status:    "completed",
				Command:   "Branch A run",
			}))
		}

		// Create runs from "Branch B"
		storeB := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, storeB.Load())

		for i := 0; i < 2; i++ {
			require.NoError(t, storeB.AddRun(&runlog.RunRecord{
				ModelFile: "model.mod",
				Status:    "completed",
				Command:   "Branch B run",
			}))
		}

		// Simulate post-merge: delete index (as if it had conflicts)
		indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
		os.Remove(indexPath)

		// New store should rebuild index and find ALL runs
		mergedStore := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, mergedStore.Load())

		// Validate all 5 runs are present
		assert.Equal(t, 5, mergedStore.Count(), "All runs from both branches should be present")

		runs := mergedStore.GetAllRuns()
		assert.Len(t, runs, 5)
	})
}

// TestValidation_AtomicWrites validates the OQ requirement that writes are atomic
// and cannot result in corrupted files.
func TestValidation_AtomicWrites(t *testing.T) {
	t.Run("no_temp_files_remain_after_write", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		record := &runlog.RunRecord{
			ID:        "atomic-test-uuid",
			ModelFile: "model.mod",
			Status:    "completed",
			Timestamp: time.Now(),
		}
		require.NoError(t, store.AddRun(record))

		// Check no .tmp files exist
		runlogDir := filepath.Join(baseDir, ".janus", "runlog")
		matches, err := filepath.Glob(filepath.Join(runlogDir, "*.tmp"))
		require.NoError(t, err)
		assert.Empty(t, matches, "No temporary files should remain after successful write")
	})

	t.Run("final_file_contains_valid_json", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		record := &runlog.RunRecord{
			ID:        "json-test-uuid",
			ModelFile: "model.mod",
			Command:   "nmfe75 model.mod",
			Status:    "completed",
			Timestamp: time.Now(),
			ExitCode:  0,
		}
		require.NoError(t, store.AddRun(record))

		// Read back and validate
		retrieved, err := store.GetRun("json-test-uuid")
		require.NoError(t, err)
		assert.Equal(t, record.ID, retrieved.ID)
		assert.Equal(t, record.Command, retrieved.Command)
		assert.Equal(t, record.Status, retrieved.Status)
	})
}

// TestValidation_UUIDGeneration validates the OQ requirement that run IDs are
// unique and time-ordered.
func TestValidation_UUIDGeneration(t *testing.T) {
	t.Run("auto_generates_uuid_when_empty", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		record := &runlog.RunRecord{
			ModelFile: "model.mod",
			Status:    "running",
		}
		require.NoError(t, store.AddRun(record))

		// ID should be auto-generated (36 char UUID format)
		assert.NotEmpty(t, record.ID)
		assert.Len(t, record.ID, 36, "Generated ID should be a UUID (36 characters)")
	})

	t.Run("uuids_are_unique_across_runs", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		ids := make(map[string]bool)

		for i := 0; i < 100; i++ {
			record := &runlog.RunRecord{
				ModelFile: "model.mod",
				Status:    "completed",
			}
			require.NoError(t, store.AddRun(record))

			assert.False(t, ids[record.ID], "UUID should be unique")
			ids[record.ID] = true
		}
	})

	t.Run("auto_sets_timestamp_when_zero", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		before := time.Now()

		record := &runlog.RunRecord{
			ModelFile: "model.mod",
			Status:    "running",
		}
		require.NoError(t, store.AddRun(record))

		after := time.Now()

		assert.True(t, record.Timestamp.After(before) || record.Timestamp.Equal(before))
		assert.True(t, record.Timestamp.Before(after) || record.Timestamp.Equal(after))
	})
}

// TestValidation_IndexRecovery validates the OQ requirement that the index
// can recover from corruption or conflicts.
func TestValidation_IndexRecovery(t *testing.T) {
	t.Run("recovers_from_missing_index", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		// Add runs
		for i := 0; i < 5; i++ {
			require.NoError(t, store.AddRun(&runlog.RunRecord{
				ModelFile: "model.mod",
				Status:    "completed",
			}))
		}

		// Delete index
		indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
		require.NoError(t, os.Remove(indexPath))

		// Reload store - should rebuild index
		newStore := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, newStore.Load())

		assert.Equal(t, 5, newStore.Count())
	})

	t.Run("recovers_from_corrupted_index", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		// Add runs
		for i := 0; i < 5; i++ {
			require.NoError(t, store.AddRun(&runlog.RunRecord{
				ModelFile: "model.mod",
				Status:    "completed",
			}))
		}

		// Corrupt index with git merge conflict markers
		indexPath := filepath.Join(baseDir, ".janus", "runlog", "index.json")
		corruptData := []byte("<<<<<<< HEAD\n{\"invalid\": true}\n=======\n{\"also\": \"invalid\"}\n>>>>>>>")
		require.NoError(t, os.WriteFile(indexPath, corruptData, 0600))

		// Reload store - should detect corruption and rebuild
		newStore := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, newStore.Load())

		assert.Equal(t, 5, newStore.Count(), "All runs should be recovered from individual files")
	})
}

// TestValidation_DataIntegrity validates the OQ requirement that run data
// maintains integrity through storage and retrieval.
func TestValidation_DataIntegrity(t *testing.T) {
	t.Run("preserves_all_record_fields", func(t *testing.T) {
		baseDir := t.TempDir()
		store := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, store.Load())

		original := &runlog.RunRecord{
			ID:         "integrity-test-uuid",
			Timestamp:  time.Now().Truncate(time.Millisecond), // Truncate for comparison
			ModelFile:  "/path/to/model.mod",
			Command:    "nmfe75 model.mod model.lst -parafile=/path/to/parafile.pnm",
			ExitCode:   0,
			IsParallel: true,
			Cores:      8,
			IsGrid:     false,
			Status:     "completed",
		}

		// Set compressed fields
		require.NoError(t, original.SetStdout("NONMEM output content"))
		require.NoError(t, original.SetStderr("Warning messages"))
		require.NoError(t, original.SetDescription("Test run for validation"))

		require.NoError(t, store.AddRun(original))

		// Create new store and retrieve
		newStore := runlog.NewRunLogStore(baseDir, "model")
		require.NoError(t, newStore.Load())

		retrieved, err := newStore.GetRun("integrity-test-uuid")
		require.NoError(t, err)

		// Validate all fields preserved
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Timestamp.Unix(), retrieved.Timestamp.Unix())
		assert.Equal(t, original.ModelFile, retrieved.ModelFile)
		assert.Equal(t, original.Command, retrieved.Command)
		assert.Equal(t, original.ExitCode, retrieved.ExitCode)
		assert.Equal(t, original.IsParallel, retrieved.IsParallel)
		assert.Equal(t, original.Cores, retrieved.Cores)
		assert.Equal(t, original.IsGrid, retrieved.IsGrid)
		assert.Equal(t, original.Status, retrieved.Status)

		// Validate compressed fields
		stdout, err := retrieved.GetStdout()
		require.NoError(t, err)
		assert.Equal(t, "NONMEM output content", stdout)

		stderr, err := retrieved.GetStderr()
		require.NoError(t, err)
		assert.Equal(t, "Warning messages", stderr)

		desc, err := retrieved.GetDescription()
		require.NoError(t, err)
		assert.Equal(t, "Test run for validation", desc)
	})
}

// TestValidation_Pagination validates the OQ requirement that run history
// can be efficiently queried with pagination.
func TestValidation_Pagination(t *testing.T) {
	baseDir := t.TempDir()
	store := runlog.NewRunLogStore(baseDir, "model")
	require.NoError(t, store.Load())

	// Add 20 runs with small delays for timestamp ordering
	for i := 0; i < 20; i++ {
		require.NoError(t, store.AddRun(&runlog.RunRecord{
			ModelFile: "model.mod",
			Status:    "completed",
		}))
		time.Sleep(time.Millisecond)
	}

	t.Run("respects_limit_parameter", func(t *testing.T) {
		runs, err := store.GetRuns(5, 0)
		require.NoError(t, err)
		assert.Len(t, runs, 5)
	})

	t.Run("respects_offset_parameter", func(t *testing.T) {
		runs, err := store.GetRuns(100, 15)
		require.NoError(t, err)
		assert.Len(t, runs, 5, "Should return remaining 5 runs after offset 15")
	})

	t.Run("returns_newest_first", func(t *testing.T) {
		runs, err := store.GetRuns(10, 0)
		require.NoError(t, err)

		for i := 1; i < len(runs); i++ {
			assert.True(t,
				runs[i-1].Timestamp.After(runs[i].Timestamp) ||
					runs[i-1].Timestamp.Equal(runs[i].Timestamp),
				"Runs should be ordered newest first")
		}
	})

	t.Run("empty_result_for_large_offset", func(t *testing.T) {
		runs, err := store.GetRuns(10, 1000)
		require.NoError(t, err)
		assert.Empty(t, runs)
	})
}
