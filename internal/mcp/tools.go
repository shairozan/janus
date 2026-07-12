package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// defaultListLimit is the page size used by list_runs when none is supplied.
const defaultListLimit = 50

// handlers binds the Bridge to the typed MCP tool handlers.
type handlers struct {
	bridge Bridge
}

// newMCPServer builds an MCP server with the read tools always registered and
// execute_run registered only when cfg.AllowExecute is set.
func newMCPServer(cfg Config, bridge Bridge) *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "janus",
		Version: cfg.Version,
	}, nil)

	h := &handlers{bridge: bridge}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_run_log_status",
		Description: "Report which model is targeted and whether its run log is available. Call this first to discover the target model.",
	}, h.status)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_runs",
		Description: "List runs from a model's run log, newest first, with the total count.",
	}, h.listRuns)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_run",
		Description: "Get full metadata for a single run by id, including the available output file keys.",
	}, h.getRun)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_run_files",
		Description: "List the output file keys available for a run (embedded extensions plus the reserved keys 'stdout' and 'stderr').",
	}, h.listRunFiles)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_run_file",
		Description: "Get the full content of one output file by key. Large content is capped and marked truncated.",
	}, h.getRunFile)

	if cfg.AllowExecute {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "execute_run",
			Description: "Launch a model run headlessly and return immediately with a run id. Poll get_run until status is completed or failed. Set dry_run to return the built command without running it.",
		}, h.executeRun)
	}

	return srv
}

// --- tool input types ---

type statusInput struct {
	ModelPath string `json:"model_path,omitempty" jsonschema:"absolute path to the target model; defaults to the currently-loaded model"`
}

type listRunsInput struct {
	ModelPath string `json:"model_path,omitempty" jsonschema:"absolute path to the target model; defaults to the currently-loaded model"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum runs to return (default 50)"`
	Offset    int    `json:"offset,omitempty" jsonschema:"number of runs to skip from the newest"`
}

type getRunInput struct {
	ModelPath string `json:"model_path,omitempty" jsonschema:"absolute path to the target model; defaults to the currently-loaded model"`
	ID        string `json:"id" jsonschema:"run id"`
}

type listRunFilesInput struct {
	ModelPath string `json:"model_path,omitempty" jsonschema:"absolute path to the target model; defaults to the currently-loaded model"`
	ID        string `json:"id" jsonschema:"run id"`
}

type getRunFileInput struct {
	ModelPath string `json:"model_path,omitempty" jsonschema:"absolute path to the target model; defaults to the currently-loaded model"`
	ID        string `json:"id" jsonschema:"run id"`
	Key       string `json:"key" jsonschema:"file key: an extension from list_run_files, or 'stdout'/'stderr'"`
}

// --- tool output types ---

type listRunsOutput struct {
	Runs  []RunSummary `json:"runs"`
	Total int          `json:"total"`
}

type listRunFilesOutput struct {
	Files []string `json:"files"`
}

// --- handlers ---

func (h *handlers) status(ctx context.Context, _ *mcpsdk.CallToolRequest, in statusInput) (*mcpsdk.CallToolResult, *BridgeStatus, error) {
	status, err := h.bridge.Status(ctx, in.ModelPath)
	if err != nil {
		return nil, nil, err
	}

	return nil, status, nil
}

func (h *handlers) listRuns(ctx context.Context, _ *mcpsdk.CallToolRequest, in listRunsInput) (*mcpsdk.CallToolResult, *listRunsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	runs, total, err := h.bridge.ListRuns(ctx, in.ModelPath, limit, in.Offset)
	if err != nil {
		return nil, nil, err
	}

	return nil, &listRunsOutput{Runs: runs, Total: total}, nil
}

func (h *handlers) getRun(ctx context.Context, _ *mcpsdk.CallToolRequest, in getRunInput) (*mcpsdk.CallToolResult, *RunDetail, error) {
	detail, err := h.bridge.GetRun(ctx, in.ModelPath, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return nil, detail, nil
}

func (h *handlers) listRunFiles(ctx context.Context, _ *mcpsdk.CallToolRequest, in listRunFilesInput) (*mcpsdk.CallToolResult, *listRunFilesOutput, error) {
	files, err := h.bridge.ListRunFiles(ctx, in.ModelPath, in.ID)
	if err != nil {
		return nil, nil, err
	}

	return nil, &listRunFilesOutput{Files: files}, nil
}

func (h *handlers) getRunFile(ctx context.Context, _ *mcpsdk.CallToolRequest, in getRunFileInput) (*mcpsdk.CallToolResult, *FileContent, error) {
	file, err := h.bridge.GetRunFile(ctx, in.ModelPath, in.ID, in.Key)
	if err != nil {
		return nil, nil, err
	}

	return nil, file, nil
}

func (h *handlers) executeRun(ctx context.Context, _ *mcpsdk.CallToolRequest, in ExecuteRequest) (*mcpsdk.CallToolResult, *ExecuteResponse, error) {
	resp, err := h.bridge.ExecuteRun(ctx, in)
	if err != nil {
		return nil, nil, err
	}

	return nil, resp, nil
}
