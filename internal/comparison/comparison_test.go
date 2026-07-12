package comparison

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
)

// Helper to create a pointer to a float64.
func ptr(f float64) *float64 {
	return &f
}

// makeTestRecord creates a test RunRecord with the given parameters.
func makeTestRecord(id string, ofv float64, thetas []float64, timestamp time.Time) *runlog.RunRecord {
	thetaParams := make([]model.ParameterEstimate, len(thetas))
	for i, v := range thetas {
		val := v
		thetaParams[i] = model.ParameterEstimate{
			Name:     fmt.Sprintf("THETA%d", i+1),
			Estimate: &val,
			Fixed:    false,
		}
	}

	return &runlog.RunRecord{
		ID:        id,
		Timestamp: timestamp,
		Status:    "completed",
		Summary: &model.ModelSummary{
			GoodnessOfFit: model.GoodnessOfFitSummary{
				ObjectiveFunctionValue: ptr(ofv),
			},
			Estimation: model.EstimationSummary{
				Minimized:    true,
				Method:       "FOCE",
				Subjects:     40,
				Observations: 760,
			},
			Diagnostics: model.DiagnosticSummary{
				CovarianceStepSuccess: true,
			},
			Parameters: model.ParameterSummary{
				Thetas: thetaParams,
				Omegas: []model.ParameterEstimate{
					{Name: "OMEGA(1,1)", Estimate: ptr(0.0625), Fixed: false},
				},
				Sigmas: []model.ParameterEstimate{
					{Name: "SIGMA(1,1)", Estimate: ptr(0.01), Fixed: false},
				},
			},
		},
	}
}

func TestCalculateDelta_BothValues(t *testing.T) {
	baseline := 100.0
	comparison := 110.0

	delta, deltaPct := CalculateDelta(&baseline, &comparison)

	if delta == nil {
		t.Fatal("expected non-nil delta")
	}
	if *delta != 10.0 {
		t.Errorf("expected delta=10, got %f", *delta)
	}

	if deltaPct == nil {
		t.Fatal("expected non-nil deltaPct")
	}
	if *deltaPct != 10.0 {
		t.Errorf("expected deltaPct=10%%, got %f%%", *deltaPct)
	}
}

func TestCalculateDelta_NegativeChange(t *testing.T) {
	baseline := 100.0
	comparison := 90.0

	delta, deltaPct := CalculateDelta(&baseline, &comparison)

	if delta == nil || *delta != -10.0 {
		t.Errorf("expected delta=-10, got %v", delta)
	}
	if deltaPct == nil || *deltaPct != -10.0 {
		t.Errorf("expected deltaPct=-10%%, got %v", deltaPct)
	}
}

func TestCalculateDelta_NilBaseline(t *testing.T) {
	comparison := 100.0

	delta, deltaPct := CalculateDelta(nil, &comparison)

	if delta != nil {
		t.Errorf("expected nil delta, got %v", delta)
	}
	if deltaPct != nil {
		t.Errorf("expected nil deltaPct, got %v", deltaPct)
	}
}

func TestCalculateDelta_NilComparison(t *testing.T) {
	baseline := 100.0

	delta, deltaPct := CalculateDelta(&baseline, nil)

	if delta != nil {
		t.Errorf("expected nil delta, got %v", delta)
	}
	if deltaPct != nil {
		t.Errorf("expected nil deltaPct, got %v", deltaPct)
	}
}

func TestCalculateDelta_BothNil(t *testing.T) {
	delta, deltaPct := CalculateDelta(nil, nil)

	if delta != nil || deltaPct != nil {
		t.Errorf("expected both nil, got delta=%v, deltaPct=%v", delta, deltaPct)
	}
}

func TestCalculateDelta_ZeroBaseline(t *testing.T) {
	baseline := 0.0
	comparison := 10.0

	delta, deltaPct := CalculateDelta(&baseline, &comparison)

	if delta == nil || *delta != 10.0 {
		t.Errorf("expected delta=10, got %v", delta)
	}
	// Percentage is undefined when baseline is 0
	if deltaPct != nil {
		t.Errorf("expected nil deltaPct for zero baseline, got %v", deltaPct)
	}
}

func TestCalculateDelta_BothZero(t *testing.T) {
	baseline := 0.0
	comparison := 0.0

	delta, deltaPct := CalculateDelta(&baseline, &comparison)

	if delta == nil || *delta != 0.0 {
		t.Errorf("expected delta=0, got %v", delta)
	}
	if deltaPct == nil || *deltaPct != 0.0 {
		t.Errorf("expected deltaPct=0, got %v", deltaPct)
	}
}

func TestDetermineHighlight_NoChange(t *testing.T) {
	pct := 0.0
	highlight := DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightNone {
		t.Errorf("expected HighlightNone, got %v", highlight)
	}
}

func TestDetermineHighlight_MinorChange(t *testing.T) {
	pct := 3.0 // < 5%
	highlight := DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightNone {
		t.Errorf("expected HighlightNone for 3%%, got %v", highlight)
	}

	pct = 5.0 // exactly 5%
	highlight = DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightMinor {
		t.Errorf("expected HighlightMinor for 5%%, got %v", highlight)
	}
}

func TestDetermineHighlight_MajorChange(t *testing.T) {
	pct := 7.0 // 5-10%
	highlight := DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightMinor {
		t.Errorf("expected HighlightMinor for 7%%, got %v", highlight)
	}

	pct = 10.0 // exactly 10%
	highlight = DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightWarning {
		t.Errorf("expected HighlightWarning for 10%%, got %v", highlight)
	}
}

func TestDetermineHighlight_WarningChange(t *testing.T) {
	pct := 15.0 // > 10%
	highlight := DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightWarning {
		t.Errorf("expected HighlightWarning for 15%%, got %v", highlight)
	}
}

func TestDetermineHighlight_NegativeChange(t *testing.T) {
	pct := -12.0 // negative but > 10% magnitude
	highlight := DetermineHighlight(&pct, DefaultThresholds)

	if highlight != HighlightWarning {
		t.Errorf("expected HighlightWarning for -12%%, got %v", highlight)
	}
}

func TestDetermineHighlight_NilDelta(t *testing.T) {
	highlight := DetermineHighlight(nil, DefaultThresholds)

	if highlight != HighlightNone {
		t.Errorf("expected HighlightNone for nil, got %v", highlight)
	}
}

func TestCompareRuns_TwoRuns(t *testing.T) {
	now := time.Now()
	rec1 := makeTestRecord("run-1", 2640.0, []float64{1.0, 2.0, 3.0}, now.Add(-time.Hour))
	rec2 := makeTestRecord("run-2", 2635.0, []float64{1.1, 2.1, 3.1}, now)

	result, err := CompareRuns([]*runlog.RunRecord{rec1, rec2})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result.Runs))
	}

	// Check runs are sorted by timestamp (oldest first)
	if result.Runs[0].ID != "run-1" {
		t.Errorf("expected first run to be run-1, got %s", result.Runs[0].ID)
	}

	// Check OFV row
	if result.OFVRow == nil {
		t.Fatal("expected OFV row")
	}
	if result.OFVRow.Delta == nil || *result.OFVRow.Delta != -5.0 {
		t.Errorf("expected OFV delta=-5, got %v", result.OFVRow.Delta)
	}

	// Check parameter rows exist
	if len(result.ParameterRows) == 0 {
		t.Error("expected parameter rows")
	}
}

func TestCompareRuns_FourRuns(t *testing.T) {
	now := time.Now()
	recs := []*runlog.RunRecord{
		makeTestRecord("run-1", 2650.0, []float64{1.0}, now.Add(-3*time.Hour)),
		makeTestRecord("run-2", 2645.0, []float64{1.1}, now.Add(-2*time.Hour)),
		makeTestRecord("run-3", 2640.0, []float64{1.2}, now.Add(-time.Hour)),
		makeTestRecord("run-4", 2635.0, []float64{1.3}, now),
	}

	result, err := CompareRuns(recs)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Runs) != 4 {
		t.Errorf("expected 4 runs, got %d", len(result.Runs))
	}

	// Delta should be between first and last
	if result.OFVRow.Delta == nil || *result.OFVRow.Delta != -15.0 {
		t.Errorf("expected OFV delta=-15, got %v", result.OFVRow.Delta)
	}
}

func TestCompareRuns_SingleRun(t *testing.T) {
	rec := makeTestRecord("run-1", 2640.0, []float64{1.0}, time.Now())

	_, err := CompareRuns([]*runlog.RunRecord{rec})

	if err == nil {
		t.Error("expected error for single run")
	}
}

func TestCompareRuns_TooManyRuns(t *testing.T) {
	now := time.Now()
	recs := make([]*runlog.RunRecord, 5)
	for i := 0; i < 5; i++ {
		recs[i] = makeTestRecord(fmt.Sprintf("run-%d", i), 2640.0, []float64{1.0}, now)
	}

	_, err := CompareRuns(recs)

	if err == nil {
		t.Error("expected error for too many runs")
	}
}

func TestCompareRuns_NoSummary(t *testing.T) {
	now := time.Now()
	rec1 := &runlog.RunRecord{
		ID:        "run-1",
		Timestamp: now.Add(-time.Hour),
		Status:    "completed",
		Summary:   nil, // No summary
	}
	rec2 := makeTestRecord("run-2", 2635.0, []float64{1.0}, now)

	result, err := CompareRuns([]*runlog.RunRecord{rec1, rec2})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still work, just with nil values for run-1
	if len(result.Runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result.Runs))
	}
}

func TestCompareRuns_DifferentParameterCounts(t *testing.T) {
	now := time.Now()
	rec1 := makeTestRecord("run-1", 2640.0, []float64{1.0, 2.0}, now.Add(-time.Hour))
	rec2 := makeTestRecord("run-2", 2635.0, []float64{1.1, 2.1, 3.0}, now) // 3 thetas vs 2

	result, err := CompareRuns([]*runlog.RunRecord{rec1, rec2})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have rows for all parameters (THETA1, THETA2, THETA3)
	thetaCount := 0
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			thetaCount++
		}
	}

	if thetaCount != 3 {
		t.Errorf("expected 3 theta rows, got %d", thetaCount)
	}
}

func TestFormatDelta(t *testing.T) {
	tests := []struct {
		delta    *float64
		expected string
	}{
		{nil, MissingValueDisplay},
		{ptr(10.0), "+10"},
		{ptr(-5.5), "-5.5"},
		{ptr(0.0), "+0"},
	}

	for _, tt := range tests {
		result := FormatDelta(tt.delta)
		if result != tt.expected {
			t.Errorf("FormatDelta(%v) = %s, expected %s", tt.delta, result, tt.expected)
		}
	}
}

func TestFormatDeltaPct(t *testing.T) {
	tests := []struct {
		deltaPct *float64
		expected string
	}{
		{nil, MissingValueDisplay},
		{ptr(10.5), "+10.5%"},
		{ptr(-5.0), "-5.0%"},
		{ptr(0.0), "+0.0%"},
	}

	for _, tt := range tests {
		result := FormatDeltaPct(tt.deltaPct)
		if result != tt.expected {
			t.Errorf("FormatDeltaPct(%v) = %s, expected %s", tt.deltaPct, result, tt.expected)
		}
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    *float64
		expected string
	}{
		{nil, MissingValueDisplay},
		{ptr(123.456), "123.5"},
		{ptr(0.001234), "0.001234"},
		{ptr(1234567.0), "1.235e+06"},
	}

	for _, tt := range tests {
		result := FormatValue(tt.value)
		if result != tt.expected {
			t.Errorf("FormatValue(%v) = %s, expected %s", tt.value, result, tt.expected)
		}
	}
}

// makeTestRecordWithLabels creates a test RunRecord with labeled parameters.
func makeTestRecordWithLabels(id string, ofv float64, thetas []struct {
	value float64
	label string
}, timestamp time.Time) *runlog.RunRecord {
	thetaParams := make([]model.ParameterEstimate, len(thetas))
	for i, t := range thetas {
		val := t.value
		thetaParams[i] = model.ParameterEstimate{
			Name:     fmt.Sprintf("THETA%d", i+1),
			Label:    t.label,
			Estimate: &val,
			Fixed:    false,
		}
	}

	return &runlog.RunRecord{
		ID:        id,
		Timestamp: timestamp,
		Status:    "completed",
		Summary: &model.ModelSummary{
			GoodnessOfFit: model.GoodnessOfFitSummary{
				ObjectiveFunctionValue: ptr(ofv),
			},
			Estimation: model.EstimationSummary{
				Minimized:    true,
				Method:       "FOCE",
				Subjects:     40,
				Observations: 760,
			},
			Diagnostics: model.DiagnosticSummary{
				CovarianceStepSuccess: true,
			},
			Parameters: model.ParameterSummary{
				Thetas: thetaParams,
			},
		},
	}
}

func TestCorrelationStatus_String(t *testing.T) {
	tests := []struct {
		status   CorrelationStatus
		expected string
	}{
		{CorrelationUnknown, "unknown"},
		{CorrelationKnown, "known"},
		{CorrelationInferred, "inferred"},
		{CorrelationUnrelated, "unrelated"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("CorrelationStatus(%d).String() = %s, expected %s",
				tt.status, tt.status.String(), tt.expected)
		}
	}
}

func TestFormatCorrelationBadge(t *testing.T) {
	tests := []struct {
		status   CorrelationStatus
		expected string
	}{
		{CorrelationKnown, "[KNOWN]"},
		{CorrelationInferred, "[INFER]"},
		{CorrelationUnrelated, "[NEW]"},
		{CorrelationUnknown, ""},
	}

	for _, tt := range tests {
		result := FormatCorrelationBadge(tt.status)
		if result != tt.expected {
			t.Errorf("FormatCorrelationBadge(%v) = %s, expected %s",
				tt.status, result, tt.expected)
		}
	}
}

func TestFormatParameterWithLabel(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		expected string
	}{
		{"THETA1", "CL", "THETA1 (CL)"},
		{"THETA2", "", "THETA2"},
		{"OMEGA(1,1)", "iiv CL", "OMEGA(1,1) (iiv CL)"},
	}

	for _, tt := range tests {
		result := FormatParameterWithLabel(tt.name, tt.label)
		if result != tt.expected {
			t.Errorf("FormatParameterWithLabel(%s, %s) = %s, expected %s",
				tt.name, tt.label, result, tt.expected)
		}
	}
}

func TestCompareRuns_WithLabels_KnownCorrelation(t *testing.T) {
	// Two runs with identical labels - should all be CorrelationKnown
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, "KA"},
		{30.0, "CL"},
		{100.0, "V"},
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, "KA"},
		{32.0, "CL"},
		{105.0, "V"},
	}, now.Add(time.Hour))

	result, err := CompareRuns([]*runlog.RunRecord{run1, run2})
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}

	// All parameters should be CorrelationKnown
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			if row.Correlation != CorrelationKnown {
				t.Errorf("Parameter %s (%s): expected CorrelationKnown, got %v",
					row.Name, row.Label, row.Correlation)
			}
		}
	}
}

func TestCompareRuns_WithoutLabels_UnrelatedCorrelation(t *testing.T) {
	// Two runs without labels - under conservative strategy, should be CorrelationUnrelated
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, ""},
		{30.0, ""},
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, ""},
		{32.0, ""},
	}, now.Add(time.Hour))

	result, err := CompareRuns([]*runlog.RunRecord{run1, run2})
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}

	// All parameters should be CorrelationUnrelated (no labels)
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			if row.Correlation != CorrelationUnrelated {
				t.Errorf("Parameter %s: expected CorrelationUnrelated, got %v",
					row.Name, row.Correlation)
			}
		}
	}
}

func TestCompareRuns_MixedLabels(t *testing.T) {
	// Run 1 has labels, Run 2 has different labels at same positions
	// CL moved from position 2 to position 1
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, "KA"},
		{30.0, "CL"},
	}, now)

	// In run2, CL is now at THETA1 position (labels should still correlate)
	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{31.0, "CL"}, // CL moved to position 1
		{2.2, "KA"},  // KA moved to position 2
		{100.0, "V"}, // New parameter
	}, now.Add(time.Hour))

	result, err := CompareRuns([]*runlog.RunRecord{run1, run2})
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}

	// Find the CL row - should be correlated across runs
	var clRow, kaRow, vRow *ParameterRow
	for i := range result.ParameterRows {
		row := &result.ParameterRows[i]
		switch row.Label {
		case "CL":
			clRow = row
		case "KA":
			kaRow = row
		case "V":
			vRow = row
		}
	}

	// CL should be known correlation (present in both)
	if clRow == nil {
		t.Fatal("expected CL row to exist")
	}
	if clRow.Correlation != CorrelationKnown {
		t.Errorf("CL: expected CorrelationKnown, got %v", clRow.Correlation)
	}
	if clRow.Values[0] == nil || *clRow.Values[0] != 30.0 {
		t.Errorf("CL run1 value: expected 30.0, got %v", clRow.Values[0])
	}
	if clRow.Values[1] == nil || *clRow.Values[1] != 31.0 {
		t.Errorf("CL run2 value: expected 31.0, got %v", clRow.Values[1])
	}

	// KA should be known correlation
	if kaRow == nil {
		t.Fatal("expected KA row to exist")
	}
	if kaRow.Correlation != CorrelationKnown {
		t.Errorf("KA: expected CorrelationKnown, got %v", kaRow.Correlation)
	}

	// V should be known but first seen in run 2
	if vRow == nil {
		t.Fatal("expected V row to exist")
	}
	if vRow.Correlation != CorrelationKnown {
		t.Errorf("V: expected CorrelationKnown, got %v", vRow.Correlation)
	}
	if vRow.FirstSeenIn != 1 {
		t.Errorf("V: expected FirstSeenIn=1, got %d", vRow.FirstSeenIn)
	}
}

func TestCompareRuns_NewParameter(t *testing.T) {
	// Run 2 has an additional parameter
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, "KA"},
		{30.0, "CL"},
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, "KA"},
		{32.0, "CL"},
		{100.0, "V"},
	}, now.Add(time.Hour))

	result, err := CompareRuns([]*runlog.RunRecord{run1, run2})
	if err != nil {
		t.Fatalf("CompareRuns failed: %v", err)
	}

	// Find the V row
	var vRow *ParameterRow
	for i := range result.ParameterRows {
		if result.ParameterRows[i].Label == "V" {
			vRow = &result.ParameterRows[i]

			break
		}
	}

	if vRow == nil {
		t.Fatal("expected V row to exist")
	}

	// V should have nil for run1, value for run2
	if vRow.Values[0] != nil {
		t.Errorf("V run1: expected nil, got %v", vRow.Values[0])
	}
	if vRow.Values[1] == nil || *vRow.Values[1] != 100.0 {
		t.Errorf("V run2: expected 100.0, got %v", vRow.Values[1])
	}

	// V should indicate it was first seen in run 2 (index 1)
	if vRow.FirstSeenIn != 1 {
		t.Errorf("V FirstSeenIn: expected 1, got %d", vRow.FirstSeenIn)
	}
}

func TestCorrelationStrategy_String(t *testing.T) {
	tests := []struct {
		strategy CorrelationStrategy
		expected string
	}{
		{StrategyConservative, "conservative"},
		{StrategyInferred, "inferred"},
		{StrategyPositional, "positional"},
	}

	for _, tt := range tests {
		if tt.strategy.String() != tt.expected {
			t.Errorf("CorrelationStrategy(%d).String() = %s, expected %s",
				tt.strategy, tt.strategy.String(), tt.expected)
		}
	}
}

func TestParseCorrelationStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected CorrelationStrategy
	}{
		{"conservative", StrategyConservative},
		{"inferred", StrategyInferred},
		{"positional", StrategyPositional},
		{"unknown", StrategyConservative}, // Default
		{"", StrategyConservative},        // Default
	}

	for _, tt := range tests {
		result := ParseCorrelationStrategy(tt.input)
		if result != tt.expected {
			t.Errorf("ParseCorrelationStrategy(%s) = %v, expected %v",
				tt.input, result, tt.expected)
		}
	}
}

func TestCompareRunsWithStrategy_Inferred(t *testing.T) {
	// Two runs without labels - with inferred strategy, should get CorrelationInferred
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, ""},
		{30.0, ""},
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, ""},
		{32.0, ""},
	}, now.Add(time.Hour))

	result, err := CompareRunsWithStrategy([]*runlog.RunRecord{run1, run2}, StrategyInferred)
	if err != nil {
		t.Fatalf("CompareRunsWithStrategy failed: %v", err)
	}

	// All parameters should be CorrelationInferred (same position, no labels)
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			if row.Correlation != CorrelationInferred {
				t.Errorf("Parameter %s: expected CorrelationInferred, got %v",
					row.Name, row.Correlation)
			}
		}
	}
}

func TestCompareRunsWithStrategy_Positional(t *testing.T) {
	// Two runs with different labels but same positions - positional should match by position
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, "KA"},       // Position 1
		{30.0, "CL"},      // Position 2
		{100.0, "Volume"}, // Position 3
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, "Absorption"}, // Position 1 - different label
		{32.0, "Clearance"}, // Position 2 - different label
		{105.0, "V"},        // Position 3 - different label
	}, now.Add(time.Hour))

	result, err := CompareRunsWithStrategy([]*runlog.RunRecord{run1, run2}, StrategyPositional)
	if err != nil {
		t.Fatalf("CompareRunsWithStrategy failed: %v", err)
	}

	// Should have exactly 3 rows (matched by position)
	thetaCount := 0
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			thetaCount++
			// All should be CorrelationInferred (positional match)
			if row.Correlation != CorrelationInferred {
				t.Errorf("Parameter %s: expected CorrelationInferred, got %v",
					row.Name, row.Correlation)
			}
			// Each row should have values from both runs
			if row.Values[0] == nil || row.Values[1] == nil {
				t.Errorf("Parameter %s: expected values in both runs", row.Name)
			}
		}
	}

	if thetaCount != 3 {
		t.Errorf("expected 3 theta rows, got %d", thetaCount)
	}
}

func TestCompareRunsWithStrategy_InferredWithMixedLabels(t *testing.T) {
	// Run1 has labels, Run2 doesn't - inferred should still use labels where available
	now := time.Now()
	run1 := makeTestRecordWithLabels("run1", 1000.0, []struct {
		value float64
		label string
	}{
		{2.0, "KA"},
		{30.0, "CL"},
	}, now)

	run2 := makeTestRecordWithLabels("run2", 950.0, []struct {
		value float64
		label string
	}{
		{2.1, ""},  // No label
		{32.0, ""}, // No label
	}, now.Add(time.Hour))

	result, err := CompareRunsWithStrategy([]*runlog.RunRecord{run1, run2}, StrategyInferred)
	if err != nil {
		t.Fatalf("CompareRunsWithStrategy failed: %v", err)
	}

	// Labeled params from run1 should be CorrelationKnown (only in run1)
	// Unlabeled params from run2 should be CorrelationInferred (positionally matched)
	labeledCount := 0
	unlabeledCount := 0
	for _, row := range result.ParameterRows {
		if row.Type == "theta" {
			if row.Label != "" {
				labeledCount++
				// Labeled rows from run1 only = unrelated (no matching label in run2)
				if row.Correlation != CorrelationKnown {
					t.Errorf("Labeled parameter %s: expected CorrelationKnown, got %v",
						row.Name, row.Correlation)
				}
			} else {
				unlabeledCount++
			}
		}
	}

	// With inferred strategy, unlabeled params from run2 get their own rows
	// They should be CorrelationInferred if positions match
	if labeledCount != 2 {
		t.Errorf("expected 2 labeled rows, got %d", labeledCount)
	}
}

func TestNaturalParamLess(t *testing.T) {
	// Each pair: a should sort before b.
	orderedPairs := []struct {
		a, b string
	}{
		{"THETA1", "THETA2"},
		{"THETA2", "THETA10"}, // numeric, not lexicographic
		{"THETA9", "THETA10"},
		{"OMEGA(1,1)", "OMEGA(2,2)"},
		{"OMEGA(2,2)", "OMEGA(10,10)"},
		{"SIGMA(1,1)", "SIGMA(1,2)"},
		{"THETA1", "THETA1A"}, // prefix sorts first
	}

	for _, p := range orderedPairs {
		if !naturalParamLess(p.a, p.b) {
			t.Errorf("expected %q < %q", p.a, p.b)
		}
		if naturalParamLess(p.b, p.a) {
			t.Errorf("expected NOT %q < %q", p.b, p.a)
		}
	}

	// Sorting a shuffled slice yields numeric order.
	names := []string{"THETA10", "THETA2", "THETA1", "THETA20", "THETA3"}
	sort.SliceStable(names, func(i, j int) bool {
		return naturalParamLess(names[i], names[j])
	})
	want := []string{"THETA1", "THETA2", "THETA3", "THETA10", "THETA20"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("sorted[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}
