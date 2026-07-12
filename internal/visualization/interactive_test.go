package visualization

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultInteractiveOptions(t *testing.T) {
	opts := DefaultInteractiveOptions()

	if opts.Width != "100%" {
		t.Errorf("expected default width='100%%', got %q", opts.Width)
	}

	if opts.Height != "500px" {
		t.Errorf("expected default height='500px', got %q", opts.Height)
	}
}

func TestGenerateInteractiveOFVChart(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, OFV: 2640.0, Minimized: true},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, OFV: 2635.0, Minimized: true},
	}

	opts := DefaultInteractiveOptions()
	opts.Title = "Test Interactive OFV"

	chart, err := GenerateInteractiveOFVChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chart == nil {
		t.Fatal("expected non-nil chart")
	}
}

func TestGenerateInteractiveOFVChart_EmptyData(t *testing.T) {
	opts := DefaultInteractiveOptions()

	_, err := GenerateInteractiveOFVChart([]OFVDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestGenerateInteractiveParameterChart(t *testing.T) {
	now := time.Now()
	val1, val2 := 1.0, 1.1
	data := []ParameterDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, Name: "THETA1", Value: &val1},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA1", Value: &val2},
	}

	opts := DefaultInteractiveOptions()
	opts.Title = "Test Interactive Parameter"

	chart, err := GenerateInteractiveParameterChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chart == nil {
		t.Fatal("expected non-nil chart")
	}
}

func TestGenerateInteractiveParameterChart_EmptyData(t *testing.T) {
	opts := DefaultInteractiveOptions()

	_, err := GenerateInteractiveParameterChart([]ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestGenerateInteractiveMultiParameterChart(t *testing.T) {
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

	opts := DefaultInteractiveOptions()
	opts.Title = "Test Multi-Parameter"

	chart, err := GenerateInteractiveMultiParameterChart([]string{"THETA1", "THETA2"}, dataByParam, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chart == nil {
		t.Fatal("expected non-nil chart")
	}
}

func TestGenerateInteractiveMultiParameterChart_NoParams(t *testing.T) {
	opts := DefaultInteractiveOptions()

	_, err := GenerateInteractiveMultiParameterChart([]string{}, map[string][]ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty parameters")
	}
}

func TestGenerateInteractiveMultiParameterChart_NoData(t *testing.T) {
	opts := DefaultInteractiveOptions()

	_, err := GenerateInteractiveMultiParameterChart([]string{"THETA1"}, map[string][]ParameterDataPoint{}, opts)

	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestSaveInteractiveChart(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, OFV: 2640.0},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, OFV: 2635.0},
	}

	opts := DefaultInteractiveOptions()
	chart, err := GenerateInteractiveOFVChart(data, opts)
	if err != nil {
		t.Fatalf("failed to generate chart: %v", err)
	}

	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_chart.html")

	err = SaveInteractiveChart(chart, tmpFile)
	if err != nil {
		t.Fatalf("failed to save chart: %v", err)
	}

	// Verify file exists and has content
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat saved file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("expected non-empty HTML file")
	}

	// Read and verify it's HTML
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	// Should contain HTML tags
	if len(content) < 100 {
		t.Error("expected larger HTML output")
	}
}

func TestSaveInteractiveChart_InvalidPath(t *testing.T) {
	now := time.Now()
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now, Label: "run-1"}, OFV: 2640.0},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now.Add(time.Hour), Label: "run-2"}, OFV: 2635.0},
	}

	opts := DefaultInteractiveOptions()
	chart, err := GenerateInteractiveOFVChart(data, opts)
	if err != nil {
		t.Fatalf("failed to generate chart: %v", err)
	}

	// Try to save to invalid path
	err = SaveInteractiveChart(chart, "/nonexistent/path/chart.html")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestShutdownAllChartServers(_ *testing.T) {
	// Just verify it doesn't panic when called with no active servers
	ShutdownAllChartServers()
}

func TestInteractiveOFVChartSortsData(t *testing.T) {
	now := time.Now()
	// Provide data out of order
	data := []OFVDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-3", Timestamp: now, Label: "run-3"}, OFV: 2630.0},
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-2 * time.Hour), Label: "run-1"}, OFV: 2640.0},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now.Add(-time.Hour), Label: "run-2"}, OFV: 2635.0},
	}

	opts := DefaultInteractiveOptions()
	chart, err := GenerateInteractiveOFVChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chart == nil {
		t.Fatal("expected non-nil chart")
	}

	// The chart should still be generated (data is sorted internally)
	// We can't easily verify the internal ordering, but we ensure no errors
}

func TestInteractiveParameterChartWithNilValues(t *testing.T) {
	now := time.Now()
	val1 := 1.0
	data := []ParameterDataPoint{
		{RunDataPoint: RunDataPoint{RunID: "run-1", Timestamp: now.Add(-time.Hour), Label: "run-1"}, Name: "THETA1", Value: &val1},
		{RunDataPoint: RunDataPoint{RunID: "run-2", Timestamp: now, Label: "run-2"}, Name: "THETA1", Value: nil}, // nil value
	}

	opts := DefaultInteractiveOptions()
	chart, err := GenerateInteractiveParameterChart(data, opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chart == nil {
		t.Fatal("expected non-nil chart")
	}
}
