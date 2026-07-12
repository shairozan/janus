//go:build unit
// +build unit

package summary

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/runlog"
)

func TestService_SummarizeModel(t *testing.T) {
	// Create temporary audit log
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "runlog.log")

	auditLogger, err := runlog.NewRunLogger(true, auditLogPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	// Create service with default options
	options := DefaultSummaryOptions()
	service := NewService(auditLogger, options)

	// Test supported formats
	formats := service.GetSupportedFormats()
	assert.Contains(t, formats, ".mod")
	assert.Contains(t, formats, ".ctl")

	// Test model summarization with mock files
	modelPath := createTestModelFile(t, tmpDir, "test.mod")
	outputPath := createTestOutputFile(t, tmpDir, "test.lst")
	_ = createTestExtFile(t, tmpDir, "test.ext")

	ctx := context.Background()
	summary, err := service.SummarizeModel(ctx, modelPath)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify basic summary structure
	assert.Equal(t, modelPath, summary.ModelPath)
	assert.Equal(t, outputPath, summary.OutputPath)
	assert.NotEmpty(t, summary.RunID)
	assert.Equal(t, "janus-nonmem-summarizer", summary.ProcessedBy)

	// Verify estimation summary was parsed
	assert.Equal(t, "FOCE WITH INTERACTION", summary.Estimation.Method)
	assert.Equal(t, 100, summary.Estimation.Subjects)
	assert.Equal(t, 500, summary.Estimation.Observations)
	assert.True(t, summary.Estimation.Minimized)

	// Verify goodness of fit was parsed
	require.NotNil(t, summary.GoodnessOfFit.ObjectiveFunctionValue)
	assert.Equal(t, -1234.567, *summary.GoodnessOfFit.ObjectiveFunctionValue)
	require.NotNil(t, summary.GoodnessOfFit.AIC)
	assert.Equal(t, 2478.134, *summary.GoodnessOfFit.AIC)

	// Verify parameters were parsed
	assert.Equal(t, 4, summary.Parameters.TotalParameters)
	assert.Len(t, summary.Parameters.Thetas, 2)

	t.Logf("Successfully generated summary for %s", modelPath)
	t.Logf("Summary run ID: %s", summary.RunID)
}

func TestService_SummarizeModelWithAudit(t *testing.T) {
	// Create temporary audit log
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "runlog.log")

	auditLogger, err := runlog.NewRunLogger(true, auditLogPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	// Create service with default options
	options := DefaultSummaryOptions()
	service := NewService(auditLogger, options)

	// Create test files
	modelPath := createTestModelFile(t, tmpDir, "test.mod")
	_ = createTestOutputFile(t, tmpDir, "test.lst")
	_ = createTestExtFile(t, tmpDir, "test.ext")

	// Test summarization with audit logging
	ctx := context.Background()
	jobID := "test-job-123"
	executionDuration := 45200 * time.Millisecond // 45.2 seconds
	slurmJobID := "12345"
	outputFiles := []string{
		filepath.Join(tmpDir, "test.lst"),
		filepath.Join(tmpDir, "test.err"),
	}

	summary, err := service.SummarizeModelWithAudit(
		ctx,
		jobID,
		modelPath,
		executionDuration,
		slurmJobID,
		outputFiles,
	)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// TODO: Once LogExecutionWithSummary is implemented in RunLogger, we can verify the audit log
	// For now, just verify the summary was generated successfully
	assert.NotEmpty(t, summary.RunID)
	assert.Equal(t, modelPath, summary.ModelPath)

	t.Logf("Summary generated for job %s (audit logging not yet implemented)", jobID)

	// FUTURE: Verify the audit log contains the summary
	// auditEntries, err := runlog.ReadRunEntries(auditLogPath)
	// require.NoError(t, err)
	// require.Len(t, auditEntries, 1)
	//
	// entry := auditEntries[0]
	// assert.Equal(t, jobID, entry.JobID)
	// assert.Equal(t, "model-summarizer", entry.Binary)
	// assert.Equal(t, []string{modelPath}, entry.Arguments)
	// assert.Equal(t, 0, entry.ExitCode) // Success
	// assert.Equal(t, slurmJobID, entry.SLURMJobID)
	// assert.Equal(t, outputFiles, entry.OutputFiles)
	// assert.Contains(t, entry.STDOUT, "Generated summary for run")
	//
	// // Verify the model summary is embedded in the audit log
	// require.NotNil(t, entry.ModelSummary)
	//
	// // Since ModelSummary is interface{}, it will be a map[string]interface{} after JSON unmarshaling
	// summaryMap, ok := entry.ModelSummary.(map[string]interface{})
	// require.True(t, ok, "ModelSummary should be a map after JSON unmarshaling")
	// assert.Equal(t, summary.RunID, summaryMap["run_id"])
	// assert.Equal(t, summary.ModelPath, summaryMap["model_path"])
	//
	// // Check nested estimation data
	// estimationMap, ok := summaryMap["estimation"].(map[string]interface{})
	// require.True(t, ok)
	// assert.Equal(t, summary.Estimation.Method, estimationMap["method"])
}

func TestService_SummarizeModelFiles(t *testing.T) {
	// Create service without audit logger for this test
	options := DefaultSummaryOptions()
	service := NewService(nil, options)

	// Create test files
	tmpDir := t.TempDir()
	modelPath := createTestModelFile(t, tmpDir, "test.mod")
	outputPath := createTestOutputFile(t, tmpDir, "test.lst")
	extPath := createTestExtFile(t, tmpDir, "test.ext")

	// Create explicit file paths
	files := ModelFiles{
		ControlFile: modelPath,
		OutputFile:  outputPath,
		ExtFile:     extPath,
		DataFiles:   []string{"data.csv"},
	}

	ctx := context.Background()
	summary, err := service.SummarizeModelFiles(ctx, modelPath, files)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify the summary used explicit file paths
	assert.Equal(t, modelPath, summary.ModelPath)
	assert.Equal(t, outputPath, summary.OutputPath)

	t.Logf("Successfully generated summary using explicit files")
}

func TestService_UnsupportedFormat(t *testing.T) {
	options := DefaultSummaryOptions()
	service := NewService(nil, options)

	ctx := context.Background()
	_, err := service.SummarizeModel(ctx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no summarizer available for file type: .txt")
}

func TestService_RegisterSummarizer(t *testing.T) {
	options := DefaultSummaryOptions()
	service := NewService(nil, options)

	// Test initial formats
	initialFormats := service.GetSupportedFormats()
	assert.Contains(t, initialFormats, ".mod")
	assert.Contains(t, initialFormats, ".ctl")

	// Register a custom summarizer
	customSummarizer := NewNONMEMSummarizer()
	service.RegisterSummarizer([]string{".nmctl", ".nonmem"}, customSummarizer)

	// Verify new formats are supported
	newFormats := service.GetSupportedFormats()
	assert.Contains(t, newFormats, ".nmctl")
	assert.Contains(t, newFormats, ".nonmem")
}

func TestService_UpdateOptions(t *testing.T) {
	options := DefaultSummaryOptions()
	service := NewService(nil, options)

	// Update options
	newOptions := SummaryOptions{
		IncludeParameters:  false,
		IncludeCovariance:  true,
		IncludeDiagnostics: false,
		MaxParameterCount:  500,
	}

	service.UpdateOptions(newOptions)

	// Options are updated internally (we can't directly test this, but it's logged)
	assert.NotNil(t, service) // Basic test to ensure service is still valid
}

// Helper functions for creating test files

func createTestModelFile(t *testing.T, dir, filename string) string {
	content := `
$PROBLEM Test pharmacokinetic model
$DATA "data.csv" IGNORE=@
$INPUT ID TIME DV AMT EVID
$SUBROUTINES ADVAN1 TRANS2
$THETA 1.0 ; CL
$THETA 10.0 ; V
$OMEGA 0.1
$SIGMA 0.05
$ESTIMATION METHOD=1 INTER MAXEVAL=9999
`
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}

func createTestOutputFile(t *testing.T, dir, filename string) string {
	content := `
 NONMEM 7.5.0

 ESTIMATION METHOD: FOCE WITH INTERACTION

 TOTAL NO. OF INDIVIDUALS: 100
 TOTAL NO. OF OBSERVATIONS: 500
 SIGNIFICANT DIGITS IN FINAL RESULTS: 3
 CONDITION NUMBER = 89.2

 MINIMIZATION SUCCESSFUL

 #OBJV:*******************  -1234.567

 AIC = 2478.134
 BIC = 2512.890

 TOTAL NO. OF ELAPSED SECONDS: 45.2
`
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}

func createTestExtFile(t *testing.T, dir, filename string) string {
	content := `
; NONMEM 7.5.0
; Final parameter estimates
 ITERATION    THETA1    THETA2    OMEGA(1,1)    SIGMA(1,1)
     -1000     1.5      2.3       0.15          0.05
`
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}
