package execution

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/remote"
	"github.com/shairozan/janus/internal/runlog"
)

// NONMEMExecutor implements direct NONMEM execution.
type NONMEMExecutor struct {
	config    *config.Config
	runLogger *runlog.RunLogger
}

// NewNONMEMExecutor creates a new NONMEM executor without run logging.
//
// Deprecated: Use NewNONMEMExecutorWithRunLog for explicit run log control.
func NewNONMEMExecutor(cfg *config.Config) Executor {
	return NewNONMEMExecutorWithRunLog(cfg, false)
}

// NewNONMEMExecutorWithRunLog creates a new NONMEM executor with optional run logging.
func NewNONMEMExecutorWithRunLog(cfg *config.Config, runLogEnabled bool) Executor {
	executor := &NONMEMExecutor{
		config: cfg,
	}

	// Initialize run logger if enabled
	if runLogEnabled {
		runLogPath := cfg.RunLog.Path
		if runLogPath == "" {
			runLogPath = "runlog.jsonl" // Default run log file
		}

		runLogger, err := runlog.NewRunLogger(true, runLogPath)
		if err != nil {
			// Log error but don't fail - run logging is supplementary
			// In production, we might want to handle this differently
			log.Printf("Warning: Failed to initialize run logger: %v", err)
		} else {
			executor.runLogger = runLogger
		}
	}

	return executor
}

// BuildCommand constructs the binary path and arguments for NONMEM execution.
// This is the public interface for testing and validation.
func (e *NONMEMExecutor) BuildCommand(modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (string, []string, error) {
	return e.buildNONMEMCommand(modelPath, isParallel, cores, isGrid, additionalOptions)
}

// buildNONMEMCommand constructs the binary path and arguments for NONMEM execution.
// This function is separated to make it easily testable.
func (e *NONMEMExecutor) buildNONMEMCommand(modelPath string, isParallel bool, _ int, _ bool, additionalOptions []string) (string, []string, error) {
	// Build command arguments - NONMEM expects: nmfe76 infile outfile [options]
	outputFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"
	args := []string{modelPath, outputFile}

	// Add parallel arguments if needed
	if isParallel {
		// For parallel runs, NONMEM expects a .pnm file to exist alongside the model
		pnmFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".pnm"
		args = append(args, "-parallel", pnmFile)
	}

	// Add any additional options provided
	if len(additionalOptions) > 0 {
		args = append(args, additionalOptions...)
	}

	// Construct full path to NONMEM binary
	nonmemBinary, err := buildNonmemBinaryPath(e.config.NonmemPath, e.config.NonmemBinary)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build NONMEM binary path: %w", err)
	}

	return nonmemBinary, args, nil
}

// Execute runs a NONMEM model directly using the NONMEM binary or submits to grid scheduler.
func (e *NONMEMExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	// Validate that the model file exists
	if _, err := os.Stat(modelPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("model file does not exist: %s", modelPath)
		}

		return nil, fmt.Errorf("failed to access model file: %w", err)
	}

	// Check if this should be submitted to a grid scheduler
	if isGrid && e.config != nil && e.config.Scheduler == "SLURM" {
		// Delegate to SLURMExecutor for proper job monitoring and output collection
		slurmExecutor, err := NewSLURMExecutor(e.config, e.runLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SLURM executor: %w", err)
		}
		defer slurmExecutor.Close()

		return slurmExecutor.Execute(ctx, modelPath, isParallel, cores, isGrid, additionalOptions)
	}

	// Other configured grid schedulers (SGE, Torque, PBS, …) go through the
	// generic, profile-driven grid executor instead of falling back to local.
	if isGrid && e.config != nil && isNonSLURMGrid(e.config.Scheduler) {
		gridExecutor, err := NewGridExecutor(e.config, e.runLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create grid executor: %w", err)
		}
		defer gridExecutor.Close()

		return gridExecutor.Execute(ctx, modelPath, isParallel, cores, isGrid, additionalOptions)
	}

	// Direct remote (SSH) execution: run nmfe on the configured host, no
	// scheduler. Mirrors the remote PsN path. The GUI sets Remote only for the
	// SSH target, so a local ("Here") run never reaches this branch.
	if !isGrid && e.config != nil && e.config.Remote.Host != "" {
		return e.runRemote(ctx, modelPath, isParallel, cores, additionalOptions)
	}

	// Apply the pre-run output policy (e.g. archive prior results when the
	// sequential overwrite policy is active). No-op by default.
	applyBeforeRun(e.config, modelPath)
	applyPreHooks(ctx, e.config, modelPath)

	// Build command using the testable function
	nonmemBinary, args, err := e.buildNONMEMCommand(modelPath, isParallel, cores, isGrid, additionalOptions)
	if err != nil {
		return nil, err
	}

	// Generate job ID for run log if run logging is enabled
	var jobID string
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		jobID = e.runLogger.GenerateJobID()
	}

	// Record start time for duration calculation
	startTime := time.Now()

	// Create the command
	cmd := exec.CommandContext(ctx, nonmemBinary, args...)

	// Set working directory to the model's directory
	workDir := filepath.Dir(modelPath)
	cmd.Dir = workDir

	// Create buffers to capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err = cmd.Run()

	// Calculate execution duration
	duration := time.Since(startTime)

	// Parse exit code
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			// Non-exit error (e.g., command not found)
			return nil, fmt.Errorf("failed to execute NONMEM: %w", err)
		}
	}

	// Log execution to run log if enabled
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		runLogErr := e.runLogger.RecordExecution(
			jobID,
			nonmemBinary,
			args,
			stdout.String(),
			stderr.String(),
			exitCode,
			duration,
			workDir,
		)
		if runLogErr != nil {
			log.Printf("Warning: Failed to log execution to run log: %v", runLogErr)
		}
	}

	// Run post-run integration hooks (e.g. R diagnostics), then apply the
	// post-run output policy (backup, cleanup). Both are no-ops by default.
	applyPostHooks(ctx, e.config, modelPath)
	applyAfterRun(e.config, modelPath)

	// Return raw bytes for both stdout and stderr
	result := &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}

	return result, nil
}

// ExecuteWithJobID runs a NONMEM model with an optional job ID for SLURM job naming.
// This method is used by the GUI to provide better job correlation.
func (e *NONMEMExecutor) ExecuteWithJobID(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string, _ string) (*ExecutionResult, error) {
	// Validate that the model file exists
	if _, err := os.Stat(modelPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("model file does not exist: %s", modelPath)
		}

		return nil, fmt.Errorf("failed to access model file: %w", err)
	}

	// Check if this should be submitted to a grid scheduler
	if isGrid && e.config != nil && e.config.Scheduler == "SLURM" {
		// Delegate to SLURMExecutor for proper job monitoring and output collection
		slurmExecutor, err := NewSLURMExecutor(e.config, e.runLogger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SLURM executor: %w", err)
		}
		defer slurmExecutor.Close()

		return slurmExecutor.Execute(ctx, modelPath, isParallel, cores, isGrid, additionalOptions)
	}

	// For non-grid execution, fall back to standard Execute method
	return e.Execute(ctx, modelPath, isParallel, cores, isGrid, additionalOptions)
}

// ExecuteWithStreaming implements StreamingExecutor interface.
func (e *NONMEMExecutor) ExecuteWithStreaming(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*StreamingOutput, *ExecutionResult, error) {
	// Validate that the model file exists
	if _, err := os.Stat(modelPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("model file does not exist: %s", modelPath)
		}

		return nil, nil, fmt.Errorf("failed to access model file: %w", err)
	}

	// Build command using the testable function
	nonmemBinary, args, err := e.buildNONMEMCommand(modelPath, isParallel, cores, isGrid, additionalOptions)
	if err != nil {
		return nil, nil, err
	}

	// Generate job ID for run log if run logging is enabled
	var jobID string
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		jobID = e.runLogger.GenerateJobID()
	}

	// Record start time for duration calculation
	startTime := time.Now()

	// Create the command
	cmd := exec.CommandContext(ctx, nonmemBinary, args...)

	// Set working directory to the model's directory
	workDir := filepath.Dir(modelPath)
	cmd.Dir = workDir

	// Create pipes for real-time output streaming
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create streaming output channels
	streaming := &StreamingOutput{
		Stdout: make(chan string, 100), // Buffered to prevent blocking
		Stderr: make(chan string, 100),
		Done:   make(chan bool, 1),
	}

	// Buffers to also capture complete output for final result
	var stdout, stderr bytes.Buffer

	// Create result struct that will be updated by the completion goroutine
	result := &ExecutionResult{
		ExitCode: 0, // Will be updated when command completes
		Stdout:   []byte{},
		Stderr:   []byte{},
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start NONMEM: %w", err)
	}

	// Stream stdout in goroutine
	go func() {
		defer close(streaming.Stdout)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdout.WriteString(line + "\n")
			select {
			case streaming.Stdout <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Stream stderr in goroutine
	go func() {
		defer close(streaming.Stderr)
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderr.WriteString(line + "\n")
			select {
			case streaming.Stderr <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for command completion in goroutine and update result
	go func() {
		defer func() {
			streaming.Done <- true
			close(streaming.Done)
		}()
		cmdErr := cmd.Wait()

		// Calculate execution duration
		duration := time.Since(startTime)

		// Update exit code in result
		exitCode := 0
		if cmdErr != nil {
			var exitError *exec.ExitError
			if errors.As(cmdErr, &exitError) {
				exitCode = exitError.ExitCode()
				result.ExitCode = exitCode
			} else {
				exitCode = -1
				result.ExitCode = -1
			}
		}

		// Update final output in result
		result.Stdout = stdout.Bytes()
		result.Stderr = stderr.Bytes()

		// Log execution to run log if enabled
		if e.runLogger != nil && e.runLogger.IsEnabled() {
			runLogErr := e.runLogger.RecordExecution(
				jobID,
				nonmemBinary,
				args,
				stdout.String(),
				stderr.String(),
				exitCode,
				duration,
				workDir,
			)
			if runLogErr != nil {
				log.Printf("Warning: Failed to log execution to run log: %v", runLogErr)
			}
		}
	}()

	return streaming, result, nil
}

// buildNonmemBinaryPath constructs the full path to the NONMEM binary
// by joining the base path with the binary name and converting to an absolute path
// for reliable cross-platform execution.
func buildNonmemBinaryPath(nonmemPath, nonmemBinary string) (string, error) {
	// Join the path components using the OS-appropriate separator
	binaryPath := filepath.Join(nonmemPath, nonmemBinary)

	// Convert to absolute path to ensure proper resolution across platforms
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for NONMEM binary %q: %w", binaryPath, err)
	}

	return absPath, nil
}

// runRemote runs nmfe directly on the configured remote host over SSH (no
// scheduler), mirroring the remote PsN path. Model/working-dir paths are
// translated to the remote host and results are read locally via the shared
// mount. Full run-log artifact collection for SSH runs is tracked in #183.
func (e *NONMEMExecutor) runRemote(ctx context.Context, modelPath string, isParallel bool, _ int, additionalOptions []string) (*ExecutionResult, error) {
	applyBeforeRun(e.config, modelPath)
	applyPreHooks(ctx, e.config, modelPath)

	mapper := remote.NewPathMapper(e.config.Remote.Mounts)

	runner, err := remote.NewRunner(e.config.Remote)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote runner: %w", err)
	}

	remoteModel := mapper.ToRemote(modelPath)
	argv := buildRemoteNonmemArgv(e.config.NonmemPath, e.config.NonmemBinary, remoteModel, isParallel, additionalOptions)

	out, exitCode, err := runner.Run(ctx, path.Dir(remoteModel), argv, "")
	if err != nil {
		return nil, fmt.Errorf("remote NONMEM execution failed: %w", err)
	}

	applyPostHooks(ctx, e.config, modelPath)
	applyAfterRun(e.config, modelPath)

	return &ExecutionResult{ExitCode: exitCode, Stdout: []byte(out)}, nil
}

// buildRemoteNonmemArgv builds the nmfe command argv for remote execution, using
// forward-slash (remote) paths and mirroring buildNONMEMCommand's argument order
// (binary, model, output.lst, [-parallel pnm], options).
func buildRemoteNonmemArgv(nonmemPath, nonmemBinary, remoteModel string, isParallel bool, additionalOptions []string) []string {
	binary := nonmemBinary
	if nonmemPath != "" {
		binary = path.Join(nonmemPath, nonmemBinary)
	}

	base := strings.TrimSuffix(remoteModel, path.Ext(remoteModel))
	argv := []string{binary, remoteModel, base + ".lst"}

	if isParallel {
		argv = append(argv, "-parallel", base+".pnm")
	}

	argv = append(argv, additionalOptions...)

	return argv
}
