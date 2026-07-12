package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/remote"
	"github.com/pharmalytica/janus/internal/scheduler"
)

// fakeGridClient drives the GridExecutor lifecycle without a live scheduler.
type fakeGridClient struct {
	submitID  string
	states    []string // returned in order; a status error is returned once exhausted
	idx       int
	cancelled bool
}

func (f *fakeGridClient) Submit(_ context.Context, _ string, _ scheduler.JobSpec) (string, error) {
	return f.submitID, nil
}

func (f *fakeGridClient) Status(_ context.Context, _ string) (string, error) {
	if f.idx < len(f.states) {
		s := f.states[f.idx]
		f.idx++

		return s, nil
	}

	return scheduler.StateUnknown, fmt.Errorf("job not found")
}

func (f *fakeGridClient) Cancel(_ context.Context, _ string) error {
	f.cancelled = true

	return nil
}

// newTestGridExecutor builds a GridExecutor with an injected fake client and a
// fast poll interval, plus a model file and (optionally) its .lst output.
func newTestGridExecutor(t *testing.T, client gridClient, lstContent string) (*GridExecutor, string) {
	t.Helper()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "run1.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROBLEM test"), 0o600))

	if lstContent != "" {
		lstPath := filepath.Join(dir, "run1.lst")
		require.NoError(t, os.WriteFile(lstPath, []byte(lstContent), 0o600))
	}

	profile, err := scheduler.Resolve("SGE", nil)
	require.NoError(t, err)

	e := &GridExecutor{
		config:       &config.Config{Input: config.Input{Scheduler: "SGE", NonmemPath: "/opt/nm", NonmemBinary: "nmfe75"}},
		profile:      profile,
		client:       client,
		pollInterval: time.Millisecond,
	}

	return e, modelPath
}

func TestGridExecutorCompletes(t *testing.T) {
	client := &fakeGridClient{submitID: "111", states: []string{scheduler.StateRunning, scheduler.StateCompleted}}
	e, modelPath := newTestGridExecutor(t, client, "NONMEM output here")

	result, err := e.Execute(context.Background(), modelPath, false, 0, true, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "NONMEM output here", string(result.Stdout))
}

func TestGridExecutorFailedJobYieldsNonZeroExit(t *testing.T) {
	client := &fakeGridClient{submitID: "111", states: []string{scheduler.StateRunning, scheduler.StateFailed}}
	e, modelPath := newTestGridExecutor(t, client, "")

	result, err := e.Execute(context.Background(), modelPath, false, 0, true, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ExitCode)
}

// A job that disappears from the queue after being seen running is treated as
// completed (SGE/Torque drop finished jobs from qstat).
func TestGridExecutorJobDisappearsAfterRunning(t *testing.T) {
	client := &fakeGridClient{submitID: "111", states: []string{scheduler.StateRunning}}
	e, modelPath := newTestGridExecutor(t, client, "done")

	result, err := e.Execute(context.Background(), modelPath, false, 0, true, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
}

// A job that never appears in the queue surfaces an error rather than looping.
func TestGridExecutorJobNeverAppears(t *testing.T) {
	client := &fakeGridClient{submitID: "111", states: nil}
	e, modelPath := newTestGridExecutor(t, client, "")

	_, err := e.Execute(context.Background(), modelPath, false, 0, true, nil)
	assert.Error(t, err)
}

func TestGridExecutorContextCancelCancelsJob(t *testing.T) {
	// Never reaches a terminal state, so cancellation must break the wait.
	client := &fakeGridClient{submitID: "111", states: manyStates(scheduler.StateRunning, 1000)}
	e, modelPath := newTestGridExecutor(t, client, "")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := e.Execute(ctx, modelPath, false, 0, true, nil)
	require.Error(t, err)
	assert.True(t, client.cancelled, "scheduler job should be cancelled on context cancellation")
}

func TestGridExecutorBuildJobScript(t *testing.T) {
	e, modelPath := newTestGridExecutor(t, &fakeGridClient{}, "")

	script := e.buildJobScript(modelPath, false, 0, []string{"-maxeval=9999"})

	assert.Contains(t, script, "#!/bin/bash")
	assert.Contains(t, script, filepath.Join("/opt/nm", "nmfe75"))
	assert.Contains(t, script, "run1.mod")
	assert.Contains(t, script, "run1.lst")
	assert.Contains(t, script, "-maxeval=9999")
}

func TestGridExecutorRemoteJobScriptUsesRemotePaths(t *testing.T) {
	profile, err := scheduler.Resolve("SGE", nil)
	require.NoError(t, err)

	e := &GridExecutor{
		config: &config.Config{Input: config.Input{
			Scheduler:    "SGE",
			NonmemPath:   "/opt/nm75/run",
			NonmemBinary: "nmfe75",
		}},
		profile:      profile,
		mapper:       remote.NewPathMapper([]config.RemoteMount{{Local: `Z:\proj`, Remote: "/home/jane/proj"}}),
		pollInterval: time.Millisecond,
	}

	script := e.buildJobScript(`Z:\proj\run1\model.mod`, false, 0, nil)

	// Remote (forward-slash) paths, not the local Windows path.
	assert.Contains(t, script, "cd /home/jane/proj/run1 &&")
	assert.Contains(t, script, "/opt/nm75/run/nmfe75")
	assert.Contains(t, script, "/home/jane/proj/run1/model.mod")
	assert.Contains(t, script, "/home/jane/proj/run1/model.lst")
	assert.NotContains(t, script, `Z:\`)

	spec := e.buildJobSpec(`Z:\proj\run1\model.mod`, false, 0)
	assert.Equal(t, "/home/jane/proj/run1", spec.WorkDir)
}

func TestIsNonSLURMGrid(t *testing.T) {
	assert.True(t, isNonSLURMGrid("SGE"))
	assert.True(t, isNonSLURMGrid("TORQUE"))
	assert.True(t, isNonSLURMGrid("PBS"))
	assert.False(t, isNonSLURMGrid("SLURM"))
	assert.False(t, isNonSLURMGrid("LOCAL"))
	assert.False(t, isNonSLURMGrid(""))
}

func manyStates(state string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = state
	}

	return out
}
