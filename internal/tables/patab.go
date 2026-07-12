package tables

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
)

// etaColumnPattern matches ETA column names (ETA1, ETA2, ETA(1), etc.)
var etaColumnPattern = regexp.MustCompile(`^ETA\(?(\d+)\)?$`)

// PATABParser handles parsing of PATAB (parameter) tables.
type PATABParser struct {
	parser *Parser
}

// NewPATABParser creates a new PATAB parser.
func NewPATABParser() *PATABParser {
	return &PATABParser{
		parser: NewParser(),
	}
}

// Parse parses a PATAB file from a reader.
func (p *PATABParser) Parse(r io.Reader) (*Table, error) {
	table, err := p.parser.Parse(r, TableTypePATAB)
	if err != nil {
		return nil, err
	}

	// Validate required columns
	if err := p.validate(table); err != nil {
		return nil, err
	}

	return table, nil
}

// validate checks that the PATAB has required columns.
func (p *PATABParser) validate(table *Table) error {
	// ID is required
	if !table.HasColumn("ID") && !table.HasColumn("SUBJ") && !table.HasColumn("SUBJECT") {
		return fmt.Errorf("PATAB missing required column ID (or alias SUBJ, SUBJECT)")
	}

	return nil
}

// ReadPATAB reads a PATAB file from disk.
func ReadPATAB(path string) (*Table, error) {
	parser := NewPATABParser()
	f, err := openTableFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	table, err := parser.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PATAB %s: %w", path, err)
	}

	table.FileName = path

	return table, nil
}

// GetETAColumns returns the names of ETA columns in order (ETA1, ETA2, etc.)
func GetETAColumns(t *Table) []string {
	type etaCol struct {
		name  string
		index int
	}

	var etas []etaCol

	for _, col := range t.Columns {
		if matches := etaColumnPattern.FindStringSubmatch(col.Name); matches != nil {
			idx, _ := strconv.Atoi(matches[1])
			etas = append(etas, etaCol{name: col.Name, index: idx})
		}
	}

	// Sort by ETA index
	sort.Slice(etas, func(i, j int) bool {
		return etas[i].index < etas[j].index
	})

	names := make([]string, len(etas))
	for i, e := range etas {
		names[i] = e.name
	}

	return names
}

// GetIndividualParameters returns non-ETA, non-ID columns (e.g., CL, V, KA).
func GetIndividualParameters(t *Table) []string {
	var params []string

	for _, col := range t.Columns {
		name := col.Name
		// Skip ID and related
		if name == "ID" || name == "SUBJ" || name == "SUBJECT" {
			continue
		}
		// Skip ETAs
		if etaColumnPattern.MatchString(name) {
			continue
		}
		// Skip common non-parameter columns
		if name == "TIME" || name == "TAD" || name == "DV" {
			continue
		}
		params = append(params, name)
	}

	return params
}

// ETAData holds ETA values for analysis.
type ETAData struct {
	Name   string    // ETA name (e.g., "ETA1")
	Values []float64 // ETA values for each subject
	IDs    []float64 // Subject IDs
}

// ExtractETAData extracts ETA values from a PATAB.
func ExtractETAData(t *Table) ([]*ETAData, error) {
	etaCols := GetETAColumns(t)
	if len(etaCols) == 0 {
		return nil, fmt.Errorf("no ETA columns found")
	}

	// Get unique subjects (first occurrence of each ID)
	type subjectRow struct {
		id  float64
		row int
	}

	seen := make(map[float64]bool)
	var subjects []subjectRow

	for row := 0; row < t.RowCount; row++ {
		id := t.GetValue(row, "ID")
		if id != nil && !seen[*id] {
			seen[*id] = true
			subjects = append(subjects, subjectRow{id: *id, row: row})
		}
	}

	// Extract ETA data
	var result []*ETAData

	for _, etaName := range etaCols {
		data := &ETAData{
			Name:   etaName,
			Values: make([]float64, len(subjects)),
			IDs:    make([]float64, len(subjects)),
		}

		for i, subj := range subjects {
			data.IDs[i] = subj.id
			val := t.GetValue(subj.row, etaName)
			if val != nil {
				data.Values[i] = *val
			}
		}

		result = append(result, data)
	}

	return result, nil
}

// ETAStatistics holds summary statistics for an ETA.
type ETAStatistics struct {
	Name     string
	Mean     float64
	StdDev   float64
	Min      float64
	Max      float64
	Median   float64
	N        int
	Variance float64
}

// CalculateETAStatistics computes summary statistics for ETAs.
func CalculateETAStatistics(data *ETAData) *ETAStatistics {
	n := len(data.Values)
	if n == 0 {
		return &ETAStatistics{Name: data.Name, N: 0}
	}

	// Calculate mean
	var sum float64
	for _, v := range data.Values {
		sum += v
	}
	mean := sum / float64(n)

	// Calculate variance and find min/max
	var variance float64
	min := data.Values[0]
	max := data.Values[0]

	for _, v := range data.Values {
		diff := v - mean
		variance += diff * diff
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	variance /= float64(n)

	// Calculate median
	sorted := make([]float64, n)
	copy(sorted, data.Values)
	sort.Float64s(sorted)

	var median float64
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	return &ETAStatistics{
		Name:     data.Name,
		Mean:     mean,
		StdDev:   sqrt(variance),
		Min:      min,
		Max:      max,
		Median:   median,
		N:        n,
		Variance: variance,
	}
}

// sqrt is a simple square root implementation.
func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}

	// Newton's method
	z := x / 2
	for i := 0; i < 100; i++ {
		zNew := (z + x/z) / 2
		if abs(zNew-z) < 1e-10 {
			break
		}
		z = zNew
	}

	return z
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}

	return x
}
