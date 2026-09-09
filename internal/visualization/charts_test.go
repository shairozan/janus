package visualization

import (
	"testing"
	"time"

	"github.com/shairozan/janus/internal/model"
	"github.com/shairozan/janus/internal/runlog"
)

// Helper to create a pointer to a float64.
func ptr(f float64) *float64 {
	return &f
}

// makeTestRunRecord creates a test RunRecord with the given parameters.
func makeTestRunRecord(id string, ofv float64, thetas []float64, timestamp time.Time) *runlog.RunRecord {
	thetaParams := make([]model.ParameterEstimate, len(thetas))
	for i, v := range thetas {
		val := v
		thetaParams[i] = model.ParameterEstimate{
			Name:     "THETA" + string(rune('1'+i)),
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

func TestExtractOFVData(t *testing.T) {
	now := time.Now()
	records := []*runlog.RunRecord{
		makeTestRunRecord("run-1", 2640.0, []float64{1.0}, now.Add(-2*time.Hour)),
		makeTestRunRecord("run-2", 2635.0, []float64{1.1}, now.Add(-time.Hour)),
		makeTestRunRecord("run-3", 2630.0, []float64{1.2}, now),
	}

	data := ExtractOFVData(records)

	if len(data) != 3 {
		t.Errorf("expected 3 data points, got %d", len(data))
	}

	// Check values
	expectedOFVs := []float64{2640.0, 2635.0, 2630.0}
	for i, dp := range data {
		if dp.OFV != expectedOFVs[i] {
			t.Errorf("data point %d: expected OFV=%f, got %f", i, expectedOFVs[i], dp.OFV)
		}
	}
}

func TestExtractOFVData_NoSummary(t *testing.T) {
	now := time.Now()
	records := []*runlog.RunRecord{
		{ID: "run-1", Timestamp: now, Summary: nil},
		makeTestRunRecord("run-2", 2635.0, []float64{1.1}, now.Add(time.Hour)),
	}

	data := ExtractOFVData(records)

	// Should only have 1 data point (the one with summary)
	if len(data) != 1 {
		t.Errorf("expected 1 data point, got %d", len(data))
	}
}

func TestExtractParameterData(t *testing.T) {
	now := time.Now()
	records := []*runlog.RunRecord{
		makeTestRunRecord("run-1", 2640.0, []float64{1.0, 2.0}, now.Add(-time.Hour)),
		makeTestRunRecord("run-2", 2635.0, []float64{1.1, 2.1}, now),
	}

	data := ExtractParameterData(records, "THETA1")

	if len(data) != 2 {
		t.Errorf("expected 2 data points, got %d", len(data))
	}

	// Check values
	if data[0].Value == nil || *data[0].Value != 1.0 {
		t.Errorf("expected first value=1.0, got %v", data[0].Value)
	}

	if data[1].Value == nil || *data[1].Value != 1.1 {
		t.Errorf("expected second value=1.1, got %v", data[1].Value)
	}
}

func TestExtractParameterData_NotFound(t *testing.T) {
	now := time.Now()
	records := []*runlog.RunRecord{
		makeTestRunRecord("run-1", 2640.0, []float64{1.0}, now),
	}

	data := ExtractParameterData(records, "NONEXISTENT")

	if len(data) != 0 {
		t.Errorf("expected 0 data points for nonexistent parameter, got %d", len(data))
	}
}

func TestGetAllParameterNames(t *testing.T) {
	now := time.Now()
	records := []*runlog.RunRecord{
		makeTestRunRecord("run-1", 2640.0, []float64{1.0, 2.0}, now.Add(-time.Hour)),
		makeTestRunRecord("run-2", 2635.0, []float64{1.1, 2.1, 3.0}, now),
	}

	names := GetAllParameterNames(records)

	// Should have THETA1, THETA2, THETA3, OMEGA(1,1), SIGMA(1,1)
	if len(names) < 5 {
		t.Errorf("expected at least 5 parameter names, got %d: %v", len(names), names)
	}

	// Check some expected names exist
	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}

	expectedNames := []string{"THETA1", "THETA2", "OMEGA(1,1)", "SIGMA(1,1)"}
	for _, expected := range expectedNames {
		if !nameMap[expected] {
			t.Errorf("expected parameter name %q not found in %v", expected, names)
		}
	}
}

func TestGenerateOFVTrendChart(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, OFV: 2640.0, Minimized: true},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, OFV: 2635.0, Minimized: true},
	}

	opts := DefaultChartOptions()
	opts.Title = "Test OFV Trend"

	result, err := GenerateOFVTrendChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}

	if result.Format != FormatPNG {
		t.Errorf("expected PNG format, got %s", result.Format)
	}
}

func TestGenerateOFVTrendChart_EmptyData(t *testing.T) {
	opts := DefaultChartOptions()

	_, err := GenerateOFVTrendChart([]OFVDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestGenerateParameterBarChart(t *testing.T) {
	now := time.Now()
	val1, val2 := 1.0, 1.1
	data := []ParameterDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, Name: "THETA1", Value: &val1},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA1", Value: &val2},
	}

	opts := DefaultChartOptions()

	result, err := GenerateParameterBarChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}
}

func TestGenerateParameterBarChart_EmptyData(t *testing.T) {
	opts := DefaultChartOptions()

	_, err := GenerateParameterBarChart([]ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestGenerateConvergenceChart(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-2 * time.Hour), Label: "run-1"}, OFV: 2640.0},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now.Add(-time.Hour), Label: "run-2"}, OFV: 2635.0},
		{RunDataPoint: RunDataPoint{RunID: "run-3", Timestamp: now, Label: "run-3"}, OFV: 2630.0},
	}

	opts := DefaultChartOptions()
	opts.Title = "Test Convergence"

	result, err := GenerateConvergenceChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}
}

func TestGenerateConvergenceChart_InsufficientData(t *testing.T) {
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Label: "run-1"}, OFV: 2640.0},
	}

	opts := DefaultChartOptions()

	_, err := GenerateConvergenceChart(data, opts)

	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestGenerateDeltaChart(t *testing.T) {
	now := time.Now()
	val1, val2 := 1.0, 1.5
	baseData := []ParameterDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now, Label: "run-1"}, Name: "THETA1", Value: &val1},
	}
	compareData := []ParameterDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA1", Value: &val2},
	}

	opts := DefaultChartOptions()

	result, err := GenerateDeltaChart(baseData, compareData, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}
}

func TestGenerateDeltaChart_EmptyData(t *testing.T) {
	opts := DefaultChartOptions()

	_, err := GenerateDeltaChart([]ParameterDataPoint{}, []ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestGeneratePieChart(t *testing.T) {
	labels := []string{"Category A", "Category B", "Category C"}
	values := []float64{30.0, 50.0, 20.0}

	opts := DefaultChartOptions()
	opts.Title = "Test Distribution"

	result, err := GeneratePieChart(labels, values, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}
}

func TestGeneratePieChart_MismatchedLength(t *testing.T) {
	labels := []string{"A", "B"}
	values := []float64{1.0, 2.0, 3.0} // Different length

	opts := DefaultChartOptions()

	_, err := GeneratePieChart(labels, values, opts)

	if err == nil {
		t.Error("expected error for mismatched labels/values")
	}
}

func TestGeneratePieChart_EmptyData(t *testing.T) {
	opts := DefaultChartOptions()

	_, err := GeneratePieChart([]string{}, []float64{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestRenderToSVG(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, OFV: 2640.0},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, OFV: 2635.0},
	}

	opts := DefaultChartOptions()

	result, err := RenderToSVG(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Format != FormatSVG {
		t.Errorf("expected SVG format, got %s", result.Format)
	}

	// SVG should contain "<svg" tag
	if len(result.Data) < 10 {
		t.Error("expected larger SVG output")
	}
}

func TestSaveChart(t *testing.T) {
	result := &ChartResult{
		Data:   []byte("test chart data"),
		Format: FormatPNG,
		Width:  800,
		Height: 400,
	}

	buf, err := SaveChart(result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "test chart data" {
		t.Errorf("expected 'test chart data', got %q", buf.String())
	}
}

func TestSaveChart_Nil(t *testing.T) {
	_, err := SaveChart(nil)

	if err == nil {
		t.Error("expected error for nil result")
	}
}

func TestDefaultChartOptions(t *testing.T) {
	opts := DefaultChartOptions()

	if opts.Width != 800 {
		t.Errorf("expected default width=800, got %d", opts.Width)
	}

	if opts.Height != 400 {
		t.Errorf("expected default height=400, got %d", opts.Height)
	}

	if opts.Format != FormatPNG {
		t.Errorf("expected default format=PNG, got %s", opts.Format)
	}

	if !opts.ShowLegend {
		t.Error("expected ShowLegend=true by default")
	}

	if !opts.ShowGrid {
		t.Error("expected ShowGrid=true by default")
	}
}

func TestGenerateMultiParameterChart(t *testing.T) {
	now := time.Now()
	val1, val2 := 1.0, 1.1
	val3, val4 := 2.0, 2.1

	dataByParam := map[string][]ParameterDataPoint{
		"THETA1": {
			{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, Name: "THETA1", Value: &val1},
			{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA1", Value: &val2},
		},
		"THETA2": {
			{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, Name: "THETA2", Value: &val3},
			{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA2", Value: &val4},
		},
	}

	opts := DefaultChartOptions()
	opts.Title = "Multi-Parameter Test"

	result, err := GenerateMultiParameterChart([]string{"THETA1", "THETA2"}, dataByParam, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Data) == 0 {
		t.Error("expected non-empty chart data")
	}
}

func TestGenerateMultiParameterChart_NoParams(t *testing.T) {
	opts := DefaultChartOptions()

	_, err := GenerateMultiParameterChart([]string{}, map[string][]ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty parameters")
	}
}
