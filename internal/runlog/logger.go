package runlog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunEntry represents a JSON run log entry structure.
type RunEntry struct {
	JobID        string          `json:"job_id"`
	Timestamp    time.Time       `json:"timestamp"`
	Binary       string          `json:"binary"`
	Arguments    []string        `json:"arguments"`
	STDOUT       string          `json:"stdout"`
	STDERR       string          `json:"stderr"`
	ExitCode     int             `json:"exit_code"`
	Duration     int64           `json:"duration_ms"`
	WorkDir      string          `json:"working_directory"`
	RestAPI      *RestAPI        `json:"rest_api,omitempty"`      // Optional REST API details
	OutputFiles  []string        `json:"output_files,omitempty"`  // Paths to output files for reference
	SLURMJobID   string          `json:"slurm_job_id,omitempty"`  // SLURM job ID for grid jobs
	Hermes       *HermesMetadata `json:"hermes,omitempty"`        // Optional Hermes container execution metadata
	ModelSummary interface{}     `json:"model_summary,omitempty"` // Optional embedded model summary for NONMEM runs
}

// RestAPI represents REST API request/response details for run logging.
type RestAPI struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	RequestBody  string `json:"request_body,omitempty"`
	ResponseCode int    `json:"response_code"`
	ResponseBody string `json:"response_body,omitempty"`
}

// HermesMetadata represents container execution metadata for Hermes runs.
// This captures complete provenance information for CFR 21 Part 11 compliance.
type HermesMetadata struct {
	ExecutionID string          `json:"execution_id"`      // Hermes execution ID
	ContainerID string          `json:"container_id"`      // Docker container ID
	Image       ContainerImage  `json:"image"`             // Complete image provenance
	Resources   HermesResources `json:"resources"`         // Resource configuration used
	ModelConfig string          `json:"model_config_path"` // Path to .janus.config.json
	FilesCount  int             `json:"files_collected"`   // Number of files collected from container
	RuntimeSec  int64           `json:"runtime_seconds"`   // Execution time inside container
}

// ContainerImage represents complete container image provenance for audit trails.
// Captures the exact image used for execution to ensure reproducibility.
type ContainerImage struct {
	Name   string `json:"name"`             // Image name without tag (e.g., "pharmalytica/hermes-nonmem")
	Tag    string `json:"tag"`              // Image tag (e.g., "nm76", "latest")
	Digest string `json:"digest,omitempty"` // SHA256 digest (e.g., "sha256:abc123...")
	Full   string `json:"full"`             // Full reference (e.g., "pharmalytica/hermes-nonmem:nm76")
}

// HermesResources represents the resource configuration used for Hermes execution.
// Records CPU and memory limits from the model config at execution time.
type HermesResources struct {
	CPUCores int    `json:"cpu_cores"` // Number of CPU cores allocated
	Memory   string `json:"memory"`    // Memory limit (e.g., "8Gi", "4096Mi")
}

// RunLogger handles execution run logging.
type RunLogger struct {
	enabled bool
	logFile *os.File
}

// NewRunLogger creates a new run logger.
func NewRunLogger(enabled bool, logPath string) (*RunLogger, error) {
	logger := &RunLogger{
		enabled: enabled,
	}

	if enabled {
		// Ensure log directory exists
		dir := filepath.Dir(logPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create run log directory: %w", err)
		}

		// Open log file for appending
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open run log file: %w", err)
		}
		logger.logFile = file
	}

	return logger, nil
}

// GenerateJobID creates a unique job ID for tracking.
func (l *RunLogger) GenerateJobID() string {
	// Generate 16 random bytes (128 bits)
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}

	// Convert to hex string for a UUID-like format
	id := hex.EncodeToString(bytes)

	return fmt.Sprintf("job-%s", id)
}

// RecordExecution records an execution event to the run log.
func (l *RunLogger) RecordExecution(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string) error {
	return l.RecordExecutionWithRestAPI(jobID, binary, arguments, stdout, stderr, exitCode, duration, workDir, nil)
}

// RecordExecutionWithRestAPI records an execution event with optional REST API details to the run log.
func (l *RunLogger) RecordExecutionWithRestAPI(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, restAPI *RestAPI) error {
	if !l.enabled {
		return nil // Run logging is disabled
	}

	entry := RunEntry{
		JobID:     jobID,
		Timestamp: time.Now().UTC(),
		Binary:    binary,
		Arguments: arguments,
		STDOUT:    stdout,
		STDERR:    stderr,
		ExitCode:  exitCode,
		Duration:  duration.Milliseconds(),
		WorkDir:   workDir,
		RestAPI:   restAPI,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal run log entry: %w", err)
	}

	// Write to log file with newline (JSONL format)
	if _, err := l.logFile.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write run log entry: %w", err)
	}

	// Flush to ensure immediate write
	if err := l.logFile.Sync(); err != nil {
		log.Printf("Warning: failed to sync run log file: %v", err)
	}

	return nil
}

// Close closes the run log file.
func (l *RunLogger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}

	return nil
}

// IsEnabled returns whether run logging is enabled.
func (l *RunLogger) IsEnabled() bool {
	return l.enabled
}

// ReadRunEntries reads all run entries from the run log file.
func ReadRunEntries(logPath string) ([]RunEntry, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read run log file: %w", err)
	}

	var entries []RunEntry
	lines := string(data)

	// Split by newlines and parse each JSON line
	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry RunEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal run log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// RecordExecutionWithFiles records an execution event together with the output
// files it produced (e.g. a PsN analysis's result artifacts), without scheduler
// metadata.
func (l *RunLogger) RecordExecutionWithFiles(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, outputFiles []string) error {
	return l.RecordExecutionWithSLURM(jobID, binary, arguments, stdout, stderr, exitCode, duration, workDir, nil, "", outputFiles)
}

// RecordSLURMExecution records a SLURM execution event with additional metadata to the run log.
func (l *RunLogger) RecordSLURMExecution(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, slurmJobID string, outputFiles []string) error {
	return l.RecordExecutionWithSLURM(jobID, binary, arguments, stdout, stderr, exitCode, duration, workDir, nil, slurmJobID, outputFiles)
}

// RecordExecutionWithSLURM records an execution event with SLURM metadata and optional REST API details to the run log.
func (l *RunLogger) RecordExecutionWithSLURM(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, restAPI *RestAPI, slurmJobID string, outputFiles []string) error {
	if !l.enabled {
		return nil // Run logging is disabled
	}

	entry := RunEntry{
		JobID:       jobID,
		Timestamp:   time.Now().UTC(),
		Binary:      binary,
		Arguments:   arguments,
		STDOUT:      stdout,
		STDERR:      stderr,
		ExitCode:    exitCode,
		Duration:    duration.Milliseconds(),
		WorkDir:     workDir,
		RestAPI:     restAPI,
		SLURMJobID:  slurmJobID,
		OutputFiles: outputFiles,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal run log entry: %w", err)
	}

	// Write to log file with newline (JSONL format)
	if _, err := l.logFile.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write run log entry: %w", err)
	}

	// Flush to ensure immediate write
	if err := l.logFile.Sync(); err != nil {
		log.Printf("Warning: failed to sync run log file: %v", err)
	}

	return nil
}

// RecordHermesExecution records a Hermes container execution event with complete metadata to the run log.
// This method captures container image provenance, resource configuration, and file collection details
// for CFR 21 Part 11 compliance (REQ-50, REQ-51, REQ-52).
func (l *RunLogger) RecordHermesExecution(
	jobID string,
	binary string,
	arguments []string,
	stdout string,
	stderr string,
	exitCode int,
	duration time.Duration,
	workDir string,
	hermesMetadata *HermesMetadata,
	outputFiles []string,
) error {
	if !l.enabled {
		return nil // Run logging is disabled
	}

	entry := RunEntry{
		JobID:       jobID,
		Timestamp:   time.Now().UTC(),
		Binary:      binary,
		Arguments:   arguments,
		STDOUT:      stdout,
		STDERR:      stderr,
		ExitCode:    exitCode,
		Duration:    duration.Milliseconds(),
		WorkDir:     workDir,
		Hermes:      hermesMetadata,
		OutputFiles: outputFiles,
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal run log entry: %w", err)
	}

	// Write to log file with newline (JSONL format)
	if _, err := l.logFile.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write run log entry: %w", err)
	}

	// Flush to ensure immediate write
	if err := l.logFile.Sync(); err != nil {
		log.Printf("Warning: failed to sync run log file: %v", err)
	}

	return nil
}
