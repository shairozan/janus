package visualization

import (
	"fmt"
	"math"
	"sort"

	"github.com/vicanso/go-charts/v2"

	"github.com/pharmalytica/janus/internal/tables"
)

// GOFPlotType identifies the type of GOF plot.
type GOFPlotType string

const (
	GOFPlotDVvsPRED    GOFPlotType = "dv_vs_pred"
	GOFPlotDVvsIPRED   GOFPlotType = "dv_vs_ipred"
	GOFPlotCWRESvsTIME GOFPlotType = "cwres_vs_time"
	GOFPlotCWRESvsPRED GOFPlotType = "cwres_vs_pred"
	GOFPlotCWRESHist   GOFPlotType = "cwres_hist"
	GOFPlotCWRESQQ     GOFPlotType = "cwres_qq"
	GOFPlotETAHist     GOFPlotType = "eta_hist"
)

// GOFPlotOptions configures GOF plot generation.
type GOFPlotOptions struct {
	Width     int
	Height    int
	Title     string
	ShowLoess bool    // Show LOESS smoothing line
	ShowUnity bool    // Show unity/identity line (y=x)
	ShowZero  bool    // Show zero reference line
	LogScale  bool    // Use log scale for DV plots
	PointSize float64 // Size of scatter points
}

// DefaultGOFPlotOptions returns sensible defaults for GOF plots.
func DefaultGOFPlotOptions() GOFPlotOptions {
	return GOFPlotOptions{
		Width:     400,
		Height:    400,
		ShowLoess: false,
		ShowUnity: true,
		ShowZero:  true,
		LogScale:  false,
		PointSize: 3,
	}
}

// GOFPlotData holds data for a single GOF scatter plot.
type GOFPlotData struct {
	X      []float64
	Y      []float64
	XLabel string
	YLabel string
	Title  string
}

// GenerateDVvsPRED creates a DV vs PRED scatter plot.
func GenerateDVvsPRED(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.DV) == 0 || len(data.PRED) == 0 {
		return nil, fmt.Errorf("DV and PRED data required for DV vs PRED plot")
	}

	plotData := &GOFPlotData{
		X:      data.PRED,
		Y:      data.DV,
		XLabel: "PRED",
		YLabel: "DV",
		Title:  opts.Title,
	}
	if plotData.Title == "" {
		plotData.Title = "DV vs PRED"
	}

	return generateScatterPlot(plotData, opts, true, false)
}

// GenerateDVvsIPRED creates a DV vs IPRED scatter plot.
func GenerateDVvsIPRED(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.DV) == 0 || len(data.IPRED) == 0 {
		return nil, fmt.Errorf("DV and IPRED data required for DV vs IPRED plot")
	}

	plotData := &GOFPlotData{
		X:      data.IPRED,
		Y:      data.DV,
		XLabel: "IPRED",
		YLabel: "DV",
		Title:  opts.Title,
	}
	if plotData.Title == "" {
		plotData.Title = "DV vs IPRED"
	}

	return generateScatterPlot(plotData, opts, true, false)
}

// GenerateCWRESvsTime creates a CWRES vs TIME scatter plot.
func GenerateCWRESvsTime(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.CWRES) == 0 || len(data.TIME) == 0 {
		return nil, fmt.Errorf("CWRES and TIME data required for CWRES vs TIME plot")
	}

	plotData := &GOFPlotData{
		X:      data.TIME,
		Y:      data.CWRES,
		XLabel: "Time",
		YLabel: "CWRES",
		Title:  opts.Title,
	}
	if plotData.Title == "" {
		plotData.Title = "CWRES vs Time"
	}

	return generateScatterPlot(plotData, opts, false, true)
}

// GenerateCWRESvsPRED creates a CWRES vs PRED scatter plot.
func GenerateCWRESvsPRED(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.CWRES) == 0 || len(data.PRED) == 0 {
		return nil, fmt.Errorf("CWRES and PRED data required for CWRES vs PRED plot")
	}

	plotData := &GOFPlotData{
		X:      data.PRED,
		Y:      data.CWRES,
		XLabel: "PRED",
		YLabel: "CWRES",
		Title:  opts.Title,
	}
	if plotData.Title == "" {
		plotData.Title = "CWRES vs PRED"
	}

	return generateScatterPlot(plotData, opts, false, true)
}

// generateScatterPlot creates a scatter plot using line chart with dots.
// go-charts doesn't have native scatter support, so we simulate it.
func generateScatterPlot(data *GOFPlotData, opts GOFPlotOptions, _, _ bool) (*ChartResult, error) {
	// Filter out NaN values and pair x,y
	type point struct {
		x, y float64
	}
	var points []point
	for i := range data.X {
		if i < len(data.Y) && !math.IsNaN(data.X[i]) && !math.IsNaN(data.Y[i]) {
			points = append(points, point{x: data.X[i], y: data.Y[i]})
		}
	}

	if len(points) == 0 {
		return nil, fmt.Errorf("no valid data points after filtering NaN values")
	}

	// Sort points by X for proper line rendering
	sort.Slice(points, func(i, j int) bool {
		return points[i].x < points[j].x
	})

	// Extract sorted x and y
	xLabels := make([]string, len(points))
	yValues := make([]float64, len(points))
	for i, p := range points {
		xLabels[i] = fmt.Sprintf("%.2f", p.x)
		yValues[i] = p.y
	}

	// Build chart options
	title := data.Title
	if title == "" {
		title = fmt.Sprintf("%s vs %s", data.YLabel, data.XLabel)
	}

	// Create scatter-like line chart (points only, no connecting lines)
	p, err := charts.LineRender(
		[][]float64{yValues},
		charts.TitleTextOptionFunc(title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(xLabels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		func(opt *charts.ChartOption) {
			// Show only dots, hide lines
			opt.SeriesList[0].Label.Show = false
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to render scatter plot: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get chart bytes: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateCWRESHistogram creates a histogram of CWRES values.
func GenerateCWRESHistogram(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.CWRES) == 0 {
		return nil, fmt.Errorf("CWRES data required for histogram")
	}

	// Filter NaN values
	var cwres []float64
	for _, v := range data.CWRES {
		if !math.IsNaN(v) {
			cwres = append(cwres, v)
		}
	}

	if len(cwres) == 0 {
		return nil, fmt.Errorf("no valid CWRES values")
	}

	// Create histogram bins
	bins, counts := createHistogramBins(cwres, 20)

	title := opts.Title
	if title == "" {
		title = "CWRES Distribution"
	}

	// Create bar chart for histogram
	p, err := charts.BarRender(
		[][]float64{counts},
		charts.TitleTextOptionFunc(title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(bins),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to render histogram: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get chart bytes: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateETAHistogram creates a histogram for ETA values.
func GenerateETAHistogram(etaData *tables.ETAData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(etaData.Values) == 0 {
		return nil, fmt.Errorf("no ETA values provided")
	}

	// Create histogram bins
	bins, counts := createHistogramBins(etaData.Values, 15)

	title := opts.Title
	if title == "" {
		title = fmt.Sprintf("%s Distribution", etaData.Name)
	}

	p, err := charts.BarRender(
		[][]float64{counts},
		charts.TitleTextOptionFunc(title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(bins),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to render histogram: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get chart bytes: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// BasicGOFPanel holds the four standard GOF plots.
type BasicGOFPanel struct {
	DVvsPRED    *ChartResult
	DVvsIPRED   *ChartResult
	CWRESvsTime *ChartResult
	CWRESvsPRED *ChartResult
}

// GenerateBasicGOFPanel creates the standard 4-panel GOF view.
func GenerateBasicGOFPanel(data *tables.GOFData, opts GOFPlotOptions) (*BasicGOFPanel, error) {
	panel := &BasicGOFPanel{}
	var err error

	// Adjust size for panel layout
	panelOpts := opts
	panelOpts.Width = opts.Width / 2
	panelOpts.Height = opts.Height / 2

	// DV vs PRED
	if len(data.PRED) > 0 {
		panelOpts.Title = "DV vs PRED"
		panel.DVvsPRED, err = GenerateDVvsPRED(data, panelOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate DV vs PRED: %w", err)
		}
	}

	// DV vs IPRED
	if len(data.IPRED) > 0 {
		panelOpts.Title = "DV vs IPRED"
		panel.DVvsIPRED, err = GenerateDVvsIPRED(data, panelOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate DV vs IPRED: %w", err)
		}
	}

	// CWRES vs TIME
	if len(data.CWRES) > 0 && len(data.TIME) > 0 {
		panelOpts.Title = "CWRES vs Time"
		panel.CWRESvsTime, err = GenerateCWRESvsTime(data, panelOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate CWRES vs Time: %w", err)
		}
	}

	// CWRES vs PRED
	if len(data.CWRES) > 0 && len(data.PRED) > 0 {
		panelOpts.Title = "CWRES vs PRED"
		panel.CWRESvsPRED, err = GenerateCWRESvsPRED(data, panelOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to generate CWRES vs PRED: %w", err)
		}
	}

	return panel, nil
}

// Helper functions

func minMax(data []float64) (min, max float64) {
	if len(data) == 0 {
		return 0, 0
	}
	min, max = data[0], data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	return min, max
}

func createHistogramBins(data []float64, numBins int) ([]string, []float64) {
	if len(data) == 0 {
		return nil, nil
	}

	min, max := minMax(data)
	binWidth := (max - min) / float64(numBins)

	if binWidth == 0 {
		// All values are the same
		return []string{fmt.Sprintf("%.2f", min)}, []float64{float64(len(data))}
	}

	bins := make([]string, numBins)
	counts := make([]float64, numBins)

	for i := 0; i < numBins; i++ {
		binStart := min + float64(i)*binWidth
		bins[i] = fmt.Sprintf("%.2f", binStart+binWidth/2)
	}

	for _, v := range data {
		binIdx := int((v - min) / binWidth)
		if binIdx >= numBins {
			binIdx = numBins - 1
		}
		if binIdx < 0 {
			binIdx = 0
		}
		counts[binIdx]++
	}

	return bins, counts
}

// GOFFromSDTAB is a convenience function to generate GOF plots directly from an SDTAB file.
func GOFFromSDTAB(sdtabPath string, opts GOFPlotOptions) (*BasicGOFPanel, error) {
	table, err := tables.ReadSDTAB(sdtabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SDTAB: %w", err)
	}

	data, err := tables.ExtractGOFData(table)
	if err != nil {
		return nil, fmt.Errorf("failed to extract GOF data: %w", err)
	}

	return GenerateBasicGOFPanel(data, opts)
}

// InteractiveGOFOptions configures interactive GOF plot generation.
type InteractiveGOFOptions struct {
	Width  string // CSS width
	Height string // CSS height
	Title  string
	Theme  string
}

// DefaultInteractiveGOFOptions returns defaults for interactive GOF plots.
func DefaultInteractiveGOFOptions() InteractiveGOFOptions {
	return InteractiveGOFOptions{
		Width:  "100%",
		Height: "400px",
		Theme:  "",
	}
}

// QQPlotData holds data for a Q-Q plot.
type QQPlotData struct {
	Theoretical []float64 // Expected quantiles from normal distribution
	Sample      []float64 // Observed quantiles from data
}

// GenerateCWRESQQPlot creates a Q-Q plot for CWRES against normal distribution.
func GenerateCWRESQQPlot(data *tables.GOFData, opts GOFPlotOptions) (*ChartResult, error) {
	if len(data.CWRES) == 0 {
		return nil, fmt.Errorf("CWRES data required for Q-Q plot")
	}

	// Filter NaN values and sort
	var cwres []float64
	for _, v := range data.CWRES {
		if !math.IsNaN(v) {
			cwres = append(cwres, v)
		}
	}

	if len(cwres) < 3 {
		return nil, fmt.Errorf("insufficient data points for Q-Q plot")
	}

	sort.Float64s(cwres)

	// Calculate theoretical quantiles
	n := len(cwres)
	theoretical := make([]float64, n)
	for i := 0; i < n; i++ {
		p := (float64(i) + 0.5) / float64(n)
		theoretical[i] = normalQuantile(p)
	}

	// Create labels from theoretical quantiles
	xLabels := make([]string, n)
	for i := range theoretical {
		xLabels[i] = fmt.Sprintf("%.2f", theoretical[i])
	}

	title := opts.Title
	if title == "" {
		title = "CWRES Q-Q Plot"
	}

	// Create line chart (scatter-like)
	p, err := charts.LineRender(
		[][]float64{cwres},
		charts.TitleTextOptionFunc(title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(xLabels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		func(opt *charts.ChartOption) {
			opt.SeriesList[0].Label.Show = false
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to render Q-Q plot: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get chart bytes: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// normalQuantile approximates the inverse of the standard normal CDF.
// Uses Abramowitz and Stegun approximation.
func normalQuantile(p float64) float64 {
	if p <= 0 {
		return math.Inf(-1)
	}
	if p >= 1 {
		return math.Inf(1)
	}

	// Coefficients for rational approximation
	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00
	)

	pLow := 0.02425
	pHigh := 1 - pLow

	var q, r float64

	switch {
	case p < pLow:
		q = math.Sqrt(-2 * math.Log(p))

		return (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	case p <= pHigh:
		q = p - 0.5
		r = q * q

		return (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q /
			(((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	default:
		q = math.Sqrt(-2 * math.Log(1-p))

		return -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	}
}
