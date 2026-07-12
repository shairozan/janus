package visualization

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/vicanso/go-charts/v2"
)

// GenerateOFVTrendChart creates a line chart showing OFV progression across runs.
func GenerateOFVTrendChart(data []OFVDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	// Sort by timestamp
	sortedData := make([]OFVDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Extract labels and values
	labels := make([]string, len(sortedData))
	values := make([]float64, len(sortedData))
	for i, dp := range sortedData {
		labels[i] = dp.Label
		values[i] = dp.OFV
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "OFV Trend Across Runs"
	}

	// Create chart
	p, err := charts.LineRender(
		[][]float64{values},
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(labels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		charts.YAxisOptionFunc(charts.YAxisOption{
			Min: getAxisMin(values),
		}),
		func(opt *charts.ChartOption) {
			opt.SeriesList[0].Label.Show = true
			opt.SeriesList[0].MarkPoint = charts.NewMarkPoint(
				charts.SeriesMarkDataTypeMin,
				charts.SeriesMarkDataTypeMax,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OFV trend chart: %w", err)
	}

	// Render to buffer
	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateParameterBarChart creates a bar chart comparing a parameter across runs.
func GenerateParameterBarChart(data []ParameterDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	// Sort by timestamp
	sortedData := make([]ParameterDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Extract labels and values
	labels := make([]string, len(sortedData))
	values := make([]float64, len(sortedData))
	for i, dp := range sortedData {
		labels[i] = dp.Label
		if dp.Value != nil {
			values[i] = *dp.Value
		} else {
			values[i] = math.NaN()
		}
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" && len(data) > 0 {
		opts.Title = fmt.Sprintf("%s Across Runs", data[0].Name)
	}

	// Create chart
	p, err := charts.BarRender(
		[][]float64{values},
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(labels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		func(opt *charts.ChartOption) {
			opt.SeriesList[0].Label.Show = true
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create parameter bar chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateMultiParameterChart creates a grouped bar chart comparing multiple parameters.
func GenerateMultiParameterChart(paramNames []string, dataByParam map[string][]ParameterDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(paramNames) == 0 {
		return nil, fmt.Errorf("no parameters specified")
	}

	// Get run labels from first parameter that has data
	var runLabels []string
	for _, name := range paramNames {
		if data, ok := dataByParam[name]; ok && len(data) > 0 {
			// Sort by timestamp
			sortedData := make([]ParameterDataPoint, len(data))
			copy(sortedData, data)
			sort.Slice(sortedData, func(i, j int) bool {
				return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
			})

			for _, dp := range sortedData {
				runLabels = append(runLabels, dp.Label)
			}

			break
		}
	}

	if len(runLabels) == 0 {
		return nil, fmt.Errorf("no data available for any parameter")
	}

	// Build series data
	seriesData := make([][]float64, len(paramNames))
	seriesNames := make([]string, len(paramNames))

	for i, name := range paramNames {
		seriesNames[i] = name
		data := dataByParam[name]

		// Sort by timestamp
		sortedData := make([]ParameterDataPoint, len(data))
		copy(sortedData, data)
		sort.Slice(sortedData, func(i, j int) bool {
			return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
		})

		values := make([]float64, len(runLabels))
		dataMap := make(map[string]*float64)
		for _, dp := range sortedData {
			dataMap[dp.Label] = dp.Value
		}

		for j, label := range runLabels {
			if val, ok := dataMap[label]; ok && val != nil {
				values[j] = *val
			} else {
				values[j] = math.NaN()
			}
		}

		seriesData[i] = values
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "Parameter Comparison"
	}

	// Create chart
	p, err := charts.BarRender(
		seriesData,
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(runLabels),
		charts.LegendLabelsOptionFunc(seriesNames),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-parameter chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateDeltaChart creates a bar chart showing parameter changes between runs.
func GenerateDeltaChart(baseData, compareData []ParameterDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(baseData) == 0 || len(compareData) == 0 {
		return nil, fmt.Errorf("both base and compare data required")
	}

	// Build map of base values
	baseMap := make(map[string]float64)
	for _, dp := range baseData {
		if dp.Value != nil {
			baseMap[dp.Name] = *dp.Value
		}
	}

	// Calculate deltas
	var labels []string
	var deltas []float64

	for _, dp := range compareData {
		if dp.Value == nil {
			continue
		}

		baseVal, ok := baseMap[dp.Name]
		if !ok {
			continue
		}

		delta := *dp.Value - baseVal
		labels = append(labels, dp.Name)
		deltas = append(deltas, delta)
	}

	if len(deltas) == 0 {
		return nil, fmt.Errorf("no comparable parameters found")
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "Parameter Changes (Δ)"
	}

	// Create chart with custom colors per bar
	p, err := charts.HorizontalBarRender(
		[][]float64{deltas},
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.YAxisDataOptionFunc(labels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		func(opt *charts.ChartOption) {
			opt.SeriesList[0].Label.Show = true
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create delta chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GenerateConvergenceChart creates a chart showing convergence metrics.
func GenerateConvergenceChart(data []OFVDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("at least 2 data points required for convergence chart")
	}

	// Sort by timestamp
	sortedData := make([]OFVDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Calculate OFV deltas between consecutive runs
	labels := make([]string, len(sortedData)-1)
	deltas := make([]float64, len(sortedData)-1)
	for i := 1; i < len(sortedData); i++ {
		labels[i-1] = fmt.Sprintf("%s→%s", sortedData[i-1].Label, sortedData[i].Label)
		deltas[i-1] = sortedData[i].OFV - sortedData[i-1].OFV
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "OFV Changes Between Runs"
	}

	// Create chart
	p, err := charts.BarRender(
		[][]float64{deltas},
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(labels),
		charts.LegendOptionFunc(charts.LegendOption{
			Show: charts.FalseFlag(),
		}),
		func(opt *charts.ChartOption) {
			opt.SeriesList[0].Label.Show = true
			// Add reference line at y=0
			opt.SeriesList[0].MarkLine = charts.NewMarkLine(
				charts.SeriesMarkDataTypeAverage,
			)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create convergence chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// GeneratePieChart creates a pie chart for distribution visualization.
func GeneratePieChart(labels []string, values []float64, opts ChartOptions) (*ChartResult, error) {
	if len(labels) != len(values) {
		return nil, fmt.Errorf("labels and values must have same length")
	}

	if len(labels) == 0 {
		return nil, fmt.Errorf("no data provided")
	}

	// Set defaults
	if opts.Width == 0 {
		opts.Width = 600
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "Distribution"
	}

	// Create chart
	p, err := charts.PieRender(
		values,
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.LegendLabelsOptionFunc(labels),
		charts.PieSeriesShowLabel(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pie chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatPNG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// RenderToSVG converts a chart result to SVG format.
func RenderToSVG(data []OFVDataPoint, opts ChartOptions) (*ChartResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	// Sort by timestamp
	sortedData := make([]OFVDataPoint, len(data))
	copy(sortedData, data)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	labels := make([]string, len(sortedData))
	values := make([]float64, len(sortedData))
	for i, dp := range sortedData {
		labels[i] = dp.Label
		values[i] = dp.OFV
	}

	if opts.Width == 0 {
		opts.Width = 800
	}
	if opts.Height == 0 {
		opts.Height = 400
	}
	if opts.Title == "" {
		opts.Title = "OFV Trend"
	}

	p, err := charts.LineRender(
		[][]float64{values},
		charts.TitleTextOptionFunc(opts.Title),
		charts.WidthOptionFunc(opts.Width),
		charts.HeightOptionFunc(opts.Height),
		charts.XAxisDataOptionFunc(labels),
		charts.SVGTypeOption(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SVG chart: %w", err)
	}

	buf, err := p.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to render SVG: %w", err)
	}

	return &ChartResult{
		Data:   buf,
		Format: FormatSVG,
		Width:  opts.Width,
		Height: opts.Height,
	}, nil
}

// SaveChart writes a chart result to a buffer.
func SaveChart(result *ChartResult) (*bytes.Buffer, error) {
	if result == nil {
		return nil, fmt.Errorf("nil chart result")
	}

	buf := bytes.NewBuffer(result.Data)

	return buf, nil
}

// getAxisMin calculates an appropriate minimum for the Y axis.
func getAxisMin(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}

	min := values[0]
	max := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	// Add 10% padding below minimum
	rangeVal := max - min
	if rangeVal == 0 {
		rangeVal = math.Abs(min) * 0.1
		if rangeVal == 0 {
			rangeVal = 1
		}
	}

	paddedMin := min - rangeVal*0.1
	// Round down to nice number
	magnitude := math.Pow(10, math.Floor(math.Log10(math.Abs(paddedMin))))
	if magnitude > 0 {
		paddedMin = math.Floor(paddedMin/magnitude) * magnitude
	}

	return &paddedMin
}
