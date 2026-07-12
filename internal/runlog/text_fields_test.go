package runlog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetGetStdout(t *testing.T) {
	record := &RunRecord{}

	// Test with typical NONMEM output
	stdout := strings.Repeat("ITERATION NO.:    0    OBJECTIVE VALUE:   15294.0021442315\n", 100)

	err := record.SetStdout(stdout)
	if err != nil {
		t.Fatalf("SetStdout failed: %v", err)
	}

	// Verify it was compressed
	if record.StdoutCompressed == "" {
		t.Error("StdoutCompressed should not be empty")
	}
	if record.Stdout != "" {
		t.Error("Legacy Stdout field should be cleared")
	}

	// Verify we can retrieve it
	retrieved, err := record.GetStdout()
	if err != nil {
		t.Fatalf("GetStdout failed: %v", err)
	}

	if retrieved != stdout {
		t.Error("Retrieved stdout doesn't match original")
	}
}

func TestSetGetStderr(t *testing.T) {
	record := &RunRecord{}

	stderr := "Error: Failed to converge\nWarning: Boundary condition reached\n"

	err := record.SetStderr(stderr)
	if err != nil {
		t.Fatalf("SetStderr failed: %v", err)
	}

	// Verify compression
	if record.StderrCompressed == "" {
		t.Error("StderrCompressed should not be empty")
	}

	retrieved, err := record.GetStderr()
	if err != nil {
		t.Fatalf("GetStderr failed: %v", err)
	}

	if retrieved != stderr {
		t.Error("Retrieved stderr doesn't match original")
	}
}

func TestSetGetDescription(t *testing.T) {
	record := &RunRecord{}

	description := "Initial run with updated dataset. Testing new covariate model."

	err := record.SetDescription(description)
	if err != nil {
		t.Fatalf("SetDescription failed: %v", err)
	}

	retrieved, err := record.GetDescription()
	if err != nil {
		t.Fatalf("GetDescription failed: %v", err)
	}

	if retrieved != description {
		t.Error("Retrieved description doesn't match original")
	}
}

func TestEmptyStringsNotCompressed(t *testing.T) {
	record := &RunRecord{}

	// Empty strings should not be compressed
	err := record.SetStdout("")
	if err != nil {
		t.Fatalf("SetStdout with empty string failed: %v", err)
	}

	if record.StdoutCompressed != "" {
		t.Error("Empty stdout should not be compressed")
	}

	retrieved, err := record.GetStdout()
	if err != nil {
		t.Fatalf("GetStdout failed: %v", err)
	}

	if retrieved != "" {
		t.Error("Retrieved stdout should be empty")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that old records with uncompressed fields still work
	record := &RunRecord{
		Stdout:      "Old stdout data",
		Stderr:      "Old stderr data",
		Description: "Old description",
	}

	// Should fall back to legacy fields
	stdout, err := record.GetStdout()
	if err != nil {
		t.Fatalf("GetStdout failed: %v", err)
	}
	if stdout != "Old stdout data" {
		t.Error("Failed to retrieve legacy stdout")
	}

	stderr, err := record.GetStderr()
	if err != nil {
		t.Fatalf("GetStderr failed: %v", err)
	}
	if stderr != "Old stderr data" {
		t.Error("Failed to retrieve legacy stderr")
	}

	desc, err := record.GetDescription()
	if err != nil {
		t.Fatalf("GetDescription failed: %v", err)
	}
	if desc != "Old description" {
		t.Error("Failed to retrieve legacy description")
	}
}

func TestCompressionSavings(t *testing.T) {
	record := &RunRecord{
		ID: "test-compression",
	}

	// Large repetitive NONMEM output
	stdout := strings.Repeat("ITERATION NO.:    0    OBJECTIVE VALUE:   15294.0021442315        NO. OF FUNC. EVALS.:   7\n", 200)

	err := record.SetStdout(stdout)
	if err != nil {
		t.Fatalf("SetStdout failed: %v", err)
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	originalSize := len(stdout)
	compressedSize := len(jsonData)
	savings := 100.0 - (float64(compressedSize) / float64(originalSize) * 100.0)

	t.Logf("Original stdout size: %d bytes", originalSize)
	t.Logf("Compressed record JSON size: %d bytes", compressedSize)
	t.Logf("Storage savings: %.1f%%", savings)

	// Should achieve significant compression
	if savings < 50.0 {
		t.Errorf("Expected at least 50%% compression, got %.1f%%", savings)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := &RunRecord{
		ID: "test-json-roundtrip",
	}

	testData := "NONMEM output with repetitive data\n" + strings.Repeat("x", 1000)

	err := original.SetStdout(testData)
	if err != nil {
		t.Fatalf("SetStdout failed: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded RunRecord
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify data survived round-trip
	retrieved, err := decoded.GetStdout()
	if err != nil {
		t.Fatalf("GetStdout failed: %v", err)
	}

	if retrieved != testData {
		t.Error("Data didn't survive JSON round-trip")
	}
}
