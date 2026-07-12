package tables

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing of NONMEM output tables.
type Parser struct {
	normalizeColumns bool // Whether to apply column name aliases
}

// NewParser creates a new table parser with default options.
func NewParser() *Parser {
	return &Parser{
		normalizeColumns: true,
	}
}

// ParserOption configures the parser.
type ParserOption func(*Parser)

// WithNormalizeColumns sets whether to normalize column names using aliases.
func WithNormalizeColumns(normalize bool) ParserOption {
	return func(p *Parser) {
		p.normalizeColumns = normalize
	}
}

// NewParserWithOptions creates a parser with custom options.
func NewParserWithOptions(opts ...ParserOption) *Parser {
	p := NewParser()
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// tableNoPattern matches "TABLE NO.  X" lines.
var tableNoPattern = regexp.MustCompile(`^\s*TABLE\s+NO\.\s+(\d+)`)

// scientificNotationPatterns match various NONMEM/Fortran number formats.
// Fortran short form: 1.23+02, 1.23-02 (no E/D).
var fortranShortForm = regexp.MustCompile(`^([+-]?\d+\.?\d*)([+-])(\d+)$`)

// Parse parses a NONMEM table from a reader.
func (p *Parser) Parse(r io.Reader, tableType TableType) (*Table, error) {
	scanner := bufio.NewScanner(r)
	table := NewTable(tableType)

	var headers []string
	format := FormatUnknown
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for TABLE NO. line
		if matches := tableNoPattern.FindStringSubmatch(line); matches != nil {
			tableNo, _ := strconv.Atoi(matches[1])
			table.TableNumber = tableNo
			format = FormatTableNo

			continue
		}

		// Detect header line (first non-empty, non-TABLE line)
		if headers == nil {
			headers = p.parseHeaderLine(line)
			if len(headers) == 0 {
				return nil, fmt.Errorf("no column headers found at line %d", lineNum)
			}

			// Initialize columns
			for _, h := range headers {
				name := h
				if p.normalizeColumns {
					name = NormalizeColumnName(h)
				}
				table.AddColumn(name)
			}

			if format == FormatUnknown {
				format = FormatOneHeader
			}

			continue
		}

		// Parse data row
		if err := p.parseDataRow(table, line, lineNum); err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading table: %w", err)
	}

	// Verify we found headers
	if headers == nil {
		return nil, fmt.Errorf("no headers found in table")
	}

	return table, nil
}

// parseHeaderLine extracts column names from a header line.
func (p *Parser) parseHeaderLine(line string) []string {
	fields := strings.Fields(line)

	// Filter out empty fields and validate as column names
	headers := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			headers = append(headers, f)
		}
	}

	return headers
}

// parseDataRow parses a single data row and adds values to columns.
func (p *Parser) parseDataRow(table *Table, line string, lineNum int) error {
	fields := strings.Fields(line)

	if len(fields) != len(table.Columns) {
		return fmt.Errorf("row %d has %d fields, expected %d columns",
			lineNum, len(fields), len(table.Columns))
	}

	for i, field := range fields {
		value, err := p.parseValue(field)
		if err != nil {
			return fmt.Errorf("row %d, column %s: %w", lineNum, table.Columns[i].Name, err)
		}
		table.Columns[i].Values = append(table.Columns[i].Values, value)
	}
	table.RowCount++

	return nil
}

// parseValue parses a single value, handling scientific notation and missing values.
func (p *Parser) parseValue(s string) (*float64, error) {
	s = strings.TrimSpace(s)

	// Handle missing values
	if s == MissingValue || s == "" {
		return nil, nil
	}

	// Try standard parsing first (handles most cases)
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return &val, nil
	}

	// Handle Fortran D notation (convert D to E)
	normalized := strings.Replace(s, "D", "E", 1)
	normalized = strings.Replace(normalized, "d", "e", 1)
	if val, err := strconv.ParseFloat(normalized, 64); err == nil {
		return &val, nil
	}

	// Handle Fortran short form (1.23+02 -> 1.23E+02)
	if matches := fortranShortForm.FindStringSubmatch(s); matches != nil {
		reconstructed := matches[1] + "E" + matches[2] + matches[3]
		if val, err := strconv.ParseFloat(reconstructed, 64); err == nil {
			return &val, nil
		}
	}

	return nil, fmt.Errorf("cannot parse value %q", s)
}

// ParseMultiple parses a file containing multiple TABLE NO. sections.
func (p *Parser) ParseMultiple(r io.Reader, tableType TableType) ([]*Table, error) {
	scanner := bufio.NewScanner(r)
	var tables []*Table
	var currentLines []string
	currentTableNo := 0
	lineNum := 0

	flushCurrentTable := func() error {
		if len(currentLines) == 0 {
			return nil
		}

		reader := strings.NewReader(strings.Join(currentLines, "\n"))
		table, err := p.Parse(reader, tableType)
		if err != nil {
			return err
		}

		table.TableNumber = currentTableNo
		tables = append(tables, table)
		currentLines = nil

		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Check for new TABLE NO. section
		if matches := tableNoPattern.FindStringSubmatch(line); matches != nil {
			// Flush previous table
			if err := flushCurrentTable(); err != nil {
				return nil, fmt.Errorf("error parsing table ending at line %d: %w", lineNum-1, err)
			}

			currentTableNo, _ = strconv.Atoi(matches[1])

			continue
		}

		currentLines = append(currentLines, line)
	}

	// Flush final table
	if err := flushCurrentTable(); err != nil {
		return nil, fmt.Errorf("error parsing final table: %w", err)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return tables, nil
}

// DetectFormat examines a reader to determine the table format.
func DetectFormat(r io.Reader) (TableFormat, error) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check for TABLE NO.
		if tableNoPattern.MatchString(line) {
			return FormatTableNo, nil
		}

		// First non-empty line is not TABLE NO., so it must be ONEHEADER
		return FormatOneHeader, nil
	}

	if err := scanner.Err(); err != nil {
		return FormatUnknown, err
	}

	return FormatUnknown, fmt.Errorf("empty or invalid table file")
}
