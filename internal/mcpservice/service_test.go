package mcpservice

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/mcp"
	"github.com/pharmalytica/janus/internal/runlog"
)

// newServiceFixture builds a Service backed by a real temp-dir store (via the
// ephemeral resolver) with one completed run that has stdout and an embedded
// .lst file. It returns the service, the model path, and the run id.
func newServiceFixture(t *testing.T) (*Service, string, string) {
	t.Helper()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB test\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "model.lst"), []byte("NONMEM OUTPUT\nOFV: 123.45\n"), 0600))

	store := runlog.NewRunLogStore(dir, "model")
	require.NoError(t, store.Load())

	record := &runlog.RunRecord{ModelFile: modelPath, Command: "nmfe75 model.mod", ExitCode: 0, Status: "completed"}
	require.NoError(t, record.SetStdout("hello from nonmem"))
	require.NoError(t, runlog.EmbedOutputFiles(record, modelPath, []string{"*.lst"}))
	require.NoError(t, store.AddRun(record))

	resolver := NewEphemeralResolver(nil, "")

	svc, err := New(Options{
		Config:  &config.Config{Input: config.Input{MCP: config.MCPConfig{AllowExecute: false}}},
		Resolve: resolver.Resolve,
		AppCtx:  context.Background(),
	})
	require.NoError(t, err)

	return svc, modelPath, record.ID
}

func TestServiceStatus(t *testing.T) {
	svc, modelPath, _ := newServiceFixture(t)

	status, err := svc.Status(context.Background(), modelPath)
	require.NoError(t, err)
	assert.True(t, status.ModelLoaded)
	assert.Equal(t, 1, status.RunCount)
	assert.False(t, status.AllowExecute)
}

func TestServiceStatusNoModelPath(t *testing.T) {
	svc, _, _ := newServiceFixture(t)

	// The daemon resolver has no default model, so an empty path reports not-loaded.
	status, err := svc.Status(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, status.ModelLoaded)
}

func TestServiceListAndGetRun(t *testing.T) {
	svc, modelPath, runID := newServiceFixture(t)

	runs, total, err := svc.ListRuns(context.Background(), modelPath, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, runs, 1)
	assert.Equal(t, runID, runs[0].ID)

	detail, err := svc.GetRun(context.Background(), modelPath, runID)
	require.NoError(t, err)
	assert.Contains(t, detail.Files, "stdout")
	assert.Contains(t, detail.Files, "model.lst")
}

func TestServiceGetRunFile(t *testing.T) {
	svc, modelPath, runID := newServiceFixture(t)

	stdout, err := svc.GetRunFile(context.Background(), modelPath, runID, "stdout")
	require.NoError(t, err)
	assert.Equal(t, "hello from nonmem", stdout.Content)

	lst, err := svc.GetRunFile(context.Background(), modelPath, runID, "model.lst")
	require.NoError(t, err)
	assert.Contains(t, lst.Content, "OFV: 123.45")
}

func TestServiceExecuteRequiresResolver(t *testing.T) {
	_, err := New(Options{Config: &config.Config{}, AppCtx: context.Background()})
	require.Error(t, err)
}

func TestServiceExecuteDryRunNoModel(t *testing.T) {
	svc, _, _ := newServiceFixture(t)

	// Empty model path with the daemon resolver -> error (no current model).
	_, _, err := svc.Execute(mcp.ExecuteRequest{DryRun: true})
	require.Error(t, err)
}
