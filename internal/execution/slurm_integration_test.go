//go:build integration
// +build integration

package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/audit"
	"github.com/pharmalytica/janus/internal/config"
)

// TestSLURMExecutorWithMockClient_FastJob tests the fast job scenario
// where SLURM writes all output immediately, testing our force final read logic
func TestSLURMExecutorWithMockClient_FastJob(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	modelPath := filepath.Join(testDir, "test_model.ctl")
	auditLogPath := filepath.Join(testDir, "audit.log")

	// Create test model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Test Model\n$DATA test.csv\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(true, auditLogPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Create mock SLURM client
	mockClient := NewMockSLURMClient(testDir)
	mockClient.SetJobDuration(500 * time.Millisecond) // Very fast job

	// Override the mock content to use immediate writing (fast job simulation)
	mockClient.SetContentGenerator(func(jobName string) MockJobContent {
		return MockJobContent{
			StdoutLines: []string{
				"NONMEM Fast Job Starting",
				"Processing model quickly...",
				"Estimation complete",
				"Final OBJ = -1000.123",
			},
			StderrLines: []string{
				"NONMEM 7.5.0 Fast Mode",
				"No warnings",
			},
			WriteMode: "immediate", // All content written at once
		}
	})

	// Create SLURM executor with mock client
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/mock/nonmem",
			NonmemBinary: "nmfe75",
		},
	}

	executor := &SLURMExecutor{
		client:      mockClient,
		config:      cfg,
		auditLogger: auditLogger,
	}

	// Execute the job
	ctx := context.Background()
	result, err := executor.Execute(ctx, modelPath, false, 1, false, []string{})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify execution result
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Verify stdout contains the fast job output
	stdoutStr := string(result.Stdout)
	expectedStdoutContent := []string{
		"NONMEM Fast Job Starting",
		"Processing model quickly...",
		"Final OBJ = -1000.123",
	}

	for _, expected := range expectedStdoutContent {
		if !strings.Contains(stdoutStr, expected) {
			t.Errorf("STDOUT missing expected content: %s\nActual STDOUT: %s", expected, stdoutStr)
		}
	}

	// Verify stderr contains expected content
	stderrStr := string(result.Stderr)
	if !strings.Contains(stderrStr, "NONMEM 7.5.0 Fast Mode") {
		t.Errorf("STDERR missing expected content. Actual: %s", stderrStr)
	}

	// Verify audit log was created and contains the job
	auditEntries, err := audit.ReadLogEntries(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if len(auditEntries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditEntries))
	}

	entry := auditEntries[0]

	// Verify audit entry contains complete output
	if !strings.Contains(entry.STDOUT, "NONMEM Fast Job Starting") {
		t.Errorf("Audit STDOUT missing expected content: %s", entry.STDOUT)
	}

	if !strings.Contains(entry.STDERR, "NONMEM 7.5.0 Fast Mode") {
		t.Errorf("Audit STDERR missing expected content: %s", entry.STDERR)
	}

	// Verify SLURM-specific audit fields
	if entry.SLURMJobID == "" {
		t.Error("Audit entry missing SLURM job ID")
	}

	if len(entry.OutputFiles) != 2 {
		t.Errorf("Expected 2 output files in audit entry, got %d", len(entry.OutputFiles))
	}

	t.Logf("Fast job test passed - Audit job ID: %s, SLURM job ID: %s", entry.JobID, entry.SLURMJobID)
}

// TestSLURMExecutorWithMockClient_SlowJob tests incremental file writing
func TestSLURMExecutorWithMockClient_SlowJob(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	modelPath := filepath.Join(testDir, "slow_model.ctl")
	auditLogPath := filepath.Join(testDir, "audit_slow.log")

	// Create test model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Slow Model\n$DATA slow.csv\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(true, auditLogPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Create mock SLURM client for slow job
	mockClient := NewMockSLURMClient(testDir)
	mockClient.SetJobDuration(2 * time.Second) // Longer running job

	// Create SLURM executor with mock client
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/mock/nonmem",
			NonmemBinary: "nmfe75",
		},
	}

	executor := &SLURMExecutor{
		client:      mockClient,
		config:      cfg,
		auditLogger: auditLogger,
	}

	// Execute the job
	ctx := context.Background()
	result, err := executor.Execute(ctx, modelPath, false, 1, false, []string{})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify execution result
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Verify output contains expected content from incremental writing
	stdoutStr := string(result.Stdout)
	expectedContent := []string{
		"Starting NONMEM execution",
		"Parameter estimation completed",
		"NONMEM execution completed",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(stdoutStr, expected) {
			t.Errorf("STDOUT missing expected content: %s", expected)
		}
	}

	// Verify audit log captures complete output despite incremental writing
	auditEntries, err := audit.ReadLogEntries(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if len(auditEntries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditEntries))
	}

	entry := auditEntries[0]

	// The key test: ensure incremental writing still results in complete audit capture
	if !strings.Contains(entry.STDOUT, "Starting NONMEM execution") ||
		!strings.Contains(entry.STDOUT, "NONMEM execution completed") {
		t.Errorf("Audit log missing complete STDOUT content from incremental job: %s", entry.STDOUT)
	}

	t.Logf("Slow job test passed - captured %d bytes of stdout in audit", len(entry.STDOUT))
}

// TestSLURMExecutorWithMockClient_StreamingOutput tests the streaming functionality
func TestSLURMExecutorWithMockClient_StreamingOutput(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	modelPath := filepath.Join(testDir, "stream_model.ctl")
	auditLogPath := filepath.Join(testDir, "audit_stream.log")

	// Create test model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Stream Model\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(true, auditLogPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Create mock SLURM client with custom slow content generation
	mockClient := NewMockSLURMClient(testDir)
	mockClient.SetJobDuration(10 * time.Second) // Much longer duration for streaming

	// Set custom content with fewer lines so each write takes longer
	mockClient.SetContentGenerator(func(jobName string) MockJobContent {
		return MockJobContent{
			StdoutLines: []string{
				"Starting NONMEM stream test",
				"Processing data slowly...",
				"Still processing...",
				"Almost done...",
				"NONMEM completed",
			},
			StderrLines: []string{
				"NONMEM version info",
				"No errors",
			},
			WriteMode: "incremental", // Write content line by line over time
		}
	})

	// Create SLURM executor
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/mock/nonmem",
			NonmemBinary: "nmfe75",
		},
	}

	executor := &SLURMExecutor{
		client:      mockClient,
		config:      cfg,
		auditLogger: auditLogger,
	}

	// Execute with streaming
	ctx := context.Background()
	streamingOutput, result, err := executor.ExecuteWithStreaming(ctx, modelPath, false, 1, false, []string{})

	if err != nil {
		t.Fatalf("ExecuteWithStreaming failed: %v", err)
	}

	// Collect streaming output
	var stdoutMessages, stderrMessages []string

	// Use timeout to avoid hanging if streaming doesn't work
	timeout := time.After(25 * time.Second)

	// Read all streaming output until Done or timeout
collectLoop:
	for {
		select {
		case msg, ok := <-streamingOutput.Stdout:
			if ok {
				stdoutMessages = append(stdoutMessages, msg)
			}
		case msg, ok := <-streamingOutput.Stderr:
			if ok {
				stderrMessages = append(stderrMessages, msg)
			}
		case <-streamingOutput.Done:
			// Drain any remaining messages after Done signal
			for {
				select {
				case msg, ok := <-streamingOutput.Stdout:
					if ok {
						stdoutMessages = append(stdoutMessages, msg)
					}
				case msg, ok := <-streamingOutput.Stderr:
					if ok {
						stderrMessages = append(stderrMessages, msg)
					}
				default:
					break collectLoop
				}
			}
		case <-timeout:
			t.Fatal("Timeout waiting for streaming output")
		}
	}

	// Verify streaming captured output
	if len(stdoutMessages) == 0 {
		t.Error("No stdout messages received from streaming")
	}

	if len(stderrMessages) == 0 {
		t.Error("No stderr messages received from streaming")
	}

	// Verify streaming messages have expected prefixes
	// Accept both [STDOUT] and [SLURM] prefixes for stdout (SLURM status messages go to stdout)
	for _, msg := range stdoutMessages {
		if !strings.Contains(msg, "[STDOUT]") && !strings.Contains(msg, "[SLURM]") {
			t.Errorf("Stdout message missing expected prefix ([STDOUT] or [SLURM]): %s", msg)
		}
	}

	for _, msg := range stderrMessages {
		if !strings.Contains(msg, "[STDERR]") {
			t.Errorf("Stderr message missing [STDERR] prefix: %s", msg)
		}
	}

	// Verify final result also contains complete output
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Verify audit log contains the streaming job
	auditEntries, err := audit.ReadLogEntries(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if len(auditEntries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditEntries))
	}

	t.Logf("Streaming test passed - received %d stdout and %d stderr messages",
		len(stdoutMessages), len(stderrMessages))

	// Small delay to ensure all goroutines and file handles are closed before cleanup
	time.Sleep(100 * time.Millisecond)
}

// TestSLURMExecutorWithMockClient_FailedJob tests job failure handling
func TestSLURMExecutorWithMockClient_FailedJob(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	modelPath := filepath.Join(testDir, "failed_model.ctl")
	auditLogPath := filepath.Join(testDir, "audit_failed.log")

	// Create test model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Failed Model\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(true, auditLogPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Create mock SLURM client that simulates failure
	mockClient := NewMockSLURMClient(testDir)
	mockClient.SetJobDuration(1 * time.Second)
	mockClient.SetSimulateFailure(true) // This will make the job fail

	// Create SLURM executor
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/mock/nonmem",
			NonmemBinary: "nmfe75",
		},
	}

	executor := &SLURMExecutor{
		client:      mockClient,
		config:      cfg,
		auditLogger: auditLogger,
	}

	// Execute the failing job
	ctx := context.Background()
	result, err := executor.Execute(ctx, modelPath, false, 1, false, []string{})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify job failed as expected
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1 for failed job, got %d", result.ExitCode)
	}

	// Verify failure output is captured
	stdoutStr := string(result.Stdout)
	stderrStr := string(result.Stderr)

	if !strings.Contains(stdoutStr, "ERROR: Parameter estimation failed") {
		t.Errorf("Failed job stdout missing error message: %s", stdoutStr)
	}

	if !strings.Contains(stderrStr, "NONMEM terminated with errors") {
		t.Errorf("Failed job stderr missing error message: %s", stderrStr)
	}

	// Verify audit log captures the failure
	auditEntries, err := audit.ReadLogEntries(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if len(auditEntries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditEntries))
	}

	entry := auditEntries[0]

	// Verify audit captures failure details
	if entry.ExitCode != 1 {
		t.Errorf("Audit entry should record exit code 1, got %d", entry.ExitCode)
	}

	if !strings.Contains(entry.STDOUT, "ERROR: Parameter estimation failed") {
		t.Errorf("Audit STDOUT missing failure message: %s", entry.STDOUT)
	}

	if !strings.Contains(entry.STDERR, "NONMEM terminated with errors") {
		t.Errorf("Audit STDERR missing failure message: %s", entry.STDERR)
	}

	t.Logf("Failed job test passed - audit captured failure with exit code %d", entry.ExitCode)
}

// TestSLURMExecutorWithMockClient_AuditTrailIntegrity tests comprehensive audit trail features
func TestSLURMExecutorWithMockClient_AuditTrailIntegrity(t *testing.T) {
	// Setup test environment
	testDir := t.TempDir()
	modelPath := filepath.Join(testDir, "audit_model.ctl")
	auditLogPath := filepath.Join(testDir, "audit_integrity.log")

	// Create test model file
	err := os.WriteFile(modelPath, []byte("$PROBLEM Audit Test Model\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(true, auditLogPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Create mock SLURM client
	mockClient := NewMockSLURMClient(testDir)
	mockClient.SetJobDuration(500 * time.Millisecond)

	// Create SLURM executor
	cfg := &config.Config{
		Input: config.Input{
			NonmemPath:   "/test/nonmem",
			NonmemBinary: "nmfe75",
		},
	}

	executor := &SLURMExecutor{
		client:      mockClient,
		config:      cfg,
		auditLogger: auditLogger,
	}

	// Execute the job
	ctx := context.Background()
	_, err = executor.Execute(ctx, modelPath, false, 1, false, []string{"-maxeval=1000"})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Read and verify comprehensive audit trail
	auditEntries, err := audit.ReadLogEntries(auditLogPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	if len(auditEntries) != 1 {
		t.Fatalf("Expected 1 audit entry, got %d", len(auditEntries))
	}

	entry := auditEntries[0]

	// Verify all required audit fields are present
	tests := []struct {
		name  string
		value string
		check func(string) bool
	}{
		{"JobID", entry.JobID, func(s string) bool { return s != "" }},
		{"Binary", entry.Binary, func(s string) bool { return s == "slurm-nonmem" }},
		{"STDOUT", entry.STDOUT, func(s string) bool { return len(s) > 0 }},
		{"STDERR", entry.STDERR, func(s string) bool { return len(s) > 0 }},
		{"SLURMJobID", entry.SLURMJobID, func(s string) bool { return strings.HasPrefix(s, "mock_") }},
		{"WorkDir", entry.WorkDir, func(s string) bool { return s == testDir }},
	}

	for _, test := range tests {
		if !test.check(test.value) {
			t.Errorf("Audit field %s failed validation: %s", test.name, test.value)
		}
	}

	// Verify arguments include model path and additional options
	if len(entry.Arguments) == 0 {
		t.Error("Audit entry missing arguments")
	}

	foundModelPath := false
	for _, arg := range entry.Arguments {
		if strings.Contains(arg, "audit_model.ctl") {
			foundModelPath = true
			break
		}
	}
	if !foundModelPath {
		t.Errorf("Audit arguments missing model path: %v", entry.Arguments)
	}

	// Verify output files are recorded
	if len(entry.OutputFiles) != 2 {
		t.Errorf("Expected 2 output files, got %d: %v", len(entry.OutputFiles), entry.OutputFiles)
	}

	// Verify duration is reasonable
	if entry.Duration < 100 { // Should be at least 100ms
		t.Errorf("Audit duration seems too short: %dms", entry.Duration)
	}

	// Verify timestamp is recent
	timeDiff := time.Since(entry.Timestamp)
	if timeDiff > time.Minute {
		t.Errorf("Audit timestamp seems too old: %v", entry.Timestamp)
	}

	t.Logf("Audit trail integrity test passed - all fields properly recorded")
	t.Logf("  Job ID: %s", entry.JobID)
	t.Logf("  SLURM Job ID: %s", entry.SLURMJobID)
	t.Logf("  Duration: %dms", entry.Duration)
	t.Logf("  Output Files: %v", entry.OutputFiles)
	t.Logf("  STDOUT Length: %d bytes", len(entry.STDOUT))
	t.Logf("  STDERR Length: %d bytes", len(entry.STDERR))
}