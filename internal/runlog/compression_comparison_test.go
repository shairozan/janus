package runlog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shairozan/janus/internal/runlog"
)

// TestFullCompressionComparison compares compressed vs uncompressed audit trail sizes.
func TestFullCompressionComparison(t *testing.T) {
	// Simulate typical NONMEM stdout with lots of iterations
	stdout := ""
	for i := 0; i < 100; i++ {
		stdout += "0ITERATION NO.:    " + strings.Repeat(" ", 4) + "OBJECTIVE VALUE:   15294.0021442315        NO. OF FUNC. EVALS.:   7\n"
		stdout += " CUMULATIVE NO. OF FUNC. EVALS.:        7\n"
		stdout += " NPARAMETR:  2.0000E+00  3.0000E+00  1.0000E+01  2.0000E-02  1.0000E+00  5.0000E-02  2.0000E-01\n"
		stdout += " PARAMETER:  1.0000E-01  1.0000E-01  1.0000E-01  1.0000E-01  1.0000E-01  1.0000E-01  1.0000E-01\n"
		stdout += " GRADIENT:  -1.7140E+03 -3.1290E+03 -1.5123E+03 -9.7238E+03 -1.2685E+05 -8.1552E+03 -5.9227E+03\n"
	}

	stderr := "Warning: Boundary condition reached\nWarning: Rounding errors detected\n"
	description := "Testing new covariate model with updated dataset. This is run 42 of the bootstrap sequence."

	// Create uncompressed record (legacy style)
	uncompressedRecord := &runlog.RunRecord{
		ID:          "test-uncompressed",
		Timestamp:   time.Now(),
		ModelFile:   "/path/to/model.mod",
		Command:     "/opt/NONMEM/nm75/run/nmfe75 model.mod",
		ExitCode:    0,
		Stdout:      stdout,
		Stderr:      stderr,
		Description: description,
		IsParallel:  true,
		Cores:       4,
		IsGrid:      false,
		Status:      "completed",
	}

	// Create compressed record (new style)
	compressedRecord := &runlog.RunRecord{
		ID:         "test-compressed",
		Timestamp:  time.Now(),
		ModelFile:  "/path/to/model.mod",
		Command:    "/opt/NONMEM/nm75/run/nmfe75 model.mod",
		ExitCode:   0,
		IsParallel: true,
		Cores:      4,
		IsGrid:     false,
		Status:     "completed",
	}

	// Use compression methods
	if err := compressedRecord.SetStdout(stdout); err != nil {
		t.Fatalf("Failed to set stdout: %v", err)
	}
	if err := compressedRecord.SetStderr(stderr); err != nil {
		t.Fatalf("Failed to set stderr: %v", err)
	}
	if err := compressedRecord.SetDescription(description); err != nil {
		t.Fatalf("Failed to set description: %v", err)
	}

	// Serialize both to JSON
	uncompressedJSON, err := json.Marshal(uncompressedRecord)
	if err != nil {
		t.Fatalf("Failed to marshal uncompressed: %v", err)
	}

	compressedJSON, err := json.Marshal(compressedRecord)
	if err != nil {
		t.Fatalf("Failed to marshal compressed: %v", err)
	}

	uncompressedSize := len(uncompressedJSON)
	compressedSize := len(compressedJSON)
	savingsPercent := 100.0 - (float64(compressedSize) / float64(uncompressedSize) * 100.0)

	t.Logf("Uncompressed JSON size: %d bytes (%.2f KB)", uncompressedSize, float64(uncompressedSize)/1024.0)
	t.Logf("Compressed JSON size:   %d bytes (%.2f KB)", compressedSize, float64(compressedSize)/1024.0)
	t.Logf("Storage savings: %.1f%%", savingsPercent)

	// Verify we can decompress back
	retrieved, err := compressedRecord.GetStdout()
	if err != nil {
		t.Fatalf("Failed to retrieve stdout: %v", err)
	}
	if retrieved != stdout {
		t.Error("Decompressed stdout doesn't match original")
	}
}

// TestRealWorldFullRecord tests compression with real NONMEM files + stdout/stderr.
func TestRealWorldFullRecord(t *testing.T) {
	// Check if testdata files exist
	testdataDir := filepath.Join("..", "..", "testdata")
	modelPath := filepath.Join(testdataDir, "acop.mod")
	lstPath := filepath.Join(testdataDir, "acop.lst")

	if _, err := os.Stat(lstPath); os.IsNotExist(err) {
		t.Skip("Testdata files not available")
	}

	// Read the actual .lst file for realistic stdout
	lstContent, err := os.ReadFile(lstPath)
	if err != nil {
		t.Fatalf("Failed to read lst file: %v", err)
	}

	// Create a full record with everything
	record := &runlog.RunRecord{
		ID:         "test-full-record",
		Timestamp:  time.Now(),
		ModelFile:  modelPath,
		Command:    "/opt/NONMEM/nm75/run/nmfe75 acop.mod",
		ExitCode:   0,
		IsParallel: false,
		Cores:      1,
		IsGrid:     false,
		Status:     "completed",
	}

	// Compress all text fields
	if err := record.SetStdout(string(lstContent)); err != nil {
		t.Fatalf("Failed to compress stdout: %v", err)
	}
	if err := record.SetStderr(""); err != nil {
		t.Fatalf("Failed to compress stderr: %v", err)
	}
	if err := record.SetDescription("Real NONMEM run from testdata"); err != nil {
		t.Fatalf("Failed to compress description: %v", err)
	}

	// Embed output files
	if err := runlog.EmbedOutputFiles(record, modelPath, []string{"*.lst", "*.ext", "*.phi", "*.xml"}); err != nil {
		t.Fatalf("Failed to embed files: %v", err)
	}

	// Serialize to JSON
	jsonData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Calculate total original size (stdout + files)
	var totalOriginal int64
	totalOriginal += int64(len(lstContent)) // stdout

	for _, ext := range []string{".mod", ".lst", ".ext", ".phi", ".xml"} {
		path := filepath.Join(testdataDir, "acop"+ext)
		if info, err := os.Stat(path); err == nil {
			totalOriginal += info.Size()
		}
	}

	jsonSize := int64(len(jsonData))
	savingsPercent := 100.0 - (float64(jsonSize) / float64(totalOriginal) * 100.0)

	t.Logf("Original data total:    %d bytes (%.2f KB)", totalOriginal, float64(totalOriginal)/1024.0)
	t.Logf("Compressed JSON total:  %d bytes (%.2f KB)", jsonSize, float64(jsonSize)/1024.0)
	t.Logf("Overall storage savings: %.1f%%", savingsPercent)

	// Verify round-trip
	var decoded runlog.RunRecord
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify stdout
	retrievedStdout, err := decoded.GetStdout()
	if err != nil {
		t.Fatalf("Failed to retrieve stdout: %v", err)
	}
	if retrievedStdout != string(lstContent) {
		t.Error("Stdout didn't survive round-trip")
	}

	// Verify embedded files
	names := runlog.GetEmbeddedFileNames(&decoded)
	if len(names) == 0 {
		t.Error("No embedded files after round-trip")
	}
	t.Logf("Successfully round-tripped %d embedded files", len(names))
}
