package tables

import (
	"strings"
	"testing"
)

func TestPATABParser_Parse(t *testing.T) {
	input := `TABLE NO.  1
 ID          ETA1        ETA2        CL          V
  1.0000E+00  2.3456E-01 -1.2345E-01  1.5200E+01  8.5300E+01
  2.0000E+00 -1.8765E-01  5.6789E-02  1.4800E+01  8.2100E+01`

	parser := NewPATABParser()
	table, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypePATAB {
		t.Errorf("expected type PATAB, got %v", table.Type)
	}

	if table.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount)
	}
}

func TestPATABParser_Parse_MissingID(t *testing.T) {
	input := `ETA1        ETA2        CL          V
  2.3456E-01 -1.2345E-01  1.5200E+01  8.5300E+01`

	parser := NewPATABParser()
	_, err := parser.Parse(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for missing ID column")
	}
}

func TestGetETAColumns(t *testing.T) {
	table := NewTable(TableTypePATAB)
	table.AddColumn("ID")
	table.AddColumn("ETA2") // Out of order
	table.AddColumn("CL")
	table.AddColumn("ETA1")
	table.AddColumn("ETA3")
	table.AddColumn("V")

	etas := GetETAColumns(table)
	if len(etas) != 3 {
		t.Fatalf("expected 3 ETA columns, got %d", len(etas))
	}

	// Should be sorted by number
	expected := []string{"ETA1", "ETA2", "ETA3"}
	for i, exp := range expected {
		if etas[i] != exp {
			t.Errorf("expected etas[%d] = %s, got %s", i, exp, etas[i])
		}
	}
}

func TestGetETAColumns_Parentheses(t *testing.T) {
	table := NewTable(TableTypePATAB)
	table.AddColumn("ID")
	table.AddColumn("ETA(1)")
	table.AddColumn("ETA(2)")

	etas := GetETAColumns(table)
	if len(etas) != 2 {
		t.Fatalf("expected 2 ETA columns, got %d", len(etas))
	}

	expected := []string{"ETA(1)", "ETA(2)"}
	for i, exp := range expected {
		if etas[i] != exp {
			t.Errorf("expected etas[%d] = %s, got %s", i, exp, etas[i])
		}
	}
}

func TestGetIndividualParameters(t *testing.T) {
	table := NewTable(TableTypePATAB)
	table.AddColumn("ID")
	table.AddColumn("ETA1")
	table.AddColumn("ETA2")
	table.AddColumn("CL")
	table.AddColumn("V")
	table.AddColumn("KA")

	params := GetIndividualParameters(table)
	if len(params) != 3 {
		t.Fatalf("expected 3 individual parameters, got %d: %v", len(params), params)
	}

	// Should include CL, V, KA (not ID or ETAs)
	expected := map[string]bool{"CL": true, "V": true, "KA": true}
	for _, p := range params {
		if !expected[p] {
			t.Errorf("unexpected parameter: %s", p)
		}
	}
}

func TestExtractETAData(t *testing.T) {
	table := NewTable(TableTypePATAB)
	colID := table.AddColumn("ID")
	colETA1 := table.AddColumn("ETA1")
	colETA2 := table.AddColumn("ETA2")

	// Subject 1 with two rows (same ETA values)
	id1a, eta1a, eta2a := 1.0, 0.234, -0.123
	id1b, eta1b, eta2b := 1.0, 0.234, -0.123
	// Subject 2
	id2, eta1c, eta2c := 2.0, -0.187, 0.056

	colID.Values = append(colID.Values, &id1a, &id1b, &id2)
	colETA1.Values = append(colETA1.Values, &eta1a, &eta1b, &eta1c)
	colETA2.Values = append(colETA2.Values, &eta2a, &eta2b, &eta2c)
	table.RowCount = 3

	etaData, err := ExtractETAData(table)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(etaData) != 2 {
		t.Fatalf("expected 2 ETA data sets, got %d", len(etaData))
	}

	// Check ETA1
	eta1 := etaData[0]
	if eta1.Name != "ETA1" {
		t.Errorf("expected first ETA name = ETA1, got %s", eta1.Name)
	}
	if len(eta1.Values) != 2 {
		t.Errorf("expected 2 unique subjects, got %d", len(eta1.Values))
	}
}

func TestExtractETAData_NoETAs(t *testing.T) {
	table := NewTable(TableTypePATAB)
	table.AddColumn("ID")
	table.AddColumn("CL")
	table.AddColumn("V")

	_, err := ExtractETAData(table)
	if err == nil {
		t.Error("expected error for no ETA columns")
	}
}

func TestCalculateETAStatistics(t *testing.T) {
	data := &ETAData{
		Name:   "ETA1",
		Values: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		IDs:    []float64{1, 2, 3, 4, 5},
	}

	stats := CalculateETAStatistics(data)

	if stats.Name != "ETA1" {
		t.Errorf("expected name = ETA1, got %s", stats.Name)
	}

	if stats.N != 5 {
		t.Errorf("expected N = 5, got %d", stats.N)
	}

	// Mean should be 0.3
	if stats.Mean < 0.29 || stats.Mean > 0.31 {
		t.Errorf("expected mean ~0.3, got %f", stats.Mean)
	}

	// Min should be 0.1
	if stats.Min != 0.1 {
		t.Errorf("expected min = 0.1, got %f", stats.Min)
	}

	// Max should be 0.5
	if stats.Max != 0.5 {
		t.Errorf("expected max = 0.5, got %f", stats.Max)
	}

	// Median should be 0.3
	if stats.Median < 0.29 || stats.Median > 0.31 {
		t.Errorf("expected median ~0.3, got %f", stats.Median)
	}
}

func TestCalculateETAStatistics_Empty(t *testing.T) {
	data := &ETAData{
		Name:   "ETA1",
		Values: []float64{},
		IDs:    []float64{},
	}

	stats := CalculateETAStatistics(data)

	if stats.N != 0 {
		t.Errorf("expected N = 0, got %d", stats.N)
	}
}

func TestReadPATAB(t *testing.T) {
	table, err := ReadPATAB("testdata/patab001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypePATAB {
		t.Errorf("expected type PATAB, got %v", table.Type)
	}

	etas := GetETAColumns(table)
	if len(etas) < 2 {
		t.Errorf("expected at least 2 ETA columns, got %d", len(etas))
	}

	params := GetIndividualParameters(table)
	if len(params) < 2 {
		t.Errorf("expected at least 2 individual parameters, got %d", len(params))
	}
}
