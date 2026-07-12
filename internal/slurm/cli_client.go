package slurm

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pharmalytica/janus/internal/config"
)

// CLIClient implements the Client interface using SLURM CLI commands.
type CLIClient struct {
	timeout time.Duration
}

// NewCLIClient creates a new CLI-based SLURM client.
// Tool validation happens at execution time, not during initialization.
func NewCLIClient(cfg config.SLURMConfig) (*CLIClient, error) {
	// Parse timeout
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	return &CLIClient{
		timeout: timeout,
	}, nil
}

// SubmitJob submits a job using sbatch command.
func (c *CLIClient) SubmitJob(ctx context.Context, jobScript string, options SubmitOptions) (*JobInfo, error) {
	// Build sbatch command arguments
	args := []string{"sbatch"}

	if options.JobName != "" {
		args = append(args, "--job-name="+options.JobName)
	}
	if options.Partition != "" {
		args = append(args, "--partition="+options.Partition)
	}
	if options.Nodes > 0 {
		args = append(args, "--nodes="+strconv.Itoa(options.Nodes))
	}
	if options.CPUs > 0 {
		args = append(args, "--cpus-per-task="+strconv.Itoa(options.CPUs))
	}
	if options.Memory != "" {
		args = append(args, "--mem="+options.Memory)
	}
	if options.TimeLimit != "" {
		args = append(args, "--time="+options.TimeLimit)
	}
	if options.WorkingDir != "" {
		args = append(args, "--chdir="+options.WorkingDir)
	}
	if options.OutputFile != "" {
		args = append(args, "--output="+options.OutputFile)
	}
	if options.ErrorFile != "" {
		args = append(args, "--error="+options.ErrorFile)
	}

	// Add environment variables
	for k, v := range options.Environment {
		args = append(args, "--export="+k+"="+v)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute sbatch command with job script as stdin
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // args[0] is always "sbatch"
	cmd.Stdin = strings.NewReader(jobScript)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("sbatch failed: %s", string(exitErr.Stderr))
		}

		return nil, fmt.Errorf("failed to execute sbatch: %w", err)
	}

	// Parse job ID from sbatch output
	// Expected format: "Submitted batch job 12345"
	jobID, err := parseJobIDFromSbatch(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse job ID from sbatch output: %w", err)
	}

	return &JobInfo{
		JobID:      jobID,
		JobName:    options.JobName,
		State:      "PENDING",
		Partition:  options.Partition,
		WorkingDir: options.WorkingDir,
	}, nil
}

// GetJobStatus retrieves job status using squeue command.
func (c *CLIClient) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute squeue command
	args := []string{
		"squeue",
		"--job=" + jobID,
		"--format=%i,%T,%r,%S,%E,%M",
		"--noheader",
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // args[0] is always "squeue"
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Job might not be in queue anymore, try scontrol
			return c.getCompletedJobStatus(ctx, jobID)
		}

		return nil, fmt.Errorf("failed to execute squeue: %w", err)
	}

	// Parse squeue output
	return parseJobStatusFromSqueue(jobID, string(output))
}

// getCompletedJobStatus retrieves status for completed jobs using scontrol.
func (c *CLIClient) getCompletedJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute scontrol command
	args := []string{"scontrol", "show", "job", jobID}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // args[0] is always "scontrol"

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Parse scontrol output
	return parseJobStatusFromScontrol(jobID, string(output))
}

// CancelJob cancels a job using scancel command.
func (c *CLIClient) CancelJob(ctx context.Context, jobID string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Execute scancel command
	cmd := exec.CommandContext(ctx, "scancel", jobID)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("scancel failed: %s", string(exitErr.Stderr))
		}

		return fmt.Errorf("failed to execute scancel: %w", err)
	}

	return nil
}

// GetJobOutput retrieves job output by reading output files.
func (c *CLIClient) GetJobOutput(ctx context.Context, jobID string) (*JobOutput, error) {
	// For CLI mode, we would need to know the output file paths
	// This would typically be configured when the job was submitted
	return nil, fmt.Errorf("job output retrieval requires output file paths for CLI mode")
}

// Close closes the CLI client (no-op for CLI client).
func (c *CLIClient) Close() error {
	return nil
}

// parseJobIDFromSbatch extracts job ID from sbatch output.
func parseJobIDFromSbatch(output string) (string, error) {
	// Regular expression to match "Submitted batch job 12345"
	re := regexp.MustCompile(`Submitted batch job (\d+)`)
	matches := re.FindStringSubmatch(strings.TrimSpace(output))

	if len(matches) < 2 {
		return "", fmt.Errorf("could not find job ID in sbatch output: %s", output)
	}

	return matches[1], nil
}

// parseJobStatusFromSqueue parses job status from squeue output.
func parseJobStatusFromSqueue(jobID, output string) (*JobStatus, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("no job data found for job ID: %s", jobID)
	}

	// Parse CSV format: JobID,State,Reason,StartTime,EndTime,RunTime
	fields := strings.Split(lines[0], ",")
	if len(fields) < 6 {
		return nil, fmt.Errorf("unexpected squeue output format: %s", output)
	}

	status := &JobStatus{
		JobID:     fields[0],
		State:     fields[1],
		Reason:    fields[2],
		StartTime: fields[3],
		EndTime:   fields[4],
		RunTime:   fields[5],
	}

	return status, nil
}

// parseJobStatusFromScontrol parses job status from scontrol output.
func parseJobStatusFromScontrol(jobID, output string) (*JobStatus, error) { //nolint:unparam // Error might be needed for future validation
	status := &JobStatus{
		JobID: jobID,
	}

	// Parse key=value pairs from scontrol output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for specific fields
		if strings.Contains(line, "JobState=") {
			if state := extractValue(line, "JobState="); state != "" {
				status.State = state
			}
		}
		if strings.Contains(line, "ExitCode=") {
			if exitCodeStr := extractValue(line, "ExitCode="); exitCodeStr != "" {
				// ExitCode format is usually "0:0" where first is exit code
				parts := strings.Split(exitCodeStr, ":")
				if len(parts) > 0 {
					if exitCode, err := strconv.Atoi(parts[0]); err == nil {
						status.ExitCode = exitCode
					}
				}
			}
		}
		if strings.Contains(line, "StartTime=") {
			if startTime := extractValue(line, "StartTime="); startTime != "" {
				status.StartTime = startTime
			}
		}
		if strings.Contains(line, "EndTime=") {
			if endTime := extractValue(line, "EndTime="); endTime != "" {
				status.EndTime = endTime
			}
		}
		if strings.Contains(line, "Reason=") {
			if reason := extractValue(line, "Reason="); reason != "" {
				status.Reason = reason
			}
		}
	}

	return status, nil
}

// extractValue extracts value from a key=value pair in scontrol output.
func extractValue(line, key string) string {
	idx := strings.Index(line, key)
	if idx == -1 {
		return ""
	}

	valueStart := idx + len(key)
	remaining := line[valueStart:]

	// Find the end of the value (space or end of line)
	spaceIdx := strings.Index(remaining, " ")
	if spaceIdx == -1 {
		return remaining
	}

	return remaining[:spaceIdx]
}
