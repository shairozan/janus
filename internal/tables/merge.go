package tables

import (
	"fmt"
	"os"
)

// MergeOptions configures table merging behavior.
type MergeOptions struct {
	KeyColumn       string // Column to join on (default: "ID")
	KeepDuplicates  bool   // Keep duplicate columns from second table
	DuplicateSuffix string // Suffix for duplicate columns (default: "_2")
}

// DefaultMergeOptions returns default merge options.
func DefaultMergeOptions() MergeOptions {
	return MergeOptions{
		KeyColumn:       "ID",
		KeepDuplicates:  false,
		DuplicateSuffix: "_2",
	}
}

// MergeTables merges two tables by a key column (typically ID).
// The resulting table contains all columns from both tables.
func MergeTables(left, right *Table, opts MergeOptions) (*Table, error) {
	if opts.KeyColumn == "" {
		opts.KeyColumn = "ID"
	}

	// Verify both tables have the key column
	if !left.HasColumn(opts.KeyColumn) {
		return nil, fmt.Errorf("left table missing key column %s", opts.KeyColumn)
	}
	if !right.HasColumn(opts.KeyColumn) {
		return nil, fmt.Errorf("right table missing key column %s", opts.KeyColumn)
	}

	// Verify row counts match
	if left.RowCount != right.RowCount {
		return nil, fmt.Errorf("row count mismatch: left=%d, right=%d", left.RowCount, right.RowCount)
	}

	// Create merged table
	merged := NewTable(left.Type)
	merged.FileName = left.FileName
	merged.TableNumber = left.TableNumber

	// Add columns from left table
	for _, col := range left.Columns {
		newCol := merged.AddColumn(col.Name)
		newCol.Values = make([]*float64, len(col.Values))
		copy(newCol.Values, col.Values)
	}
	merged.RowCount = left.RowCount

	// Add columns from right table (skip key column and handle duplicates)
	for _, col := range right.Columns {
		if col.Name == opts.KeyColumn {
			continue // Skip key column
		}

		colName := col.Name
		if merged.HasColumn(colName) {
			if !opts.KeepDuplicates {
				continue // Skip duplicate
			}
			colName += opts.DuplicateSuffix
		}

		newCol := merged.AddColumn(colName)
		newCol.Values = make([]*float64, len(col.Values))
		copy(newCol.Values, col.Values)
	}

	return merged, nil
}

// MergeByRow performs a simple row-wise merge, assuming tables are aligned.
// This is faster than MergeTables but requires pre-aligned data.
func MergeByRow(tables ...*Table) (*Table, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables to merge")
	}

	if len(tables) == 1 {
		return tables[0], nil
	}

	// Verify all tables have same row count
	rowCount := tables[0].RowCount
	for i, t := range tables[1:] {
		if t.RowCount != rowCount {
			return nil, fmt.Errorf("table %d has %d rows, expected %d", i+1, t.RowCount, rowCount)
		}
	}

	// Create merged table
	merged := NewTable(tables[0].Type)
	merged.FileName = tables[0].FileName
	merged.TableNumber = tables[0].TableNumber
	merged.RowCount = rowCount

	// Track column names to avoid duplicates
	seen := make(map[string]int)

	for _, t := range tables {
		for _, col := range t.Columns {
			name := col.Name

			// Handle duplicate column names
			if count, exists := seen[name]; exists {
				// Skip ID column duplicates
				if name == "ID" || name == "SUBJ" || name == "SUBJECT" {
					continue
				}
				name = fmt.Sprintf("%s_%d", col.Name, count+1)
			}
			seen[col.Name]++

			newCol := merged.AddColumn(name)
			newCol.Values = make([]*float64, len(col.Values))
			copy(newCol.Values, col.Values)
		}
	}

	return merged, nil
}

// AppendRows appends rows from source to target table.
// Both tables must have identical column structure.
func AppendRows(target, source *Table) error {
	// Verify column structure matches
	if len(target.Columns) != len(source.Columns) {
		return fmt.Errorf("column count mismatch: target=%d, source=%d",
			len(target.Columns), len(source.Columns))
	}

	for i, col := range target.Columns {
		if col.Name != source.Columns[i].Name {
			return fmt.Errorf("column name mismatch at index %d: target=%s, source=%s",
				i, col.Name, source.Columns[i].Name)
		}
	}

	// Append rows
	for i, col := range target.Columns {
		col.Values = append(col.Values, source.Columns[i].Values...)
	}
	target.RowCount += source.RowCount

	return nil
}

// ConcatenateTables combines multiple tables vertically (row-wise).
// All tables must have identical column structure.
func ConcatenateTables(tables ...*Table) (*Table, error) {
	if len(tables) == 0 {
		return nil, fmt.Errorf("no tables to concatenate")
	}

	if len(tables) == 1 {
		return tables[0], nil
	}

	// Create result table as copy of first
	result := NewTable(tables[0].Type)
	result.FileName = tables[0].FileName

	// Copy columns from first table
	for _, col := range tables[0].Columns {
		newCol := result.AddColumn(col.Name)
		newCol.Values = make([]*float64, len(col.Values))
		copy(newCol.Values, col.Values)
	}
	result.RowCount = tables[0].RowCount

	// Append remaining tables
	for i, t := range tables[1:] {
		if err := AppendRows(result, t); err != nil {
			return nil, fmt.Errorf("failed to append table %d: %w", i+1, err)
		}
	}

	return result, nil
}

// SelectColumns creates a new table with only the specified columns.
func SelectColumns(t *Table, columns ...string) (*Table, error) {
	result := NewTable(t.Type)
	result.FileName = t.FileName
	result.TableNumber = t.TableNumber
	result.RowCount = t.RowCount

	for _, name := range columns {
		srcCol := t.GetColumn(name)
		if srcCol == nil {
			return nil, fmt.Errorf("column %s not found", name)
		}

		newCol := result.AddColumn(name)
		newCol.Values = make([]*float64, len(srcCol.Values))
		copy(newCol.Values, srcCol.Values)
	}

	return result, nil
}

// openTableFile is a helper to open table files.
func openTableFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open table file: %w", err)
	}

	return f, nil
}
