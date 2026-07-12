package gui

import (
	"testing"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
)

func TestTruncateID(t *testing.T) {
	tests := []struct {
		id       string
		length   int
		expected string
	}{
		{"abc123def456", 8, "abc123de..."},
		{"short", 10, "short"},
		{"exactly8c", 9, "exactly8c"},
		{"", 5, ""},
	}

	for _, tc := range tests {
		result := truncateID(tc.id, tc.length)
		if result != tc.expected {
			t.Errorf("truncateID(%q, %d) = %q, expected %q", tc.id, tc.length, result, tc.expected)
		}
	}
}

func TestNewDiagnosticsDialog(t *testing.T) {
	record := &runlog.RunRecord{
		ID:        "test-run",
		ModelFile: "test.mod",
	}

	dlg := NewDiagnosticsDialog(record, nil)

	if dlg == nil {
		t.Fatal("expected non-nil dialog")
	}

	if dlg.record != record {
		t.Error("expected dialog record to match input")
	}
}

func TestDiagnosticsDialog_NilRecord(t *testing.T) {
	dlg := NewDiagnosticsDialog(nil, nil)

	if dlg == nil {
		t.Fatal("expected non-nil dialog even with nil record")
	}

	if dlg.record != nil {
		t.Error("expected nil record")
	}
}

func TestDiagnosticsDialog_NoSummary(t *testing.T) {
	record := &runlog.RunRecord{
		ID:        "test-run",
		ModelFile: "test.mod",
		// Summary is nil
	}

	dlg := NewDiagnosticsDialog(record, nil)

	// Dialog should be created but Show() would display "No Diagnostics"
	// We can't test Show() without a real fyne.Window
	if dlg == nil {
		t.Fatal("expected non-nil dialog")
	}
}

func TestDiagnosticsDialog_WithGOFData(t *testing.T) {
	record := &runlog.RunRecord{
		ID:        "test-run",
		ModelFile: "test.mod",
		Summary: &model.ModelSummary{
			TableDiagnostics: &model.TableDiagnostics{
				GOFData: &model.GOFDiagnostics{
					DV:    []float64{1, 2, 3, 4, 5},
					PRED:  []float64{1.1, 2.1, 3.1, 4.1, 5.1},
					IPRED: []float64{1.05, 2.05, 3.05, 4.05, 5.05},
					TIME:  []float64{0, 1, 2, 3, 4},
					CWRES: []float64{0.1, 0.2, -0.1, 0.05, -0.15},
					ID:    []float64{1, 1, 1, 1, 1},
					N:     5,
				},
				ETAData: []model.ETADiagnostics{
					{
						Name:   "ETA1",
						Values: []float64{0.1, 0.2, -0.1},
						IDs:    []float64{1, 2, 3},
						N:      3,
						Mean:   0.0667,
						SD:     0.153,
					},
				},
			},
		},
	}

	dlg := NewDiagnosticsDialog(record, nil)

	if dlg == nil {
		t.Fatal("expected non-nil dialog")
	}

	// Verify record has GOF data
	if dlg.record.Summary == nil {
		t.Fatal("expected non-nil summary")
	}

	if dlg.record.Summary.TableDiagnostics == nil {
		t.Fatal("expected non-nil table diagnostics")
	}

	if dlg.record.Summary.TableDiagnostics.GOFData == nil {
		t.Fatal("expected non-nil GOF data")
	}

	if dlg.record.Summary.TableDiagnostics.GOFData.N != 5 {
		t.Errorf("expected 5 observations, got %d", dlg.record.Summary.TableDiagnostics.GOFData.N)
	}
}
