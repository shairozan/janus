package tables

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectTableType(t *testing.T) {
	testCases := []struct {
		filename string
		expected TableType
	}{
		{"sdtab001", TableTypeSDTAB},
		{"sdtab123", TableTypeSDTAB},
		{"SDTAB001", TableTypeSDTAB},
		{"sdtab_final", TableTypeSDTAB},
		{"patab001", TableTypePATAB},
		{"PATAB001", TableTypePATAB},
		{"cotab001", TableTypeCOTAB},
		{"catab001", TableTypeCATAB},
		{"mytab001", TableTypeOther},
		{"tab001", TableTypeOther},
		{"output.tab", TableTypeOther},
		{"randomfile.txt", TableTypeOther},
	}

	for _, tc := range testCases {
		result := DetectTableType(tc.filename)
		if result != tc.expected {
			t.Errorf("DetectTableType(%q) = %v, expected %v", tc.filename, result, tc.expected)
		}
	}
}

func TestReadFile_TableNoFormat(t *testing.T) {
	table, err := ReadFile("testdata/sdtab001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}

	if table.TableNumber != 1 {
		t.Errorf("expected table number 1, got %d", table.TableNumber)
	}

	if table.RowCount != 8 {
		t.Errorf("expected 8 rows, got %d", table.RowCount)
	}

	if len(table.Columns) != 6 {
		t.Errorf("expected 6 columns, got %d", len(table.Columns))
	}

	// Check column names
	expectedCols := []string{"ID", "TIME", "DV", "PRED", "IPRED", "CWRES"}
	for i, expected := range expectedCols {
		if table.Columns[i].Name != expected {
			t.Errorf("column %d: expected %q, got %q", i, expected, table.Columns[i].Name)
		}
	}
}

func TestReadFile_OneHeaderFormat(t *testing.T) {
	table, err := ReadFile("testdata/sdtab_oneheader")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}

	if table.RowCount != 5 {
		t.Errorf("expected 5 rows, got %d", table.RowCount)
	}
}

func TestReadFile_FortranShortNotation(t *testing.T) {
	table, err := ReadFile("testdata/sdtab_fortran")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that values are parsed correctly
	dv := table.GetValue(1, "DV")
	if dv == nil {
		t.Fatal("expected non-nil DV at row 1")
	}
	if *dv < 5.23 || *dv > 5.24 {
		t.Errorf("expected DV ~5.2341, got %f", *dv)
	}
}

func TestReadFile_FortranDNotation(t *testing.T) {
	table, err := ReadFile("testdata/sdtab_dnotation")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that D notation values are parsed correctly
	dv := table.GetValue(1, "DV")
	if dv == nil {
		t.Fatal("expected non-nil DV at row 1")
	}
	if *dv < 5.23 || *dv > 5.24 {
		t.Errorf("expected DV ~5.2341, got %f", *dv)
	}
}

func TestReadFile_MissingValues(t *testing.T) {
	table, err := ReadFile("testdata/sdtab_missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Row 0, DV should be nil (missing)
	dv0 := table.GetValue(0, "DV")
	if dv0 != nil {
		t.Errorf("expected nil DV at row 0, got %f", *dv0)
	}

	// Row 1, DV should have value
	dv1 := table.GetValue(1, "DV")
	if dv1 == nil {
		t.Error("expected non-nil DV at row 1")
	}

	// Row 2, DV and IPRED should be nil
	dv2 := table.GetValue(2, "DV")
	ipred2 := table.GetValue(2, "IPRED")
	if dv2 != nil {
		t.Errorf("expected nil DV at row 2, got %f", *dv2)
	}
	if ipred2 != nil {
		t.Errorf("expected nil IPRED at row 2, got %f", *ipred2)
	}
}

func TestReadFile_PATAB(t *testing.T) {
	table, err := ReadFile("testdata/patab001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypePATAB {
		t.Errorf("expected type PATAB, got %v", table.Type)
	}

	if !table.HasColumn("ETA1") {
		t.Error("expected ETA1 column")
	}

	if !table.HasColumn("ETA2") {
		t.Error("expected ETA2 column")
	}

	if !table.HasColumn("CL") {
		t.Error("expected CL column")
	}

	if !table.HasColumn("V") {
		t.Error("expected V column")
	}
}

func TestReadFile_NonExistent(t *testing.T) {
	_, err := ReadFile("testdata/nonexistent")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFileAs(t *testing.T) {
	table, err := ReadFileAs("testdata/sdtab001", TableTypePATAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Type should be PATAB since we forced it
	if table.Type != TableTypePATAB {
		t.Errorf("expected forced type PATAB, got %v", table.Type)
	}
}

func TestFindTableFiles(t *testing.T) {
	files, err := FindTableFiles("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find sdtab and patab files
	if len(files) == 0 {
		t.Error("expected to find table files")
	}

	// Check that we found expected files
	foundSDTAB := false
	foundPATAB := false
	for _, f := range files {
		base := filepath.Base(f)
		if base == "sdtab001" || base == "sdtab_oneheader" || base == "sdtab_fortran" || base == "sdtab_dnotation" || base == "sdtab_missing" {
			foundSDTAB = true
		}
		if base == "patab001" {
			foundPATAB = true
		}
	}

	if !foundSDTAB {
		t.Error("expected to find sdtab file")
	}
	if !foundPATAB {
		t.Error("expected to find patab file")
	}
}

func TestReadAllTables(t *testing.T) {
	tables, err := ReadAllTables("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tables[TableTypeSDTAB]) == 0 {
		t.Error("expected SDTAB tables")
	}

	if len(tables[TableTypePATAB]) == 0 {
		t.Error("expected PATAB tables")
	}
}

func TestReadTableSet(t *testing.T) {
	ts, err := ReadTableSet("testdata")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ts.SDTAB) == 0 {
		t.Error("expected SDTAB tables")
	}

	if len(ts.PATAB) == 0 {
		t.Error("expected PATAB tables")
	}
}

func TestTableSet_PrimaryTables(t *testing.T) {
	ts := &TableSet{
		SDTAB: []*Table{NewTable(TableTypeSDTAB)},
		PATAB: []*Table{NewTable(TableTypePATAB)},
	}

	if ts.PrimarySDTAB() == nil {
		t.Error("expected non-nil PrimarySDTAB")
	}

	if ts.PrimaryPATAB() == nil {
		t.Error("expected non-nil PrimaryPATAB")
	}

	// Empty set
	emptyTS := &TableSet{}
	if emptyTS.PrimarySDTAB() != nil {
		t.Error("expected nil PrimarySDTAB for empty set")
	}
}

func TestTableSet_HasDiagnosticTables(t *testing.T) {
	// Create SDTAB with required columns
	sdtab := NewTable(TableTypeSDTAB)
	sdtab.AddColumn("ID")
	sdtab.AddColumn("DV")
	sdtab.AddColumn("PRED")
	sdtab.AddColumn("IPRED")

	ts := &TableSet{
		SDTAB: []*Table{sdtab},
	}

	if !ts.HasDiagnosticTables() {
		t.Error("expected HasDiagnosticTables = true")
	}

	// Without PRED/IPRED
	sdtabMinimal := NewTable(TableTypeSDTAB)
	sdtabMinimal.AddColumn("ID")
	sdtabMinimal.AddColumn("DV")

	tsMinimal := &TableSet{
		SDTAB: []*Table{sdtabMinimal},
	}

	if tsMinimal.HasDiagnosticTables() {
		t.Error("expected HasDiagnosticTables = false without PRED/IPRED")
	}

	// Empty set
	emptyTS := &TableSet{}
	if emptyTS.HasDiagnosticTables() {
		t.Error("expected HasDiagnosticTables = false for empty set")
	}
}

func TestReadFile_WithWorkingDir(t *testing.T) {
	// Get absolute path to testdata
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	absPath := filepath.Join(wd, "testdata", "sdtab001")
	table, err := ReadFile(absPath)
	if err != nil {
		t.Fatalf("unexpected error reading absolute path: %v", err)
	}

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}
}
