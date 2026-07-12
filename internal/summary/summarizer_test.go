//go:build unit
// +build unit

package summary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSummarizer(t *testing.T) {
	tests := []struct {
		name        string
		modelPath   string
		expectError bool
		expectType  string
	}{
		{
			name:        "NONMEM mod file",
			modelPath:   "/path/to/model.mod",
			expectError: false,
			expectType:  "*summary.NONMEMSummarizer",
		},
		{
			name:        "NONMEM ctl file",
			modelPath:   "/path/to/model.ctl",
			expectError: false,
			expectType:  "*summary.NONMEMSummarizer",
		},
		{
			name:        "unsupported format",
			modelPath:   "/path/to/model.txt",
			expectError: true,
		},
		{
			name:        "no extension",
			modelPath:   "/path/to/model",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summarizer, err := NewSummarizer(tt.modelPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, summarizer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, summarizer)

				// Verify it supports the expected formats
				formats := summarizer.GetSupportedFormats()
				assert.Contains(t, formats, ".mod")
				assert.Contains(t, formats, ".ctl")
			}
		})
	}
}

func TestNONMEMSummarizer_ValidateModelFiles(t *testing.T) {
	summarizer := NewNONMEMSummarizer()

	tests := []struct {
		name        string
		modelPath   string
		expectError bool
	}{
		{
			name:        "valid model path",
			modelPath:   "/project/models/run001.mod",
			expectError: false,
		},
		{
			name:        "empty model path",
			modelPath:   "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := summarizer.ValidateModelFiles(tt.modelPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, files)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, files)
				assert.Equal(t, tt.modelPath, files.ControlFile)

				// Verify expected file paths are generated
				if tt.modelPath != "" {
					assert.NotEmpty(t, files.OutputFile)
					assert.NotEmpty(t, files.ExtFile)
				}
			}
		})
	}
}

func TestModelSummary_Structure(t *testing.T) {
	// Test that we can create and populate a ModelSummary
	summary := &ModelSummary{
		ModelPath: "/project/models/run001.mod",
		RunID:     "run001-20231201-120000",
	}

	// Add some sample data
	summary.Parameters = ParameterSummary{
		TotalParameters:     10,
		EstimatedParameters: 8,
		FixedParameters:     2,
		Thetas: []ParameterEstimate{
			{Name: "THETA1", Estimate: ptrFloat64(1.5), StdError: ptrFloat64(0.15), Fixed: false},
			{Name: "THETA2", Estimate: ptrFloat64(2.3), StdError: ptrFloat64(0.23), Fixed: false},
		},
	}

	summary.GoodnessOfFit = GoodnessOfFitSummary{
		ObjectiveFunctionValue: ptrFloat64(-1234.56),
		AIC:                    ptrFloat64(2478.12),
	}

	summary.Diagnostics = DiagnosticSummary{
		CovarianceStepSuccess: true,
		ConditionNumber:       ptrFloat64(125.3),
		Warnings:              []string{"Parameter near boundary"},
	}

	// Verify structure
	assert.Equal(t, "/project/models/run001.mod", summary.ModelPath)
	assert.Equal(t, 10, summary.Parameters.TotalParameters)
	assert.Len(t, summary.Parameters.Thetas, 2)
	assert.Equal(t, "THETA1", summary.Parameters.Thetas[0].Name)
	assert.NotNil(t, summary.GoodnessOfFit.ObjectiveFunctionValue)
	assert.True(t, summary.Diagnostics.CovarianceStepSuccess)
	assert.Len(t, summary.Diagnostics.Warnings, 1)
}

func TestSummaryOptions(t *testing.T) {
	// Test default options
	defaults := DefaultSummaryOptions()
	assert.True(t, defaults.IncludeParameters)
	assert.False(t, defaults.IncludeCovariance) // Expensive for large models
	assert.True(t, defaults.IncludeDiagnostics)
	assert.True(t, defaults.IncludeSourceFiles)
	assert.True(t, defaults.ComputeChecksum)
	assert.False(t, defaults.FailOnMissingFiles) // Be tolerant
	assert.Equal(t, 1000, defaults.MaxParameterCount)

	// Test custom options
	custom := SummaryOptions{
		IncludeParameters:  false,
		IncludeCovariance:  true,
		FailOnMissingFiles: true,
		MaxParameterCount:  500,
	}

	assert.False(t, custom.IncludeParameters)
	assert.True(t, custom.IncludeCovariance)
	assert.True(t, custom.FailOnMissingFiles)
	assert.Equal(t, 500, custom.MaxParameterCount)
}

func TestSummaryError(t *testing.T) {
	// Test error without file
	err1 := SummaryError{
		Type:    "parse_error",
		Message: "invalid syntax",
	}
	assert.Equal(t, "parse_error: invalid syntax", err1.Error())

	// Test error with file
	err2 := SummaryError{
		Type:    "missing_file",
		Message: "file not found",
		File:    "/path/to/model.ext",
	}
	assert.Equal(t, "missing_file in /path/to/model.ext: file not found", err2.Error())
}

func TestFunctionalAPI_Example(t *testing.T) {
	// This test demonstrates how the API would be used functionally
	tmpDir := t.TempDir()
	modelPath := createTestModelFile(t, tmpDir, "analysis.mod")
	_ = createTestOutputFile(t, tmpDir, "analysis.lst")
	_ = createTestExtFile(t, tmpDir, "analysis.ext")

	// Create a summarizer
	summarizer, err := NewSummarizer(modelPath)
	require.NoError(t, err)
	require.NotNil(t, summarizer)

	// Validate the model files
	files, err := summarizer.ValidateModelFiles(modelPath)
	require.NoError(t, err)
	require.NotNil(t, files)

	// Configure summary options
	options := SummaryOptions{
		IncludeParameters:  true,
		IncludeDiagnostics: true,
		IncludeSourceFiles: true,
		ComputeChecksum:    false, // Skip for test
		FailOnMissingFiles: false,
	}

	// Generate summary using actual files
	ctx := context.Background()
	summary, err := summarizer.SummarizeModelWithFiles(ctx, *files, options)

	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify basic structure
	assert.Equal(t, modelPath, summary.ModelPath)
	assert.NotEmpty(t, summary.RunID)
	assert.Equal(t, "janus-nonmem-summarizer", summary.ProcessedBy)
	assert.NotZero(t, summary.ProcessedAt)

	t.Logf("Generated summary for model: %s", summary.ModelPath)
	t.Logf("Run ID: %s", summary.RunID)
	t.Logf("Processed by: %s", summary.ProcessedBy)
}

// Helper function for test data
func ptrFloat64(f float64) *float64 {
	return &f
}
