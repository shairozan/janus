package tables

import (
	"strings"
	"testing"
)

func TestParser_Parse_TableNoFormat(t *testing.T) {
	input := `TABLE NO.  1
 ID          TIME        DV          PRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00
  1.0000E+00  5.0000E-01  5.2341E+00  4.8923E+00
  2.0000E+00  1.0000E+00  8.1234E+00  7.9456E+00`

	parser := NewParser()
	table, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.TableNumber != 1 {
		t.Errorf("expected table number 1, got %d", table.TableNumber)
	}

	if len(table.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(table.Columns))
	}

	if table.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", table.RowCount)
	}

	// Check column names
	expectedCols := []string{"ID", "TIME", "DV", "PRED"}
	for i, expected := range expectedCols {
		if table.Columns[i].Name != expected {
			t.Errorf("column %d: expected %q, got %q", i, expected, table.Columns[i].Name)
		}
	}

	// Check a specific value
	dvVal := table.GetValue(1, "DV")
	if dvVal == nil {
		t.Fatal("expected non-nil DV value at row 1")
	}
	if *dvVal < 5.23 || *dvVal > 5.24 {
		t.Errorf("expected DV ~5.2341, got %f", *dvVal)
	}
}

func TestParser_Parse_OneHeaderFormat(t *testing.T) {
	input := `ID          TIME        DV          PRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00
  1.0000E+00  5.0000E-01  5.2341E+00  4.8923E+00`

	parser := NewParser()
	table, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.TableNumber != 0 {
		t.Errorf("expected table number 0 for ONEHEADER format, got %d", table.TableNumber)
	}

	if len(table.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(table.Columns))
	}

	if table.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount)
	}
}

func TestParser_Parse_ScientificNotation_Standard(t *testing.T) {
	testCases := []struct {
		input    string
		expected float64
	}{
		{"1.2345E+02", 123.45},
		{"1.2345e+02", 123.45},
		{"-1.2345E-02", -0.012345},
		{"1.0E+00", 1.0},
		{"0.0000E+00", 0.0},
	}

	parser := NewParser()
	for _, tc := range testCases {
		val, err := parser.parseValue(tc.input)
		if err != nil {
			t.Errorf("parseValue(%q) error: %v", tc.input, err)
			continue
		}
		if val == nil {
			t.Errorf("parseValue(%q) returned nil", tc.input)
			continue
		}
		diff := *val - tc.expected
		if diff < -0.0001 || diff > 0.0001 {
			t.Errorf("parseValue(%q) = %f, expected %f", tc.input, *val, tc.expected)
		}
	}
}

func TestParser_Parse_ScientificNotation_FortranD(t *testing.T) {
	testCases := []struct {
		input    string
		expected float64
	}{
		{"1.2345D+02", 123.45},
		{"1.2345d+02", 123.45},
		{"-1.2345D-02", -0.012345},
	}

	parser := NewParser()
	for _, tc := range testCases {
		val, err := parser.parseValue(tc.input)
		if err != nil {
			t.Errorf("parseValue(%q) error: %v", tc.input, err)
			continue
		}
		if val == nil {
			t.Errorf("parseValue(%q) returned nil", tc.input)
			continue
		}
		diff := *val - tc.expected
		if diff < -0.0001 || diff > 0.0001 {
			t.Errorf("parseValue(%q) = %f, expected %f", tc.input, *val, tc.expected)
		}
	}
}

func TestParser_Parse_ScientificNotation_FortranShort(t *testing.T) {
	testCases := []struct {
		input    string
		expected float64
	}{
		{"1.2345+02", 123.45},
		{"-1.2345-02", -0.012345},
		{"1.0000+00", 1.0},
		{"5.0000-01", 0.5},
	}

	parser := NewParser()
	for _, tc := range testCases {
		val, err := parser.parseValue(tc.input)
		if err != nil {
			t.Errorf("parseValue(%q) error: %v", tc.input, err)
			continue
		}
		if val == nil {
			t.Errorf("parseValue(%q) returned nil", tc.input)
			continue
		}
		diff := *val - tc.expected
		if diff < -0.0001 || diff > 0.0001 {
			t.Errorf("parseValue(%q) = %f, expected %f", tc.input, *val, tc.expected)
		}
	}
}

func TestParser_Parse_MissingValues(t *testing.T) {
	input := `ID          TIME        DV          PRED
  1.0000E+00  0.0000E+00  .           0.0000E+00
  1.0000E+00  5.0000E-01  5.2341E+00  .`

	parser := NewParser()
	table, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Row 0, DV should be nil (missing)
	dv0 := table.GetValue(0, "DV")
	if dv0 != nil {
		t.Errorf("expected nil DV at row 0, got %f", *dv0)
	}

	// Row 0, PRED should have value
	pred0 := table.GetValue(0, "PRED")
	if pred0 == nil {
		t.Error("expected non-nil PRED at row 0")
	}

	// Row 1, PRED should be nil
	pred1 := table.GetValue(1, "PRED")
	if pred1 != nil {
		t.Errorf("expected nil PRED at row 1, got %f", *pred1)
	}

	// Row 1, DV should have value
	dv1 := table.GetValue(1, "DV")
	if dv1 == nil {
		t.Error("expected non-nil DV at row 1")
	}
}

func TestParser_Parse_ColumnNormalization(t *testing.T) {
	input := `SUBJ        IDV         Y           TPRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00`

	parser := NewParser() // With normalization enabled by default
	table, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that column names are normalized
	expectedCols := []string{"ID", "TIME", "DV", "PRED"}
	for i, expected := range expectedCols {
		if table.Columns[i].Name != expected {
			t.Errorf("column %d: expected normalized name %q, got %q", i, expected, table.Columns[i].Name)
		}
	}
}

func TestParser_Parse_WithoutNormalization(t *testing.T) {
	input := `SUBJ        IDV         Y           TPRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00`

	parser := NewParserWithOptions(WithNormalizeColumns(false))
	table, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that column names are NOT normalized
	expectedCols := []string{"SUBJ", "IDV", "Y", "TPRED"}
	for i, expected := range expectedCols {
		if table.Columns[i].Name != expected {
			t.Errorf("column %d: expected original name %q, got %q", i, expected, table.Columns[i].Name)
		}
	}
}

func TestParser_ParseMultiple(t *testing.T) {
	input := `TABLE NO.  1
 ID          TIME        DV
  1.0000E+00  0.0000E+00  0.0000E+00
  1.0000E+00  5.0000E-01  5.2341E+00
TABLE NO.  2
 ID          TIME        DV
  1.0000E+00  0.0000E+00  1.0000E+00
  1.0000E+00  5.0000E-01  6.2341E+00
  1.0000E+00  1.0000E+00  7.1234E+00`

	parser := NewParser()
	tables, err := parser.ParseMultiple(strings.NewReader(input), TableTypeSDTAB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}

	if tables[0].TableNumber != 1 {
		t.Errorf("first table number: expected 1, got %d", tables[0].TableNumber)
	}
	if tables[0].RowCount != 2 {
		t.Errorf("first table rows: expected 2, got %d", tables[0].RowCount)
	}

	if tables[1].TableNumber != 2 {
		t.Errorf("second table number: expected 2, got %d", tables[1].TableNumber)
	}
	if tables[1].RowCount != 3 {
		t.Errorf("second table rows: expected 3, got %d", tables[1].RowCount)
	}
}

func TestParser_Parse_EmptyInput(t *testing.T) {
	parser := NewParser()
	_, err := parser.Parse(strings.NewReader(""), TableTypeSDTAB)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParser_Parse_MismatchedColumns(t *testing.T) {
	input := `ID          TIME        DV
  1.0000E+00  0.0000E+00  0.0000E+00
  1.0000E+00  5.0000E-01`

	parser := NewParser()
	_, err := parser.Parse(strings.NewReader(input), TableTypeSDTAB)
	if err == nil {
		t.Error("expected error for mismatched column count")
	}
}

func TestDetectFormat(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected TableFormat
	}{
		{
			name: "TABLE NO format",
			input: `TABLE NO.  1
 ID          TIME        DV
  1.0000E+00  0.0000E+00  0.0000E+00`,
			expected: FormatTableNo,
		},
		{
			name: "ONEHEADER format",
			input: `ID          TIME        DV
  1.0000E+00  0.0000E+00  0.0000E+00`,
			expected: FormatOneHeader,
		},
		{
			name:     "Empty",
			input:    "",
			expected: FormatUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			format, err := DetectFormat(strings.NewReader(tc.input))
			if tc.expected == FormatUnknown && err == nil {
				t.Error("expected error for unknown format")
			}
			if tc.expected != FormatUnknown && format != tc.expected {
				t.Errorf("expected format %v, got %v", tc.expected, format)
			}
		})
	}
}
