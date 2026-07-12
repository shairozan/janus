package visualization

import (
	"testing"

	"github.com/pharmalytica/janus/internal/tables"
)

func makeTestGOFData() *tables.GOFData {
	return &tables.GOFData{
		DV:    []float64{5.0, 10.0, 15.0, 20.0, 25.0},
		PRED:  []float64{4.8, 9.5, 14.2, 19.8, 24.5},
		IPRED: []float64{4.9, 9.8, 14.8, 20.1, 25.2},
		TIME:  []float64{0.5, 1.0, 2.0, 4.0, 8.0},
		CWRES: []float64{0.15, 0.32, -0.12, 0.08, -0.25},
		ID:    []float64{1, 1, 1, 1, 1},
	}
}

func TestGenerateDVvsPRED(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()

	result, err := GenerateDVvsPRED(data, opts)
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

func TestGenerateDVvsPRED_MissingData(t *testing.T) {
	data := &tables.GOFData{
		DV: []float64{5.0, 10.0},
		// Missing PRED
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateDVvsPRED(data, opts)
	if err == nil {
		t.Error("expected error for missing PRED data")
	}
}

func TestGenerateDVvsIPRED(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()

	result, err := GenerateDVvsIPRED(data, opts)
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

func TestGenerateDVvsIPRED_MissingData(t *testing.T) {
	data := &tables.GOFData{
		DV: []float64{5.0, 10.0},
		// Missing IPRED
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateDVvsIPRED(data, opts)
	if err == nil {
		t.Error("expected error for missing IPRED data")
	}
}

func TestGenerateCWRESvsTime(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()

	result, err := GenerateCWRESvsTime(data, opts)
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

func TestGenerateCWRESvsTime_MissingData(t *testing.T) {
	data := &tables.GOFData{
		CWRES: []float64{0.1, 0.2},
		// Missing TIME
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateCWRESvsTime(data, opts)
	if err == nil {
		t.Error("expected error for missing TIME data")
	}
}

func TestGenerateCWRESvsPRED(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()

	result, err := GenerateCWRESvsPRED(data, opts)
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

func TestGenerateCWRESHistogram(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()

	result, err := GenerateCWRESHistogram(data, opts)
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

func TestGenerateCWRESHistogram_EmptyData(t *testing.T) {
	data := &tables.GOFData{
		CWRES: []float64{},
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateCWRESHistogram(data, opts)
	if err == nil {
		t.Error("expected error for empty CWRES data")
	}
}

func TestGenerateETAHistogram(t *testing.T) {
	etaData := &tables.ETAData{
		Name:   "ETA1",
		Values: []float64{0.1, 0.2, -0.1, 0.15, -0.05, 0.08, 0.12, -0.08},
		IDs:    []float64{1, 2, 3, 4, 5, 6, 7, 8},
	}
	opts := DefaultGOFPlotOptions()

	result, err := GenerateETAHistogram(etaData, opts)
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

func TestGenerateETAHistogram_EmptyData(t *testing.T) {
	etaData := &tables.ETAData{
		Name:   "ETA1",
		Values: []float64{},
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateETAHistogram(etaData, opts)
	if err == nil {
		t.Error("expected error for empty ETA data")
	}
}

func TestGenerateBasicGOFPanel(t *testing.T) {
	data := makeTestGOFData()
	opts := DefaultGOFPlotOptions()
	opts.Width = 800
	opts.Height = 800

	panel, err := GenerateBasicGOFPanel(data, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if panel.DVvsPRED == nil {
		t.Error("expected non-nil DVvsPRED")
	}

	if panel.DVvsIPRED == nil {
		t.Error("expected non-nil DVvsIPRED")
	}

	if panel.CWRESvsTime == nil {
		t.Error("expected non-nil CWRESvsTime")
	}

	if panel.CWRESvsPRED == nil {
		t.Error("expected non-nil CWRESvsPRED")
	}
}

func TestGenerateBasicGOFPanel_PartialData(t *testing.T) {
	// Only DV and PRED, no IPRED or CWRES
	data := &tables.GOFData{
		DV:   []float64{5.0, 10.0, 15.0},
		PRED: []float64{4.8, 9.5, 14.2},
	}
	opts := DefaultGOFPlotOptions()

	panel, err := GenerateBasicGOFPanel(data, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if panel.DVvsPRED == nil {
		t.Error("expected non-nil DVvsPRED")
	}

	// These should be nil since data is missing
	if panel.DVvsIPRED != nil {
		t.Error("expected nil DVvsIPRED without IPRED data")
	}

	if panel.CWRESvsTime != nil {
		t.Error("expected nil CWRESvsTime without CWRES data")
	}
}

func TestGenerateCWRESQQPlot(t *testing.T) {
	data := &tables.GOFData{
		CWRES: []float64{-1.5, -1.0, -0.5, 0.0, 0.5, 1.0, 1.5, -0.8, 0.3, 0.7},
	}
	opts := DefaultGOFPlotOptions()

	result, err := GenerateCWRESQQPlot(data, opts)
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

func TestGenerateCWRESQQPlot_InsufficientData(t *testing.T) {
	data := &tables.GOFData{
		CWRES: []float64{0.1, 0.2}, // Only 2 points
	}
	opts := DefaultGOFPlotOptions()

	_, err := GenerateCWRESQQPlot(data, opts)
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestDefaultGOFPlotOptions(t *testing.T) {
	opts := DefaultGOFPlotOptions()

	if opts.Width != 400 {
		t.Errorf("expected default width 400, got %d", opts.Width)
	}

	if opts.Height != 400 {
		t.Errorf("expected default height 400, got %d", opts.Height)
	}

	if !opts.ShowUnity {
		t.Error("expected ShowUnity = true by default")
	}

	if !opts.ShowZero {
		t.Error("expected ShowZero = true by default")
	}
}

func TestMinMax(t *testing.T) {
	testCases := []struct {
		data        []float64
		expectedMin float64
		expectedMax float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 1, 5},
		{[]float64{5, 4, 3, 2, 1}, 1, 5},
		{[]float64{-5, 0, 5}, -5, 5},
		{[]float64{3.14}, 3.14, 3.14},
	}

	for _, tc := range testCases {
		min, max := minMax(tc.data)
		if min != tc.expectedMin {
			t.Errorf("minMax(%v): expected min %f, got %f", tc.data, tc.expectedMin, min)
		}
		if max != tc.expectedMax {
			t.Errorf("minMax(%v): expected max %f, got %f", tc.data, tc.expectedMax, max)
		}
	}
}

func TestMinMax_Empty(t *testing.T) {
	min, max := minMax([]float64{})
	if min != 0 || max != 0 {
		t.Errorf("expected (0, 0) for empty slice, got (%f, %f)", min, max)
	}
}

func TestCreateHistogramBins(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	bins, counts := createHistogramBins(data, 5)

	if len(bins) != 5 {
		t.Errorf("expected 5 bins, got %d", len(bins))
	}

	if len(counts) != 5 {
		t.Errorf("expected 5 counts, got %d", len(counts))
	}

	// Sum of counts should equal number of data points
	var total float64
	for _, c := range counts {
		total += c
	}
	if total != float64(len(data)) {
		t.Errorf("expected total count %d, got %f", len(data), total)
	}
}

func TestCreateHistogramBins_SingleValue(t *testing.T) {
	data := []float64{5, 5, 5, 5}

	bins, counts := createHistogramBins(data, 10)

	if len(bins) != 1 {
		t.Errorf("expected 1 bin for identical values, got %d", len(bins))
	}

	if counts[0] != 4 {
		t.Errorf("expected count 4, got %f", counts[0])
	}
}

func TestNormalQuantile(t *testing.T) {
	testCases := []struct {
		p         float64
		expected  float64
		tolerance float64
	}{
		{0.5, 0.0, 0.01},     // Median
		{0.025, -1.96, 0.01}, // Lower 2.5%
		{0.975, 1.96, 0.01},  // Upper 2.5%
		{0.1587, -1.0, 0.01}, // ~-1 SD
		{0.8413, 1.0, 0.01},  // ~+1 SD
	}

	for _, tc := range testCases {
		result := normalQuantile(tc.p)
		diff := result - tc.expected
		if diff < -tc.tolerance || diff > tc.tolerance {
			t.Errorf("normalQuantile(%f) = %f, expected ~%f", tc.p, result, tc.expected)
		}
	}
}
