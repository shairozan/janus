package summary

import (
	"github.com/shairozan/janus/internal/model"
)

// Type aliases for backward compatibility.
// The canonical definitions are in the model package to avoid import cycles.
type (
	ModelSummary         = model.ModelSummary
	EstimationSummary    = model.EstimationSummary
	ParameterSummary     = model.ParameterSummary
	ParameterEstimate    = model.ParameterEstimate
	GoodnessOfFitSummary = model.GoodnessOfFitSummary
	DiagnosticSummary    = model.DiagnosticSummary
	SourceFile           = model.SourceFile
)

// SummaryOptions configures how model summaries are generated.
type SummaryOptions struct {
	IncludeParameters  bool `json:"include_parameters"`
	IncludeCovariance  bool `json:"include_covariance"`
	IncludeDiagnostics bool `json:"include_diagnostics"`
	IncludeSourceFiles bool `json:"include_source_files"`
	ComputeChecksum    bool `json:"compute_checksum"`
	FailOnMissingFiles bool `json:"fail_on_missing_files"`
	MaxParameterCount  int  `json:"max_parameter_count"` // Limit for large models
}

// DefaultSummaryOptions returns sensible defaults for summary generation.
func DefaultSummaryOptions() SummaryOptions {
	return SummaryOptions{
		IncludeParameters:  true,
		IncludeCovariance:  false, // Can be expensive for large models
		IncludeDiagnostics: true,
		IncludeSourceFiles: true,
		ComputeChecksum:    true,
		FailOnMissingFiles: false, // Be tolerant of incomplete runs
		MaxParameterCount:  1000,  // Reasonable limit
	}
}

// SummaryError represents errors during summary generation.
type SummaryError struct {
	Type    string `json:"type"` // e.g., "parse_error", "missing_file"
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
}

func (e SummaryError) Error() string {
	if e.File != "" {
		return e.Type + " in " + e.File + ": " + e.Message
	}

	return e.Type + ": " + e.Message
}
