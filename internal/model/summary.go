// Package model contains shared data types for pharmacometric model representation.
// These types are used across multiple packages (runlog, summary) without creating
// import cycles.
package model

import (
	"time"
)

// ModelSummary represents a comprehensive summary of a pharmacometric model run.
// Based on BBR patterns, this includes key metrics scientists expect to see.
type ModelSummary struct {
	// Basic Model Information
	ModelPath  string    `json:"model_path"`
	OutputPath string    `json:"output_path"`
	RunID      string    `json:"run_id"`
	Timestamp  time.Time `json:"timestamp"`
	RunTime    *int64    `json:"run_time_ms,omitempty"` // Runtime in milliseconds

	// Estimation Results
	Estimation EstimationSummary `json:"estimation"`

	// Parameter Results
	Parameters ParameterSummary `json:"parameters"`

	// Model Fit Metrics
	GoodnessOfFit GoodnessOfFitSummary `json:"goodness_of_fit"`

	// Convergence and Stability
	Diagnostics DiagnosticSummary `json:"diagnostics"`

	// Table-based diagnostics (from SDTAB, PATAB, etc.)
	TableDiagnostics *TableDiagnostics `json:"table_diagnostics,omitempty"`

	// File References for Audit Trail
	SourceFiles []SourceFile `json:"source_files"`

	// Processing Metadata
	SummaryVersion string    `json:"summary_version"`
	ProcessedBy    string    `json:"processed_by"`
	ProcessedAt    time.Time `json:"processed_at"`
}

// EstimationSummary contains core estimation method and run characteristics.
type EstimationSummary struct {
	Method            string `json:"method"`             // e.g., "FOCE", "SAEM", "IMP"
	Subjects          int    `json:"subjects"`           // Number of subjects
	Observations      int    `json:"observations"`       // Number of observations
	SignificantDigits int    `json:"significant_digits"` // Precision setting
	TerminationReason string `json:"termination_reason"` // How the run ended
	Minimized         bool   `json:"minimized"`          // Successfully minimized
}

// ParameterSummary contains parameter estimates and related statistics.
type ParameterSummary struct {
	TotalParameters     int                 `json:"total_parameters"`
	EstimatedParameters int                 `json:"estimated_parameters"`
	FixedParameters     int                 `json:"fixed_parameters"`
	Thetas              []ParameterEstimate `json:"thetas,omitempty"`
	Omegas              []ParameterEstimate `json:"omegas,omitempty"`
	Sigmas              []ParameterEstimate `json:"sigmas,omitempty"`
}

// ParameterEstimate represents a single parameter with its statistics.
type ParameterEstimate struct {
	Name      string   `json:"name"`                // e.g., "THETA1", "OMEGA(1,1)"
	Label     string   `json:"label,omitempty"`     // Human-readable label from control stream (e.g., "CL", "Ka")
	Estimate  *float64 `json:"estimate,omitempty"`  // Parameter estimate
	StdError  *float64 `json:"std_error,omitempty"` // Standard error
	Fixed     bool     `json:"fixed"`               // Whether parameter was fixed
	Shrinkage *float64 `json:"shrinkage,omitempty"` // Shrinkage (for random effects)
}

// GoodnessOfFitSummary contains model fit statistics.
type GoodnessOfFitSummary struct {
	ObjectiveFunctionValue *float64 `json:"ofv,omitempty"`            // Primary fit metric
	AIC                    *float64 `json:"aic,omitempty"`            // Akaike Information Criterion
	BIC                    *float64 `json:"bic,omitempty"`            // Bayesian Information Criterion
	LogLikelihood          *float64 `json:"log_likelihood,omitempty"` // -2 Log-likelihood
}

// DiagnosticSummary contains convergence and stability indicators.
type DiagnosticSummary struct {
	ConditionNumber        *float64        `json:"condition_number,omitempty"`
	CovarianceStepSuccess  bool            `json:"covariance_step_success"`
	CovarianceStepAborted  bool            `json:"covariance_step_aborted"`
	LargeConditionNumber   bool            `json:"large_condition_number"`
	EigenvalueIssues       bool            `json:"eigenvalue_issues"`
	ParametersNearBoundary bool            `json:"parameters_near_boundary"`
	HessianReset           bool            `json:"hessian_reset"`
	MinimizationTerminated bool            `json:"minimization_terminated"`
	Warnings               []string        `json:"warnings,omitempty"`
	Errors                 []string        `json:"errors,omitempty"`
	HeuristicFlags         map[string]bool `json:"heuristic_flags,omitempty"`
}

// SourceFile represents a file used in generating the summary (for audit trail).
type SourceFile struct {
	Path         string    `json:"path"`
	Type         string    `json:"type"` // e.g., "control", "output", "ext"
	Size         int64     `json:"size"` // File size in bytes
	ModifiedTime time.Time `json:"modified_time"`
	Checksum     string    `json:"checksum"` // For integrity verification
}
