package gui

import (
	"encoding/csv"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// vpcBin is one independent-variable bin of a continuous VPC.
type vpcBin struct {
	Lower     float64 // bin lower edge (IDV)
	Upper     float64 // bin upper edge (IDV)
	MedianIDV float64 // median IDV in the bin — the x-coordinate for plotting
	NObs      int
}

// vpcPercentile is one tracked percentile (e.g. "10", "50", "90") across the
// bins: the observed value and the simulated prediction-interval band.
type vpcPercentile struct {
	Label  string    // "10", "50", "90", "mean", …
	Pct    float64   // numeric percentile for ordering (NaN for "mean")
	Real   []float64 // observed P-th percentile of DV, per bin
	Sim    []float64 // simulated median of the P-th percentile, per bin
	CIFrom []float64 // simulated CI lower, per bin (ribbon bottom)
	CITo   []float64 // simulated CI upper, per bin (ribbon top)
}

// vpcData is the parsed continuous block of a PsN vpc_results.csv: the bins plus
// the per-percentile observed lines and simulated CI ribbons.
type vpcData struct {
	IDVName     string // independent variable, e.g. "TIME"
	DVName      string // dependent variable, e.g. "DV"
	Bins        []vpcBin
	Percentiles []*vpcPercentile // ordered low → high percentile
}

// VPC continuous-block header patterns. PsN names percentile columns by value and
// role ("10% real", "10% sim", "95%CI for 10% from/to"); the leading "95%" is the
// fixed CI level, the captured number is the percentile. "mean" is also valid.
// The "%" is optional because PsN writes percentile columns as "50% real" but the
// mean column as "mean real" (no percent sign).
var (
	vpcReReal   = regexp.MustCompile(`^(\d+(?:\.\d+)?|mean)%? real$`)
	vpcReSim    = regexp.MustCompile(`^(\d+(?:\.\d+)?|mean)%? sim$`)
	vpcReCIFrom = regexp.MustCompile(`^95%CI for (\d+(?:\.\d+)?|mean)%? from$`)
	vpcReCITo   = regexp.MustCompile(`^95%CI for (\d+(?:\.\d+)?|mean)%? to$`)
)

// parseVPCResults parses the Continuous data block of a PsN vpc_results.csv into
// bins and per-percentile observed/simulated series. It is tolerant: the file is
// multi-block and the percentile columns are named dynamically, so columns are
// located by header regex rather than fixed offsets, and unparseable cells become
// NaN so a single bad value can't fail the whole parse.
func parseVPCResults(data []byte) (*vpcData, error) {
	lines := strings.Split(string(data), "\n")

	idv, dv := vpcVariableNames(lines)

	headerIdx, err := vpcContinuousHeaderIndex(lines)
	if err != nil {
		return nil, err
	}

	header, err := splitCSVLine(lines[headerIdx])
	if err != nil {
		return nil, fmt.Errorf("vpc: parsing continuous header: %w", err)
	}

	cols := mapVPCColumns(header)
	if cols.medianIDV < 0 {
		return nil, fmt.Errorf("vpc: continuous block has no median.idv column")
	}

	if len(cols.percentiles) == 0 {
		return nil, fmt.Errorf("vpc: continuous block has no percentile columns")
	}

	out := &vpcData{IDVName: idv, DVName: dv}
	for label := range cols.percentiles {
		out.Percentiles = append(out.Percentiles, &vpcPercentile{Label: label, Pct: vpcPctValue(label)})
	}
	sortVPCPercentiles(out.Percentiles)

	// Data rows run from after the header until the first blank/section line.
	for _, raw := range lines[headerIdx+1:] {
		if strings.TrimSpace(raw) == "" {
			break
		}

		row, err := splitCSVLine(raw)
		if err != nil {
			continue // tolerate a malformed row rather than abort
		}

		out.Bins = append(out.Bins, vpcBin{
			Lower:     cellFloat(row, cols.lower),
			Upper:     cellFloat(row, cols.upper),
			MedianIDV: cellFloat(row, cols.medianIDV),
			NObs:      int(cellFloat(row, cols.nObs)),
		})

		for _, p := range out.Percentiles {
			c := cols.percentiles[p.Label]
			p.Real = append(p.Real, cellFloat(row, c.real))
			p.Sim = append(p.Sim, cellFloat(row, c.sim))
			p.CIFrom = append(p.CIFrom, cellFloat(row, c.ciFrom))
			p.CITo = append(p.CITo, cellFloat(row, c.ciTo))
		}
	}

	if len(out.Bins) == 0 {
		return nil, fmt.Errorf("vpc: continuous block has no data rows")
	}

	return out, nil
}

// vpcColumnSet holds the resolved column indices for the continuous block.
type vpcColumnSet struct {
	lower, upper, medianIDV, nObs int
	percentiles                   map[string]vpcPercentileCols
}

type vpcPercentileCols struct {
	real, sim, ciFrom, ciTo int
}

// mapVPCColumns resolves continuous-block columns from the (trimmed) header.
func mapVPCColumns(header []string) vpcColumnSet {
	cols := vpcColumnSet{lower: -1, upper: -1, medianIDV: -1, nObs: -1, percentiles: map[string]vpcPercentileCols{}}

	get := func(label string) vpcPercentileCols {
		c, ok := cols.percentiles[label]
		if !ok {
			c = vpcPercentileCols{real: -1, sim: -1, ciFrom: -1, ciTo: -1}
		}

		return c
	}

	for i, h := range header {
		name := strings.TrimSpace(h)

		switch {
		case strings.HasPrefix(name, "< "): // "< TIME" — bin lower edge
			cols.lower = i
		case name == "<=":
			cols.upper = i
		case name == "median.idv":
			cols.medianIDV = i
		case name == "no. of obs":
			cols.nObs = i
		}

		if m := vpcReReal.FindStringSubmatch(name); m != nil {
			c := get(m[1])
			c.real = i
			cols.percentiles[m[1]] = c
		} else if m := vpcReSim.FindStringSubmatch(name); m != nil {
			c := get(m[1])
			c.sim = i
			cols.percentiles[m[1]] = c
		} else if m := vpcReCIFrom.FindStringSubmatch(name); m != nil {
			c := get(m[1])
			c.ciFrom = i
			cols.percentiles[m[1]] = c
		} else if m := vpcReCITo.FindStringSubmatch(name); m != nil {
			c := get(m[1])
			c.ciTo = i
			cols.percentiles[m[1]] = c
		}
	}

	return cols
}

// vpcContinuousHeaderIndex finds the header row of the Continuous data block: the
// first quoted-CSV row after the "Continuous data" marker.
func vpcContinuousHeaderIndex(lines []string) (int, error) {
	marker := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "Continuous data" {
			marker = i

			break
		}
	}

	if marker < 0 {
		return 0, fmt.Errorf("vpc: no 'Continuous data' block found (only continuous VPCs are supported)")
	}

	for i := marker + 1; i < len(lines); i++ {
		// The header is the first row that starts a quoted CSV record and carries
		// the percentile columns; the "N observations out of M" line in between does
		// not start with a quote.
		if strings.HasPrefix(strings.TrimSpace(lines[i]), `""`) {
			return i, nil
		}
	}

	return 0, fmt.Errorf("vpc: continuous block header not found")
}

// vpcVariableNames pulls the IDV/DV names from the "VPC run info" metadata block,
// falling back to generic labels.
func vpcVariableNames(lines []string) (idv, dv string) {
	idv, dv = "IDV", "DV"

	var headerRow, valueRow []string
	for i, l := range lines {
		if strings.Contains(l, "Independent variable") && strings.Contains(l, "Dependent variable") {
			headerRow, _ = splitCSVLine(l)
			if i+1 < len(lines) {
				valueRow, _ = splitCSVLine(lines[i+1])
			}

			break
		}
	}

	for i, h := range headerRow {
		if i >= len(valueRow) {
			break
		}

		switch strings.TrimSpace(h) {
		case "Independent variable":
			if v := strings.TrimSpace(valueRow[i]); v != "" {
				idv = v
			}
		case "Dependent variable":
			if v := strings.TrimSpace(valueRow[i]); v != "" {
				dv = v
			}
		}
	}

	return idv, dv
}

// splitCSVLine parses a single CSV line into trimmed fields.
func splitCSVLine(line string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1 // rows have a trailing comma → variable field count

	rec, err := r.Read()
	if err != nil {
		return nil, err
	}

	return rec, nil
}

// cellFloat returns the float at col, or NaN when missing/blank/unparseable so a
// single bad value doesn't abort the parse (PsN can emit blanks/NA).
func cellFloat(row []string, col int) float64 {
	if col < 0 || col >= len(row) {
		return math.NaN()
	}

	s := strings.TrimSpace(row[col])
	if s == "" || strings.EqualFold(s, "na") {
		return math.NaN()
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}

	return v
}

// vpcPctValue maps a percentile label to a number for ordering; "mean" sorts last.
func vpcPctValue(label string) float64 {
	if strings.EqualFold(label, "mean") {
		return math.NaN()
	}

	v, err := strconv.ParseFloat(label, 64)
	if err != nil {
		return math.NaN()
	}

	return v
}

// sortVPCPercentiles orders percentiles ascending; non-numeric ("mean") last.
func sortVPCPercentiles(ps []*vpcPercentile) {
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i].Pct, ps[j].Pct
		switch {
		case math.IsNaN(a) && math.IsNaN(b):
			return ps[i].Label < ps[j].Label
		case math.IsNaN(a):
			return false
		case math.IsNaN(b):
			return true
		default:
			return a < b
		}
	})
}
