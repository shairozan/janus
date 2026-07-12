package runlog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/runlog"
)

// TestRealWorldCompression tests compression with actual NONMEM output files.
func TestRealWorldCompression(t *testing.T) {
	// Check if testdata files exist
	testdataDir := filepath.Join("..", "..", "testdata")
	modelPath := filepath.Join(testdataDir, "acop.mod")

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		t.Skip("Testdata files not available")
	}

	// Create a run record
	record := &runlog.RunRecord{
		ID:        "test-integration-run",
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Command:   "/opt/NONMEM/nm75/run/nmfe75 acop.mod",
		ExitCode:  0,
		Status:    "completed",
	}

	// Embed output files (default NONMEM retain set plus the always-embedded
	// control file).
	retain := []string{"*.lst", "*.ext", "*.phi", "*.xml"}
	err := runlog.EmbedOutputFiles(record, modelPath, retain)
	if err != nil {
		t.Fatalf("Failed to embed output files: %v", err)
	}

	// Check which files were embedded
	names := runlog.GetEmbeddedFileNames(record)
	t.Logf("Embedded files: %v", names)

	// Verify we can extract each file
	for _, name := range names {
		data, err := runlog.ExtractEmbeddedFile(record, name)
		if err != nil {
			t.Errorf("Failed to extract %s: %v", name, err)
			continue
		}
		t.Logf("Extracted %s: %d bytes", name, len(data))
	}

	// Serialize to JSON and check size
	jsonData, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal record: %v", err)
	}

	t.Logf("JSON size: %d bytes", len(jsonData))

	// Calculate total original size
	var totalOriginal int64
	for _, ext := range []string{".mod", ".lst", ".ext", ".phi", ".xml"} {
		path := filepath.Join(testdataDir, "acop"+ext)
		if info, err := os.Stat(path); err == nil {
			totalOriginal += info.Size()
		}
	}

	compressionRatio := float64(len(jsonData)) / float64(totalOriginal)
	t.Logf("Original total: %d bytes", totalOriginal)
	t.Logf("Compressed (JSON): %d bytes", len(jsonData))
	t.Logf("Compression ratio: %.2f%%", compressionRatio*100)

	// Verify we can round-trip through JSON
	var decoded runlog.RunRecord
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal record: %v", err)
	}

	// Verify decoded data matches
	if decoded.ID != record.ID {
		t.Errorf("ID mismatch after JSON round-trip")
	}
	if len(decoded.EmbeddedFiles) != len(record.EmbeddedFiles) {
		t.Errorf("EmbeddedFiles count mismatch: got %d, want %d",
			len(decoded.EmbeddedFiles), len(record.EmbeddedFiles))
	}
}
