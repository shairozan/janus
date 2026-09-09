package mcpservice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
	"github.com/pharmalytica/janus/internal/mcp"
	"github.com/pharmalytica/janus/internal/runlog"
)

// headlessRunTimeout bounds a headless run, mirroring the GUI execution timeout.
const headlessRunTimeout = 30 * time.Minute

// Service implements the transport-neutral Bridge consumed by the MCP server.
var _ mcp.Bridge = (*Service)(nil)

// Service implements mcp.Bridge from the run-log and execution layers without any
// GUI dependency. It is constructed once and shared by the daemon directly and by
// the GUI through a thin wrapper.
type Service struct {
	cfg     *config.Config
	resolve StoreResolver
	appCtx  context.Context //nolint:containedctx // app/daemon-lifetime context for background runs

	// onComplete, if non-nil, is called after a headless run finishes recording
	// its result, with the store that was written. The GUI uses it to refresh the
	// run-history view; the daemon leaves it nil.
	onComplete func(store *runlog.RunLogStore)

	// errSink, if non-nil, receives non-fatal background errors (e.g. an
	// execution failure after the tool call already returned). The GUI routes
	// these to its error channel; the daemon logs them.
	errSink func(error)
}

// Options configures a Service.
type Options struct {
	Config     *config.Config
	Resolve    StoreResolver
	AppCtx     context.Context //nolint:containedctx // app/daemon-lifetime context handed to background runs
	OnComplete func(store *runlog.RunLogStore)
	ErrSink    func(error)
}

// New constructs a Service. Config, Resolve and AppCtx are required.
func New(opts Options) (*Service, error) {
	if opts.Config == nil {
		return nil, errors.New("mcpservice: config is required")
	}

	if opts.Resolve == nil {
		return nil, errors.New("mcpservice: store resolver is required")
	}

	if opts.AppCtx == nil {
		return nil, errors.New("mcpservice: app context is required")
	}

	return &Service{
		cfg:        opts.Config,
		resolve:    opts.Resolve,
		appCtx:     opts.AppCtx,
		onComplete: opts.OnComplete,
		errSink:    opts.ErrSink,
	}, nil
}

func (s *Service) allowExecute() bool {
	return s.cfg.MCP.AllowExecute
}

func (s *Service) reportErr(err error) {
	if s.errSink != nil {
		s.errSink(err)
	}
}

// --- mcp.Bridge: reads ---

func (s *Service) Status(_ context.Context, modelPath string) (*mcp.BridgeStatus, error) {
	store, resolved, err := s.resolve(modelPath)
	if err != nil {
		return &mcp.BridgeStatus{ModelLoaded: false, AllowExecute: s.allowExecute()}, nil
	}

	return &mcp.BridgeStatus{
		ModelPath:       resolved,
		ModelLoaded:     true,
		RunLogAvailable: true,
		RunCount:        store.Count(),
		AllowExecute:    s.allowExecute(),
	}, nil
}

func (s *Service) ListRuns(_ context.Context, modelPath string, limit, offset int) ([]mcp.RunSummary, int, error) {
	store, _, err := s.resolve(modelPath)
	if err != nil {
		return nil, 0, err
	}

	records, err := store.GetRuns(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list runs: %w", err)
	}

	summaries := make([]mcp.RunSummary, 0, len(records))
	for _, record := range records {
		summaries = append(summaries, toRunSummary(record))
	}

	return summaries, store.Count(), nil
}

func (s *Service) GetRun(_ context.Context, modelPath, id string) (*mcp.RunDetail, error) {
	store, _, err := s.resolve(modelPath)
	if err != nil {
		return nil, err
	}

	record, err := store.GetRun(id)
	if err != nil {
		return nil, fmt.Errorf("run %q not found: %w", id, err)
	}

	detail := toRunDetail(record)

	return &detail, nil
}

func (s *Service) ListRunFiles(_ context.Context, modelPath, id string) ([]string, error) {
	store, _, err := s.resolve(modelPath)
	if err != nil {
		return nil, err
	}

	record, err := store.GetRun(id)
	if err != nil {
		return nil, fmt.Errorf("run %q not found: %w", id, err)
	}

	return runFileKeys(record), nil
}

func (s *Service) GetRunFile(_ context.Context, modelPath, id, key string) (*mcp.FileContent, error) {
	store, _, err := s.resolve(modelPath)
	if err != nil {
		return nil, err
	}

	record, err := store.GetRun(id)
	if err != nil {
		return nil, fmt.Errorf("run %q not found: %w", id, err)
	}

	raw, err := runFileBytes(record, key)
	if err != nil {
		return nil, err
	}

	return encodeFileContent(key, raw), nil
}

// --- mcp.Bridge: execute ---

func (s *Service) ExecuteRun(_ context.Context, req mcp.ExecuteRequest) (*mcp.ExecuteResponse, error) {
	//nolint:contextcheck // the headless run uses the service's app-lifetime context, not the request context
	runID, command, err := s.Execute(req)
	if err != nil {
		return nil, err
	}

	if req.DryRun {
		return &mcp.ExecuteResponse{Status: "dry_run", Command: command}, nil
	}

	return &mcp.ExecuteResponse{RunID: runID, Status: "running", Command: command}, nil
}

// Execute launches a model run without any UI interaction and returns
// immediately. It resolves and validates the request, builds the executor,
// records a "running" run in the target model's store, and runs the executor in
// a background goroutine that records the result on completion. When req.DryRun
// is set it returns the built command without running anything.
//
// Grid execution is intentionally dry-run only: a real grid submission needs the
// interactive scheduler settings the GUI grid modal collects, which an MCP
// request cannot supply.
func (s *Service) Execute(req mcp.ExecuteRequest) (runID, command string, err error) {
	store, modelPath, err := s.resolve(req.ModelPath)
	if err != nil {
		return "", "", err
	}

	isHermes := s.cfg.ExecutionMode == config.ExecutionModeHERMES

	if req.IsGrid && isHermes {
		return "", "", errors.New("hermes execution mode does not support grid execution")
	}

	if config.RequiresNONMEMLicense(s.cfg.ExecutionMode) {
		if _, licErr := config.ValidateNONMEMLicenseForExecution(s.cfg); licErr != nil {
			return "", "", fmt.Errorf("cannot execute model: %w", licErr)
		}
	}

	factory := execution.NewExecutorFactory(s.cfg)
	if concrete, ok := factory.(*execution.DefaultExecutorFactory); ok {
		concrete.SetRunLogEnabled(true)
	}

	var executor execution.Executor
	if isHermes {
		// CreateHermesExecutor errors when .janus.config.json is missing; the
		// headless path never prompts for it.
		executor, err = factory.CreateHermesExecutor(modelPath)
	} else {
		executor, err = factory.CreateExecutor(s.cfg.ExecutionMode)
	}

	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var additionalOptions []string
	if req.NonmemOptions != "" {
		additionalOptions = parseNonmemOptions(req.NonmemOptions)
	}

	command = headlessCommand(executor, modelPath, req, additionalOptions)

	if req.DryRun {
		return "", command, nil
	}

	if req.IsGrid {
		return "", "", errors.New("grid execution is not supported headlessly; use dry_run to inspect the command")
	}

	if req.IsParallel {
		if pnmErr := WritePnmFile(modelPath, req.Cores); pnmErr != nil {
			return "", "", fmt.Errorf("failed to generate .pnm file: %w", pnmErr)
		}
	}

	record := &runlog.RunRecord{
		ModelFile:  modelPath,
		Command:    command,
		ExitCode:   -1,
		IsParallel: req.IsParallel,
		Cores:      req.Cores,
		IsGrid:     req.IsGrid,
		Status:     "running",
	}

	if req.Description != "" {
		if descErr := record.SetDescription(req.Description); descErr != nil {
			record.Description = req.Description // Fallback to uncompressed
		}
	}

	if req.NonmemOptions != "" && s.cfg.ExecutionMode == config.ExecutionModeNONMEM {
		opts := req.NonmemOptions
		record.NonmemOptions = &opts
	}

	if addErr := store.AddRun(record); addErr != nil {
		return "", "", fmt.Errorf("failed to save run record: %w", addErr)
	}

	runID = record.ID

	go s.runHeadless(store, executor, modelPath, runID, req, additionalOptions)

	return runID, command, nil
}

// runHeadless executes the run and records its result on a background goroutine.
func (s *Service) runHeadless(store *runlog.RunLogStore, executor execution.Executor, modelPath, runID string, req mcp.ExecuteRequest, additionalOptions []string) {
	ctx, cancel := context.WithTimeout(s.appCtx, headlessRunTimeout)
	defer cancel()

	// Resolve the model's output files of interest (per-model → global → NONMEM
	// default) so the run log embeds exactly those files.
	retain := config.ResolveRetain(config.LoadModelRetain(modelPath), s.cfg.Hermes.Retain, config.DefaultNONMEMRetain())

	result, err := executor.Execute(ctx, modelPath, req.IsParallel, req.Cores, false, additionalOptions)
	if err != nil {
		if applyErr := ApplyRunResult(ctx, store, runID, -1, "", fmt.Sprintf("Execution failed: %s", err.Error()), nil, retain); applyErr != nil {
			s.reportErr(applyErr)
		}

		s.reportErr(fmt.Errorf("headless execution failed: %w", err))
		s.fireOnComplete(store)

		return
	}

	if applyErr := ApplyRunResult(ctx, store, runID, result.ExitCode, string(result.Stdout), string(result.Stderr), result.Container, retain); applyErr != nil {
		s.reportErr(applyErr)
	}

	s.fireOnComplete(store)
}

func (s *Service) fireOnComplete(store *runlog.RunLogStore) {
	if s.onComplete != nil {
		s.onComplete(store)
	}
}
