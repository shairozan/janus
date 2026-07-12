package gui

import (
	"bytes"
	"fmt"
	"image/png"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/extraction"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/tables"
	"github.com/pharmalytica/janus/internal/visualization"
)

// DiagnosticsDialog displays GOF plots and diagnostics for a single run.
type DiagnosticsDialog struct {
	record *runlog.RunRecord
	window fyne.Window
}

// NewDiagnosticsDialog creates a new diagnostics dialog for the given run record.
func NewDiagnosticsDialog(record *runlog.RunRecord, parent fyne.Window) *DiagnosticsDialog {
	return &DiagnosticsDialog{
		record: record,
		window: parent,
	}
}

// Show displays the diagnostics dialog.
func (d *DiagnosticsDialog) Show() {
	if d.record == nil {
		dialog.ShowError(fmt.Errorf("no run record provided"), d.window)

		return
	}

	// Check if we have summary data with table diagnostics
	if d.record.Summary == nil || d.record.Summary.TableDiagnostics == nil ||
		d.record.Summary.TableDiagnostics.GOFData == nil {

		dialog.ShowInformation("No Diagnostics",
			"No diagnostic data available for this run.\nThis may be because output tables (SDTAB) were not generated.",
			d.window)

		return
	}

	// Convert model GOF data to tables GOF data for visualization
	gofData := extraction.ConvertToVisualizationGOF(d.record.Summary.TableDiagnostics.GOFData)
	if gofData == nil {
		dialog.ShowInformation("No GOF Data",
			"Unable to load GOF data for visualization.",
			d.window)

		return
	}

	// Build the diagnostics content
	content := d.buildContent(gofData)

	// Create and show dialog
	dlg := dialog.NewCustom(
		fmt.Sprintf("Diagnostics - Run %s", truncateID(d.record.ID, 8)),
		"Close",
		content,
		d.window,
	)
	dlg.Resize(fyne.NewSize(900, 700))
	dlg.Show()
}

// buildContent creates the main content with GOF plots.
func (d *DiagnosticsDialog) buildContent(gofData *tables.GOFData) fyne.CanvasObject {
	opts := visualization.DefaultGOFPlotOptions()
	opts.Width = 400
	opts.Height = 350

	// Create tabs for different plot groups
	tabs := container.NewAppTabs()

	// GOF Panel tab - 4 basic plots
	gofPanel := d.buildGOFPanel(gofData, opts)
	if gofPanel != nil {
		tabs.Append(container.NewTabItem("GOF Panel", gofPanel))
	}

	// Residuals tab
	residualsTab := d.buildResidualsTab(gofData, opts)
	if residualsTab != nil {
		tabs.Append(container.NewTabItem("Residuals", residualsTab))
	}

	// ETAs tab
	etasTab := d.buildETAsTab(opts)
	if etasTab != nil {
		tabs.Append(container.NewTabItem("ETAs", etasTab))
	}

	// Statistics tab
	statsTab := d.buildStatisticsTab()
	tabs.Append(container.NewTabItem("Statistics", statsTab))

	return tabs
}

// buildGOFPanel creates the 4-panel GOF plot grid.
func (d *DiagnosticsDialog) buildGOFPanel(gofData *tables.GOFData, opts visualization.GOFPlotOptions) fyne.CanvasObject {
	// Generate the 4 basic GOF plots
	var dvPredImg, dvIPredImg, cwresTimeImg, cwresPredImg fyne.CanvasObject

	// DV vs PRED
	if result, err := visualization.GenerateDVvsPRED(gofData, opts); err == nil && result != nil {
		dvPredImg = d.chartToImage(result)
	} else {
		dvPredImg = widget.NewLabel("DV vs PRED: No data")
	}

	// DV vs IPRED
	if result, err := visualization.GenerateDVvsIPRED(gofData, opts); err == nil && result != nil {
		dvIPredImg = d.chartToImage(result)
	} else {
		dvIPredImg = widget.NewLabel("DV vs IPRED: No data")
	}

	// CWRES vs TIME
	if result, err := visualization.GenerateCWRESvsTime(gofData, opts); err == nil && result != nil {
		cwresTimeImg = d.chartToImage(result)
	} else {
		cwresTimeImg = widget.NewLabel("CWRES vs TIME: No data")
	}

	// CWRES vs PRED
	if result, err := visualization.GenerateCWRESvsPRED(gofData, opts); err == nil && result != nil {
		cwresPredImg = d.chartToImage(result)
	} else {
		cwresPredImg = widget.NewLabel("CWRES vs PRED: No data")
	}

	// Arrange in 2x2 grid
	topRow := container.NewHBox(
		container.NewVBox(widget.NewLabelWithStyle("DV vs PRED", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), dvPredImg),
		container.NewVBox(widget.NewLabelWithStyle("DV vs IPRED", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), dvIPredImg),
	)
	bottomRow := container.NewHBox(
		container.NewVBox(widget.NewLabelWithStyle("CWRES vs TIME", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), cwresTimeImg),
		container.NewVBox(widget.NewLabelWithStyle("CWRES vs PRED", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), cwresPredImg),
	)

	return container.NewVBox(topRow, bottomRow)
}

// buildResidualsTab creates the residuals analysis tab.
func (d *DiagnosticsDialog) buildResidualsTab(gofData *tables.GOFData, opts visualization.GOFPlotOptions) fyne.CanvasObject {
	var histImg, qqImg fyne.CanvasObject

	// CWRES Histogram
	if result, err := visualization.GenerateCWRESHistogram(gofData, opts); err == nil && result != nil {
		histImg = d.chartToImage(result)
	} else {
		histImg = widget.NewLabel("CWRES Histogram: No data")
	}

	// CWRES Q-Q Plot
	if result, err := visualization.GenerateCWRESQQPlot(gofData, opts); err == nil && result != nil {
		qqImg = d.chartToImage(result)
	} else {
		qqImg = widget.NewLabel("CWRES Q-Q Plot: Insufficient data")
	}

	return container.NewVBox(
		container.NewHBox(
			container.NewVBox(widget.NewLabelWithStyle("CWRES Histogram", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), histImg),
			container.NewVBox(widget.NewLabelWithStyle("CWRES Q-Q Plot", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), qqImg),
		),
	)
}

// buildETAsTab creates the ETA analysis tab.
func (d *DiagnosticsDialog) buildETAsTab(opts visualization.GOFPlotOptions) fyne.CanvasObject {
	if d.record.Summary == nil || d.record.Summary.TableDiagnostics == nil ||
		len(d.record.Summary.TableDiagnostics.ETAData) == 0 {

		return widget.NewLabel("No ETA data available")
	}

	etaData := d.record.Summary.TableDiagnostics.ETAData

	// Create histograms for each ETA
	var etaPlots []fyne.CanvasObject
	for i := range etaData {
		eta := &etaData[i]
		tablesETA := extraction.ConvertToVisualizationETA(eta)
		if tablesETA == nil {
			continue
		}

		if result, err := visualization.GenerateETAHistogram(tablesETA, opts); err == nil && result != nil {
			etaPlots = append(etaPlots,
				container.NewVBox(
					widget.NewLabelWithStyle(eta.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
					d.chartToImage(result),
				),
			)
		}
	}

	if len(etaPlots) == 0 {
		return widget.NewLabel("No ETA histograms generated")
	}

	// Arrange ETA plots in rows of 2
	rows := make([]fyne.CanvasObject, 0)
	for i := 0; i < len(etaPlots); i += 2 {
		if i+1 < len(etaPlots) {
			rows = append(rows, container.NewHBox(etaPlots[i], etaPlots[i+1]))
		} else {
			rows = append(rows, etaPlots[i])
		}
	}

	return container.NewVBox(rows...)
}

// buildStatisticsTab creates the statistics summary tab.
func (d *DiagnosticsDialog) buildStatisticsTab() fyne.CanvasObject {
	if d.record.Summary == nil || d.record.Summary.TableDiagnostics == nil {
		return widget.NewLabel("No statistics available")
	}

	diag := d.record.Summary.TableDiagnostics

	// Build statistics text
	var stats strings.Builder

	stats.WriteString("## GOF Statistics\n\n")

	if diag.GOFData != nil {
		fmt.Fprintf(&stats, "- **Observations:** %d\n", diag.GOFData.N)
		fmt.Fprintf(&stats, "- **CWRES Mean:** %.4f (expected ~0)\n", diag.GOFData.CWRESMean)
		fmt.Fprintf(&stats, "- **CWRES SD:** %.4f (expected ~1)\n", diag.GOFData.CWRESSD)
		fmt.Fprintf(&stats, "- **DV-PRED Correlation:** %.4f\n", diag.GOFData.CorrelationDV)
		fmt.Fprintf(&stats, "- **DV-IPRED Correlation:** %.4f\n", diag.GOFData.CorrelationIDV)
	}

	if len(diag.ETAData) > 0 {
		stats.WriteString("\n## ETA Statistics\n\n")
		for _, eta := range diag.ETAData {
			fmt.Fprintf(&stats, "### %s\n", eta.Name)
			fmt.Fprintf(&stats, "- **N:** %d subjects\n", eta.N)
			fmt.Fprintf(&stats, "- **Mean:** %.4f (expected ~0)\n", eta.Mean)
			fmt.Fprintf(&stats, "- **SD:** %.4f\n", eta.SD)
			fmt.Fprintf(&stats, "- **Median:** %.4f\n", eta.Median)
			fmt.Fprintf(&stats, "- **Range:** [%.4f, %.4f]\n\n", eta.Min, eta.Max)
		}
	}

	if len(diag.AvailableTables) > 0 {
		stats.WriteString("\n## Available Tables\n\n")
		for _, tbl := range diag.AvailableTables {
			fmt.Fprintf(&stats, "- **%s:** %d rows, %d columns\n", tbl.Type, tbl.RowCount, tbl.Columns)
		}
	}

	richText := widget.NewRichTextFromMarkdown(stats.String())

	return container.NewScroll(richText)
}

// chartToImage converts a ChartResult to a Fyne canvas image.
func (d *DiagnosticsDialog) chartToImage(result *visualization.ChartResult) fyne.CanvasObject {
	if result == nil || len(result.Data) == 0 {
		return widget.NewLabel("Chart generation failed")
	}

	// Decode PNG data to image
	img, err := png.Decode(bytes.NewReader(result.Data))
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Image decode error: %v", err))
	}

	// Create Fyne canvas image
	fyneImg := canvas.NewImageFromImage(img)
	fyneImg.FillMode = canvas.ImageFillContain
	fyneImg.SetMinSize(fyne.NewSize(float32(result.Width), float32(result.Height)))

	return fyneImg
}

// truncateID truncates a UUID for display.
func truncateID(id string, length int) string {
	if len(id) <= length {
		return id
	}

	return id[:length] + "..."
}

// ShowDiagnosticsDialog is a convenience function to show the diagnostics dialog.
func ShowDiagnosticsDialog(record *runlog.RunRecord, parent fyne.Window) {
	dlg := NewDiagnosticsDialog(record, parent)
	dlg.Show()
}
