package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEmbedOutputFiles(t *testing.T) {
	// Create temporary directory with mock NONMEM output files
	tmpDir := t.TempDir()

	// Create a mock model file
	modelPath := filepath.Join(tmpDir, "test.mod")
	err := os.WriteFile(modelPath, []byte("$PROB Test Model\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// Create mock output files
	outputs := map[string]string{
		"test.lst": "NONMEM LIST FILE OUTPUT\nITERATION NO.:    0    OBJECTIVE VALUE:   15294.0021442315\n",
		"test.ext": "TABLE NO.  1\nITER      OBJ\n0  15294.0\n",
		"test.phi": "TABLE NO.  1: Individual Parameters\nID    ETA1\n1     0.123\n",
		"test.xml": "<nm:output><nm:problem></nm:problem></nm:output>\n",
	}

	for filename, content := range outputs {
		outputPath := filepath.Join(tmpDir, filename)
		err := os.WriteFile(outputPath, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create output file %s: %v", filename, err)
		}
	}

	// Create a run record
	record := &RunRecord{
		ID:        1,
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Status:    "completed",
	}

	// Embed the output files
	err = EmbedOutputFiles(record, modelPath)
	if err != nil {
		t.Fatalf("EmbedOutputFiles failed: %v", err)
	}

	// Verify embedded files
	expectedExtensions := []string{"mod", "lst", "ext", "phi", "xml"}
	for _, ext := range expectedExtensions {
		if _, exists := record.EmbeddedFiles[ext]; !exists {
			t.Errorf("Expected embedded file with extension %s not found", ext)
		}
	}

	// Verify we can extract the embedded content
	for ext, originalContent := range outputs {
		key := filepath.Ext(ext)[1:] // Remove leading dot
		extracted, err := ExtractEmbeddedFile(record, key)
		if err != nil {
			t.Errorf("Failed to extract %s: %v", ext, err)
			continue
		}

		if string(extracted) != originalContent {
			t.Errorf("Extracted content for %s doesn't match.\nExpected: %s\nGot: %s",
				ext, originalContent, extracted)
		}
	}
}

func TestExtractEmbeddedFile(t *testing.T) {
	record := &RunRecord{
		EmbeddedFiles: make(map[string]string),
	}

	// Manually add an embedded file
	testContent := []byte("Test LST file content")
	encoded, err := CompressAndEncode(testContent)
	if err != nil {
		t.Fatalf("Failed to encode test content: %v", err)
	}
	record.EmbeddedFiles["lst"] = encoded

	// Extract it
	extracted, err := ExtractEmbeddedFile(record, "lst")
	if err != nil {
		t.Fatalf("ExtractEmbeddedFile failed: %v", err)
	}

	if string(extracted) != string(testContent) {
		t.Errorf("Extracted content doesn't match.\nExpected: %s\nGot: %s", testContent, extracted)
	}

	// Test non-existent file
	_, err = ExtractEmbeddedFile(record, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestGetEmbeddedFileExtensions(t *testing.T) {
	record := &RunRecord{
		EmbeddedFiles: map[string]string{
			"mod": "encoded_mod",
			"lst": "encoded_lst",
			"ext": "encoded_ext",
		},
	}

	extensions := GetEmbeddedFileExtensions(record)

	if len(extensions) != 3 {
		t.Errorf("Expected 3 extensions, got %d", len(extensions))
	}

	// Verify all expected extensions are present
	expectedExts := map[string]bool{"mod": true, "lst": true, "ext": true}
	for _, ext := range extensions {
		if !expectedExts[ext] {
			t.Errorf("Unexpected extension: %s", ext)
		}
		delete(expectedExts, ext)
	}

	if len(expectedExts) > 0 {
		t.Errorf("Missing expected extensions: %v", expectedExts)
	}
}

func TestCalculateChecksum(t *testing.T) {
	data := []byte("test data")
	checksum1 := CalculateChecksum(data)

	// Same data should produce same checksum
	checksum2 := CalculateChecksum(data)
	if checksum1 != checksum2 {
		t.Errorf("Checksums don't match for same data: %s vs %s", checksum1, checksum2)
	}

	// Different data should produce different checksum
	differentData := []byte("different test data")
	checksum3 := CalculateChecksum(differentData)
	if checksum1 == checksum3 {
		t.Error("Different data produced same checksum")
	}

	// Checksum should be 64 hex characters (SHA256)
	if len(checksum1) != 64 {
		t.Errorf("Expected checksum length 64, got %d", len(checksum1))
	}
}

func TestEmbedOutputFilesWithMissingFiles(t *testing.T) {
	// Create temporary directory with only some output files
	tmpDir := t.TempDir()

	modelPath := filepath.Join(tmpDir, "test.mod")
	err := os.WriteFile(modelPath, []byte("$PROB Test\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// Only create .mod and .lst, skip other files
	lstPath := filepath.Join(tmpDir, "test.lst")
	err = os.WriteFile(lstPath, []byte("LST content\n"), 0600)
	if err != nil {
		t.Fatalf("Failed to create lst file: %v", err)
	}

	record := &RunRecord{
		ID:        1,
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Status:    "completed",
	}

	// Should not fail even if some files are missing
	err = EmbedOutputFiles(record, modelPath)
	if err != nil {
		t.Fatalf("EmbedOutputFiles should not fail for missing files: %v", err)
	}

	// Should have embedded mod and lst
	if _, exists := record.EmbeddedFiles["mod"]; !exists {
		t.Error("Expected mod file to be embedded")
	}
	if _, exists := record.EmbeddedFiles["lst"]; !exists {
		t.Error("Expected lst file to be embedded")
	}

	// Should not have ext, phi, xml
	if _, exists := record.EmbeddedFiles["ext"]; exists {
		t.Error("Did not expect ext file to be embedded")
	}
}