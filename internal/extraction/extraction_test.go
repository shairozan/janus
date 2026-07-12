package extraction

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pharmalytica/janus/internal/runlog"
)

func TestExtractSummary_NilRecord(t *testing.T) {
	_, err := ExtractSummary(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil record")
	}
}

func TestExtractSummary_NoEmbeddedFiles(t *testing.T) {
	record := &runlog.RunRecord{
		ID:        "test-1",
		ModelFile: "test.mod",
	}

	summary, err := ExtractSummary(context.Background(), record)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Error("expected nil summary for record without embedded files")
	}
}

func TestExtractSummary_EmptyEmbeddedFiles(t *testing.T) {
	record := &runlog.RunRecord{
		ID:            "test-1",
		ModelFile:     "test.mod",
		EmbeddedFiles: make(map[string]string),
	}

	summary, err := ExtractSummary(context.Background(), record)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Error("expected nil summary for record with empty embedded files")
	}
}

func TestExtractSummary_WithRealFiles(t *testing.T) {
	// Find testdata directory relative to this test file
	testdataDir := filepath.Join("..", "..", "testdata")

	extPath := filepath.Join(testdataDir, "acop.ext")
	lstPath := filepath.Join(testdataDir, "acop.lst")

	// Skip if test data not available
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.ext not found, skipping integration test")
	}
	if _, err := os.Stat(lstPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.lst not found, skipping integration test")
	}

	// Read and compress the test files
	extData, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("failed to read ext file: %v", err)
	}

	lstData, err := os.ReadFile(lstPath)
	if err != nil {
		t.Fatalf("failed to read lst file: %v", err)
	}

	extEncoded, err := runlog.CompressAndEncode(extData)
	if err != nil {
		t.Fatalf("failed to compress ext file: %v", err)
	}

	lstEncoded, err := runlog.CompressAndEncode(lstData)
	if err != nil {
		t.Fatalf("failed to compress lst file: %v", err)
	}

	// Create a run record with embedded files
	record := &runlog.RunRecord{
		ID:        "test-extraction",
		ModelFile: filepath.Join(testdataDir, "acop.mod"),
		EmbeddedFiles: map[string]string{
			"acop.ext": extEncoded,
			"acop.lst": lstEncoded,
		},
	}

	// Extract summary
	summary, err := ExtractSummary(context.Background(), record)
	if err != nil {
		t.Fatalf("ExtractSummary failed: %v", err)
	}

	if summary == nil {
		t.Fatal("expected non-nil summary")
	}

	// Verify OFV was extracted (from acop.ext, final line shows ~2636.86)
	if summary.GoodnessOfFit.ObjectiveFunctionValue == nil {
		t.Error("expected OFV to be extracted")
	} else {
		ofv := *summary.GoodnessOfFit.ObjectiveFunctionValue
		if ofv < 2636 || ofv > 2637 {
			t.Errorf("unexpected OFV value: got %f, expected ~2636.86", ofv)
		}
	}

	// Verify THETAs were extracted (acop has 5 THETAs)
	if len(summary.Parameters.Thetas) == 0 {
		t.Error("expected THETAs to be extracted")
	}

	// Verify OMEGAs were extracted (acop has 2 diagonal OMEGAs)
	if len(summary.Parameters.Omegas) == 0 {
		t.Error("expected OMEGAs to be extracted")
	}

	// Verify estimation info was parsed
	if summary.Estimation.Subjects == 0 {
		t.Error("expected subjects count to be extracted")
	}

	if summary.Estimation.Observations == 0 {
		t.Error("expected observations count to be extracted")
	}

	t.Logf("Extracted summary: OFV=%.2f, THETAs=%d, OMEGAs=%d, Subjects=%d, Obs=%d",
		*summary.GoodnessOfFit.ObjectiveFunctionValue,
		len(summary.Parameters.Thetas),
		len(summary.Parameters.Omegas),
		summary.Estimation.Subjects,
		summary.Estimation.Observations)
}

func TestExtractSummary_WithLabels(t *testing.T) {
	// Find testdata directory relative to this test file
	testdataDir := filepath.Join("..", "..", "testdata")

	extPath := filepath.Join(testdataDir, "acop.ext")
	lstPath := filepath.Join(testdataDir, "acop.lst")
	modPath := filepath.Join(testdataDir, "acop.mod")

	// Skip if test data not available
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.ext not found, skipping integration test")
	}
	if _, err := os.Stat(lstPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.lst not found, skipping integration test")
	}
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.mod not found, skipping integration test")
	}

	// Read and compress the test files
	extData, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("failed to read ext file: %v", err)
	}
	lstData, err := os.ReadFile(lstPath)
	if err != nil {
		t.Fatalf("failed to read lst file: %v", err)
	}
	modData, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("failed to read mod file: %v", err)
	}

	extEncoded, _ := runlog.CompressAndEncode(extData)
	lstEncoded, _ := runlog.CompressAndEncode(lstData)
	modEncoded, _ := runlog.CompressAndEncode(modData)

	// Create a run record with all embedded files including .mod
	record := &runlog.RunRecord{
		ID:        "test-labels",
		ModelFile: modPath,
		EmbeddedFiles: map[string]string{
			"acop.ext": extEncoded,
			"acop.lst": lstEncoded,
			"acop.mod": modEncoded,
		},
	}

	// Extract summary
	summary, err := ExtractSummary(context.Background(), record)
	if err != nil {
		t.Fatalf("ExtractSummary failed: %v", err)
	}

	if summary == nil {
		t.Fatal("expected non-nil summary")
	}

	// Verify THETAs have labels from acop.mod:
	// $THETA
	// (0, 2)  ; KA
	// (0, 3)  ; CL
	// (0, 10) ; V2
	// (0.02)  ; RUVp
	// (1)     ; RUVa
	if len(summary.Parameters.Thetas) < 5 {
		t.Fatalf("expected at least 5 THETAs, got %d", len(summary.Parameters.Thetas))
	}

	expectedLabels := []string{"KA", "CL", "V2", "RUVp", "RUVa"}
	for i, expected := range expectedLabels {
		actual := summary.Parameters.Thetas[i].Label
		if actual != expected {
			t.Errorf("THETA%d: expected label %q, got %q", i+1, expected, actual)
		}
	}

	// Verify OMEGAs have labels
	// $OMEGA
	// 0.05    ; iiv CL
	// 0.2     ; iiv V2
	if len(summary.Parameters.Omegas) < 2 {
		t.Fatalf("expected at least 2 OMEGAs, got %d", len(summary.Parameters.Omegas))
	}

	expectedOmegaLabels := []string{"iiv CL", "iiv V2"}
	for i, expected := range expectedOmegaLabels {
		actual := summary.Parameters.Omegas[i].Label
		if actual != expected {
			t.Errorf("OMEGA(%d,%d): expected label %q, got %q", i+1, i+1, expected, actual)
		}
	}

	t.Logf("Successfully extracted labels: THETAs have %v", expectedLabels)
}

func TestExtractSummary_ExtFileOnly(t *testing.T) {
	// Find testdata directory
	testdataDir := filepath.Join("..", "..", "testdata")
	extPath := filepath.Join(testdataDir, "acop.ext")

	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		t.Skip("testdata/acop.ext not found, skipping test")
	}

	extData, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatalf("failed to read ext file: %v", err)
	}

	extEncoded, err := runlog.CompressAndEncode(extData)
	if err != nil {
		t.Fatalf("failed to compress ext file: %v", err)
	}

	record := &runlog.RunRecord{
		ID:        "test-ext-only",
		ModelFile: filepath.Join(testdataDir, "acop.mod"),
		EmbeddedFiles: map[string]string{
			"acop.ext": extEncoded,
		},
	}

	summary, err := ExtractSummary(context.Background(), record)
	if err != nil {
		t.Fatalf("ExtractSummary failed: %v", err)
	}

	if summary == nil {
		t.Fatal("expected non-nil summary even with only .ext file")
	}

	// Should still have parameters from .ext
	if len(summary.Parameters.Thetas) == 0 {
		t.Error("expected THETAs to be extracted from .ext file")
	}
}

func TestHasExtractableFiles(t *testing.T) {
	tests := []struct {
		name     string
		record   *runlog.RunRecord
		expected bool
	}{
		{
			name:     "nil record",
			record:   nil,
			expected: false,
		},
		{
			name: "no embedded files",
			record: &runlog.RunRecord{
				ID: "test",
			},
			expected: false,
		},
		{
			name: "empty embedded files",
			record: &runlog.RunRecord{
				ID:            "test",
				EmbeddedFiles: map[string]string{},
			},
			expected: false,
		},
		{
			name: "has ext file",
			record: &runlog.RunRecord{
				ID:            "test",
				EmbeddedFiles: map[string]string{"run1.ext": "data"},
			},
			expected: true,
		},
		{
			name: "has lst file",
			record: &runlog.RunRecord{
				ID:            "test",
				EmbeddedFiles: map[string]string{"run1.lst": "data"},
			},
			expected: true,
		},
		{
			name: "has both files",
			record: &runlog.RunRecord{
				ID:            "test",
				EmbeddedFiles: map[string]string{"run1.ext": "data", "run1.lst": "data"},
			},
			expected: true,
		},
		{
			name: "has only other files",
			record: &runlog.RunRecord{
				ID:            "test",
				EmbeddedFiles: map[string]string{"run1.xml": "data", "run1.phi": "data"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasExtractableFiles(tt.record)
			if result != tt.expected {
				t.Errorf("HasExtractableFiles() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
