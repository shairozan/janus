//go:build unit
// +build unit

package summary

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNONMEMParser_EstimationSummary(t *testing.T) {
	// Create a temporary output file with sample NONMEM content
	sampleOutput := `
 NONMEM 7.5.0

 ESTIMATION METHOD: FOCE WITH INTERACTION

 TOTAL NO. OF INDIVIDUALS: 100
 TOTAL NO. OF OBSERVATIONS: 500
 SIGNIFICANT DIGITS IN FINAL RESULTS: 3

 MINIMIZATION SUCCESSFUL
`

	tmpFile := createTempFile(t, "test_output.lst", sampleOutput)
	defer os.Remove(tmpFile)

	summarizer := NewNONMEMSummarizer()
	var estimation EstimationSummary

	err := summarizer.parseEstimationSummary(tmpFile, &estimation)
	require.NoError(t, err)

	assert.Equal(t, "FOCE WITH INTERACTION", estimation.Method)
	assert.Equal(t, 100, estimation.Subjects)
	assert.Equal(t, 500, estimation.Observations)
	assert.Equal(t, 3, estimation.SignificantDigits)
	assert.Equal(t, "MINIMIZATION SUCCESSFUL", estimation.TerminationReason)
	assert.True(t, estimation.Minimized)
}

func TestNONMEMParser_GoodnessOfFit(t *testing.T) {
	sampleOutput := `
 #OBJV:*******************  -1234.567

 AIC = 2478.134
 BIC = 2512.890
`

	tmpFile := createTempFile(t, "test_gof.lst", sampleOutput)
	defer os.Remove(tmpFile)

	summarizer := NewNONMEMSummarizer()
	var gof GoodnessOfFitSummary

	err := summarizer.parseGoodnessOfFit(tmpFile, &gof)
	require.NoError(t, err)

	require.NotNil(t, gof.ObjectiveFunctionValue)
	assert.Equal(t, -1234.567, *gof.ObjectiveFunctionValue)

	require.NotNil(t, gof.LogLikelihood)
	assert.InDelta(t, 617.2835, *gof.LogLikelihood, 0.001)

	require.NotNil(t, gof.AIC)
	assert.Equal(t, 2478.134, *gof.AIC)

	require.NotNil(t, gof.BIC)
	assert.Equal(t, 2512.890, *gof.BIC)
}

func TestNONMEMParser_DiagnosticSummary(t *testing.T) {
	sampleOutput := `
 CONDITION NUMBER = 125.34

 COVARIANCE STEP ABORTED
 WARNING: PARAMETER NEAR BOUNDARY
 HESSIAN RESET
 ERROR: EIGENVALUE PROBLEM DETECTED
`

	tmpFile := createTempFile(t, "test_diag.lst", sampleOutput)
	defer os.Remove(tmpFile)

	summarizer := NewNONMEMSummarizer()
	var diagnostics DiagnosticSummary

	err := summarizer.parseDiagnosticSummary(tmpFile, &diagnostics)
	require.NoError(t, err)

	require.NotNil(t, diagnostics.ConditionNumber)
	assert.Equal(t, 125.34, *diagnostics.ConditionNumber)
	assert.False(t, diagnostics.LargeConditionNumber) // 125 < 1000

	assert.True(t, diagnostics.CovarianceStepAborted)
	assert.False(t, diagnostics.CovarianceStepSuccess)
	assert.True(t, diagnostics.ParametersNearBoundary)
	assert.True(t, diagnostics.HessianReset)
	assert.True(t, diagnostics.EigenvalueIssues)

	// Check heuristic flags
	assert.True(t, diagnostics.HeuristicFlags["covariance_step_aborted"])
	assert.True(t, diagnostics.HeuristicFlags["parameters_near_boundary"])
	assert.True(t, diagnostics.HeuristicFlags["hessian_reset"])
	assert.True(t, diagnostics.HeuristicFlags["eigenvalue_issues"])

	// Check collected warnings and errors
	// Should have: "WARNING: PARAMETER NEAR BOUNDARY" and "HESSIAN RESET"
	assert.Len(t, diagnostics.Warnings, 2) // WARNING and HESSIAN RESET lines
	assert.Len(t, diagnostics.Errors, 1)   // ERROR line
}

func TestNONMEMParser_ParameterSummary(t *testing.T) {
	sampleExtFile := `
; NONMEM 7.5.0
; Final parameter estimates
 ITERATION    THETA1    THETA2    OMEGA(1,1)    SIGMA(1,1)
     -1000     1.5      2.3       0.15          0.05
`

	tmpFile := createTempFile(t, "test.ext", sampleExtFile)
	defer os.Remove(tmpFile)

	summarizer := NewNONMEMSummarizer()
	var parameters ParameterSummary

	err := summarizer.parseParameterSummary(tmpFile, &parameters)
	require.NoError(t, err)

	assert.Equal(t, 4, parameters.TotalParameters)
	assert.Equal(t, 4, parameters.EstimatedParameters)
	assert.Equal(t, 0, parameters.FixedParameters)

	// Check THETA parameters
	require.Len(t, parameters.Thetas, 2)
	assert.Equal(t, "THETA1", parameters.Thetas[0].Name)
	require.NotNil(t, parameters.Thetas[0].Estimate)
	assert.Equal(t, 1.5, *parameters.Thetas[0].Estimate)

	assert.Equal(t, "THETA2", parameters.Thetas[1].Name)
	require.NotNil(t, parameters.Thetas[1].Estimate)
	assert.Equal(t, 2.3, *parameters.Thetas[1].Estimate)

	// Check OMEGA parameters
	require.Len(t, parameters.Omegas, 1)
	assert.Equal(t, "OMEGA(1,1)", parameters.Omegas[0].Name)
	require.NotNil(t, parameters.Omegas[0].Estimate)
	assert.Equal(t, 0.15, *parameters.Omegas[0].Estimate)

	// Check SIGMA parameters
	require.Len(t, parameters.Sigmas, 1)
	assert.Equal(t, "SIGMA(1,1)", parameters.Sigmas[0].Name)
	require.NotNil(t, parameters.Sigmas[0].Estimate)
	assert.Equal(t, 0.05, *parameters.Sigmas[0].Estimate)
}

func TestNONMEMParser_DataFileExtraction(t *testing.T) {
	sampleControlFile := `
$PROBLEM Test model
$DATA "data.csv" IGNORE=@
$INPUT ID TIME DV AMT
$SUBROUTINES ADVAN1 TRANS2
`

	tmpFile := createTempFile(t, "test.mod", sampleControlFile)
	defer os.Remove(tmpFile)

	summarizer := NewNONMEMSummarizer()

	dataFiles, err := summarizer.extractDataFiles(tmpFile)
	require.NoError(t, err)

	require.Len(t, dataFiles, 1)
	assert.Equal(t, "data.csv", dataFiles[0])
}

func TestNONMEMParser_CompleteWorkflow(t *testing.T) {
	// Create a complete set of test files
	tmpDir := t.TempDir()

	// Control file
	controlContent := `
$PROBLEM Test pharmacokinetic model
$DATA "data.csv" IGNORE=@
$INPUT ID TIME DV AMT EVID
$SUBROUTINES ADVAN1 TRANS2
`
	controlFile := filepath.Join(tmpDir, "test.mod")
	writeFile(t, controlFile, controlContent)

	// Output file
	outputContent := `
 NONMEM 7.5.0

 ESTIMATION METHOD: FOCE WITH INTERACTION
 TOTAL NO. OF INDIVIDUALS: 50
 TOTAL NO. OF OBSERVATIONS: 250
 SIGNIFICANT DIGITS IN FINAL RESULTS: 3
 CONDITION NUMBER = 89.2
 COVARIANCE STEP
 MINIMIZATION SUCCESSFUL
 #OBJV:*******************  -987.654
 AIC = 1985.308
 TOTAL NO. OF ELAPSED SECONDS: 45.2
`
	outputFile := filepath.Join(tmpDir, "test.lst")
	writeFile(t, outputFile, outputContent)

	// Ext file
	extContent := `
; Final parameter estimates
 ITERATION    THETA1    THETA2    OMEGA(1,1)    SIGMA(1,1)
     -1000     0.8      15.2      0.12          0.03
`
	extFile := filepath.Join(tmpDir, "test.ext")
	writeFile(t, extFile, extContent)

	// Test the complete summarization workflow
	summarizer := NewNONMEMSummarizer()
	options := DefaultSummaryOptions()

	summary, err := summarizer.SummarizeModel(context.Background(), controlFile, options)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify basic summary information
	assert.Equal(t, controlFile, summary.ModelPath)
	assert.Equal(t, outputFile, summary.OutputPath)
	assert.NotEmpty(t, summary.RunID)
	assert.Equal(t, "janus-nonmem-summarizer", summary.ProcessedBy)

	// Verify estimation summary
	assert.Equal(t, "FOCE WITH INTERACTION", summary.Estimation.Method)
	assert.Equal(t, 50, summary.Estimation.Subjects)
	assert.Equal(t, 250, summary.Estimation.Observations)
	assert.True(t, summary.Estimation.Minimized)

	// Verify goodness of fit
	require.NotNil(t, summary.GoodnessOfFit.ObjectiveFunctionValue)
	assert.Equal(t, -987.654, *summary.GoodnessOfFit.ObjectiveFunctionValue)
	require.NotNil(t, summary.GoodnessOfFit.AIC)
	assert.Equal(t, 1985.308, *summary.GoodnessOfFit.AIC)

	// Verify parameters
	assert.Equal(t, 4, summary.Parameters.TotalParameters)
	assert.Len(t, summary.Parameters.Thetas, 2)

	// Verify diagnostics
	require.NotNil(t, summary.Diagnostics.ConditionNumber)
	assert.Equal(t, 89.2, *summary.Diagnostics.ConditionNumber)
	assert.True(t, summary.Diagnostics.CovarianceStepSuccess)

	// Verify runtime
	require.NotNil(t, summary.RunTime)
	assert.Equal(t, int64(45200), *summary.RunTime) // 45.2 seconds = 45200 ms

	// Verify source files
	assert.Len(t, summary.SourceFiles, 3) // control, output, ext files

	t.Logf("Generated complete summary:")
	t.Logf("  Model: %s", summary.ModelPath)
	t.Logf("  Method: %s", summary.Estimation.Method)
	t.Logf("  Subjects: %d", summary.Estimation.Subjects)
	t.Logf("  OFV: %.3f", *summary.GoodnessOfFit.ObjectiveFunctionValue)
	t.Logf("  Parameters: %d", summary.Parameters.TotalParameters)
	t.Logf("  Runtime: %d ms", *summary.RunTime)
}

// Helper functions for testing

func createTempFile(t *testing.T, name, content string) string {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, name)
	writeFile(t, tmpFile, content)
	return tmpFile
}

func writeFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	require.NoError(t, err)
}
