package audit

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

// LogEntry represents a JSON audit log entry structure.
type LogEntry struct {
	JobID       string     `json:"job_id"`
	Timestamp   time.Time  `json:"timestamp"`
	Binary      string     `json:"binary"`
	Arguments   []string   `json:"arguments"`
	STDOUT      string     `json:"stdout"`
	STDERR      string     `json:"stderr"`
	ExitCode    int        `json:"exit_code"`
	Duration    int64      `json:"duration_ms"`
	WorkDir     string     `json:"working_directory"`
	RestAPI     *RestAPI   `json:"rest_api,omitempty"` // Optional REST API details
	OutputFiles []string   `json:"output_files,omitempty"` // Paths to output files for reference
	SLURMJobID  string     `json:"slurm_job_id,omitempty"` // SLURM job ID for grid jobs
}

// RestAPI represents REST API request/response details for audit logging.
type RestAPI struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	RequestBody  string `json:"request_body,omitempty"`
	ResponseCode int    `json:"response_code"`
	ResponseBody string `json:"response_body,omitempty"`
}

// Logger handles audit trail logging.
type Logger struct {
	enabled bool
	logFile *os.File
}

// NewLogger creates a new audit logger.
func NewLogger(enabled bool, logPath string) (*Logger, error) {
	logger := &Logger{
		enabled: enabled,
	}

	if enabled {
		// Ensure log directory exists
		dir := filepath.Dir(logPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}

		// Open log file for appending
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		logger.logFile = file
	}

	return logger, nil
}

// GenerateJobID creates a unique job ID for tracking.
func (l *Logger) GenerateJobID() string {
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

// LogExecution logs an execution event to the audit trail.
func (l *Logger) LogExecution(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string) error {
	return l.LogExecutionWithRestAPI(jobID, binary, arguments, stdout, stderr, exitCode, duration, workDir, nil)
}

// LogExecutionWithRestAPI logs an execution event with optional REST API details to the audit trail.
func (l *Logger) LogExecutionWithRestAPI(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, restAPI *RestAPI) error {
	if !l.enabled {
		return nil // Audit logging is disabled
	}

	entry := LogEntry{
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
		return fmt.Errorf("failed to marshal audit log entry: %w", err)
	}

	// Write to log file with newline (JSONL format)
	if _, err := l.logFile.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write audit log entry: %w", err)
	}

	// Flush to ensure immediate write
	if err := l.logFile.Sync(); err != nil {
		log.Printf("Warning: failed to sync audit log file: %v", err)
	}

	return nil
}

// Close closes the audit log file.
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}

	return nil
}

// IsEnabled returns whether audit logging is enabled.
func (l *Logger) IsEnabled() bool {
	return l.enabled
}

// ReadLogEntries reads all log entries from the audit log file.
func ReadLogEntries(logPath string) ([]LogEntry, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log file: %w", err)
	}

	var entries []LogEntry
	lines := string(data)

	// Split by newlines and parse each JSON line
	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal audit log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
// LogSLURMExecution logs a SLURM execution event with additional metadata to the audit trail.
func (l *Logger) LogSLURMExecution(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, slurmJobID string, outputFiles []string) error {
	return l.LogExecutionWithSLURM(jobID, binary, arguments, stdout, stderr, exitCode, duration, workDir, nil, slurmJobID, outputFiles)
}

// LogExecutionWithSLURM logs an execution event with SLURM metadata and optional REST API details to the audit trail.
func (l *Logger) LogExecutionWithSLURM(jobID, binary string, arguments []string, stdout, stderr string, exitCode int, duration time.Duration, workDir string, restAPI *RestAPI, slurmJobID string, outputFiles []string) error {
	if !l.enabled {
		return nil // Audit logging is disabled
	}

	entry := LogEntry{
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
		return fmt.Errorf("failed to marshal audit log entry: %w", err)
	}

	// Write to log file with newline (JSONL format)
	if _, err := l.logFile.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("failed to write audit log entry: %w", err)
	}

	// Flush to ensure immediate write
	if err := l.logFile.Sync(); err != nil {
		log.Printf("Warning: failed to sync audit log file: %v", err)
	}

	return nil
}
