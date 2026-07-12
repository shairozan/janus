package tables

import (
	"fmt"
	"io"
)

// SDTABColumns defines known SDTAB column names.
var SDTABColumns = struct {
	// Required
	ID string
	DV string

	// Time/Independent variable
	TIME string
	IDV  string
	TAD  string

	// Predictions
	PRED  string
	IPRED string
	CPRED string

	// Residuals
	RES   string
	IRES  string
	WRES  string
	IWRES string
	CWRES string
	NPDE  string

	// Event/Dose flags
	EVID string
	MDV  string
	CMT  string
	AMT  string
}{
	ID:    "ID",
	DV:    "DV",
	TIME:  "TIME",
	IDV:   "TIME", // Normalized alias
	TAD:   "TAD",
	PRED:  "PRED",
	IPRED: "IPRED",
	CPRED: "IPRED", // Alias
	RES:   "RES",
	IRES:  "IRES",
	WRES:  "WRES",
	IWRES: "IWRES",
	CWRES: "CWRES",
	NPDE:  "NPDE",
	EVID:  "EVID",
	MDV:   "MDV",
	CMT:   "CMT",
	AMT:   "AMT",
}

// SDTABParser handles parsing of SDTAB (standard diagnostics) tables.
type SDTABParser struct {
	parser *Parser
}

// NewSDTABParser creates a new SDTAB parser.
func NewSDTABParser() *SDTABParser {
	return &SDTABParser{
		parser: NewParser(),
	}
}

// Parse parses an SDTAB file from a reader.
func (p *SDTABParser) Parse(r io.Reader) (*Table, error) {
	table, err := p.parser.Parse(r, TableTypeSDTAB)
	if err != nil {
		return nil, err
	}

	// Validate required columns
	if err := p.validate(table); err != nil {
		return nil, err
	}

	return table, nil
}

// validate checks that the SDTAB has required columns.
func (p *SDTABParser) validate(table *Table) error {
	// DV is required
	if !table.HasColumn("DV") && !table.HasColumn("Y") && !table.HasColumn("CONC") {
		return fmt.Errorf("SDTAB missing required column DV (or alias Y, CONC)")
	}

	// ID is required
	if !table.HasColumn("ID") && !table.HasColumn("SUBJ") && !table.HasColumn("SUBJECT") {
		return fmt.Errorf("SDTAB missing required column ID (or alias SUBJ, SUBJECT)")
	}

	return nil
}

// ReadSDTAB reads an SDTAB file from disk.
func ReadSDTAB(path string) (*Table, error) {
	parser := NewSDTABParser()
	f, err := openTableFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	table, err := parser.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SDTAB %s: %w", path, err)
	}

	table.FileName = path

	return table, nil
}

// HasPredictions checks if the SDTAB has prediction columns for GOF plots.
func HasPredictions(t *Table) bool {
	return t.HasColumn("PRED") || t.HasColumn("IPRED") || t.HasColumn("TPRED") || t.HasColumn("CPRED")
}

// HasResiduals checks if the SDTAB has residual columns.
func HasResiduals(t *Table) bool {
	return t.HasColumn("CWRES") || t.HasColumn("IWRES") || t.HasColumn("WRES") ||
		t.HasColumn("RES") || t.HasColumn("IRES") || t.HasColumn("NPDE")
}

// GetObservationRows returns a filtered table containing only observation rows (EVID=0 or MDV=0).
func GetObservationRows(t *Table) *Table {
	return t.FilterRows(func(row int) bool {
		// Check EVID if present
		if t.HasColumn("EVID") {
			evid := t.GetValue(row, "EVID")
			if evid != nil && *evid != 0 {
				return false // Dose record, not observation
			}
		}

		// Check MDV if present
		if t.HasColumn("MDV") {
			mdv := t.GetValue(row, "MDV")
			if mdv != nil && *mdv != 0 {
				return false // Missing DV
			}
		}

		// Check DV is not missing
		dv := t.GetValue(row, "DV")

		return dv != nil
	})
}

// SubjectIDs returns unique subject IDs from the table.
func SubjectIDs(t *Table) []float64 {
	if !t.HasColumn("ID") {
		return nil
	}

	seen := make(map[float64]bool)
	var ids []float64

	for row := 0; row < t.RowCount; row++ {
		id := t.GetValue(row, "ID")
		if id != nil && !seen[*id] {
			seen[*id] = true
			ids = append(ids, *id)
		}
	}

	return ids
}

// GetSubjectRows returns rows for a specific subject.
func GetSubjectRows(t *Table, subjectID float64) *Table {
	return t.FilterRows(func(row int) bool {
		id := t.GetValue(row, "ID")

		return id != nil && *id == subjectID
	})
}

// GOFData holds data extracted from SDTAB for GOF plotting.
type GOFData struct {
	DV    []float64
	PRED  []float64
	IPRED []float64
	TIME  []float64
	CWRES []float64
	ID    []float64
}

// ExtractGOFData extracts data needed for standard GOF plots from an SDTAB.
func ExtractGOFData(t *Table) (*GOFData, error) {
	// Get observation rows only
	obs := GetObservationRows(t)
	if obs.RowCount == 0 {
		return nil, fmt.Errorf("no observation rows found")
	}

	data := &GOFData{}

	// DV is required
	if obs.HasColumn("DV") {
		data.DV = obs.GetColumnValues("DV")
	} else {
		return nil, fmt.Errorf("DV column not found")
	}

	// PRED (optional)
	if obs.HasColumn("PRED") {
		data.PRED = obs.GetColumnValues("PRED")
	}

	// IPRED (optional)
	if obs.HasColumn("IPRED") {
		data.IPRED = obs.GetColumnValues("IPRED")
	}

	// TIME (optional)
	if obs.HasColumn("TIME") {
		data.TIME = obs.GetColumnValues("TIME")
	} else if obs.HasColumn("TAD") {
		data.TIME = obs.GetColumnValues("TAD")
	}

	// CWRES (optional, fall back to other residuals)
	switch {
	case obs.HasColumn("CWRES"):
		data.CWRES = obs.GetColumnValues("CWRES")
	case obs.HasColumn("IWRES"):
		data.CWRES = obs.GetColumnValues("IWRES")
	case obs.HasColumn("WRES"):
		data.CWRES = obs.GetColumnValues("WRES")
	}

	// ID (optional)
	if obs.HasColumn("ID") {
		data.ID = obs.GetColumnValues("ID")
	}

	return data, nil
}
