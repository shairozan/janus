package runlog

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrentAddRunCrossStore simulates a GUI and a daemon (two independent
// RunLogStore instances over the same directory, each with its own in-memory
// index) writing runs at the same time. Without the cross-process locked
// read-modify-write, the two index writers would clobber each other and lose
// entries. With it, every run must survive in a fresh read of the index.
func TestConcurrentAddRunCrossStore(t *testing.T) {
	dir := t.TempDir()

	storeA := NewRunLogStore(dir, "model")
	require.NoError(t, storeA.Load())

	storeB := NewRunLogStore(dir, "model")
	require.NoError(t, storeB.Load())

	const perStore = 25

	add := func(store *RunLogStore, prefix string) {
		for i := 0; i < perStore; i++ {
			rec := &RunRecord{
				ModelFile: "model.mod",
				Command:   fmt.Sprintf("%s-%d", prefix, i),
				Status:    "completed",
			}
			require.NoError(t, store.AddRun(rec))
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() { defer wg.Done(); add(storeA, "a") }()
	go func() { defer wg.Done(); add(storeB, "b") }()

	wg.Wait()

	// A fresh reader must see every run from both writers — none lost to a clobber.
	reader := NewRunLogStore(dir, "model")
	require.NoError(t, reader.Load())

	assert.Equal(t, 2*perStore, reader.Count(), "all concurrently-added runs must be indexed")

	runs, err := reader.GetRuns(1000, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 2*perStore)
}

// TestConcurrentUpdateRunCrossStore exercises the locked RMW on the update path:
// one store adds runs while another updates them; the index must stay complete.
func TestConcurrentUpdateRunCrossStore(t *testing.T) {
	dir := t.TempDir()

	writer := NewRunLogStore(dir, "model")
	require.NoError(t, writer.Load())

	const total = 20

	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		rec := &RunRecord{ModelFile: "model.mod", Status: "running"}
		require.NoError(t, writer.AddRun(rec))
		ids = append(ids, rec.ID)
	}

	updater := NewRunLogStore(dir, "model")
	require.NoError(t, updater.Load())

	var wg sync.WaitGroup
	wg.Add(2)

	// Updater flips each run to completed via its own (initially stale) store.
	go func() {
		defer wg.Done()
		for _, id := range ids {
			rec, err := updater.GetRun(id)
			require.NoError(t, err)
			rec.Status = "completed"
			rec.Signature = ""
			require.NoError(t, updater.UpdateRun(rec))
		}
	}()

	// Concurrently, the writer adds more runs.
	go func() {
		defer wg.Done()
		for i := 0; i < total; i++ {
			require.NoError(t, writer.AddRun(&RunRecord{ModelFile: "model.mod", Status: "completed"}))
		}
	}()

	wg.Wait()

	reader := NewRunLogStore(dir, "model")
	require.NoError(t, reader.Load())
	assert.Equal(t, 2*total, reader.Count(), "updates must not drop concurrently-added runs")
}
