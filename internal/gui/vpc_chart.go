package gui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	echartopts "github.com/go-echarts/go-echarts/v2/opts"
)

// vpcPlotPercentiles is the preferred set of percentile triples (low, median,
// high) to plot, tried in order of preference.
var vpcPlotPercentiles = [][3]string{
	{"5", "50", "95"},
	{"10", "50", "90"},
	{"2.5", "50", "97.5"},
}

// selectVPCPercentiles picks the percentile triple to plot: the first preferred
// set fully present, else the lowest/median/highest of the available numeric
// percentiles (mean excluded). Returns whatever is available if fewer than three.
func selectVPCPercentiles(ps []*vpcPercentile) []*vpcPercentile {
	byLabel := make(map[string]*vpcPercentile, len(ps))
	for _, p := range ps {
		byLabel[p.Label] = p
	}

	for _, set := range vpcPlotPercentiles {
		if byLabel[set[0]] != nil && byLabel[set[1]] != nil && byLabel[set[2]] != nil {
			return []*vpcPercentile{byLabel[set[0]], byLabel[set[1]], byLabel[set[2]]}
		}
	}

	// Fallback: numeric percentiles only, lowest / median / highest.
	var nums []*vpcPercentile
	for _, p := range ps {
		if !math.IsNaN(p.Pct) {
			nums = append(nums, p)
		}
	}

	switch {
	case len(nums) == 0:
		return ps
	case len(nums) <= 3:
		return nums
	default:
		return []*vpcPercentile{nums[0], nums[len(nums)/2], nums[len(nums)-1]}
	}
}

// vpcLineData converts a per-bin float series to ECharts line data, mapping NaN
// (PsN blanks/NA) to a nil value so the line gaps rather than dropping to zero.
func vpcLineData(vals []float64) []echartopts.LineData {
	out := make([]echartopts.LineData, len(vals))
	for i, v := range vals {
		if math.IsNaN(v) {
			out[i] = echartopts.LineData{Value: nil}

			continue
		}

		out[i] = echartopts.LineData{Value: v}
	}

	return out
}

// buildVPCChart renders a continuous VPC as an interactive line chart: for each
// selected percentile, the observed line (solid, with symbols) over its simulated
// CI bounds (dashed edge lines). Filled ribbons are intentionally avoided — PsN's
// CI columns can be negative, which breaks ECharts' stacked-area band trick.
func buildVPCChart(vpc *vpcData) *charts.Line {
	xLabels := make([]string, len(vpc.Bins))
	for i, b := range vpc.Bins {
		xLabels[i] = strconv.FormatFloat(b.MedianIDV, 'g', 4, 64)
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(echartopts.Title{
			Title:    "Visual Predictive Check",
			Subtitle: fmt.Sprintf("%s vs %s — observed percentiles over simulated 95%% CIs", vpc.DVName, vpc.IDVName),
		}),
		charts.WithInitializationOpts(echartopts.Initialization{Width: "1000px", Height: "640px"}),
		charts.WithXAxisOpts(echartopts.XAxis{Name: vpc.IDVName}),
		charts.WithYAxisOpts(echartopts.YAxis{Name: vpc.DVName, Scale: echartopts.Bool(true)}),
		charts.WithTooltipOpts(echartopts.Tooltip{Show: echartopts.Bool(true), Trigger: "axis"}),
		// Place the legend below the title/subtitle and start the plot below the
		// legend, so the two-row legend can't collide with the graph.
		charts.WithLegendOpts(echartopts.Legend{Show: echartopts.Bool(true), Top: "58"}),
		charts.WithGridOpts(echartopts.Grid{Top: "165", ContainLabel: echartopts.Bool(true)}),
		charts.WithDataZoomOpts(echartopts.DataZoom{Type: "slider", Start: 0, End: 100}),
		charts.WithToolboxOpts(echartopts.Toolbox{
			Show: echartopts.Bool(true),
			Feature: &echartopts.ToolBoxFeature{
				SaveAsImage: &echartopts.ToolBoxFeatureSaveAsImage{Show: echartopts.Bool(true), Title: "Save as PNG"},
				Restore:     &echartopts.ToolBoxFeatureRestore{Show: echartopts.Bool(true)},
			},
		}),
	)

	line.SetXAxis(xLabels)

	dashed := charts.WithLineStyleOpts(echartopts.LineStyle{Type: "dashed"})
	noSymbol := charts.WithLineChartOpts(echartopts.LineChart{ShowSymbol: echartopts.Bool(false)})
	withSymbol := charts.WithLineChartOpts(echartopts.LineChart{ShowSymbol: echartopts.Bool(true)})

	for _, p := range selectVPCPercentiles(vpc.Percentiles) {
		line.AddSeries(p.Label+"% observed", vpcLineData(p.Real), withSymbol)
		line.AddSeries(p.Label+"% sim CI low", vpcLineData(p.CIFrom), noSymbol, dashed)
		line.AddSeries(p.Label+"% sim CI high", vpcLineData(p.CITo), noSymbol, dashed)
	}

	return line
}
