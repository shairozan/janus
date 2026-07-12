package execution

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/slurm"
)

// OutputHandler is a function type for handling streaming output.
type OutputHandler func([]byte)

// FileStreamData holds accumulated output data and streaming state.
type FileStreamData struct {
	mu         sync.RWMutex
	filePath   string
	lastOffset int64
	data       strings.Builder
}

// SLURMExecutor implements the Executor interface for SLURM-based execution.
type SLURMExecutor struct {
	client    slurm.Client
	config    *config.Config
	runLogger *runlog.RunLogger
}

// NewSLURMExecutor creates a new SLURM executor with the provided configuration.
func NewSLURMExecutor(cfg *config.Config, runLogger *runlog.RunLogger) (*SLURMExecutor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	// Create SLURM client based on configuration
	client, err := slurm.NewClient(cfg.SLURM)
	if err != nil {
		return nil, fmt.Errorf("failed to create SLURM client: %w", err)
	}

	return &SLURMExecutor{
		client:    client,
		config:    cfg,
		runLogger: runLogger,
	}, nil
}

// jobCompletionResult holds the result of a job lifecycle.
type jobCompletionResult struct {
	result *ExecutionResult
	err    error
}

// Execute executes a NONMEM model using SLURM job submission.
func (e *SLURMExecutor) Execute(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*ExecutionResult, error) {
	startTime := time.Now()

	// Generate unique job ID for run log
	jobID := generateJobID()

	// Build job script for NONMEM execution
	jobScript, err := e.buildNonmemJobScript(modelPath, isParallel, cores, isGrid, additionalOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to build job script: %w", err)
	}

	// Prepare SLURM job submission options
	submitOptions := e.buildSubmitOptions(modelPath, cores, isParallel, additionalOptions)

	// Submit job to SLURM
	log.Printf("Submitting NONMEM job to SLURM: %s", modelPath)
	jobInfo, err := e.client.SubmitJob(ctx, jobScript, submitOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job to SLURM: %w", err)
	}

	log.Printf("SLURM job submitted with ID: %s", jobInfo.JobID)

	// Channel to receive completion result
	resultChan := make(chan jobCompletionResult, 1)

	// Start job lifecycle goroutine
	go e.monitorJobLifecycle(ctx, jobID, jobInfo.JobID, modelPath, submitOptions, startTime, resultChan)

	// Wait for completion
	completion := <-resultChan

	return completion.result, completion.err
}

// ExecuteWithStreaming executes a NONMEM model using SLURM with streaming output.
func (e *SLURMExecutor) ExecuteWithStreaming(ctx context.Context, modelPath string, isParallel bool, cores int, isGrid bool, additionalOptions []string) (*StreamingOutput, *ExecutionResult, error) {
	startTime := time.Now()

	// Generate unique job ID for run log
	jobID := generateJobID()

	// Create streaming output channels
	streamingOutput := &StreamingOutput{
		Stdout: make(chan string, 100),
		Stderr: make(chan string, 100),
		Done:   make(chan bool, 1),
	}

	// Build job script for NONMEM execution
	jobScript, err := e.buildNonmemJobScript(modelPath, isParallel, cores, isGrid, additionalOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build job script: %w", err)
	}

	// Prepare SLURM job submission options
	submitOptions := e.buildSubmitOptions(modelPath, cores, isParallel, additionalOptions)

	// Submit job to SLURM
	log.Printf("Submitting NONMEM job to SLURM with streaming: %s", modelPath)
	jobInfo, err := e.client.SubmitJob(ctx, jobScript, submitOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to submit job to SLURM: %w", err)
	}

	log.Printf("SLURM job submitted with ID: %s", jobInfo.JobID)

	// Channel to receive completion result
	resultChan := make(chan jobCompletionResult, 1)

	// Start job lifecycle goroutine with streaming
	go e.monitorJobLifecycleWithStreaming(ctx, jobID, jobInfo.JobID, modelPath, submitOptions, startTime, streamingOutput, resultChan)

	// Wait for completion
	completion := <-resultChan

	return streamingOutput, completion.result, completion.err
}

// Close closes the SLURM executor and cleans up resources.
func (e *SLURMExecutor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}

	return nil
}

// streamOutputFile watches for file creation and streams new content in real-time.
func (e *SLURMExecutor) streamOutputFile(ctx context.Context, streamData *FileStreamData, outputChan chan<- string, prefix string) {
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms for better responsiveness
	defer ticker.Stop()

	log.Printf("Starting file watcher for: %s", streamData.filePath)

	for {
		select {
		case <-ctx.Done():
			log.Printf("File watcher for %s stopping due to context cancellation", streamData.filePath)

			return

		case <-ticker.C:
			streamData.mu.Lock()
			// Always try to read new content - no dependency on isComplete flag
			e.readFileContent(streamData, outputChan, prefix)
			streamData.mu.Unlock()
		}
	}
}

// readFileContent reads new content from a file and updates the stream data.
// This function handles file creation gracefully and streams content as it becomes available.
func (e *SLURMExecutor) readFileContent(streamData *FileStreamData, outputChan chan<- string, prefix string) {
	// Check if file exists - don't log errors for missing files as they may not be created yet
	file, err := os.Open(streamData.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - this is normal for SLURM jobs that haven't started writing

			return
		}
		// Log other errors (permissions, etc.)
		log.Printf("Warning: Failed to open output file %s: %v", streamData.filePath, err)

		return
	}
	defer file.Close()

	// Get current file size to detect if it's growing
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Warning: Failed to stat file %s: %v", streamData.filePath, err)

		return
	}

	// If file is smaller than our last offset, it may have been truncated/recreated
	if fileInfo.Size() < streamData.lastOffset {
		log.Printf("File %s appears to have been truncated, resetting offset", streamData.filePath)
		streamData.lastOffset = 0
		streamData.data.Reset() // Clear accumulated data
	}

	// Seek to last read position
	if _, err := file.Seek(streamData.lastOffset, io.SeekStart); err != nil {
		log.Printf("Warning: Failed to seek in file %s: %v", streamData.filePath, err)

		return
	}

	// Read new content line by line
	scanner := bufio.NewScanner(file)
	var newLines []string
	var bytesRead int64

	for scanner.Scan() {
		line := scanner.Text()
		newLines = append(newLines, line)
		bytesRead += int64(len(line) + 1) // +1 for newline

		// Add to accumulated data
		if streamData.data.Len() > 0 {
			streamData.data.WriteString("\n")
		}
		streamData.data.WriteString(line)

		// For real-time streaming, send lines immediately if channel is available
		if outputChan != nil {
			lineToSend := line
			if prefix != "" {
				lineToSend = prefix + " " + lineToSend
			}
			select {
			case outputChan <- lineToSend:
				// Successfully sent
			case <-time.After(50 * time.Millisecond):
				// Channel is full or slow, buffer this line for later
				log.Printf("Output channel busy, buffering line from %s", streamData.filePath)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Warning: Error reading file %s: %v", streamData.filePath, err)

		return
	}

	// Update offset for next read
	streamData.lastOffset += bytesRead

	// Log progress for debugging
	if len(newLines) > 0 {
		log.Printf("Read %d new lines from %s (total offset: %d)", len(newLines), streamData.filePath, streamData.lastOffset)
	}
}

// buildNonmemJobScript creates a job script for NONMEM execution.
// For REST API submission, this creates a simple command string (like --wrap).
// For CLI submission, this creates a full bash script.
func (e *SLURMExecutor) buildNonmemJobScript(modelPath string, isParallel bool, cores int, _ bool, additionalOptions []string) (string, error) { //nolint:unparam // Error might be needed for future validation
	// Build NONMEM command
	nonmemPath := e.config.NonmemPath
	nonmemBinary := e.config.NonmemBinary
	if nonmemBinary == "" {
		nonmemBinary = "nmfe75"
	}

	// Full path to NONMEM binary
	nonmemFullPath := filepath.Join(nonmemPath, nonmemBinary)

	// Build command arguments - NONMEM expects: nmfe76 infile outfile [options]
	outputFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"
	args := []string{modelPath, outputFile}

	// Handle parallel execution
	if isParallel && cores > 1 {
		pnmFile := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".pnm"
		args = append(args, "-parallel", pnmFile)
	}

	// Grid execution options can be added here if needed in the future
	// This would depend on specific NONMEM grid configuration

	// Add any additional options
	args = append(args, additionalOptions...)

	// Build the NONMEM execution command
	nonmemCommand := fmt.Sprintf("%s %s", nonmemFullPath, strings.Join(args, " "))

	// For REST API: Return a simple command string (equivalent to sbatch --wrap "command")
	// The working directory and environment are set via REST API options
	// This creates a simple wrapper script that changes to the model directory and runs NONMEM
	script := fmt.Sprintf("#!/bin/bash\ncd %s && %s", filepath.Dir(modelPath), nonmemCommand)

	return script, nil
}

// buildSubmitOptions creates SLURM submission options based on the execution parameters.
//
// SLURM Output File Path Resolution:
// - WorkingDir: Set to filepath.Dir(modelPath) - the directory containing the model
// - OutputFile: Absolute path = modelPath + ".slurm.out"
// - ErrorFile: Absolute path = modelPath + ".slurm.err"
//
// This approach ensures that:
// 1. The job runs in the model's directory (access to data files, etc.)
// 2. Output files are written to predictable absolute locations
// 3. File watchers monitor the exact paths SLURM will write to
// 4. No path translation or resolution ambiguity
//
// SLURM resolves paths as follows:
// - Absolute paths (used here): Written to exact location regardless of working directory.
// - Relative paths: Resolved against the job's working directory (--chdir).
func (e *SLURMExecutor) buildSubmitOptions(modelPath string, cores int, isParallel bool, _ []string) slurm.SubmitOptions {
	submitOptions := slurm.SubmitOptions{
		JobName:    fmt.Sprintf("nonmem-%s", filepath.Base(modelPath)),
		WorkingDir: filepath.Dir(modelPath),
		OutputFile: modelPath + ".slurm.out",
		ErrorFile:  modelPath + ".slurm.err",
	}

	// Only set CPU count if parallelism is enabled and cores > 1
	if isParallel && cores > 1 {
		submitOptions.CPUs = cores
	}
	// Otherwise let SLURM use cluster defaults (typically 1 CPU)

	// Let SLURM use cluster defaults for memory and time limit
	// These can be overridden by SLURM configuration or user's sbatch directives

	return submitOptions
}

// waitForJobCompletion waits for a SLURM job to complete and returns final status.
func (e *SLURMExecutor) waitForJobCompletion(ctx context.Context, jobID string) (*slurm.JobStatus, error) {
	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for job completion")

		case <-ticker.C:
			status, err := e.client.GetJobStatus(ctx, jobID)
			if err != nil {
				log.Printf("Warning: Failed to get job status for %s: %v", jobID, err)

				continue
			}

			// Check if job is completed
			switch status.State {
			case "COMPLETED", "FAILED", "CANCELLED", "TIMEOUT", "NODE_FAIL", "PREEMPTED":
				log.Printf("Job %s completed with status: %s", jobID, status.State)
				// Add a grace period to allow final output to be written to files
				log.Printf("Waiting 10 seconds for final output to be written...")
				time.Sleep(10 * time.Second)
				log.Printf("Grace period completed, job %s is ready for final processing", jobID)

				return status, nil
			case "PENDING", "RUNNING", "SUSPENDED", "COMPLETING":
				// Job is still active, continue polling
				log.Printf("Job %s status: %s", jobID, status.State)

				continue
			default:
				log.Printf("Job %s has unknown status: %s", jobID, status.State)

				continue
			}
		}
	}
}

// streamJobStatusToChannels streams job status updates to streaming output channels.
func (e *SLURMExecutor) streamJobStatusToChannels(ctx context.Context, jobID string, streamingOutput *StreamingOutput) {
	ticker := time.NewTicker(10 * time.Second) // Stream updates every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			status, err := e.client.GetJobStatus(ctx, jobID)
			if err != nil {
				continue
			}

			// Send status update to stdout channel
			statusMsg := fmt.Sprintf("[SLURM] Job %s: %s", jobID, status.State)
			if status.Reason != "" {
				statusMsg += fmt.Sprintf(" (%s)", status.Reason)
			}

			select {
			case streamingOutput.Stdout <- statusMsg:
			case <-ctx.Done():
				return
			}

			// Stop streaming if job is completed
			switch status.State {
			case "COMPLETED", "FAILED", "CANCELLED", "TIMEOUT", "NODE_FAIL", "PREEMPTED":
				return
			}
		}
	}
}

// monitorJobLifecycle is a goroutine that monitors a SLURM job and handles post-completion tasks.
// It is responsible for:
// 1. Monitoring job status until completion
// 2. Collecting stdout/stderr files
// 3. Collecting NONMEM output files (via run log.EmbedOutputFiles)
// 4. Writing run log
// 5. Sending result back to caller.
func (e *SLURMExecutor) monitorJobLifecycle(ctx context.Context, jobID, slurmJobID, modelPath string, submitOptions slurm.SubmitOptions, startTime time.Time, resultChan chan<- jobCompletionResult) {
	log.Printf("Job lifecycle monitor started for SLURM job %s", slurmJobID)

	// Step 1: Monitor job status until completion
	finalStatus, err := e.waitForJobCompletion(ctx, slurmJobID)
	if err != nil {
		resultChan <- jobCompletionResult{
			result: nil,
			err:    fmt.Errorf("error while waiting for job completion: %w", err),
		}

		return
	}

	log.Printf("Job %s completed with status: %s, starting post-completion tasks", slurmJobID, finalStatus.State)

	// Step 2: Collect stdout/stderr files
	stdout, err := os.ReadFile(submitOptions.OutputFile)
	if err != nil {
		log.Printf("Warning: Failed to read stdout file %s: %v", submitOptions.OutputFile, err)
		stdout = []byte{}
	} else {
		log.Printf("Collected stdout: %d bytes from %s", len(stdout), submitOptions.OutputFile)
	}

	stderr, err := os.ReadFile(submitOptions.ErrorFile)
	if err != nil {
		log.Printf("Warning: Failed to read stderr file %s: %v", submitOptions.ErrorFile, err)
		stderr = []byte{}
	} else {
		log.Printf("Collected stderr: %d bytes from %s", len(stderr), submitOptions.ErrorFile)
	}

	duration := time.Since(startTime)

	// Create result
	result := &ExecutionResult{
		ExitCode: finalStatus.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	// Step 3 & 4: Write run log (which will also collect NONMEM output files)
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		outputFiles := []string{submitOptions.OutputFile, submitOptions.ErrorFile}
		runLogErr := e.runLogger.RecordSLURMExecution(
			jobID,
			"slurm-nonmem",
			[]string{modelPath},
			string(stdout),
			string(stderr),
			result.ExitCode,
			duration,
			submitOptions.WorkingDir,
			slurmJobID,
			outputFiles,
		)
		if runLogErr != nil {
			log.Printf("Warning: Failed to log execution to run log: %v", runLogErr)
		} else {
			log.Printf("Audit log written for job %s", slurmJobID)
		}
	}

	// Step 5: Send result back
	log.Printf("Job lifecycle complete for SLURM job %s", slurmJobID)
	resultChan <- jobCompletionResult{
		result: result,
		err:    nil,
	}
}

// monitorJobLifecycleWithStreaming is like monitorJobLifecycle but also streams output in real-time.
// It streams job status and output files while monitoring, then handles post-completion tasks.
func (e *SLURMExecutor) monitorJobLifecycleWithStreaming(ctx context.Context, jobID, slurmJobID, modelPath string, submitOptions slurm.SubmitOptions, startTime time.Time, streamingOutput *StreamingOutput, resultChan chan<- jobCompletionResult) {
	log.Printf("Job lifecycle monitor (with streaming) started for SLURM job %s", slurmJobID)

	// Create a cancellable context for streaming goroutines
	streamingCtx, cancelStreaming := context.WithCancel(ctx)
	defer cancelStreaming()

	// Start streaming output files
	var wg sync.WaitGroup
	wg.Add(2)

	stdoutData := &FileStreamData{filePath: submitOptions.OutputFile}
	stderrData := &FileStreamData{filePath: submitOptions.ErrorFile}

	// Stream stdout file
	go func() {
		defer wg.Done()
		e.streamOutputFile(streamingCtx, stdoutData, streamingOutput.Stdout, "[STDOUT]")
	}()

	// Stream stderr file
	go func() {
		defer wg.Done()
		e.streamOutputFile(streamingCtx, stderrData, streamingOutput.Stderr, "[STDERR]")
	}()

	// Stream job status updates
	go e.streamJobStatusToChannels(streamingCtx, slurmJobID, streamingOutput)

	// Step 1: Monitor job status until completion
	finalStatus, err := e.waitForJobCompletion(ctx, slurmJobID)
	if err != nil {
		cancelStreaming()
		streamingOutput.Done <- true
		resultChan <- jobCompletionResult{
			result: nil,
			err:    fmt.Errorf("error while waiting for job completion: %w", err),
		}

		return
	}

	log.Printf("Job %s completed with status: %s, starting post-completion tasks", slurmJobID, finalStatus.State)

	// Stop streaming goroutines
	cancelStreaming()
	wg.Wait()

	// Step 2: Collect stdout/stderr files (final read)
	stdout, err := os.ReadFile(submitOptions.OutputFile)
	if err != nil {
		log.Printf("Warning: Failed to read stdout file %s: %v", submitOptions.OutputFile, err)
		stdout = []byte{}
	} else {
		log.Printf("Collected stdout: %d bytes from %s", len(stdout), submitOptions.OutputFile)
	}

	stderr, err := os.ReadFile(submitOptions.ErrorFile)
	if err != nil {
		log.Printf("Warning: Failed to read stderr file %s: %v", submitOptions.ErrorFile, err)
		stderr = []byte{}
	} else {
		log.Printf("Collected stderr: %d bytes from %s", len(stderr), submitOptions.ErrorFile)
	}

	duration := time.Since(startTime)

	// Create result
	result := &ExecutionResult{
		ExitCode: finalStatus.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	// Signal streaming completion
	streamingOutput.Done <- true

	// Step 3 & 4: Write run log
	if e.runLogger != nil && e.runLogger.IsEnabled() {
		outputFiles := []string{submitOptions.OutputFile, submitOptions.ErrorFile}
		runLogErr := e.runLogger.RecordSLURMExecution(
			jobID,
			"slurm-nonmem",
			[]string{modelPath},
			string(stdout),
			string(stderr),
			result.ExitCode,
			duration,
			submitOptions.WorkingDir,
			slurmJobID,
			outputFiles,
		)
		if runLogErr != nil {
			log.Printf("Warning: Failed to log execution to run log: %v", runLogErr)
		} else {
			log.Printf("Audit log written for job %s", slurmJobID)
		}
	}

	// Step 5: Send result back
	log.Printf("Job lifecycle complete for SLURM job %s", slurmJobID)
	resultChan <- jobCompletionResult{
		result: result,
		err:    nil,
	}
}

// generateJobID generates a unique job ID for run log purposes.
func generateJobID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp if random generation fails
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(bytes)
}
