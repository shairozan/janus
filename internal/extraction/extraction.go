// Package extraction provides functionality to extract model parameters from
// run log records. It sits above both runlog and summary packages to avoid
// import cycles.
package extraction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/summary"
)

var (
	logger     *logrus.Logger
	loggerOnce sync.Once
)

// getLogger returns the package logger, initializing it on first use.
func getLogger() *logrus.Logger {
	loggerOnce.Do(func() {
		logger = logrus.New()
		logger.SetOutput(os.Stderr)

		// Enable debug logging via JANUS_DEBUG environment variable
		debugEnv := strings.ToLower(os.Getenv("JANUS_DEBUG"))
		if debugEnv == "true" || debugEnv == "1" {
			logger.SetLevel(logrus.DebugLevel)
		} else {
			logger.SetLevel(logrus.WarnLevel)
		}

		logger.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: true,
			ForceColors:      false,
			DisableColors:    true,
			PadLevelText:     true,
		})
	})

	return logger
}

// ExtractSummary extracts model parameters from embedded output files in a RunRecord.
// Returns nil with no error if required files are not available (non-fatal).
// Returns error only for actual parsing failures. ctx is passed to the summarizer
// so the parse honors the caller's cancellation/timeout.
func ExtractSummary(ctx context.Context, record *runlog.RunRecord) (*model.ModelSummary, error) {
	if record == nil {
		return nil, fmt.Errorf("record is nil")
	}

	// Check if we have the required embedded files
	if record.EmbeddedFiles == nil {
		return nil, nil // No embedded files, nothing to extract
	}

	// We need at least one of .ext or .lst to extract anything useful. Embedded
	// files are keyed by filename now, so resolve the key by extension.
	extName := runlog.EmbeddedNameForExt(record, "ext")
	lstName := runlog.EmbeddedNameForExt(record, "lst")

	if extName == "" && lstName == "" {
		return nil, nil // No relevant files to parse
	}

	// Create temp directory for extracted files
	tempDir, err := os.MkdirTemp("", "janus-extraction-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract files to temp directory
	files := summary.ModelFiles{}

	if extName != "" {
		extPath, err := extractToTemp(record, extName, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract .ext file: %w", err)
		}
		files.ExtFile = extPath
	}

	if lstName != "" {
		lstPath, err := extractToTemp(record, lstName, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract .lst file: %w", err)
		}
		files.OutputFile = lstPath
	}

	// Set control file path from record for context
	files.ControlFile = record.ModelFile

	// Use the NONMEM summarizer to parse the extracted files
	summarizer := summary.NewNONMEMSummarizer()
	opts := summary.SummaryOptions{
		IncludeParameters:  true,
		IncludeDiagnostics: true,
		IncludeSourceFiles: false, // Don't include temp file paths
		ComputeChecksum:    false,
		FailOnMissingFiles: false,
	}

	result, err := summarizer.SummarizeModelWithFiles(ctx, files, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to parse model output: %w", err)
	}

	// Try to extract parameter labels from the embedded control file.
	if modName := runlog.EmbeddedNameForExt(record, "mod"); modName != "" {
		modData, err := runlog.ExtractEmbeddedFile(record, modName)
		if err != nil {
			getLogger().WithError(err).WithField("model", record.ModelFile).
				Debug("failed to extract .mod file for label parsing")
		} else if len(modData) > 0 {
			labels := model.ParseParameterLabels(string(modData))
			applyLabelsToParameters(&result.Parameters, labels)
		}
	}

	return result, nil
}

// applyLabelsToParameters merges parsed labels into parameter estimates.
func applyLabelsToParameters(params *model.ParameterSummary, labels *model.ParameterLabels) {
	if params == nil || labels == nil {
		return
	}

	// Apply theta labels (1-based index)
	for i := range params.Thetas {
		if label := labels.GetThetaLabel(i + 1); label != "" {
			params.Thetas[i].Label = label
		}
	}

	// Apply omega labels by matching matrix position
	for i := range params.Omegas {
		// Extract position from name like "OMEGA(1,1)" -> "1,1"
		pos := extractMatrixPosition(params.Omegas[i].Name)
		if pos != "" {
			if label := labels.GetOmegaLabel(pos); label != "" {
				params.Omegas[i].Label = label
			}
		}
	}

	// Apply sigma labels by matching matrix position
	for i := range params.Sigmas {
		pos := extractMatrixPosition(params.Sigmas[i].Name)
		if pos != "" {
			if label := labels.GetSigmaLabel(pos); label != "" {
				params.Sigmas[i].Label = label
			}
		}
	}
}

// extractMatrixPosition extracts the position from a matrix parameter name.
// For example, "OMEGA(1,1)" returns "1,1" and "SIGMA(2,2)" returns "2,2".
func extractMatrixPosition(name string) string {
	start := strings.Index(name, "(")
	end := strings.Index(name, ")")

	// Log debug info for malformed input that contains parentheses but is invalid
	if (start != -1 || end != -1) && (start == -1 || end == -1 || start >= end) {
		getLogger().WithField("name", name).Debug("malformed matrix parameter name, unable to extract position")

		return ""
	}

	if start == -1 || end == -1 {
		return ""
	}

	return name[start+1 : end]
}

// extractToTemp extracts an embedded file (by its key) to a temp directory and
// returns the path. The key may be a relative subpath (e.g. a PsN run tree), so
// only its base name is used for the temp file — the summarizer needs the content
// and the correct extension, not the directory structure.
func extractToTemp(record *runlog.RunRecord, name, tempDir string) (string, error) {
	data, err := runlog.ExtractEmbeddedFile(record, name)
	if err != nil {
		return "", err
	}

	filename := filepath.Join(tempDir, filepath.Base(name))
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return filename, nil
}

// HasExtractableFiles checks if a RunRecord has files that can be parsed for parameters.
func HasExtractableFiles(record *runlog.RunRecord) bool {
	if record == nil || record.EmbeddedFiles == nil {
		return false
	}

	return runlog.EmbeddedNameForExt(record, "ext") != "" ||
		runlog.EmbeddedNameForExt(record, "lst") != ""
}
