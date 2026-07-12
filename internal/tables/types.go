// Package tables provides parsing and data structures for NONMEM output tables.
// It supports SDTAB, PATAB, COTAB, CATAB, and other $TABLE output formats.
package tables

import (
	"math"
)

// TableType identifies the type of NONMEM output table.
type TableType string

const (
	TableTypeSDTAB   TableType = "sdtab"   // Standard diagnostics table
	TableTypePATAB   TableType = "patab"   // Parameter table (ETAs)
	TableTypeCOTAB   TableType = "cotab"   // Continuous covariates
	TableTypeCATAB   TableType = "catab"   // Categorical covariates
	TableTypeOther   TableType = "other"   // Unknown/generic table
	TableTypeUnknown TableType = "unknown" // Undetectable type
)

// String returns the string representation of the TableType.
func (t TableType) String() string {
	return string(t)
}

// Column represents a single column in a NONMEM table.
type Column struct {
	Name   string     // Column name (e.g., "ID", "TIME", "DV")
	Index  int        // Column index (0-based)
	Values []*float64 // Values for each row (nil for missing values)
}

// Table represents a parsed NONMEM output table.
type Table struct {
	Type        TableType      // Table type (SDTAB, PATAB, etc.)
	FileName    string         // Source file name
	TableNumber int            // TABLE NO. value (0 if not present)
	Columns     []*Column      // Columns in order
	ColumnIndex map[string]int // Column name to index mapping
	RowCount    int            // Number of data rows
}

// NewTable creates a new Table with initialized maps.
func NewTable(tableType TableType) *Table {
	return &Table{
		Type:        tableType,
		Columns:     make([]*Column, 0),
		ColumnIndex: make(map[string]int),
	}
}

// AddColumn adds a column to the table.
func (t *Table) AddColumn(name string) *Column {
	col := &Column{
		Name:   name,
		Index:  len(t.Columns),
		Values: make([]*float64, 0),
	}
	t.Columns = append(t.Columns, col)
	t.ColumnIndex[name] = col.Index

	return col
}

// GetColumn returns a column by name, or nil if not found.
func (t *Table) GetColumn(name string) *Column {
	if idx, ok := t.ColumnIndex[name]; ok {
		return t.Columns[idx]
	}

	return nil
}

// HasColumn checks if a column exists in the table.
func (t *Table) HasColumn(name string) bool {
	_, ok := t.ColumnIndex[name]

	return ok
}

// GetValue returns the value at a specific row and column.
// Returns nil if the row/column doesn't exist or if the value is missing.
func (t *Table) GetValue(row int, colName string) *float64 {
	col := t.GetColumn(colName)
	if col == nil || row < 0 || row >= len(col.Values) {
		return nil
	}

	return col.Values[row]
}

// GetColumnValues returns all values for a column as a slice of floats.
// Missing values are represented as NaN.
func (t *Table) GetColumnValues(colName string) []float64 {
	col := t.GetColumn(colName)
	if col == nil {
		return nil
	}

	values := make([]float64, len(col.Values))
	for i, v := range col.Values {
		if v != nil {
			values[i] = *v
		} else {
			values[i] = math.NaN()
		}
	}

	return values
}

// GetRow returns all values for a specific row.
func (t *Table) GetRow(row int) map[string]*float64 {
	if row < 0 || row >= t.RowCount {
		return nil
	}

	result := make(map[string]*float64)
	for _, col := range t.Columns {
		if row < len(col.Values) {
			result[col.Name] = col.Values[row]
		}
	}

	return result
}

// ColumnNames returns the names of all columns in order.
func (t *Table) ColumnNames() []string {
	names := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = col.Name
	}

	return names
}

// FilterRows returns a new table containing only rows where the filter function returns true.
func (t *Table) FilterRows(filter func(row int) bool) *Table {
	newTable := NewTable(t.Type)
	newTable.FileName = t.FileName
	newTable.TableNumber = t.TableNumber

	// Create columns with same names
	for _, col := range t.Columns {
		newTable.AddColumn(col.Name)
	}

	// Copy matching rows
	for row := 0; row < t.RowCount; row++ {
		if filter(row) {
			for colIdx, col := range t.Columns {
				if row < len(col.Values) {
					newTable.Columns[colIdx].Values = append(newTable.Columns[colIdx].Values, col.Values[row])
				}
			}
			newTable.RowCount++
		}
	}

	return newTable
}

// TableFormat identifies the header format of a NONMEM table.
type TableFormat int

const (
	FormatUnknown   TableFormat = iota
	FormatTableNo               // TABLE NO. X followed by header
	FormatOneHeader             // Single header row (ONEHEADER option)
)

// MissingValue represents a missing value in NONMEM tables (typically ".").
const MissingValue = "."

// ColumnAlias maps common alternative column names to standard names.
var ColumnAlias = map[string]string{
	"SUBJ":    "ID",
	"SUBJECT": "ID",
	"IDV":     "TIME",
	"Y":       "DV",
	"CONC":    "DV",
	"TPRED":   "PRED",
	"CPRED":   "IPRED",
	"CRES":    "CWRES",
}

// NormalizeColumnName returns the standard column name for an alias.
// If the name is not an alias, returns the original name.
func NormalizeColumnName(name string) string {
	if standard, ok := ColumnAlias[name]; ok {
		return standard
	}

	return name
}

// RequiredColumns defines required columns for each table type.
var RequiredColumns = map[TableType][]string{
	TableTypeSDTAB: {"ID", "DV"}, // TIME often required but may be IDV
	TableTypePATAB: {"ID"},       // ETAs are auto-detected
	TableTypeCOTAB: {"ID"},       // Covariates are model-specific
	TableTypeCATAB: {"ID"},       // Categories are model-specific
}

// ValidateRequiredColumns checks if a table has all required columns.
func ValidateRequiredColumns(t *Table) []string {
	required := RequiredColumns[t.Type]
	missing := make([]string, 0)

	for _, colName := range required {
		// Check standard name and common aliases
		found := t.HasColumn(colName)
		if !found {
			// Check aliases
			for alias, standard := range ColumnAlias {
				if standard == colName && t.HasColumn(alias) {
					found = true

					break
				}
			}
		}

		if !found {
			missing = append(missing, colName)
		}
	}

	return missing
}
