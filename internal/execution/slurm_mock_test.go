//go:build integration
// +build integration

package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pharmalytica/janus/internal/slurm"
)

// MockSLURMClient implements the slurm.Client interface for testing
type MockSLURMClient struct {
	mu         sync.RWMutex
	jobCounter int
	jobs       map[string]*MockJob
	testDir    string

	// Test control
	jobDuration      time.Duration
	simulateFailure  bool
	jobStates        []string                    // Progression of states for realistic simulation
	contentGenerator func(string) MockJobContent // Custom content generation function
}

// MockJob represents a simulated SLURM job
type MockJob struct {
	ID         string
	JobName    string
	State      string
	ExitCode   int
	OutputFile string
	ErrorFile  string
	WorkingDir string

	// Simulation control
	startTime  time.Time
	duration   time.Duration
	content    MockJobContent
	stateIndex int
	states     []string
}

// MockJobContent defines what content to write to output files
type MockJobContent struct {
	StdoutLines []string
	StderrLines []string
	WriteMode   string // "immediate", "incremental", or "delayed"
}

// NewMockSLURMClient creates a new mock SLURM client for testing
func NewMockSLURMClient(testDir string) *MockSLURMClient {
	return &MockSLURMClient{
		jobs:        make(map[string]*MockJob),
		testDir:     testDir,
		jobDuration: 2 * time.Second, // Default test job duration
		jobStates:   []string{"PENDING", "RUNNING", "COMPLETED"},
	}
}

// SubmitJob simulates job submission and starts file writing simulation
func (m *MockSLURMClient) SubmitJob(ctx context.Context, jobScript string, options slurm.SubmitOptions) (*slurm.JobInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.jobCounter++
	jobID := fmt.Sprintf("mock_%d", m.jobCounter)

	// Create mock job
	job := &MockJob{
		ID:         jobID,
		JobName:    options.JobName,
		State:      "PENDING",
		ExitCode:   0,
		OutputFile: options.OutputFile,
		ErrorFile:  options.ErrorFile,
		WorkingDir: options.WorkingDir,
		startTime:  time.Now(),
		duration:   m.jobDuration,
		states:     m.jobStates,
		stateIndex: 0,
		content:    m.generateMockContent(options.JobName),
	}

	// Set exit code for failure simulation
	if m.simulateFailure {
		job.ExitCode = 1
		job.states = []string{"PENDING", "RUNNING", "FAILED"}
	}

	m.jobs[jobID] = job

	// Start job simulation in background
	go m.simulateJobExecution(job)

	return &slurm.JobInfo{
		JobID:      jobID,
		JobName:    options.JobName,
		State:      "PENDING",
		SubmitTime: time.Now().Format(time.RFC3339),
		WorkingDir: options.WorkingDir,
	}, nil
}

// GetJobStatus returns the current status of a mock job
func (m *MockSLURMClient) GetJobStatus(ctx context.Context, jobID string) (*slurm.JobStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return &slurm.JobStatus{
		JobID:    jobID,
		State:    job.State,
		ExitCode: job.ExitCode,
		Reason:   m.getJobReason(job),
	}, nil
}

// CancelJob simulates job cancellation
func (m *MockSLURMClient) CancelJob(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	job.State = "CANCELLED"
	job.ExitCode = 1
	return nil
}

// GetJobOutput simulates retrieving job output
func (m *MockSLURMClient) GetJobOutput(ctx context.Context, jobID string) (*slurm.JobOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	// Read output files if they exist
	var stdout, stderr string
	if job.OutputFile != "" {
		if content, err := os.ReadFile(job.OutputFile); err == nil {
			stdout = string(content)
		}
	}
	if job.ErrorFile != "" {
		if content, err := os.ReadFile(job.ErrorFile); err == nil {
			stderr = string(content)
		}
	}

	return &slurm.JobOutput{
		JobID:  jobID,
		Stdout: stdout,
		Stderr: stderr,
	}, nil
}

// Close cleans up the mock client
func (m *MockSLURMClient) Close() error {
	return nil
}

// simulateJobExecution runs the job simulation in a separate goroutine
func (m *MockSLURMClient) simulateJobExecution(job *MockJob) {
	// Transition through job states
	stateDuration := job.duration / time.Duration(len(job.states))

	for i, state := range job.states {
		// Update job state
		m.mu.Lock()
		job.State = state
		job.stateIndex = i
		m.mu.Unlock()

		// When job starts running, begin writing output files
		if state == "RUNNING" {
			go m.writeOutputFiles(job)
		}

		// Wait before next state transition (except for the last state)
		if i < len(job.states)-1 {
			time.Sleep(stateDuration)
		}
	}

	// Job is complete, ensure all output is written
	time.Sleep(100 * time.Millisecond) // Small delay to ensure file writes complete
}

// writeOutputFiles simulates SLURM writing output to files
func (m *MockSLURMClient) writeOutputFiles(job *MockJob) {
	// Create output directories if needed
	if job.OutputFile != "" {
		os.MkdirAll(filepath.Dir(job.OutputFile), 0755)
	}
	if job.ErrorFile != "" {
		os.MkdirAll(filepath.Dir(job.ErrorFile), 0755)
	}

	switch job.content.WriteMode {
	case "immediate":
		m.writeImmediateContent(job)
	case "incremental":
		m.writeIncrementalContent(job)
	case "delayed":
		m.writeDelayedContent(job)
	default:
		m.writeImmediateContent(job)
	}
}

// writeImmediateContent writes all content at once (simulates fast jobs)
func (m *MockSLURMClient) writeImmediateContent(job *MockJob) {
	if job.OutputFile != "" && len(job.content.StdoutLines) > 0 {
		content := strings.Join(job.content.StdoutLines, "\n")
		os.WriteFile(job.OutputFile, []byte(content), 0644)
	}

	if job.ErrorFile != "" && len(job.content.StderrLines) > 0 {
		content := strings.Join(job.content.StderrLines, "\n")
		os.WriteFile(job.ErrorFile, []byte(content), 0644)
	}
}

// writeIncrementalContent writes content line by line over time
func (m *MockSLURMClient) writeIncrementalContent(job *MockJob) {
	writeDelay := job.duration / time.Duration(len(job.content.StdoutLines)+len(job.content.StderrLines))

	// Write stdout lines incrementally
	if job.OutputFile != "" {
		outFile, _ := os.Create(job.OutputFile)
		defer outFile.Close()

		for _, line := range job.content.StdoutLines {
			outFile.WriteString(line + "\n")
			outFile.Sync()
			time.Sleep(writeDelay)
		}
	}

	// Write stderr lines incrementally
	if job.ErrorFile != "" {
		errFile, _ := os.Create(job.ErrorFile)
		defer errFile.Close()

		for _, line := range job.content.StderrLines {
			errFile.WriteString(line + "\n")
			errFile.Sync()
			time.Sleep(writeDelay)
		}
	}
}

// writeDelayedContent writes content near the end of job execution
func (m *MockSLURMClient) writeDelayedContent(job *MockJob) {
	// Wait for most of the job duration
	time.Sleep(job.duration * 3 / 4)

	// Then write all content quickly
	m.writeImmediateContent(job)
}

// generateMockContent creates realistic NONMEM-like output content
func (m *MockSLURMClient) generateMockContent(jobName string) MockJobContent {
	// Use custom content generator if provided
	if m.contentGenerator != nil {
		return m.contentGenerator(jobName)
	}

	stdout := []string{
		fmt.Sprintf("Starting NONMEM execution for %s", jobName),
		"Loading model file...",
		"Parsing control stream...",
		"Setting up estimation method...",
		"Beginning parameter estimation...",
		"Iteration 1: OBJ = -1234.567",
		"Iteration 2: OBJ = -1235.123",
		"Iteration 3: OBJ = -1235.456",
		"Parameter estimation completed successfully",
		"Final OBJ = -1235.456",
		"Writing output files...",
		"NONMEM execution completed",
	}

	stderr := []string{
		"NONMEM version 7.5.0",
		"License check passed",
		"Model compilation successful",
	}

	// Add failure content if simulating failure
	if m.simulateFailure {
		stdout = append(stdout, "ERROR: Parameter estimation failed")
		stderr = append(stderr, "NONMEM terminated with errors")
	}

	return MockJobContent{
		StdoutLines: stdout,
		StderrLines: stderr,
		WriteMode:   "incremental", // Default to incremental writing
	}
}

// getJobReason returns a reason string for the job state
func (m *MockSLURMClient) getJobReason(job *MockJob) string {
	switch job.State {
	case "PENDING":
		return "Waiting for resources"
	case "RUNNING":
		return "Executing on node"
	case "COMPLETED":
		return "Job completed successfully"
	case "FAILED":
		return "Job failed with errors"
	case "CANCELLED":
		return "Job cancelled by user"
	default:
		return ""
	}
}

// Test helper methods for configuring mock behavior

func (m *MockSLURMClient) SetJobDuration(duration time.Duration) {
	m.jobDuration = duration
}

func (m *MockSLURMClient) SetSimulateFailure(fail bool) {
	m.simulateFailure = fail
}

func (m *MockSLURMClient) SetJobStates(states []string) {
	m.jobStates = states
}

func (m *MockSLURMClient) SetContentGenerator(generator func(string) MockJobContent) {
	m.contentGenerator = generator
}

// Helper to get all jobs (for testing verification)
func (m *MockSLURMClient) GetAllJobs() map[string]*MockJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*MockJob)
	for k, v := range m.jobs {
		result[k] = v
	}
	return result
}
