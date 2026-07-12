package summary

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/pharmalytica/janus/internal/runlog"
)

// Service provides model summarization functionality with run log integration.
type Service struct {
	summarizers map[string]Summarizer
	auditLogger *runlog.RunLogger
	options     SummaryOptions
}

// NewService creates a new model summary service.
func NewService(auditLogger *runlog.RunLogger, options SummaryOptions) *Service {
	service := &Service{
		summarizers: make(map[string]Summarizer),
		auditLogger: auditLogger,
		options:     options,
	}

	// Register default summarizers
	nonmemSummarizer := NewNONMEMSummarizer()
	for _, format := range nonmemSummarizer.GetSupportedFormats() {
		service.summarizers[format] = nonmemSummarizer
	}

	return service
}

// SummarizeModel generates a model summary and optionally logs it to audit trail.
func (s *Service) SummarizeModel(ctx context.Context, modelPath string) (*ModelSummary, error) {
	// Determine summarizer based on file extension
	ext := filepath.Ext(modelPath)
	summarizer, exists := s.summarizers[ext]
	if !exists {
		return nil, fmt.Errorf("no summarizer available for file type: %s", ext)
	}

	log.Printf("Generating summary for model: %s", modelPath)
	startTime := time.Now()

	// Generate the summary
	summary, err := summarizer.SummarizeModel(ctx, modelPath, s.options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate model summary: %w", err)
	}

	duration := time.Since(startTime)
	log.Printf("Model summary generated in %v for %s", duration, modelPath)

	// Validate that we got essential summary components
	if summary.RunID == "" {
		log.Printf("Warning: Generated summary has empty RunID for %s", modelPath)
	}

	return summary, nil
}

// SummarizeModelWithAudit generates a model summary and logs it to the audit trail.
func (s *Service) SummarizeModelWithAudit(ctx context.Context, jobID, modelPath string, _ time.Duration, _ string, _ []string) (*ModelSummary, error) {
	// Generate the summary
	summary, err := s.SummarizeModel(ctx, modelPath)
	if err != nil {
		// TODO: Log the failure to run log once LogExecutionWithSummary is implemented
		// For now, just log to console
		if s.auditLogger != nil && s.auditLogger.IsEnabled() {
			log.Printf("Summary generation failed for job %s: %v", jobID, err)
		}

		return nil, err
	}

	// TODO: Log the successful summary to run log once LogExecutionWithSummary is implemented
	// For now, just log to console
	if s.auditLogger != nil && s.auditLogger.IsEnabled() {
		log.Printf("Model summary generated for job %s (run: %s)", jobID, summary.RunID)
	}

	return summary, nil
}

// SummarizeModelFiles generates a summary using explicitly provided file paths.
func (s *Service) SummarizeModelFiles(ctx context.Context, modelPath string, files ModelFiles) (*ModelSummary, error) {
	// Determine summarizer based on file extension
	ext := filepath.Ext(modelPath)
	summarizer, exists := s.summarizers[ext]
	if !exists {
		return nil, fmt.Errorf("no summarizer available for file type: %s", ext)
	}

	log.Printf("Generating summary for model files: %+v", files)
	startTime := time.Now()

	// Generate the summary using explicit files
	summary, err := summarizer.SummarizeModelWithFiles(ctx, files, s.options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate model summary from files: %w", err)
	}

	duration := time.Since(startTime)
	log.Printf("Model summary generated in %v for files: %s", duration, files.ControlFile)

	return summary, nil
}

// ValidateModelFiles checks if required files exist for summarization.
func (s *Service) ValidateModelFiles(modelPath string) (*ModelFiles, error) {
	// Determine summarizer based on file extension
	ext := filepath.Ext(modelPath)
	summarizer, exists := s.summarizers[ext]
	if !exists {
		return nil, fmt.Errorf("no summarizer available for file type: %s", ext)
	}

	return summarizer.ValidateModelFiles(modelPath)
}

// RegisterSummarizer adds a new summarizer for specific file formats.
func (s *Service) RegisterSummarizer(formats []string, summarizer Summarizer) {
	for _, format := range formats {
		s.summarizers[format] = summarizer
		log.Printf("Registered summarizer for format: %s", format)
	}
}

// GetSupportedFormats returns all supported model file formats.
func (s *Service) GetSupportedFormats() []string {
	formats := make([]string, 0, len(s.summarizers))
	for format := range s.summarizers {
		formats = append(formats, format)
	}

	return formats
}

// UpdateOptions updates the summary generation options.
func (s *Service) UpdateOptions(options SummaryOptions) {
	s.options = options
	log.Printf("Updated summary options: %+v", options)
}
