package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBridge is a programmable Bridge used to assert handler translation: what
// arguments a handler passes through, and how it marshals/propagates results.
type fakeBridge struct {
	// captured arguments from the most recent call
	gotModelPath string
	gotLimit     int
	gotOffset    int
	gotID        string
	gotKey       string
	gotExec      ExecuteRequest

	// programmed responses
	runs     []RunSummary
	total    int
	detail   *RunDetail
	files    []string
	file     *FileContent
	execResp *ExecuteResponse
	status   *BridgeStatus
	err      error
}

func (f *fakeBridge) ListRuns(_ context.Context, modelPath string, limit, offset int) ([]RunSummary, int, error) {
	f.gotModelPath, f.gotLimit, f.gotOffset = modelPath, limit, offset

	return f.runs, f.total, f.err
}

func (f *fakeBridge) GetRun(_ context.Context, modelPath, id string) (*RunDetail, error) {
	f.gotModelPath, f.gotID = modelPath, id

	return f.detail, f.err
}

func (f *fakeBridge) ListRunFiles(_ context.Context, modelPath, id string) ([]string, error) {
	f.gotModelPath, f.gotID = modelPath, id

	return f.files, f.err
}

func (f *fakeBridge) GetRunFile(_ context.Context, modelPath, id, key string) (*FileContent, error) {
	f.gotModelPath, f.gotID, f.gotKey = modelPath, id, key

	return f.file, f.err
}

func (f *fakeBridge) ExecuteRun(_ context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	f.gotExec = req

	return f.execResp, f.err
}

func (f *fakeBridge) Status(_ context.Context, modelPath string) (*BridgeStatus, error) {
	f.gotModelPath = modelPath

	return f.status, f.err
}

func TestStatusHandler(t *testing.T) {
	fb := &fakeBridge{status: &BridgeStatus{ModelPath: "/m/a.mod", ModelLoaded: true, RunCount: 3}}
	h := &handlers{bridge: fb}

	_, out, err := h.status(context.Background(), nil, statusInput{ModelPath: "/m/a.mod"})

	require.NoError(t, err)
	assert.Equal(t, "/m/a.mod", fb.gotModelPath)
	assert.Equal(t, 3, out.RunCount)
}

func TestListRunsDefaultsLimit(t *testing.T) {
	fb := &fakeBridge{runs: []RunSummary{{ID: "1"}}, total: 1}
	h := &handlers{bridge: fb}

	_, out, err := h.listRuns(context.Background(), nil, listRunsInput{ModelPath: "/m/a.mod"})

	require.NoError(t, err)
	assert.Equal(t, defaultListLimit, fb.gotLimit, "empty limit should default")
	assert.Equal(t, 0, fb.gotOffset)
	assert.Equal(t, "/m/a.mod", fb.gotModelPath)
	assert.Equal(t, 1, out.Total)
	assert.Len(t, out.Runs, 1)
}

func TestListRunsHonorsExplicitLimit(t *testing.T) {
	fb := &fakeBridge{}
	h := &handlers{bridge: fb}

	_, _, err := h.listRuns(context.Background(), nil, listRunsInput{Limit: 5, Offset: 10})

	require.NoError(t, err)
	assert.Equal(t, 5, fb.gotLimit)
	assert.Equal(t, 10, fb.gotOffset)
}

func TestGetRunPassesID(t *testing.T) {
	fb := &fakeBridge{detail: &RunDetail{RunSummary: RunSummary{ID: "run-7"}}}
	h := &handlers{bridge: fb}

	_, out, err := h.getRun(context.Background(), nil, getRunInput{ModelPath: "/m/a.mod", ID: "run-7"})

	require.NoError(t, err)
	assert.Equal(t, "run-7", fb.gotID)
	assert.Equal(t, "run-7", out.ID)
}

func TestGetRunFileRoutesKey(t *testing.T) {
	fb := &fakeBridge{file: &FileContent{Key: "stdout", Content: "hello", Bytes: 5}}
	h := &handlers{bridge: fb}

	_, out, err := h.getRunFile(context.Background(), nil, getRunFileInput{ID: "run-1", Key: "stdout"})

	require.NoError(t, err)
	assert.Equal(t, "stdout", fb.gotKey)
	assert.Equal(t, "hello", out.Content)
}

func TestExecuteRunPassesRequest(t *testing.T) {
	fb := &fakeBridge{execResp: &ExecuteResponse{RunID: "r1", Status: "running"}}
	h := &handlers{bridge: fb}

	req := ExecuteRequest{ModelPath: "/m/a.mod", IsGrid: true, Cores: 4, DryRun: true}
	_, out, err := h.executeRun(context.Background(), nil, req)

	require.NoError(t, err)
	assert.Equal(t, req, fb.gotExec)
	assert.Equal(t, "running", out.Status)
}

func TestHandlerPropagatesBridgeError(t *testing.T) {
	fb := &fakeBridge{err: errors.New("no model loaded")}
	h := &handlers{bridge: fb}

	_, out, err := h.listRuns(context.Background(), nil, listRunsInput{})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "no model loaded")
}
