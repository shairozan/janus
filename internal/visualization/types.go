// Package visualization provides chart generation for NONMEM run analysis.
// It supports static PNG charts via go-charts and interactive HTML export via go-echarts.
package visualization

import (
	"time"

	"github.com/pharmalytica/janus/internal/runlog"
)

// ChartType identifies the type of chart to generate.
type ChartType string

const (
	ChartTypeOFVTrend     ChartType = "ofv_trend"     // OFV values over runs
	ChartTypeParameterBar ChartType = "parameter_bar" // Parameter comparison bar chart
	ChartTypeDeltaBar     ChartType = "delta_bar"     // Parameter delta bar chart
	ChartTypeConvergence  ChartType = "convergence"   // Convergence metrics over runs
)

// ChartFormat specifies the output format for charts.
type ChartFormat string

const (
	FormatPNG  ChartFormat = "png"  // Static PNG image
	FormatSVG  ChartFormat = "svg"  // Scalable vector graphics
	FormatHTML ChartFormat = "html" // Interactive HTML (go-echarts)
)

// ChartOptions configures chart generation.
type ChartOptions struct {
	Width       int         // Chart width in pixels (default 800)
	Height      int         // Chart height in pixels (default 400)
	Title       string      // Chart title
	Format      ChartFormat // Output format
	ShowLegend  bool        // Whether to show legend
	ShowGrid    bool        // Whether to show grid lines
	ColorScheme string      // Color scheme name (default "default")
}

// DefaultChartOptions returns sensible default chart options.
func DefaultChartOptions() ChartOptions {
	return ChartOptions{
		Width:       800,
		Height:      400,
		Format:      FormatPNG,
		ShowLegend:  true,
		ShowGrid:    true,
		ColorScheme: "default",
	}
}

// RunDataPoint represents a single data point from a run.
type RunDataPoint struct {
	RunID     string    // Run identifier
	Timestamp time.Time // When the run was executed
	Label     string    // Display label for the point
}

// OFVDataPoint represents an OFV value for charting.
type OFVDataPoint struct {
	RunDataPoint
	OFV       float64 // Objective function value
	Minimized bool    // Whether minimization succeeded
}

// ParameterDataPoint represents a parameter value for charting.
type ParameterDataPoint struct {
	RunDataPoint
	Name      string   // Parameter name (e.g., "THETA1")
	Type      string   // Parameter type ("theta", "omega", "sigma")
	Value     *float64 // Parameter estimate (nil if not available)
	StdError  *float64 // Standard error (nil if not available)
	Shrinkage *float64 // Shrinkage (for random effects)
	Fixed     bool     // Whether parameter was fixed
}

// ChartData holds prepared data for chart generation.
type ChartData struct {
	Title      string          // Chart title
	XLabels    []string        // X-axis labels (run IDs or timestamps)
	Series     []ChartSeries   // Data series
	Thresholds []ThresholdLine // Optional threshold lines
}

// ChartSeries represents a single data series in a chart.
type ChartSeries struct {
	Name   string    // Series name (for legend)
	Values []float64 // Y-axis values (NaN for missing)
	Color  string    // Optional color override
}

// ThresholdLine represents a horizontal reference line.
type ThresholdLine struct {
	Value float64 // Y-axis value
	Label string  // Label text
	Color string  // Line color
	Style string  // "solid", "dashed", "dotted"
}

// ChartResult holds the generated chart data.
type ChartResult struct {
	Data   []byte      // Chart image/HTML data
	Format ChartFormat // Format of the data
	Width  int         // Actual width
	Height int         // Actual height
}

// ExtractOFVData extracts OFV data points from run records.
func ExtractOFVData(records []*runlog.RunRecord) []OFVDataPoint {
	points := make([]OFVDataPoint, 0, len(records))

	for _, rec := range records {
		if rec.Summary == nil {
			continue
		}

		ofv := rec.Summary.GoodnessOfFit.ObjectiveFunctionValue
		if ofv == nil {
			continue
		}

		points = append(points, OFVDataPoint{
			RunDataPoint: RunDataPoint{
				RunID:     rec.ID,
				Timestamp: rec.Timestamp,
				Label:     rec.ID,
			},
			OFV:       *ofv,
			Minimized: rec.Summary.Estimation.Minimized,
		})
	}

	return points
}

// ExtractParameterData extracts parameter data for a specific parameter name.
func ExtractParameterData(records []*runlog.RunRecord, paramName string) []ParameterDataPoint {
	points := make([]ParameterDataPoint, 0, len(records))

	for _, rec := range records {
		if rec.Summary == nil {
			continue
		}

		// Search in all parameter types
		var found *ParameterDataPoint

		// Check thetas
		for _, p := range rec.Summary.Parameters.Thetas {
			if p.Name == paramName {
				found = &ParameterDataPoint{
					RunDataPoint: RunDataPoint{
						RunID:     rec.ID,
						Timestamp: rec.Timestamp,
						Label:     rec.ID,
					},
					Name:      p.Name,
					Type:      "theta",
					Value:     p.Estimate,
					StdError:  p.StdError,
					Shrinkage: p.Shrinkage,
					Fixed:     p.Fixed,
				}

				break
			}
		}

		// Check omegas if not found
		if found == nil {
			for _, p := range rec.Summary.Parameters.Omegas {
				if p.Name == paramName {
					found = &ParameterDataPoint{
						RunDataPoint: RunDataPoint{
							RunID:     rec.ID,
							Timestamp: rec.Timestamp,
							Label:     rec.ID,
						},
						Name:      p.Name,
						Type:      "omega",
						Value:     p.Estimate,
						StdError:  p.StdError,
						Shrinkage: p.Shrinkage,
						Fixed:     p.Fixed,
					}

					break
				}
			}
		}

		// Check sigmas if not found
		if found == nil {
			for _, p := range rec.Summary.Parameters.Sigmas {
				if p.Name == paramName {
					found = &ParameterDataPoint{
						RunDataPoint: RunDataPoint{
							RunID:     rec.ID,
							Timestamp: rec.Timestamp,
							Label:     rec.ID,
						},
						Name:      p.Name,
						Type:      "sigma",
						Value:     p.Estimate,
						StdError:  p.StdError,
						Shrinkage: p.Shrinkage,
						Fixed:     p.Fixed,
					}

					break
				}
			}
		}

		if found != nil {
			points = append(points, *found)
		}
	}

	return points
}

// GetAllParameterNames returns all unique parameter names across records.
func GetAllParameterNames(records []*runlog.RunRecord) []string {
	nameSet := make(map[string]bool)

	for _, rec := range records {
		if rec.Summary == nil {
			continue
		}

		for _, p := range rec.Summary.Parameters.Thetas {
			nameSet[p.Name] = true
		}

		for _, p := range rec.Summary.Parameters.Omegas {
			nameSet[p.Name] = true
		}

		for _, p := range rec.Summary.Parameters.Sigmas {
			nameSet[p.Name] = true
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	return names
}
