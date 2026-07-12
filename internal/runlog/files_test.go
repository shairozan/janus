package runlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmbedOutputFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock model (control) file.
	modelPath := filepath.Join(tmpDir, "test.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB Test Model\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// Create mock output files. sdtab1/patab1 are filename-style (no dotted
	// extension) to exercise glob matching and filename keys.
	outputs := map[string]string{
		"test.lst": "NONMEM LIST FILE OUTPUT\nITERATION NO.:    0    OBJECTIVE VALUE:   15294.0021442315\n",
		"test.ext": "TABLE NO.  1\nITER      OBJ\n0  15294.0\n",
		"test.phi": "TABLE NO.  1: Individual Parameters\nID    ETA1\n1     0.123\n",
		"test.xml": "<nm:output><nm:problem></nm:problem></nm:output>\n",
		"sdtab1":   "TABLE NO.  1\n ID DV PRED\n1 1.0 0.9\n",
		"patab1":   "TABLE NO.  1\n ID ETA1\n1 0.1\n",
	}
	for filename, content := range outputs {
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0600); err != nil {
			t.Fatalf("Failed to create output file %s: %v", filename, err)
		}
	}

	record := &RunRecord{
		ID:        "test-run-files",
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Status:    "completed",
	}

	retain := []string{"*.lst", "*.ext", "*.phi", "*.xml", "sdtab*", "patab*"}
	if err := EmbedOutputFiles(record, modelPath, retain); err != nil {
		t.Fatalf("EmbedOutputFiles failed: %v", err)
	}

	// Files are keyed by base filename; the control file is always embedded even
	// though the retain set does not include *.mod.
	expectedNames := []string{"test.mod", "test.lst", "test.ext", "test.phi", "test.xml", "sdtab1", "patab1"}
	for _, name := range expectedNames {
		if _, exists := record.EmbeddedFiles[name]; !exists {
			t.Errorf("Expected embedded file %q not found", name)
		}
	}

	// Verify we can extract each embedded file's content by filename.
	for name, originalContent := range outputs {
		extracted, err := ExtractEmbeddedFile(record, name)
		if err != nil {
			t.Errorf("Failed to extract %s: %v", name, err)

			continue
		}

		if string(extracted) != originalContent {
			t.Errorf("Extracted content for %s doesn't match.\nExpected: %s\nGot: %s",
				name, originalContent, extracted)
		}
	}
}

func TestEmbedOutputFilesAlwaysEmbedsControlFile(t *testing.T) {
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "run1.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	record := &RunRecord{ModelFile: modelPath}

	// An empty retain set — the control file must still be embedded.
	if err := EmbedOutputFiles(record, modelPath, nil); err != nil {
		t.Fatalf("EmbedOutputFiles failed: %v", err)
	}

	if _, exists := record.EmbeddedFiles["run1.mod"]; !exists {
		t.Error("control file should always be embedded, even with an empty retain set")
	}
}

func TestEmbedOutputFilesRecursiveGlob(t *testing.T) {
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "run1.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// A nested PsN-style run tree. Two files share the base name "run.lst" in
	// different subdirectories — they must not collide.
	for _, rel := range []string{"psn_janus/m1/run.lst", "psn_janus/m2/run.lst", "psn_janus/raw_results.csv"} {
		full := filepath.Join(tmpDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("data for "+rel+"\n"), 0600); err != nil {
			t.Fatalf("Failed to create %s: %v", rel, err)
		}
	}

	record := &RunRecord{ModelFile: modelPath}
	if err := EmbedOutputFiles(record, modelPath, []string{"psn_janus/**"}); err != nil {
		t.Fatalf("EmbedOutputFiles failed: %v", err)
	}

	// Recursive matches are embedded under their relative (forward-slashed) path,
	// so the two same-named files coexist.
	for _, key := range []string{"psn_janus/m1/run.lst", "psn_janus/m2/run.lst", "psn_janus/raw_results.csv"} {
		if _, exists := record.EmbeddedFiles[key]; !exists {
			t.Errorf("expected embedded file %q from recursive glob", key)
		}
	}

	// The content is keyed correctly (no last-write-wins collision).
	got, err := ExtractEmbeddedFile(record, "psn_janus/m2/run.lst")
	if err != nil {
		t.Fatalf("ExtractEmbeddedFile failed: %v", err)
	}
	if string(got) != "data for psn_janus/m2/run.lst\n" {
		t.Errorf("unexpected content for m2/run.lst: %q", got)
	}
}

func TestEmbedOutputFilesSkipsOversizedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "big.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// A .lst larger than MaxEmbeddedFileSize must be skipped.
	bigPath := filepath.Join(tmpDir, "big.lst")
	if err := os.WriteFile(bigPath, []byte(strings.Repeat("x", MaxEmbeddedFileSize+1)), 0600); err != nil {
		t.Fatalf("Failed to create big lst file: %v", err)
	}

	record := &RunRecord{ModelFile: modelPath}
	if err := EmbedOutputFiles(record, modelPath, []string{"*.lst"}); err != nil {
		t.Fatalf("EmbedOutputFiles failed: %v", err)
	}

	if _, exists := record.EmbeddedFiles["big.lst"]; exists {
		t.Error("oversized file should not be embedded")
	}
}

func TestEmbedOutputFilesInvalidGlob(t *testing.T) {
	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "test.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	record := &RunRecord{ModelFile: modelPath}
	if err := EmbedOutputFiles(record, modelPath, []string{"[bad"}); err == nil {
		t.Error("expected an error for a malformed retain glob")
	}
}

func TestExtractEmbeddedFile(t *testing.T) {
	record := &RunRecord{
		EmbeddedFiles: make(map[string]string),
	}

	testContent := []byte("Test LST file content")
	encoded, err := CompressAndEncode(testContent)
	if err != nil {
		t.Fatalf("Failed to encode test content: %v", err)
	}
	record.EmbeddedFiles["run1.lst"] = encoded

	extracted, err := ExtractEmbeddedFile(record, "run1.lst")
	if err != nil {
		t.Fatalf("ExtractEmbeddedFile failed: %v", err)
	}

	if string(extracted) != string(testContent) {
		t.Errorf("Extracted content doesn't match.\nExpected: %s\nGot: %s", testContent, extracted)
	}

	// Test non-existent file
	if _, err = ExtractEmbeddedFile(record, "nonexistent"); err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestGetEmbeddedFileNames(t *testing.T) {
	record := &RunRecord{
		EmbeddedFiles: map[string]string{
			"run1.mod": "encoded_mod",
			"run1.lst": "encoded_lst",
			"sdtab1":   "encoded_sdtab",
		},
	}

	names := GetEmbeddedFileNames(record)
	if len(names) != 3 {
		t.Errorf("Expected 3 names, got %d", len(names))
	}

	expected := map[string]bool{"run1.mod": true, "run1.lst": true, "sdtab1": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("Unexpected name: %s", name)
		}
		delete(expected, name)
	}

	if len(expected) > 0 {
		t.Errorf("Missing expected names: %v", expected)
	}
}

func TestEmbeddedNameForExt(t *testing.T) {
	record := &RunRecord{
		EmbeddedFiles: map[string]string{
			"run1.mod": "encoded_mod",
			"run1.lst": "encoded_lst",
			"sdtab1":   "encoded_sdtab",
		},
	}

	if got := EmbeddedNameForExt(record, "lst"); got != "run1.lst" {
		t.Errorf("EmbeddedNameForExt(lst) = %q, want run1.lst", got)
	}

	// With or without a leading dot.
	if got := EmbeddedNameForExt(record, ".mod"); got != "run1.mod" {
		t.Errorf("EmbeddedNameForExt(.mod) = %q, want run1.mod", got)
	}

	// A file with no dotted extension does not match an extension query.
	if got := EmbeddedNameForExt(record, "ext"); got != "" {
		t.Errorf("EmbeddedNameForExt(ext) = %q, want empty", got)
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
	tmpDir := t.TempDir()

	modelPath := filepath.Join(tmpDir, "test.mod")
	if err := os.WriteFile(modelPath, []byte("$PROB Test\n"), 0600); err != nil {
		t.Fatalf("Failed to create model file: %v", err)
	}

	// Only create .lst; the other retained files are absent.
	lstPath := filepath.Join(tmpDir, "test.lst")
	if err := os.WriteFile(lstPath, []byte("LST content\n"), 0600); err != nil {
		t.Fatalf("Failed to create lst file: %v", err)
	}

	record := &RunRecord{
		ID:        "test-run-partial",
		Timestamp: time.Now(),
		ModelFile: modelPath,
		Status:    "completed",
	}

	// Should not fail even when some retained files are missing.
	if err := EmbedOutputFiles(record, modelPath, []string{"*.lst", "*.ext", "*.phi", "*.xml"}); err != nil {
		t.Fatalf("EmbedOutputFiles should not fail for missing files: %v", err)
	}

	// Should have embedded the control file and the lst.
	if _, exists := record.EmbeddedFiles["test.mod"]; !exists {
		t.Error("Expected control file to be embedded")
	}
	if _, exists := record.EmbeddedFiles["test.lst"]; !exists {
		t.Error("Expected lst file to be embedded")
	}

	// Should not have ext, phi, xml (they were never written).
	if _, exists := record.EmbeddedFiles["test.ext"]; exists {
		t.Error("Did not expect ext file to be embedded")
	}
}
