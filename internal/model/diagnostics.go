// Package model contains shared data types for pharmacometric model representation.
package model

// TableDiagnostics contains metadata and extracted data from NONMEM output tables.
// This provides the data needed for single-model diagnostic visualizations.
type TableDiagnostics struct {
	// Available tables found in the run output directory
	AvailableTables []TableInfo `json:"available_tables,omitempty"`

	// Extracted GOF (Goodness-of-Fit) data from SDTAB
	GOFData *GOFDiagnostics `json:"gof_data,omitempty"`

	// Extracted ETA data from PATAB
	ETAData []ETADiagnostics `json:"eta_data,omitempty"`
}

// TableInfo describes a NONMEM output table file.
type TableInfo struct {
	Path     string `json:"path"`      // File path
	Type     string `json:"type"`      // "sdtab", "patab", "cotab", "catab", or "unknown"
	RowCount int    `json:"row_count"` // Number of data rows
	Columns  int    `json:"columns"`   // Number of columns
}

// GOFDiagnostics contains pre-extracted GOF data for visualization.
// Storing pre-extracted data avoids re-parsing tables each time.
type GOFDiagnostics struct {
	// Core diagnostic columns
	DV    []float64 `json:"dv,omitempty"`    // Dependent variable (observations)
	PRED  []float64 `json:"pred,omitempty"`  // Population predictions
	IPRED []float64 `json:"ipred,omitempty"` // Individual predictions
	TIME  []float64 `json:"time,omitempty"`  // Time
	CWRES []float64 `json:"cwres,omitempty"` // Conditional weighted residuals
	ID    []float64 `json:"id,omitempty"`    // Subject ID

	// Statistics
	N              int     `json:"n"`                         // Number of observations
	CWRESMean      float64 `json:"cwres_mean,omitempty"`      // Mean CWRES (should be ~0)
	CWRESSD        float64 `json:"cwres_sd,omitempty"`        // SD of CWRES (should be ~1)
	CorrelationDV  float64 `json:"correlation_dv,omitempty"`  // DV vs PRED correlation
	CorrelationIDV float64 `json:"correlation_idv,omitempty"` // DV vs IPRED correlation
}

// ETADiagnostics contains pre-extracted ETA data for visualization.
type ETADiagnostics struct {
	Name   string    `json:"name"`   // ETA name (e.g., "ETA1", "ETA(1)")
	Values []float64 `json:"values"` // ETA values (one per subject)
	IDs    []float64 `json:"ids"`    // Subject IDs

	// Statistics
	N        int     `json:"n"`                  // Number of subjects
	Mean     float64 `json:"mean"`               // Mean (should be ~0)
	SD       float64 `json:"sd"`                 // Standard deviation
	Median   float64 `json:"median"`             // Median
	Min      float64 `json:"min"`                // Minimum
	Max      float64 `json:"max"`                // Maximum
	Skewness float64 `json:"skewness,omitempty"` // Skewness (normality check)
	Kurtosis float64 `json:"kurtosis,omitempty"` // Kurtosis (normality check)
}
