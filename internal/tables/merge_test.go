package tables

import (
	"testing"
)

func TestMergeTables(t *testing.T) {
	// Create left table
	left := NewTable(TableTypeSDTAB)
	colID1 := left.AddColumn("ID")
	colDV := left.AddColumn("DV")

	id1, id2 := 1.0, 2.0
	dv1, dv2 := 5.0, 10.0
	colID1.Values = append(colID1.Values, &id1, &id2)
	colDV.Values = append(colDV.Values, &dv1, &dv2)
	left.RowCount = 2

	// Create right table
	right := NewTable(TableTypePATAB)
	colID2 := right.AddColumn("ID")
	colETA := right.AddColumn("ETA1")

	eta1, eta2 := 0.1, 0.2
	colID2.Values = append(colID2.Values, &id1, &id2)
	colETA.Values = append(colETA.Values, &eta1, &eta2)
	right.RowCount = 2

	// Merge
	merged, err := MergeTables(left, right, DefaultMergeOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check columns
	if !merged.HasColumn("ID") {
		t.Error("expected ID column")
	}
	if !merged.HasColumn("DV") {
		t.Error("expected DV column")
	}
	if !merged.HasColumn("ETA1") {
		t.Error("expected ETA1 column")
	}

	// Should not have duplicate ID column
	idCount := 0
	for _, col := range merged.Columns {
		if col.Name == "ID" {
			idCount++
		}
	}
	if idCount != 1 {
		t.Errorf("expected 1 ID column, got %d", idCount)
	}

	if merged.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", merged.RowCount)
	}
}

func TestMergeTables_MissingKeyColumn(t *testing.T) {
	left := NewTable(TableTypeSDTAB)
	left.AddColumn("DV")
	left.RowCount = 1

	right := NewTable(TableTypePATAB)
	right.AddColumn("ID")
	right.AddColumn("ETA1")
	right.RowCount = 1

	_, err := MergeTables(left, right, DefaultMergeOptions())
	if err == nil {
		t.Error("expected error for missing key column in left table")
	}
}

func TestMergeTables_RowCountMismatch(t *testing.T) {
	left := NewTable(TableTypeSDTAB)
	colID1 := left.AddColumn("ID")
	id1 := 1.0
	colID1.Values = append(colID1.Values, &id1)
	left.RowCount = 1

	right := NewTable(TableTypePATAB)
	colID2 := right.AddColumn("ID")
	id2, id3 := 1.0, 2.0
	colID2.Values = append(colID2.Values, &id2, &id3)
	right.RowCount = 2

	_, err := MergeTables(left, right, DefaultMergeOptions())
	if err == nil {
		t.Error("expected error for row count mismatch")
	}
}

func TestMergeTables_KeepDuplicates(t *testing.T) {
	left := NewTable(TableTypeSDTAB)
	colID1 := left.AddColumn("ID")
	colDV1 := left.AddColumn("DV")
	id1, dv1 := 1.0, 5.0
	colID1.Values = append(colID1.Values, &id1)
	colDV1.Values = append(colDV1.Values, &dv1)
	left.RowCount = 1

	right := NewTable(TableTypeSDTAB)
	colID2 := right.AddColumn("ID")
	colDV2 := right.AddColumn("DV") // Same column name
	dv2 := 10.0
	colID2.Values = append(colID2.Values, &id1)
	colDV2.Values = append(colDV2.Values, &dv2)
	right.RowCount = 1

	opts := DefaultMergeOptions()
	opts.KeepDuplicates = true

	merged, err := MergeTables(left, right, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have both DV and DV_2
	if !merged.HasColumn("DV") {
		t.Error("expected DV column")
	}
	if !merged.HasColumn("DV_2") {
		t.Error("expected DV_2 column for duplicate")
	}
}

func TestMergeByRow(t *testing.T) {
	t1 := NewTable(TableTypeSDTAB)
	colID1 := t1.AddColumn("ID")
	colDV := t1.AddColumn("DV")
	id1, dv1 := 1.0, 5.0
	colID1.Values = append(colID1.Values, &id1)
	colDV.Values = append(colDV.Values, &dv1)
	t1.RowCount = 1

	t2 := NewTable(TableTypePATAB)
	colID2 := t2.AddColumn("ID")
	colETA := t2.AddColumn("ETA1")
	eta1 := 0.1
	colID2.Values = append(colID2.Values, &id1)
	colETA.Values = append(colETA.Values, &eta1)
	t2.RowCount = 1

	merged, err := MergeByRow(t1, t2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have ID (once), DV, ETA1
	expectedCols := 3 // ID is deduplicated
	if len(merged.Columns) != expectedCols {
		t.Errorf("expected %d columns, got %d", expectedCols, len(merged.Columns))
	}
}

func TestMergeByRow_RowCountMismatch(t *testing.T) {
	t1 := NewTable(TableTypeSDTAB)
	t1.AddColumn("ID")
	t1.RowCount = 1

	t2 := NewTable(TableTypePATAB)
	t2.AddColumn("ID")
	t2.RowCount = 2

	_, err := MergeByRow(t1, t2)
	if err == nil {
		t.Error("expected error for row count mismatch")
	}
}

func TestMergeByRow_Empty(t *testing.T) {
	_, err := MergeByRow()
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestMergeByRow_Single(t *testing.T) {
	t1 := NewTable(TableTypeSDTAB)
	t1.AddColumn("ID")
	t1.RowCount = 1

	merged, err := MergeByRow(t1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if merged != t1 {
		t.Error("expected same table for single input")
	}
}

func TestAppendRows(t *testing.T) {
	target := NewTable(TableTypeSDTAB)
	colID := target.AddColumn("ID")
	colDV := target.AddColumn("DV")
	id1, dv1 := 1.0, 5.0
	colID.Values = append(colID.Values, &id1)
	colDV.Values = append(colDV.Values, &dv1)
	target.RowCount = 1

	source := NewTable(TableTypeSDTAB)
	srcID := source.AddColumn("ID")
	srcDV := source.AddColumn("DV")
	id2, dv2 := 2.0, 10.0
	srcID.Values = append(srcID.Values, &id2)
	srcDV.Values = append(srcDV.Values, &dv2)
	source.RowCount = 1

	err := AppendRows(target, source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target.RowCount != 2 {
		t.Errorf("expected 2 rows after append, got %d", target.RowCount)
	}

	// Check values
	if *target.GetValue(0, "ID") != 1.0 {
		t.Error("expected first row ID = 1")
	}
	if *target.GetValue(1, "ID") != 2.0 {
		t.Error("expected second row ID = 2")
	}
}

func TestAppendRows_ColumnMismatch(t *testing.T) {
	target := NewTable(TableTypeSDTAB)
	target.AddColumn("ID")
	target.AddColumn("DV")
	target.RowCount = 1

	source := NewTable(TableTypeSDTAB)
	source.AddColumn("ID")
	// Missing DV column
	source.RowCount = 1

	err := AppendRows(target, source)
	if err == nil {
		t.Error("expected error for column count mismatch")
	}
}

func TestConcatenateTables(t *testing.T) {
	t1 := NewTable(TableTypeSDTAB)
	colID1 := t1.AddColumn("ID")
	colDV1 := t1.AddColumn("DV")
	id1, dv1 := 1.0, 5.0
	colID1.Values = append(colID1.Values, &id1)
	colDV1.Values = append(colDV1.Values, &dv1)
	t1.RowCount = 1

	t2 := NewTable(TableTypeSDTAB)
	colID2 := t2.AddColumn("ID")
	colDV2 := t2.AddColumn("DV")
	id2, dv2 := 2.0, 10.0
	colID2.Values = append(colID2.Values, &id2)
	colDV2.Values = append(colDV2.Values, &dv2)
	t2.RowCount = 1

	result, err := ConcatenateTables(t1, t2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
}

func TestSelectColumns(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")
	colDV := table.AddColumn("DV")
	colPRED := table.AddColumn("PRED")
	colIPRED := table.AddColumn("IPRED")

	id1, dv1, pred1, ipred1 := 1.0, 5.0, 4.8, 4.9
	colID.Values = append(colID.Values, &id1)
	colDV.Values = append(colDV.Values, &dv1)
	colPRED.Values = append(colPRED.Values, &pred1)
	colIPRED.Values = append(colIPRED.Values, &ipred1)
	table.RowCount = 1

	selected, err := SelectColumns(table, "ID", "DV")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(selected.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(selected.Columns))
	}

	if !selected.HasColumn("ID") || !selected.HasColumn("DV") {
		t.Error("expected ID and DV columns")
	}

	if selected.HasColumn("PRED") || selected.HasColumn("IPRED") {
		t.Error("should not have PRED or IPRED columns")
	}
}

func TestSelectColumns_NotFound(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	table.AddColumn("ID")
	table.AddColumn("DV")
	table.RowCount = 1

	_, err := SelectColumns(table, "ID", "NONEXISTENT")
	if err == nil {
		t.Error("expected error for non-existent column")
	}
}
