package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/mcpservice"
	"github.com/pharmalytica/janus/internal/runlog"
)

// The bridge/DTO/execute logic now lives in internal/mcpservice and is tested
// there. These tests cover the GUI's only contribution: the resolver that
// prefers the live store for the loaded model and delegates everything else to
// the shared ephemeral resolver.

func newResolverApp(t *testing.T) (*App, string, *runlog.RunLogStore) {
	t.Helper()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB test\n"), 0600))

	store := runlog.NewRunLogStore(dir, "model")
	require.NoError(t, store.Load())

	app := &App{
		currentFilePath:   modelPath,
		runLogStore:       store,
		ephemeralResolver: mcpservice.NewEphemeralResolver(nil, ""),
	}

	return app, modelPath, store
}

func TestResolveStoreCurrentModelReusesLiveStore(t *testing.T) {
	app, modelPath, live := newResolverApp(t)

	// Empty path defaults to the loaded model and returns the live store.
	store, resolved, err := app.resolveStore("")
	require.NoError(t, err)
	assert.Same(t, live, store)

	abs, _ := filepath.Abs(modelPath)
	assert.Equal(t, abs, resolved)

	// Explicit path matching the loaded model also returns the live store.
	store2, _, err := app.resolveStore(modelPath)
	require.NoError(t, err)
	assert.Same(t, live, store2)
}

func TestResolveStoreOtherModelUsesEphemeral(t *testing.T) {
	app, _, live := newResolverApp(t)

	otherDir := t.TempDir()
	otherModel := filepath.Join(otherDir, "other.mod")
	require.NoError(t, os.WriteFile(otherModel, []byte("$PROB other\n"), 0600))

	store, resolved, err := app.resolveStore(otherModel)
	require.NoError(t, err)
	assert.NotSame(t, live, store, "a different model must not reuse the live store")

	abs, _ := filepath.Abs(otherModel)
	assert.Equal(t, abs, resolved)
}

func TestResolveStoreNoModel(t *testing.T) {
	app := &App{ephemeralResolver: mcpservice.NewEphemeralResolver(nil, "")}

	_, _, err := app.resolveStore("")
	require.Error(t, err)
}
