package runlog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunLogStore_AddSaga(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "acop.mod")
	require.NoError(t, store.Load())

	parent := &RunRecord{ModelFile: "acop.mod", Command: "bootstrap acop.mod -samples=2", Status: "completed"}
	children := []*RunRecord{
		{ModelFile: "acop.mod", Command: "nonmem bs_pr1_1.mod", Status: "completed"},
		{ModelFile: "acop.mod", Command: "nonmem bs_pr1_2.mod", Status: "completed"},
	}

	require.NoError(t, store.AddSaga(parent, children))

	// Parent marked as the saga; not itself a child.
	require.NotEmpty(t, parent.ID)
	require.Equal(t, KindSaga, parent.Kind)
	require.Empty(t, parent.ParentID)

	// Children marked as fits, linked to the parent.
	for _, c := range children {
		require.NotEmpty(t, c.ID)
		require.Equal(t, KindFit, c.Kind)
		require.Equal(t, parent.ID, c.ParentID)
	}

	got, err := store.ChildrenOf(parent.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestRunLogStore_SagaPersistsLinkage(t *testing.T) {
	baseDir := t.TempDir()
	store := NewRunLogStore(baseDir, "acop.mod")
	require.NoError(t, store.Load())

	parent := &RunRecord{ModelFile: "acop.mod", Status: "completed"}
	require.NoError(t, store.AddSaga(parent, []*RunRecord{
		{ModelFile: "acop.mod", Status: "completed"},
		{ModelFile: "acop.mod", Status: "failed"},
	}))

	// A fresh store rebuilds from disk; the index must carry the linkage.
	fresh := NewRunLogStore(baseDir, "acop.mod")
	require.NoError(t, fresh.Load())

	reloaded, err := fresh.ChildrenOf(parent.ID)
	require.NoError(t, err)
	require.Len(t, reloaded, 2)

	for _, c := range reloaded {
		require.Equal(t, KindFit, c.Kind)
		require.Equal(t, parent.ID, c.ParentID)
	}

	p, err := fresh.GetRun(parent.ID)
	require.NoError(t, err)
	require.Equal(t, KindSaga, p.Kind)
}

func TestRunLogStore_AddSagaNilParent(t *testing.T) {
	store := NewRunLogStore(t.TempDir(), "m.mod")
	require.NoError(t, store.Load())
	require.Error(t, store.AddSaga(nil, nil))
}

func TestRunLogStore_SingleRunHasNoSagaLinkage(t *testing.T) {
	// A normal single run is unaffected: empty Kind, no parent, no children.
	store := NewRunLogStore(t.TempDir(), "m.mod")
	require.NoError(t, store.Load())

	r := &RunRecord{ModelFile: "m.mod", Status: "completed"}
	require.NoError(t, store.AddRun(r))

	require.Empty(t, r.Kind)
	require.Empty(t, r.ParentID)

	children, err := store.ChildrenOf(r.ID)
	require.NoError(t, err)
	require.Empty(t, children)
}
