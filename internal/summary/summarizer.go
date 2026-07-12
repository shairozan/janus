package summary

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Summarizer provides functionality to generate comprehensive model summaries.
type Summarizer interface {
	// SummarizeModel generates a complete summary for a model run.
	SummarizeModel(ctx context.Context, modelPath string, options SummaryOptions) (*ModelSummary, error)

	// SummarizeModelWithFiles generates a summary using explicitly provided file paths.
	SummarizeModelWithFiles(ctx context.Context, files ModelFiles, options SummaryOptions) (*ModelSummary, error)

	// ValidateModelFiles checks if required files exist for summarization.
	ValidateModelFiles(modelPath string) (*ModelFiles, error)

	// GetSupportedFormats returns the model formats this summarizer supports.
	GetSupportedFormats() []string
}

// ModelFiles represents the set of files associated with a model run.
type ModelFiles struct {
	ControlFile string   `json:"control_file"` // .mod, .ctl file
	OutputFile  string   `json:"output_file"`  // .lst, .out file
	ExtFile     string   `json:"ext_file"`     // .ext file (parameter estimates)
	CovFile     string   `json:"cov_file"`     // .cov file (covariance matrix)
	CorFile     string   `json:"cor_file"`     // .cor file (correlation matrix)
	PhiFile     string   `json:"phi_file"`     // .phi file (individual parameters)
	DataFiles   []string `json:"data_files"`   // Associated data files
}

// NONMEMSummarizer implements Summarizer for NONMEM models.
type NONMEMSummarizer struct {
	version string
}

// NewNONMEMSummarizer creates a new NONMEM model summarizer.
func NewNONMEMSummarizer() *NONMEMSummarizer {
	return &NONMEMSummarizer{
		version: "1.0.0",
	}
}

// GetSupportedFormats returns the model formats this summarizer supports.
func (s *NONMEMSummarizer) GetSupportedFormats() []string {
	return []string{".mod", ".ctl"}
}

// SummarizeModel generates a complete summary for a NONMEM model run.
func (s *NONMEMSummarizer) SummarizeModel(ctx context.Context, modelPath string, options SummaryOptions) (*ModelSummary, error) {
	// First, discover and validate the model files
	files, err := s.ValidateModelFiles(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate model files: %w", err)
	}

	return s.SummarizeModelWithFiles(ctx, *files, options)
}

// SummarizeModelWithFiles generates a summary using explicitly provided file paths.
func (s *NONMEMSummarizer) SummarizeModelWithFiles(ctx context.Context, files ModelFiles, options SummaryOptions) (*ModelSummary, error) {
	startTime := time.Now()

	summary := &ModelSummary{
		ModelPath:      files.ControlFile,
		OutputPath:     files.OutputFile,
		RunID:          generateRunID(files.ControlFile),
		Timestamp:      startTime,
		SummaryVersion: s.version,
		ProcessedBy:    "janus-nonmem-summarizer",
		ProcessedAt:    startTime,
	}

	// Parse estimation results from output file
	if files.OutputFile != "" {
		if err := s.parseEstimationSummary(files.OutputFile, &summary.Estimation); err != nil {
			return nil, fmt.Errorf("failed to parse estimation summary: %w", err)
		}

		if options.IncludeDiagnostics {
			if err := s.parseDiagnosticSummary(files.OutputFile, &summary.Diagnostics); err != nil {
				return nil, fmt.Errorf("failed to parse diagnostics: %w", err)
			}
		}

		if err := s.parseGoodnessOfFit(files.OutputFile, &summary.GoodnessOfFit); err != nil {
			return nil, fmt.Errorf("failed to parse goodness of fit: %w", err)
		}
	}

	// Parse parameter estimates from .ext file
	if options.IncludeParameters && files.ExtFile != "" {
		if err := s.parseParameterSummary(files.ExtFile, &summary.Parameters); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
	}

	// Collect source files for audit trail
	if options.IncludeSourceFiles {
		sourceFiles, err := s.collectSourceFiles(files, options.ComputeChecksum)
		if err != nil && options.FailOnMissingFiles {
			return nil, fmt.Errorf("failed to collect source files: %w", err)
		}
		summary.SourceFiles = sourceFiles
	}

	// Calculate runtime if we have timing information
	summary.RunTime = s.calculateRunTime(files.OutputFile)

	return summary, nil
}

// ValidateModelFiles discovers and validates the files associated with a model.
func (s *NONMEMSummarizer) ValidateModelFiles(modelPath string) (*ModelFiles, error) {
	if modelPath == "" {
		return nil, SummaryError{Type: "invalid_input", Message: "model path cannot be empty"}
	}

	// Get the base path without extension
	basePath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath))
	dir := filepath.Dir(modelPath)

	files := &ModelFiles{
		ControlFile: modelPath,
	}

	// Discover common NONMEM output files
	files.OutputFile = s.findFile(basePath, []string{".lst", ".out"})
	files.ExtFile = s.findFile(basePath, []string{".ext"})
	files.CovFile = s.findFile(basePath, []string{".cov"})
	files.CorFile = s.findFile(basePath, []string{".cor"})
	files.PhiFile = s.findFile(basePath, []string{".phi"})

	// Look for data files referenced in the control file
	dataFiles, err := s.extractDataFiles(modelPath)
	if err == nil {
		// Make data file paths absolute relative to model directory
		for _, dataFile := range dataFiles {
			if !filepath.IsAbs(dataFile) {
				dataFile = filepath.Join(dir, dataFile)
			}
			files.DataFiles = append(files.DataFiles, dataFile)
		}
	}

	return files, nil
}

// Note: Implementation of parsing methods are in nonmem_parser.go

func generateRunID(modelPath string) string {
	// Generate a unique run ID based on model path and timestamp
	base := filepath.Base(modelPath)
	timestamp := time.Now().Format("20060102-150405")

	return fmt.Sprintf("%s-%s", strings.TrimSuffix(base, filepath.Ext(base)), timestamp)
}

// NewSummarizer is a factory function to create appropriate summarizer based on model type.
func NewSummarizer(modelPath string) (Summarizer, error) {
	ext := strings.ToLower(filepath.Ext(modelPath))

	switch ext {
	case ".mod", ".ctl":
		return NewNONMEMSummarizer(), nil
	default:
		return nil, SummaryError{
			Type:    "unsupported_format",
			Message: fmt.Sprintf("unsupported model format: %s", ext),
			File:    modelPath,
		}
	}
}
