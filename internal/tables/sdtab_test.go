package tables

import (
	"strings"
	"testing"
)

func TestSDTABParser_Parse(t *testing.T) {
	input := `TABLE NO.  1
 ID          TIME        DV          PRED        IPRED       CWRES
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00
  1.0000E+00  5.0000E-01  5.2341E+00  4.8923E+00  4.9015E+00  3.2010E-01
  2.0000E+00  1.0000E+00  8.1234E+00  7.9456E+00  7.9823E+00  1.5620E-01`

	parser := NewSDTABParser()
	table, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}

	if table.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", table.RowCount)
	}
}

func TestSDTABParser_Parse_MissingDV(t *testing.T) {
	input := `ID          TIME        PRED        IPRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00`

	parser := NewSDTABParser()
	_, err := parser.Parse(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for missing DV column")
	}
}

func TestSDTABParser_Parse_MissingID(t *testing.T) {
	input := `TIME        DV          PRED        IPRED
  0.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00`

	parser := NewSDTABParser()
	_, err := parser.Parse(strings.NewReader(input))
	if err == nil {
		t.Error("expected error for missing ID column")
	}
}

func TestSDTABParser_Parse_WithAliases(t *testing.T) {
	// Using aliases: SUBJ for ID, Y for DV
	input := `SUBJ        TIME        Y           PRED
  1.0000E+00  0.0000E+00  0.0000E+00  0.0000E+00`

	parser := NewSDTABParser()
	table, err := parser.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error with aliases: %v", err)
	}

	// Columns should be normalized
	if !table.HasColumn("ID") {
		t.Error("expected SUBJ to be normalized to ID")
	}
	if !table.HasColumn("DV") {
		t.Error("expected Y to be normalized to DV")
	}
}

func TestHasPredictions(t *testing.T) {
	t.Run("with PRED", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")
		table.AddColumn("DV")
		table.AddColumn("PRED")

		if !HasPredictions(table) {
			t.Error("expected HasPredictions = true with PRED")
		}
	})

	t.Run("with IPRED", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")
		table.AddColumn("DV")
		table.AddColumn("IPRED")

		if !HasPredictions(table) {
			t.Error("expected HasPredictions = true with IPRED")
		}
	})

	t.Run("without predictions", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")
		table.AddColumn("DV")

		if HasPredictions(table) {
			t.Error("expected HasPredictions = false")
		}
	})
}

func TestHasResiduals(t *testing.T) {
	t.Run("with CWRES", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("CWRES")

		if !HasResiduals(table) {
			t.Error("expected HasResiduals = true with CWRES")
		}
	})

	t.Run("without residuals", func(t *testing.T) {
		table := NewTable(TableTypeSDTAB)
		table.AddColumn("ID")
		table.AddColumn("DV")

		if HasResiduals(table) {
			t.Error("expected HasResiduals = false")
		}
	})
}

func TestGetObservationRows(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")
	colDV := table.AddColumn("DV")
	colEVID := table.AddColumn("EVID")

	// Row 0: Observation (EVID=0)
	id1, dv1, evid0 := 1.0, 5.0, 0.0
	colID.Values = append(colID.Values, &id1)
	colDV.Values = append(colDV.Values, &dv1)
	colEVID.Values = append(colEVID.Values, &evid0)

	// Row 1: Dose (EVID=1)
	id2, evid1 := 1.0, 1.0
	colID.Values = append(colID.Values, &id2)
	colDV.Values = append(colDV.Values, nil) // Missing DV for dose
	colEVID.Values = append(colEVID.Values, &evid1)

	// Row 2: Observation (EVID=0)
	id3, dv3, evid2 := 1.0, 10.0, 0.0
	colID.Values = append(colID.Values, &id3)
	colDV.Values = append(colDV.Values, &dv3)
	colEVID.Values = append(colEVID.Values, &evid2)

	table.RowCount = 3

	obs := GetObservationRows(table)
	if obs.RowCount != 2 {
		t.Errorf("expected 2 observation rows, got %d", obs.RowCount)
	}
}

func TestSubjectIDs(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")

	id1, id2, id3 := 1.0, 1.0, 2.0
	colID.Values = append(colID.Values, &id1, &id2, &id3)
	table.RowCount = 3

	ids := SubjectIDs(table)
	if len(ids) != 2 {
		t.Errorf("expected 2 unique IDs, got %d", len(ids))
	}

	// Should be in order of first appearance
	if ids[0] != 1.0 || ids[1] != 2.0 {
		t.Errorf("expected IDs [1, 2], got %v", ids)
	}
}

func TestGetSubjectRows(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")
	colDV := table.AddColumn("DV")

	id1, id2, id3 := 1.0, 1.0, 2.0
	dv1, dv2, dv3 := 5.0, 10.0, 15.0
	colID.Values = append(colID.Values, &id1, &id2, &id3)
	colDV.Values = append(colDV.Values, &dv1, &dv2, &dv3)
	table.RowCount = 3

	subj1 := GetSubjectRows(table, 1.0)
	if subj1.RowCount != 2 {
		t.Errorf("expected 2 rows for subject 1, got %d", subj1.RowCount)
	}

	subj2 := GetSubjectRows(table, 2.0)
	if subj2.RowCount != 1 {
		t.Errorf("expected 1 row for subject 2, got %d", subj2.RowCount)
	}
}

func TestExtractGOFData(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	colID := table.AddColumn("ID")
	colDV := table.AddColumn("DV")
	colPRED := table.AddColumn("PRED")
	colIPRED := table.AddColumn("IPRED")
	colTIME := table.AddColumn("TIME")
	colCWRES := table.AddColumn("CWRES")

	id1, dv1, pred1, ipred1, time1, cwres1 := 1.0, 5.0, 4.8, 4.9, 0.5, 0.32
	id2, dv2, pred2, ipred2, time2, cwres2 := 1.0, 10.0, 9.8, 9.9, 1.0, 0.15

	colID.Values = append(colID.Values, &id1, &id2)
	colDV.Values = append(colDV.Values, &dv1, &dv2)
	colPRED.Values = append(colPRED.Values, &pred1, &pred2)
	colIPRED.Values = append(colIPRED.Values, &ipred1, &ipred2)
	colTIME.Values = append(colTIME.Values, &time1, &time2)
	colCWRES.Values = append(colCWRES.Values, &cwres1, &cwres2)
	table.RowCount = 2

	data, err := ExtractGOFData(table)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.DV) != 2 {
		t.Errorf("expected 2 DV values, got %d", len(data.DV))
	}

	if len(data.PRED) != 2 {
		t.Errorf("expected 2 PRED values, got %d", len(data.PRED))
	}

	if len(data.IPRED) != 2 {
		t.Errorf("expected 2 IPRED values, got %d", len(data.IPRED))
	}

	if len(data.CWRES) != 2 {
		t.Errorf("expected 2 CWRES values, got %d", len(data.CWRES))
	}
}

func TestExtractGOFData_NoDV(t *testing.T) {
	table := NewTable(TableTypeSDTAB)
	table.AddColumn("ID")
	table.AddColumn("PRED")
	table.RowCount = 0

	_, err := ExtractGOFData(table)
	if err == nil {
		t.Error("expected error for missing DV column")
	}
}

func TestReadSDTAB(t *testing.T) {
	table, err := ReadSDTAB("testdata/sdtab001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if table.Type != TableTypeSDTAB {
		t.Errorf("expected type SDTAB, got %v", table.Type)
	}

	if !table.HasColumn("DV") {
		t.Error("expected DV column")
	}

	if !table.HasColumn("PRED") {
		t.Error("expected PRED column")
	}
}
