package tables

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// filenamePatterns maps filename patterns to table types.
var filenamePatterns = map[*regexp.Regexp]TableType{
	regexp.MustCompile(`(?i)^sdtab\d*$`): TableTypeSDTAB,
	regexp.MustCompile(`(?i)^patab\d*$`): TableTypePATAB,
	regexp.MustCompile(`(?i)^cotab\d*$`): TableTypeCOTAB,
	regexp.MustCompile(`(?i)^catab\d*$`): TableTypeCATAB,
	regexp.MustCompile(`(?i)^mytab\d*$`): TableTypeOther,
	regexp.MustCompile(`(?i)^tab\d*$`):   TableTypeOther,
	regexp.MustCompile(`(?i)\.tab$`):     TableTypeOther,
	regexp.MustCompile(`(?i)^sdtab.*$`):  TableTypeSDTAB,
	regexp.MustCompile(`(?i)^patab.*$`):  TableTypePATAB,
	regexp.MustCompile(`(?i)^cotab.*$`):  TableTypeCOTAB,
	regexp.MustCompile(`(?i)^catab.*$`):  TableTypeCATAB,
}

// DetectTableType determines the table type from a filename.
func DetectTableType(filename string) TableType {
	base := filepath.Base(filename)
	// Remove extension if present
	if idx := strings.LastIndex(base, "."); idx > 0 {
		base = base[:idx]
	}

	for pattern, tableType := range filenamePatterns {
		if pattern.MatchString(base) {
			return tableType
		}
	}

	return TableTypeOther
}

// ReadFile reads and parses a NONMEM table file.
func ReadFile(path string) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open table file: %w", err)
	}
	defer f.Close()

	tableType := DetectTableType(path)
	parser := NewParser()

	table, err := parser.Parse(f, tableType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse table file %s: %w", path, err)
	}

	table.FileName = filepath.Base(path)

	return table, nil
}

// ReadFileAs reads and parses a NONMEM table file with explicit type.
func ReadFileAs(path string, tableType TableType) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open table file: %w", err)
	}
	defer f.Close()

	parser := NewParser()

	table, err := parser.Parse(f, tableType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse table file %s: %w", path, err)
	}

	table.FileName = filepath.Base(path)

	return table, nil
}

// ReadMultipleFromFile reads a file containing multiple TABLE NO. sections.
func ReadMultipleFromFile(path string) ([]*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open table file: %w", err)
	}
	defer f.Close()

	tableType := DetectTableType(path)
	parser := NewParser()

	tables, err := parser.ParseMultiple(f, tableType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse table file %s: %w", path, err)
	}

	baseName := filepath.Base(path)
	for _, t := range tables {
		t.FileName = baseName
	}

	return tables, nil
}

// FindTableFiles searches a directory for NONMEM table files.
func FindTableFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var tableFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		tableType := DetectTableType(name)
		if tableType != TableTypeOther {
			tableFiles = append(tableFiles, filepath.Join(dir, name))
		}
	}

	return tableFiles, nil
}

// ReadAllTables reads all table files in a directory.
func ReadAllTables(dir string) (map[TableType][]*Table, error) {
	files, err := FindTableFiles(dir)
	if err != nil {
		return nil, err
	}

	result := make(map[TableType][]*Table)

	for _, path := range files {
		table, err := ReadFile(path)
		if err != nil {
			// Log warning but continue with other files
			continue
		}

		result[table.Type] = append(result[table.Type], table)
	}

	return result, nil
}

// TableSet holds all parsed tables for a run.
type TableSet struct {
	SDTAB []*Table
	PATAB []*Table
	COTAB []*Table
	CATAB []*Table
	Other []*Table
}

// ReadTableSet reads all table files from a directory and organizes them.
func ReadTableSet(dir string) (*TableSet, error) {
	allTables, err := ReadAllTables(dir)
	if err != nil {
		return nil, err
	}

	return &TableSet{
		SDTAB: allTables[TableTypeSDTAB],
		PATAB: allTables[TableTypePATAB],
		COTAB: allTables[TableTypeCOTAB],
		CATAB: allTables[TableTypeCATAB],
		Other: allTables[TableTypeOther],
	}, nil
}

// PrimarySDTAB returns the first SDTAB or nil if none exist.
func (ts *TableSet) PrimarySDTAB() *Table {
	if len(ts.SDTAB) > 0 {
		return ts.SDTAB[0]
	}

	return nil
}

// PrimaryPATAB returns the first PATAB or nil if none exist.
func (ts *TableSet) PrimaryPATAB() *Table {
	if len(ts.PATAB) > 0 {
		return ts.PATAB[0]
	}

	return nil
}

// HasDiagnosticTables checks if the set contains tables needed for GOF plots.
func (ts *TableSet) HasDiagnosticTables() bool {
	sdtab := ts.PrimarySDTAB()
	if sdtab == nil {
		return false
	}

	// Need at least ID, DV, and some prediction column
	return sdtab.HasColumn("DV") &&
		(sdtab.HasColumn("PRED") || sdtab.HasColumn("IPRED"))
}
