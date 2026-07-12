package gui

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/visualization"
)

// paramCI is a parameter's bootstrap confidence interval.
type paramCI struct {
	Name   string
	Median float64
	Lo     float64 // 2.5th percentile
	Hi     float64 // 97.5th percentile
}

// parseBootstrapCIs parses a PsN bootstrap raw_results CSV: for each column whose
// values are numeric across the sample rows, it computes the median and the
// 2.5/97.5 percentiles. Format-tolerant — non-numeric (label) columns are
// skipped, so it degrades gracefully across PsN versions.
func parseBootstrapCIs(data []byte) ([]paramCI, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse bootstrap csv: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("bootstrap csv has no data rows")
	}

	header, rows := records[0], records[1:]

	var out []paramCI

	for col, name := range header {
		var vals []float64

		for _, row := range rows {
			if col >= len(row) {
				continue
			}

			if v, err := strconv.ParseFloat(strings.TrimSpace(row[col]), 64); err == nil {
				vals = append(vals, v)
			}
		}

		// Treat as a parameter column only if most rows are numeric.
		if len(vals) == 0 || len(vals) < len(rows)/2 {
			continue
		}

		sort.Float64s(vals)
		out = append(out, paramCI{
			Name:   strings.TrimSpace(name),
			Median: percentile(vals, 50),
			Lo:     percentile(vals, 2.5),
			Hi:     percentile(vals, 97.5),
		})
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no numeric parameter columns found")
	}

	return out, nil
}

// percentile returns the p-th percentile (0..100) of sorted values, linearly
// interpolated.
func percentile(sorted []float64, p float64) float64 {
	switch len(sorted) {
	case 0:
		return math.NaN()
	case 1:
		return sorted[0]
	}

	rank := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))

	if lo == hi {
		return sorted[lo]
	}

	frac := rank - float64(lo)

	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// parseScmSummary extracts the covariate-selection lines (chosen relations, OFV
// steps) from an scmlog.
func parseScmSummary(logText string) []string {
	var out []string

	for _, line := range strings.Split(logText, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}

		low := strings.ToLower(l)
		if strings.Contains(low, "chosen") || (strings.Contains(low, "step") && strings.Contains(low, "ofv")) {
			out = append(out, l)
		}
	}

	return out
}

// formatFloat renders a parameter value compactly.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', 5, 64)
}

// findPSNResultFile globs modelDir and its immediate subdirs for the first file
// matching any of the patterns.
func findPSNResultFile(modelDir string, patterns ...string) string {
	for _, p := range patterns {
		for _, dir := range []string{modelDir, filepath.Join(modelDir, "*")} {
			matches, _ := filepath.Glob(filepath.Join(dir, p))
			if len(matches) > 0 {
				sort.Strings(matches)

				return matches[0]
			}
		}
	}

	return ""
}

// psnToolFromCommand returns the PsN analysis tool (vpc/bootstrap/scm) a run's
// command invoked, or "" for a non-PsN run. It keys off the command's first token
// (the binary), tolerating an absolute path.
func psnToolFromCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}

	switch strings.ToLower(filepath.Base(fields[0])) {
	case "vpc":
		return "vpc"
	case "bootstrap":
		return "bootstrap"
	case "scm":
		return "scm"
	default:
		return ""
	}
}

// psnResultButtonLabel is the run-details button label for a PsN analysis tool.
func psnResultButtonLabel(tool string) string {
	switch tool {
	case "vpc":
		return "View VPC Plot"
	case "bootstrap":
		return "View Bootstrap CIs"
	case "scm":
		return "View SCM Summary"
	default:
		return "View PsN Results"
	}
}

// showPSNResultsDialog renders the result of a completed PsN analysis.
func showPSNResultsDialog(win fyne.Window, tool, modelDir string) {
	switch tool {
	case "bootstrap":
		showBootstrapResults(win, modelDir)
	case "scm":
		showScmResults(win, modelDir)
	case "vpc":
		showVpcResults(win, modelDir)
	}
}

func showBootstrapResults(win fyne.Window, modelDir string) {
	path := findPSNResultFile(modelDir, "raw_results*.csv")
	if path == "" {
		dialog.ShowInformation("Bootstrap", "No raw_results file found in the run directory yet.", win)

		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, win)

		return
	}

	cis, err := parseBootstrapCIs(data)
	if err != nil {
		dialog.ShowError(err, win)

		return
	}

	headers := []string{"Parameter", "Median", "2.5%", "97.5%"}
	table := widget.NewTable(
		func() (int, int) { return len(cis) + 1, len(headers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			lbl, _ := o.(*widget.Label)
			if id.Row == 0 {
				lbl.SetText(headers[id.Col])
				lbl.TextStyle = fyne.TextStyle{Bold: true}

				return
			}

			lbl.TextStyle = fyne.TextStyle{}
			ci := cis[id.Row-1]

			switch id.Col {
			case 0:
				lbl.SetText(ci.Name)
			case 1:
				lbl.SetText(formatFloat(ci.Median))
			case 2:
				lbl.SetText(formatFloat(ci.Lo))
			case 3:
				lbl.SetText(formatFloat(ci.Hi))
			}
		},
	)
	table.SetColumnWidth(0, 200)
	table.SetColumnWidth(1, 110)
	table.SetColumnWidth(2, 110)
	table.SetColumnWidth(3, 110)

	d := dialog.NewCustom("Bootstrap parameter CIs", "Close", table, win)
	d.Resize(fyne.NewSize(580, 500))
	d.Show()
}

func showScmResults(win fyne.Window, modelDir string) {
	path := findPSNResultFile(modelDir, "scmlog.txt")
	if path == "" {
		dialog.ShowInformation("SCM", "No scmlog.txt found in the run directory yet.", win)

		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, win)

		return
	}

	text := "No covariate-selection summary lines found in scmlog.txt."
	if summary := parseScmSummary(string(data)); len(summary) > 0 {
		text = strings.Join(summary, "\n")
	}

	lbl := widget.NewLabel(text)
	lbl.Wrapping = fyne.TextWrapWord

	d := dialog.NewCustom("SCM covariate selection", "Close", container.NewVScroll(lbl), win)
	d.Resize(fyne.NewSize(620, 460))
	d.Show()
}

func showVpcResults(win fyne.Window, modelDir string) {
	path := findPSNResultFile(modelDir, "vpc_results.csv")
	if path == "" {
		dialog.ShowInformation("VPC", "No vpc_results.csv found in the run directory yet.", win)

		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, win)

		return
	}

	vpc, err := parseVPCResults(data)
	if err != nil {
		dialog.ShowError(fmt.Errorf("could not read VPC results: %w", err), win)

		return
	}

	// Render the VPC ourselves (no Xpose/R): observed percentiles over the
	// simulated 95% CIs, as an interactive chart opened in the browser.
	if err := visualization.ServeInteractiveChart(buildVPCChart(vpc)); err != nil {
		dialog.ShowError(fmt.Errorf("could not render VPC plot: %w", err), win)
	}
}
