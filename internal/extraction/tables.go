package extraction

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/tables"
)

// ExtractTableDiagnostics extracts diagnostic data from NONMEM output tables.
// It looks for embedded tables first, then falls back to the run directory.
func ExtractTableDiagnostics(record *runlog.RunRecord, runDir string) (*model.TableDiagnostics, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}

	diagnostics := &model.TableDiagnostics{
		AvailableTables: make([]model.TableInfo, 0),
	}

	// Try to find tables in the run directory
	if runDir != "" {
		if err := extractFromDirectory(runDir, diagnostics); err != nil {
			// Log but don't fail - tables are optional
			return diagnostics, nil
		}
	}

	// Try embedded tables if available
	if err := extractFromEmbedded(record, diagnostics); err != nil {
		// Log but don't fail
		return diagnostics, nil
	}

	return diagnostics, nil
}

// extractFromDirectory finds and parses NONMEM tables in the run directory.
func extractFromDirectory(runDir string, diagnostics *model.TableDiagnostics) error {
	// Find all potential table files
	tableFiles, err := tables.FindTableFiles(runDir)
	if err != nil {
		return err
	}

	for _, path := range tableFiles {
		tableType := tables.DetectTableType(path)

		// Parse the table
		table, err := tables.ReadFile(path)
		if err != nil {
			continue // Skip unparseable tables
		}

		// Record table info
		info := model.TableInfo{
			Path:     path,
			Type:     tableType.String(),
			RowCount: table.RowCount,
			Columns:  len(table.Columns),
		}
		diagnostics.AvailableTables = append(diagnostics.AvailableTables, info)

		// Extract specific data based on table type
		switch tableType {
		case tables.TableTypeSDTAB:
			extractGOFFromTable(table, diagnostics)
		case tables.TableTypePATAB:
			extractETAFromTable(table, diagnostics)
		case tables.TableTypeCOTAB, tables.TableTypeCATAB, tables.TableTypeOther, tables.TableTypeUnknown:
			// These table types don't require special extraction
		}
	}

	return nil
}

// extractFromEmbedded extracts tables from embedded files in the RunRecord.
// Embedded files are keyed by filename, so table files are recognized by running
// each key through DetectTableType (sdtab*/patab*/…) rather than by an exact key.
func extractFromEmbedded(record *runlog.RunRecord, diagnostics *model.TableDiagnostics) error {
	if record.EmbeddedFiles == nil {
		return nil
	}

	for name := range record.EmbeddedFiles {
		tableType := tables.DetectTableType(name)
		if tableType != tables.TableTypeSDTAB && tableType != tables.TableTypePATAB {
			continue
		}

		if err := extractEmbeddedTable(record, name, tableType, diagnostics); err != nil {
			return err
		}
	}

	return nil
}

// extractEmbeddedTable extracts and parses an embedded table file (by filename).
func extractEmbeddedTable(record *runlog.RunRecord, name string, tableType tables.TableType, diagnostics *model.TableDiagnostics) error {
	data, err := runlog.ExtractEmbeddedFile(record, name)
	if err != nil {
		return err
	}

	// Create temp file for parsing. The key may be a relative subpath, so use its
	// base name — the table reader keys off the file name, not the directory.
	tempDir, err := os.MkdirTemp("", "janus-table-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	tempPath := filepath.Join(tempDir, filepath.Base(name))
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	// Parse the table
	table, err := tables.ReadFile(tempPath)
	if err != nil {
		return err
	}

	info := model.TableInfo{
		Path:     name + " (embedded)",
		Type:     tableType.String(),
		RowCount: table.RowCount,
		Columns:  len(table.Columns),
	}
	diagnostics.AvailableTables = append(diagnostics.AvailableTables, info)

	switch tableType {
	case tables.TableTypeSDTAB:
		extractGOFFromTable(table, diagnostics)
	case tables.TableTypePATAB:
		extractETAFromTable(table, diagnostics)
	case tables.TableTypeCOTAB, tables.TableTypeCATAB, tables.TableTypeOther, tables.TableTypeUnknown:
		// These table types carry no special diagnostics.
	}

	return nil
}

// extractGOFFromTable extracts GOF data from an SDTAB table.
func extractGOFFromTable(table *tables.Table, diagnostics *model.TableDiagnostics) {
	gofData, err := tables.ExtractGOFData(table)
	if err != nil {
		return
	}

	gof := &model.GOFDiagnostics{
		DV:    gofData.DV,
		PRED:  gofData.PRED,
		IPRED: gofData.IPRED,
		TIME:  gofData.TIME,
		CWRES: gofData.CWRES,
		ID:    gofData.ID,
		N:     len(gofData.DV),
	}

	// Calculate statistics
	if len(gofData.CWRES) > 0 {
		gof.CWRESMean, gof.CWRESSD = meanSD(gofData.CWRES)
	}

	if len(gofData.DV) > 0 && len(gofData.PRED) > 0 {
		gof.CorrelationDV = correlation(gofData.DV, gofData.PRED)
	}

	if len(gofData.DV) > 0 && len(gofData.IPRED) > 0 {
		gof.CorrelationIDV = correlation(gofData.DV, gofData.IPRED)
	}

	diagnostics.GOFData = gof
}

// extractETAFromTable extracts ETA data from a PATAB table.
func extractETAFromTable(table *tables.Table, diagnostics *model.TableDiagnostics) {
	etaData, err := tables.ExtractETAData(table)
	if err != nil {
		return
	}

	for _, eta := range etaData {
		stats := tables.CalculateETAStatistics(eta)

		etaDiag := model.ETADiagnostics{
			Name:   eta.Name,
			Values: eta.Values,
			IDs:    eta.IDs,
			N:      stats.N,
			Mean:   stats.Mean,
			SD:     stats.StdDev,
			Median: stats.Median,
			Min:    stats.Min,
			Max:    stats.Max,
			// Skewness and Kurtosis could be calculated if needed
		}

		diagnostics.ETAData = append(diagnostics.ETAData, etaDiag)
	}
}

// meanSD calculates mean and standard deviation.
func meanSD(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}

	// Mean
	var sum float64
	for _, v := range data {
		sum += v
	}
	mean := sum / float64(len(data))

	// Standard deviation
	var sumSq float64
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}
	sd := math.Sqrt(sumSq / float64(len(data)))

	return mean, sd
}

// correlation calculates Pearson correlation coefficient.
func correlation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	var sumX, sumY float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation
	var sumXY, sumXX, sumYY float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sumXY += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	denominator := math.Sqrt(sumXX * sumYY)
	if denominator == 0 {
		return 0
	}

	return sumXY / denominator
}

// HasTableFiles checks if a run directory contains NONMEM output tables.
func HasTableFiles(runDir string) bool {
	files, err := tables.FindTableFiles(runDir)
	if err != nil {
		return false
	}

	return len(files) > 0
}

// GetAvailableTableTypes returns the types of tables available in a run directory.
func GetAvailableTableTypes(runDir string) []string {
	files, err := tables.FindTableFiles(runDir)
	if err != nil {
		return nil
	}

	types := make(map[string]bool)
	for _, f := range files {
		t := tables.DetectTableType(f)
		types[t.String()] = true
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	sort.Strings(result)

	return result
}

// DiagnoseTableExtraction provides diagnostic information about why table extraction may fail.
// Returns a human-readable string describing what was found and any issues.
func DiagnoseTableExtraction(runDir string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Run directory: %s\n", runDir)

	// Check if directory exists
	info, err := os.Stat(runDir)
	if err != nil {
		fmt.Fprintf(&sb, "ERROR: Cannot access directory: %v\n", err)

		return sb.String()
	}
	if !info.IsDir() {
		sb.WriteString("ERROR: Path is not a directory\n")

		return sb.String()
	}

	// List all files in directory
	entries, err := os.ReadDir(runDir)
	if err != nil {
		fmt.Fprintf(&sb, "ERROR: Cannot read directory: %v\n", err)

		return sb.String()
	}

	fmt.Fprintf(&sb, "Files in directory: %d\n", len(entries))

	// Find table files
	tableFiles, _ := tables.FindTableFiles(runDir)
	fmt.Fprintf(&sb, "Detected table files: %d\n", len(tableFiles))

	for _, f := range tableFiles {
		tableType := tables.DetectTableType(f)
		fmt.Fprintf(&sb, "  - %s (type: %s)\n", filepath.Base(f), tableType)

		// Try to parse and check columns
		table, err := tables.ReadFile(f)
		if err != nil {
			fmt.Fprintf(&sb, "    Parse error: %v\n", err)

			continue
		}

		fmt.Fprintf(&sb, "    Rows: %d, Columns: %v\n", table.RowCount, table.ColumnNames())

		// Check for GOF-required columns
		if tableType == tables.TableTypeSDTAB {
			hasDV := table.HasColumn("DV")
			hasPRED := table.HasColumn("PRED")
			hasIPRED := table.HasColumn("IPRED")
			hasCWRES := table.HasColumn("CWRES")
			hasTIME := table.HasColumn("TIME")

			fmt.Fprintf(&sb, "    GOF columns: DV=%v, PRED=%v, IPRED=%v, CWRES=%v, TIME=%v\n",
				hasDV, hasPRED, hasIPRED, hasCWRES, hasTIME)

			if !hasDV {
				sb.WriteString("    WARNING: Missing DV column - GOF plots will not work\n")
			}
			if !hasPRED && !hasIPRED {
				sb.WriteString("    WARNING: Missing PRED/IPRED columns - prediction plots will not work\n")
			}
		}
	}

	if len(tableFiles) == 0 {
		sb.WriteString("\nNo table files found. Check that:\n")
		sb.WriteString("  1. Your NONMEM model has $TABLE statements\n")
		sb.WriteString("  2. Table files are named sdtab*, patab*, cotab*, or catab*\n")
		sb.WriteString("  3. The run completed successfully\n")
	}

	return sb.String()
}

// ConvertToVisualizationGOF converts model.GOFDiagnostics to tables.GOFData for visualization.
func ConvertToVisualizationGOF(gof *model.GOFDiagnostics) *tables.GOFData {
	if gof == nil {
		return nil
	}

	return &tables.GOFData{
		DV:    gof.DV,
		PRED:  gof.PRED,
		IPRED: gof.IPRED,
		TIME:  gof.TIME,
		CWRES: gof.CWRES,
		ID:    gof.ID,
	}
}

// ConvertToVisualizationETA converts model.ETADiagnostics to tables.ETAData for visualization.
func ConvertToVisualizationETA(eta *model.ETADiagnostics) *tables.ETAData {
	if eta == nil {
		return nil
	}

	return &tables.ETAData{
		Name:   eta.Name,
		Values: eta.Values,
		IDs:    eta.IDs,
	}
}
