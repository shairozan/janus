package tables

import (
	"math"
	"testing"
)

func TestNewTable(t *testing.T) {
	table := NewTable(TableTypeSDTAB)

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}

	if table.Columns == nil {
		t.Error("expected non-nil Columns slice")
	}

	if table.ColumnIndex == nil {
		t.Error("expected non-nil ColumnIndex map")
	}
}

func TestTable_AddColumn(t *testing.T) {
	table := NewTable(TableTypeSDTAB)

	col := table.AddColumn("DV")

	if col.Name != "DV" {
		t.Errorf("expected column name 'DV', got %q", col.Name)
	}

	if col.Index != 0 {
		t.Errorf("expected column index 0, got %d", col.Index)
	}

	if len(table.Columns) != 1 {
		t.Errorf("expected 1 column, got %d", len(table.Columns))
	}

	if table.ColumnIndex["DV"] != 0 {
		t.Errorf("expected ColumnIndex['DV'] = 0, got %d", table.ColumnIndex["DV"])
	}
}

func TestTable_GetColumn(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	table.AddColumn("ID")
	table.AddColumn("DV")

	col := table.GetColumn("DV")
	if col == nil {
		t.Fatal("expected non-nil column for 'DV'")
	}
	if col.Name != "DV" {
		t.Errorf("expected column name 'DV', got %q", col.Name)
	}

	// Non-existent column
	nilCol := table.GetColumn("NONEXISTENT")
	if nilCol != nil {
		t.Error("expected nil for non-existent column")
	}
}

func TestTable_HasColumn(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	table.AddColumn("ID")
	table.AddColumn("DV")

	if !table.HasColumn("ID") {
		t.Error("expected HasColumn('ID') = true")
	}

	if !table.HasColumn("DV") {
		t.Error("expected HasColumn('DV') = true")
	}

	if table.HasColumn("PRED") {
		t.Error("expected HasColumn('PRED') = false")
	}
}

func TestTable_GetValue(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	col := table.AddColumn("DV")

	val1 := 5.0
	val2 := 10.0
	col.Values = append(col.Values, &val1, nil, &val2)
	table.RowCount = 3

	// Valid value
	v0 := table.GetValue(0, "DV")
	if v0 == nil || *v0 != 5.0 {
		t.Errorf("expected value 5.0 at row 0, got %v", v0)
	}

	// Missing value
	v1 := table.GetValue(1, "DV")
	if v1 != nil {
		t.Errorf("expected nil at row 1, got %v", *v1)
	}

	// Valid value
	v2 := table.GetValue(2, "DV")
	if v2 == nil || *v2 != 10.0 {
		t.Errorf("expected value 10.0 at row 2, got %v", v2)
	}

	// Out of bounds
	vOOB := table.GetValue(10, "DV")
	if vOOB != nil {
		t.Error("expected nil for out-of-bounds row")
	}

	// Non-existent column
	vNC := table.GetValue(0, "NONEXISTENT")
	if vNC != nil {
		t.Error("expected nil for non-existent column")
	}
}

func TestTable_GetColumnValues(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	col := table.AddColumn("DV")

	val1 := 5.0
	val2 := 10.0
	col.Values = append(col.Values, &val1, nil, &val2)
	table.RowCount = 3

	values := table.GetColumnValues("DV")
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}

	if values[0] != 5.0 {
		t.Errorf("expected values[0] = 5.0, got %f", values[0])
	}

	if !math.IsNaN(values[1]) {
		t.Errorf("expected values[1] = NaN, got %f", values[1])
	}

	if values[2] != 10.0 {
		t.Errorf("expected values[2] = 10.0, got %f", values[2])
	}

	// Non-existent column
	nilValues := table.GetColumnValues("NONEXISTENT")
	if nilValues != nil {
		t.Error("expected nil for non-existent column")
	}
}

func TestTable_GetRow(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	col1 := table.AddColumn("ID")
	col2 := table.AddColumn("DV")

	id1 := 1.0
	dv1 := 5.0
	col1.Values = append(col1.Values, &id1)
	col2.Values = append(col2.Values, &dv1)
	table.RowCount = 1

	row := table.GetRow(0)
	if row == nil {
		t.Fatal("expected non-nil row")
	}

	if row["ID"] == nil || *row["ID"] != 1.0 {
		t.Errorf("expected row['ID'] = 1.0, got %v", row["ID"])
	}

	if row["DV"] == nil || *row["DV"] != 5.0 {
		t.Errorf("expected row['DV'] = 5.0, got %v", row["DV"])
	}

	// Out of bounds
	oobRow := table.GetRow(10)
	if oobRow != nil {
		t.Error("expected nil for out-of-bounds row")
	}
}

func TestTable_ColumnNames(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	table.AddColumn("ID")
	table.AddColumn("TIME")
	table.AddColumn("DV")

	names := table.ColumnNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	expected := []string{"ID", "TIME", "DV"}
	for i, exp := range expected {
		if names[i] != exp {
			t.Errorf("expected names[%d] = %q, got %q", i, exp, names[i])
		}
	}
}

func TestTable_FilterRows(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")
	colDV := table.AddColumn("DV")

	id1, id2, id3 := 1.0, 1.0, 2.0
	dv1, dv2, dv3 := 5.0, 10.0, 15.0
	colID.Values = append(colID.Values, &id1, &id2, &id3)
	colDV.Values = append(colDV.Values, &dv1, &dv2, &dv3)
	table.RowCount = 3

	// Filter to keep only ID=1
	filtered := table.FilterRows(func(row int) bool {
		id := table.GetValue(row, "ID")

		return id != nil && *id == 1.0
	})

	if filtered.RowCount != 2 {
		t.Errorf("expected 2 rows after filter, got %d", filtered.RowCount)
	}

	// Verify filtered values
	if filtered.GetValue(0, "DV") == nil || *filtered.GetValue(0, "DV") != 5.0 {
		t.Error("expected first filtered row DV = 5.0")
	}
	if filtered.GetValue(1, "DV") == nil || *filtered.GetValue(1, "DV") != 10.0 {
		t.Error("expected second filtered row DV = 10.0")
	}
}

func TestNormalizeColumnName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"SUBJ", "ID"},
		{"SUBJECT", "ID"},
		{"IDV", "TIME"},
		{"Y", "DV"},
		{"CONC", "DV"},
		{"TPRED", "PRED"},
		{"CPRED", "IPRED"},
		{"CRES", "CWRES"},
		{"ID", "ID"},           // Already standard
		{"UNKNOWN", "UNKNOWN"}, // Not an alias
	}

	for _, tc := range testCases {
		result := NormalizeColumnName(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeColumnName(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestValidateRequiredColumns(t *testing.T) {
	t.Run("SDTAB with required columns", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")
		table.AddColumn("DV")

		missing := ValidateRequiredColumns(table)
		if len(missing) != 0 {
			t.Errorf("expected no missing columns, got %v", missing)
		}
	})

	t.Run("SDTAB missing DV", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")

		missing := ValidateRequiredColumns(table)
		if len(missing) != 1 || missing[0] != "DV" {
			t.Errorf("expected missing ['DV'], got %v", missing)
		}
	})

	t.Run("SDTAB with alias columns", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("SUBJ") // Alias for ID
		table.AddColumn("Y")    // Alias for DV

		missing := ValidateRequiredColumns(table)
		if len(missing) != 0 {
			t.Errorf("expected no missing columns (aliases should work), got %v", missing)
		}
	})

	t.Run("PATAB with ID", func(t *testing.T) {
		table := NewTable(TableTypePATAB)
		table.AddColumn("ID")
		table.AddColumn("ETA1")

		missing := ValidateRequiredColumns(table)
		if len(missing) != 0 {
			t.Errorf("expected no missing columns, got %v", missing)
		}
	})
}
