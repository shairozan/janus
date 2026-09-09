package extraction

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shairozan/janus/internal/model"
	"github.com/shairozan/janus/internal/runlog"
)

func TestExtractTableDiagnostics_FromDirectory(t *testing.T) {
	// Create temp directory with test tables
	tempDir, err := os.MkdirTemp("", "table-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a minimal SDTAB file
	sdtabContent := `TABLE NO.  1
 ID          TIME        DV          PRED        IPRED       CWRES
  1.0000E+00  0.0000E+00  5.0000E+00  4.8000E+00  4.9000E+00  1.5000E-01
  1.0000E+00  1.0000E+00  1.0000E+01  9.5000E+00  9.8000E+00  3.2000E-01
  2.0000E+00  0.0000E+00  1.5000E+01  1.4200E+01  1.4800E+01 -1.2000E-01`
	if err := os.WriteFile(filepath.Join(tempDir, "sdtab001"), []byte(sdtabContent), 0644); err != nil {
		t.Fatalf("failed to write sdtab: %v", err)
	}

	// Create a minimal PATAB file
	patabContent := `TABLE NO.  1
 ID          ETA1        ETA2        CL          V
  1.0000E+00  2.3456E-01 -1.2345E-01  1.5200E+01  8.5300E+01
  2.0000E+00 -1.8765E-01  5.6789E-02  1.4800E+01  8.2100E+01`
	if err := os.WriteFile(filepath.Join(tempDir, "patab001"), []byte(patabContent), 0644); err != nil {
		t.Fatalf("failed to write patab: %v", err)
	}

	record := &runlog.RunRecord{
		ID:        "test-run",
		ModelFile: "test.mod",
	}

	diagnostics, err := ExtractTableDiagnostics(record, tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diagnostics == nil {
		t.Fatal("expected non-nil diagnostics")
	}

	// Check available tables
	if len(diagnostics.AvailableTables) != 2 {
		t.Errorf("expected 2 available tables, got %d", len(diagnostics.AvailableTables))
	}

	// Check GOF data was extracted
	if diagnostics.GOFData == nil {
		t.Error("expected GOF data to be extracted")
	} else {
		if diagnostics.GOFData.N != 3 {
			t.Errorf("expected 3 observations, got %d", diagnostics.GOFData.N)
		}
		if len(diagnostics.GOFData.DV) != 3 {
			t.Errorf("expected 3 DV values, got %d", len(diagnostics.GOFData.DV))
		}
		if len(diagnostics.GOFData.CWRES) != 3 {
			t.Errorf("expected 3 CWRES values, got %d", len(diagnostics.GOFData.CWRES))
		}
	}

	// Check ETA data was extracted
	if len(diagnostics.ETAData) != 2 {
		t.Errorf("expected 2 ETA sets, got %d", len(diagnostics.ETAData))
	} else {
		// Check ETA1
		eta1 := diagnostics.ETAData[0]
		if eta1.Name != "ETA1" {
			t.Errorf("expected first ETA name = ETA1, got %s", eta1.Name)
		}
		if eta1.N != 2 {
			t.Errorf("expected 2 subjects for ETA1, got %d", eta1.N)
		}
	}
}

func TestExtractTableDiagnostics_NilRecord(t *testing.T) {
	_, err := ExtractTableDiagnostics(nil, "")
	if err == nil {
		t.Error("expected error for nil record")
	}
}

func TestExtractTableDiagnostics_NoTables(t *testing.T) {
	// Create empty temp directory
	tempDir, err := os.MkdirTemp("", "empty-table-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	record := &runlog.RunRecord{
		ID:        "test-run",
		ModelFile: "test.mod",
	}

	diagnostics, err := ExtractTableDiagnostics(record, tempDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diagnostics == nil {
		t.Fatal("expected non-nil diagnostics even with no tables")
	}

	if len(diagnostics.AvailableTables) != 0 {
		t.Errorf("expected 0 available tables, got %d", len(diagnostics.AvailableTables))
	}
}

func TestHasTableFiles(t *testing.T) {
	// Create temp directory with a table
	tempDir, err := os.MkdirTemp("", "has-table-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initially should have no tables
	if HasTableFiles(tempDir) {
		t.Error("expected no tables in empty directory")
	}

	// Add a table file
	sdtabContent := `TABLE NO.  1
 ID          TIME        DV
  1.0000E+00  0.0000E+00  5.0000E+00`
	if err := os.WriteFile(filepath.Join(tempDir, "sdtab001"), []byte(sdtabContent), 0644); err != nil {
		t.Fatalf("failed to write sdtab: %v", err)
	}

	if !HasTableFiles(tempDir) {
		t.Error("expected tables to be found")
	}
}

func TestGetAvailableTableTypes(t *testing.T) {
	// Create temp directory with multiple table types
	tempDir, err := os.MkdirTemp("", "table-types-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sdtabContent := `TABLE NO.  1
 ID          DV
  1.0000E+00  5.0000E+00`
	patabContent := `TABLE NO.  1
 ID          ETA1
  1.0000E+00  0.1000E+00`

	if err := os.WriteFile(filepath.Join(tempDir, "sdtab001"), []byte(sdtabContent), 0644); err != nil {
		t.Fatalf("failed to write sdtab: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "patab001"), []byte(patabContent), 0644); err != nil {
		t.Fatalf("failed to write patab: %v", err)
	}

	types := GetAvailableTableTypes(tempDir)
	if len(types) != 2 {
		t.Errorf("expected 2 table types, got %d", len(types))
	}

	// Check for expected types
	hasSDTAB := false
	hasPATAB := false
	for _, tt := range types {
		if tt == "sdtab" {
			hasSDTAB = true
		}
		if tt == "patab" {
			hasPATAB = true
		}
	}

	if !hasSDTAB {
		t.Error("expected sdtab type")
	}
	if !hasPATAB {
		t.Error("expected patab type")
	}
}

// TestExtractTableDiagnostics_FromEmbedded verifies that filename-keyed embedded
// tables (sdtab1/patab1) are recognized by DetectTableType and parsed — the path
// that only became reachable once the glob embedder started keying by filename.
func TestExtractTableDiagnostics_FromEmbedded(t *testing.T) {
	sdtabContent := `TABLE NO.  1
 ID          DV          PRED        IPRED       CWRES       TIME
  1.0000E+00  5.0000E+00  4.8000E+00  4.9000E+00  1.0000E-01  0.0000E+00`
	patabContent := `TABLE NO.  1
 ID          ETA1
  1.0000E+00  0.1000E+00`

	sdtabEncoded, err := runlog.CompressAndEncode([]byte(sdtabContent))
	if err != nil {
		t.Fatalf("failed to encode sdtab: %v", err)
	}
	patabEncoded, err := runlog.CompressAndEncode([]byte(patabContent))
	if err != nil {
		t.Fatalf("failed to encode patab: %v", err)
	}

	record := &runlog.RunRecord{
		ID: "test-embedded-tables",
		EmbeddedFiles: map[string]string{
			"sdtab1": sdtabEncoded,
			"patab1": patabEncoded,
		},
	}

	// runDir empty so only the embedded path runs.
	diagnostics, err := ExtractTableDiagnostics(record, "")
	if err != nil {
		t.Fatalf("ExtractTableDiagnostics failed: %v", err)
	}

	if len(diagnostics.AvailableTables) != 2 {
		t.Errorf("expected 2 available tables, got %d", len(diagnostics.AvailableTables))
	}

	if diagnostics.GOFData == nil {
		t.Error("expected GOF data extracted from the embedded sdtab")
	}
	if len(diagnostics.ETAData) == 0 {
		t.Error("expected ETA data extracted from the embedded patab")
	}
}

func TestMeanSD(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}

	mean, sd := meanSD(data)

	expectedMean := 3.0
	if mean != expectedMean {
		t.Errorf("expected mean %f, got %f", expectedMean, mean)
	}

	// SD of 1,2,3,4,5 = sqrt(2)
	expectedSD := 1.4142135623730951
	tolerance := 0.0001
	if sd < expectedSD-tolerance || sd > expectedSD+tolerance {
		t.Errorf("expected SD ~%f, got %f", expectedSD, sd)
	}
}

func TestMeanSD_Empty(t *testing.T) {
	mean, sd := meanSD([]float64{})

	if mean != 0 || sd != 0 {
		t.Errorf("expected (0, 0) for empty slice, got (%f, %f)", mean, sd)
	}
}

func TestCorrelation(t *testing.T) {
	// Perfect positive correlation
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}

	corr := correlation(x, y)
	if corr < 0.99 || corr > 1.01 {
		t.Errorf("expected correlation ~1.0, got %f", corr)
	}

	// Perfect negative correlation
	y2 := []float64{10, 8, 6, 4, 2}
	corr2 := correlation(x, y2)
	if corr2 < -1.01 || corr2 > -0.99 {
		t.Errorf("expected correlation ~-1.0, got %f", corr2)
	}
}

func TestCorrelation_Empty(t *testing.T) {
	corr := correlation([]float64{}, []float64{})
	if corr != 0 {
		t.Errorf("expected 0 for empty slices, got %f", corr)
	}
}

func TestCorrelation_MismatchedLength(t *testing.T) {
	corr := correlation([]float64{1, 2, 3}, []float64{1, 2})
	if corr != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", corr)
	}
}

func TestConvertToVisualizationGOF(t *testing.T) {
	gof := &model.GOFDiagnostics{
		DV:    []float64{1, 2, 3},
		PRED:  []float64{1.1, 2.1, 3.1},
		IPRED: []float64{1.05, 2.05, 3.05},
		TIME:  []float64{0, 1, 2},
		CWRES: []float64{0.1, 0.2, 0.3},
		ID:    []float64{1, 1, 1},
	}

	result := ConvertToVisualizationGOF(gof)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.DV) != 3 {
		t.Errorf("expected 3 DV values, got %d", len(result.DV))
	}

	if len(result.PRED) != 3 {
		t.Errorf("expected 3 PRED values, got %d", len(result.PRED))
	}
}

func TestConvertToVisualizationGOF_Nil(t *testing.T) {
	result := ConvertToVisualizationGOF(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestConvertToVisualizationETA(t *testing.T) {
	eta := &model.ETADiagnostics{
		Name:   "ETA1",
		Values: []float64{0.1, 0.2, 0.3},
		IDs:    []float64{1, 2, 3},
	}

	result := ConvertToVisualizationETA(eta)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Name != "ETA1" {
		t.Errorf("expected name ETA1, got %s", result.Name)
	}

	if len(result.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(result.Values))
	}
}

func TestConvertToVisualizationETA_Nil(t *testing.T) {
	result := ConvertToVisualizationETA(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}
