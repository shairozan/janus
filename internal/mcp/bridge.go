// Package mcp exposes a Model Context Protocol server from the running Janus
// process so local agents (e.g. Claude Code) can query the run log and launch
// runs over loopback HTTP.
//
// To respect Janus's orthogonal architecture, this package is a lower layer: it
// must NOT import internal/gui, internal/runlog, or internal/execution. It
// defines a narrow Bridge interface plus plain data-transfer objects (DTOs); the
// GUI App (the highest layer) implements Bridge and hands it in at construction,
// so the dependency points gui -> mcp, never the reverse.
package mcp

import (
	"context"
	"time"
)

// Bridge is the single seam between the MCP transport layer and Janus's internal
// state. It is path-based: every read/execute call carries the target model path
// (empty means "use the currently-loaded model").
type Bridge interface {
	// ListRuns returns run summaries for the model, newest first, plus the total count.
	ListRuns(ctx context.Context, modelPath string, limit, offset int) ([]RunSummary, int, error)

	// GetRun returns full metadata for a single run.
	GetRun(ctx context.Context, modelPath, id string) (*RunDetail, error)

	// ListRunFiles lists the output file keys available for a run (embedded
	// extensions plus the reserved keys "stdout" and "stderr").
	ListRunFiles(ctx context.Context, modelPath, id string) ([]string, error)

	// GetRunFile returns the full content of one output file by key.
	GetRunFile(ctx context.Context, modelPath, id, key string) (*FileContent, error)

	// ExecuteRun launches a model run headlessly and returns immediately.
	ExecuteRun(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)

	// Status describes which model is targeted and run-log availability.
	Status(ctx context.Context, modelPath string) (*BridgeStatus, error)
}

// RunSummary is a compact view of a run, used by ListRuns.
type RunSummary struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	ModelFile  string    `json:"model_file"`
	Status     string    `json:"status"` // "running", "completed", "failed"
	ExitCode   int       `json:"exit_code"`
	IsGrid     bool      `json:"is_grid"`
	IsParallel bool      `json:"is_parallel"`
	Cores      int       `json:"cores,omitempty"`
	Command    string    `json:"command"`
}

// RunDetail is the full metadata for a single run, used by GetRun.
type RunDetail struct {
	RunSummary

	NonmemOptions string   `json:"nonmem_options,omitempty"`
	Description   string   `json:"description,omitempty"`
	SignerEmail   string   `json:"signer_email,omitempty"`
	Signed        bool     `json:"signed"`
	Files         []string `json:"files"` // available file keys (see ListRunFiles)
}

// FileContent is the content of one run output file, used by GetRunFile.
type FileContent struct {
	Key       string `json:"key"`     // "lst", "ext", "mod", ..., or "stdout"/"stderr"
	Content   string `json:"content"` // text, or base64 if non-UTF-8
	Bytes     int    `json:"bytes"`
	Base64    bool   `json:"base64"`
	Truncated bool   `json:"truncated"` // true if the content was capped
}

// ExecuteRequest is the input to ExecuteRun.
type ExecuteRequest struct {
	ModelPath     string `json:"model_path,omitempty"`
	IsGrid        bool   `json:"is_grid,omitempty"`
	IsParallel    bool   `json:"is_parallel,omitempty"`
	Cores         int    `json:"cores,omitempty"`
	NonmemOptions string `json:"nonmem_options,omitempty"`
	Description   string `json:"description,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"` // return the built command without running
}

// ExecuteResponse is the result of ExecuteRun.
type ExecuteResponse struct {
	RunID   string `json:"run_id"`
	Status  string `json:"status"`            // "running"
	Command string `json:"command,omitempty"` // populated for dry runs
}

// BridgeStatus describes the targeted model and run-log availability, used by Status.
type BridgeStatus struct {
	ModelPath       string `json:"model_path"`        // resolved target model path ("" if none)
	ModelLoaded     bool   `json:"model_loaded"`      // whether a model is loaded/targetable
	RunLogAvailable bool   `json:"run_log_available"` // whether the run log opened successfully
	RunCount        int    `json:"run_count"`
	AllowExecute    bool   `json:"allow_execute"` // whether execute_run is registered
}
