package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/audit"
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
	record := &audit.RunRecord{
		ID:        1,
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Command:   "/opt/NONMEM/nm75/run/nmfe75 acop.mod",
		ExitCode:  0,
		Status:    "completed",
	}

	// Embed output files
	err := audit.EmbedOutputFiles(record, modelPath)
	if err != nil {
		t.Fatalf("Failed to embed output files: %v", err)
	}

	// Check which files were embedded
	extensions := audit.GetEmbeddedFileExtensions(record)
	t.Logf("Embedded files: %v", extensions)

	// Verify we can extract each file
	for _, ext := range extensions {
		data, err := audit.ExtractEmbeddedFile(record, ext)
		if err != nil {
			t.Errorf("Failed to extract %s: %v", ext, err)
			continue
		}
		t.Logf("Extracted %s: %d bytes", ext, len(data))
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
	var decoded audit.RunRecord
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