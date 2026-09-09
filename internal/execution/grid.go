package execution

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/remote"
	"github.com/shairozan/janus/internal/runlog"
	"github.com/shairozan/janus/internal/scheduler"
)

// gridPollInterval is how often the grid executor polls a job's status.
const gridPollInterval = 5 * time.Second

// isNonSLURMGrid reports whether a scheduler name designates a grid that the
// generic GridExecutor handles — i.e. a real scheduler other than SLURM (which
// has its own executor) and other than LOCAL/empty (local execution).
func isNonSLURMGrid(name string) bool {
	switch name {
	case "", "LOCAL", "SLURM":
		return false
	default:
		return true
	}
}

// maxStatusErrors bounds consecutive status-lookup failures before a job is
// declared lost (only applies before the job has been observed in the queue).
const maxStatusErrors = 3

// gridClient is the subset of scheduler.CLIClient the GridExecutor needs. It is
// an interface so the submit→poll→collect lifecycle can be unit-tested with a
// fake client — no live scheduler required, per the IQ/OQ strategy.
type gridClient interface {
	Submit(ctx context.Context, script string, spec scheduler.JobSpec) (string, error)
	Status(ctx context.Context, jobID string) (string, error)
	Cancel(ctx context.Context, jobID string) error
}

// GridExecutor submits NONMEM jobs to a configurable workload manager (SGE,
// Torque, …) resolved from a scheduler.Profile. SLURM keeps its dedicated
// executor (REST + streaming); this generalizes the path that previously fell
// back to local execution for every other scheduler.
type GridExecutor struct {
	config       *config.Config
	profile      scheduler.Profile
	client       gridClient
	mapper       *remote.PathMapper // non-nil for remote (SSH) execution
	runLogger    *runlog.RunLogger
	pollInterval time.Duration
}

// NewGridExecutor resolves the configured scheduler to a profile and returns a
// grid executor. When a remote host is configured it runs commands over SSH and
// translates paths via the configured mounts; otherwise it runs locally.
func NewGridExecutor(cfg *config.Config, runLogger *runlog.RunLogger) (*GridExecutor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	profile, err := scheduler.Resolve(cfg.Scheduler, cfg.Schedulers)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve scheduler %q: %w", cfg.Scheduler, err)
	}

	e := &GridExecutor{
		config:       cfg,
		profile:      profile,
		runLogger:    runLogger,
		pollInterval: gridPollInterval,
	}

	if cfg.Remote.Host != "" {
		sshClient, err := remote.NewSSHClient(profile, cfg.Remote)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH client: %w", err)
		}

		e.client = sshClient
		e.mapper = remote.NewPathMapper(cfg.Remote.Mounts)
	} else {
		e.client = scheduler.NewCLIClient(profile, 0)
	}

	return e, nil
}

// Execute submits the model to the grid, waits for completion, and returns the
// result. The model's .lst output is returned as stdout; the exit code is
// derived from the final job state.
func (e *GridExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, _ bool, additionalOptions []string) (*ExecutionResult, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not found: %s: %w", modelPath, err)
	}

	startTime := time.Now()

	// Apply the pre-run output policy (sequential archiving). No-op by default.
	applyBeforeRun(e.config, modelPath)
	applyPreHooks(ctx, e.config, modelPath)

	script := e.buildJobScript(modelPath, isParallel, cores, additionalOptions)
	spec := e.buildJobSpec(modelPath, isParallel, cores)

	jobID, err := e.client.Submit(ctx, script, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job to %s: %w", e.profile.Name, err)
	}

	log.Printf("Submitted NONMEM job to %s with ID: %s", e.profile.Name, jobID)

	state, err := e.waitForCompletion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	result := e.collectResult(modelPath, state)

	e.recordRunLog(modelPath, jobID, result, time.Since(startTime))

	// Run post-run integration hooks (e.g. R diagnostics), then apply the
	// post-run output policy (backup, cleanup). Both are no-ops by default.
	applyPostHooks(ctx, e.config, modelPath)
	applyAfterRun(e.config, modelPath)

	return result, nil
}

// recordRunLog records a grid execution to the run log (when enabled),
// mirroring the SLURM executor's run-log capture, with the scheduler job ID.
func (e *GridExecutor) recordRunLog(modelPath, schedulerJobID string, result *ExecutionResult, duration time.Duration) {
	if e.runLogger == nil || !e.runLogger.IsEnabled() {
		return
	}

	binary := filepath.Join(e.config.NonmemPath, e.config.NonmemBinary)
	lstPath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"

	err := e.runLogger.RecordSLURMExecution(
		e.runLogger.GenerateJobID(),
		binary,
		[]string{modelPath},
		string(result.Stdout),
		string(result.Stderr),
		result.ExitCode,
		duration,
		filepath.Dir(modelPath),
		schedulerJobID,
		[]string{lstPath},
	)
	if err != nil {
		log.Printf("Warning: failed to record grid run log for %s: %v", modelPath, err)
	}
}

// Close releases resources (no-op for the CLI client).
func (e *GridExecutor) Close() error {
	return nil
}

// waitForCompletion polls the job until it reaches a terminal state. A status
// error after the job has been seen in the queue is treated as completion
// (SGE/Torque drop finished jobs from `qstat`); repeated errors before the job
// is ever observed are surfaced. Context cancellation cancels the job.
func (e *GridExecutor) waitForCompletion(ctx context.Context, jobID string) (string, error) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	seen := false
	errCount := 0

	for {
		select {
		case <-ctx.Done():
			// Best-effort cancel on the scheduler before returning. The parent
			// ctx is already cancelled, so derive a detached, time-bounded
			// context (inherits values, ignores the cancellation) for cleanup.
			cancelCtx, cancelDone := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			_ = e.client.Cancel(cancelCtx, jobID)
			cancelDone()

			return "", fmt.Errorf("context cancelled while waiting for job %s", jobID)

		case <-ticker.C:
			state, err := e.client.Status(ctx, jobID)
			if err != nil {
				if seen {
					// The job left the queue after we saw it running: done.
					return scheduler.StateCompleted, nil
				}

				errCount++
				if errCount >= maxStatusErrors {
					return "", fmt.Errorf("job %s never appeared in %s: %w", jobID, e.profile.Name, err)
				}

				continue
			}

			seen = true
			errCount = 0

			switch state {
			case scheduler.StateCompleted, scheduler.StateFailed, scheduler.StateCancelled:
				return state, nil
			}
		}
	}
}

// collectResult reads the model's .lst output and maps the final state to an
// exit code.
func (e *GridExecutor) collectResult(modelPath, state string) *ExecutionResult {
	lstPath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"

	var stdout []byte
	if data, err := os.ReadFile(lstPath); err == nil {
		stdout = data
	}

	exitCode := 0
	if state == scheduler.StateFailed || state == scheduler.StateCancelled {
		exitCode = 1
	}

	return &ExecutionResult{ExitCode: exitCode, Stdout: stdout}
}

// buildJobScript builds the bash job script that runs NONMEM. For remote
// execution every path is translated to its remote equivalent and joined with
// forward slashes; otherwise local (OS-native) paths are used.
func (e *GridExecutor) buildJobScript(modelPath string, isParallel bool, cores int, additionalOptions []string) string {
	binary := e.config.NonmemBinary
	if binary == "" {
		binary = "nmfe75"
	}

	if e.mapper != nil {
		return e.buildRemoteJobScript(modelPath, binary, isParallel, cores, additionalOptions)
	}

	fullPath := filepath.Join(e.config.NonmemPath, binary)
	outputFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"

	args := []string{modelPath, outputFile}
	if isParallel && cores > 1 {
		pnmFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".pnm"
		args = append(args, "-parallel", pnmFile)
	}

	args = append(args, additionalOptions...)

	nonmemCommand := fmt.Sprintf("%s %s", fullPath, strings.Join(args, " "))

	return fmt.Sprintf("#!/bin/bash\ncd %s && %s", filepath.Dir(modelPath), nonmemCommand)
}

// buildRemoteJobScript builds the job script using remote (forward-slash) paths.
// The remote directory is derived from the remote model path (path.Dir), not
// filepath.Dir on the local path, so it is correct regardless of the host OS
// (e.g. a Windows local path mapped to a Linux remote).
func (e *GridExecutor) buildRemoteJobScript(modelPath, binary string, isParallel bool, cores int, additionalOptions []string) string {
	remoteModel := e.mapper.ToRemote(modelPath)
	remoteDir := path.Dir(remoteModel)
	fullPath := path.Join(e.config.NonmemPath, binary)
	outputFile := strings.TrimSuffix(remoteModel, path.Ext(remoteModel)) + ".lst"

	args := []string{remoteModel, outputFile}
	if isParallel && cores > 1 {
		pnmFile := strings.TrimSuffix(remoteModel, path.Ext(remoteModel)) + ".pnm"
		args = append(args, "-parallel", pnmFile)
	}

	args = append(args, additionalOptions...)

	nonmemCommand := fmt.Sprintf("%s %s", fullPath, strings.Join(args, " "))

	return fmt.Sprintf("#!/bin/bash\ncd %s && %s", remoteDir, nonmemCommand)
}

// buildJobSpec maps the model and resources onto the profile's job spec, using
// remote paths for the working directory and output files when remote.
func (e *GridExecutor) buildJobSpec(modelPath string, isParallel bool, cores int) scheduler.JobSpec {
	project := filepath.Base(filepath.Dir(modelPath))
	jobName := e.config.RunPrefix + scheduler.RenderJobName(e.profile, filepath.Base(modelPath), project)

	workDir := filepath.Dir(modelPath)
	output := modelPath + ".out"
	errFile := modelPath + ".err"

	if e.mapper != nil {
		remoteModel := e.mapper.ToRemote(modelPath)
		workDir = path.Dir(remoteModel)
		output = remoteModel + ".out"
		errFile = remoteModel + ".err"
	}

	spec := scheduler.JobSpec{
		JobName: jobName,
		WorkDir: workDir,
		Output:  output,
		Error:   errFile,
	}

	if isParallel && cores > 1 {
		spec.CPUs = cores
	}

	return spec
}
