//go:build gui
// +build gui

package gui

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/runlog"
)

// TestRefreshCachedRunsCollapsesSagaChildren verifies that a saga's per-fit child
// records are filtered out of the top-level run history, leaving only the parent.
func TestRefreshCachedRunsCollapsesSagaChildren(t *testing.T) {
	baseDir := t.TempDir()
	store := runlog.NewRunLogStore(baseDir, "acop.mod")
	require.NoError(t, store.Load())

	// A normal standalone run, plus a saga with two fits.
	require.NoError(t, store.AddRun(&runlog.RunRecord{ModelFile: "acop.mod", Command: "nonmem acop.mod", Status: "completed"}))

	parent := &runlog.RunRecord{ModelFile: "acop.mod", Command: "bootstrap acop.mod -samples=2", Status: "completed"}
	require.NoError(t, store.AddSaga(parent, []*runlog.RunRecord{
		{ModelFile: "acop.mod", Command: "nonmem bs_pr1_1.mod bs_pr1_1.lst", Status: "completed"},
		{ModelFile: "acop.mod", Command: "nonmem bs_pr1_2.mod bs_pr1_2.lst", Status: "completed"},
	}))

	a := &App{runLogStore: store}
	a.refreshCachedRuns()

	// 1 standalone + 1 saga parent; the two child fits are collapsed away.
	require.Len(t, a.cachedRuns, 2)
	for _, run := range a.cachedRuns {
		require.Empty(t, run.ParentID, "child fit leaked into top-level history: %s", run.Command)
	}
}

// TestZipDirectory verifies the saga workspace zip captures every file under the
// directory, prefixed with the workspace name and using forward-slash paths.
func TestZipDirectory(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(src, "m1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "bootstrap_results.csv"), []byte("ci"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "m1", "bs_pr1_1.lst"), []byte("fit1"), 0o600))

	var buf bytes.Buffer
	require.NoError(t, zipDirectory(src, "bs", &buf))

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	names := make(map[string]string, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		var contents bytes.Buffer
		_, err = contents.ReadFrom(rc)
		require.NoError(t, err)
		rc.Close()
		names[f.Name] = contents.String()
	}

	require.Equal(t, "ci", names["bs/bootstrap_results.csv"])
	require.Equal(t, "fit1", names["bs/m1/bs_pr1_1.lst"])
}
