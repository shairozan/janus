//go:build integration
// +build integration

package summary

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/runlog"
)

func TestIntegration_CompleteWorkflow(t *testing.T) {
	// Create a complete test directory structure
	testDir := t.TempDir()
	modelsDir := filepath.Join(testDir, "models")
	err := os.MkdirAll(modelsDir, 0755)
	require.NoError(t, err)

	// Create a realistic NONMEM model setup
	modelPath := createCompleteModelSetup(t, modelsDir, "pkmodel001")

	// Set up audit logging
	auditLogPath := filepath.Join(testDir, "runlog.jsonl")
	auditLogger, err := runlog.NewRunLogger(true, auditLogPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	// Create summary service
	options := SummaryOptions{
		IncludeParameters:  true,
		IncludeDiagnostics: true,
		IncludeSourceFiles: true,
		ComputeChecksum:    true,
		FailOnMissingFiles: false,
		MaxParameterCount:  1000,
	}
	service := NewService(auditLogger, options)

	// Test complete workflow: validation -> summarization -> audit logging
	t.Run("ValidateModelFiles", func(t *testing.T) {
		files, err := service.ValidateModelFiles(modelPath)
		require.NoError(t, err)
		require.NotNil(t, files)

		assert.Equal(t, modelPath, files.ControlFile)
		assert.Contains(t, files.OutputFile, "pkmodel001.lst")
		assert.Contains(t, files.ExtFile, "pkmodel001.ext")
		assert.Len(t, files.DataFiles, 1)
		assert.Equal(t, "pkdata.csv", files.DataFiles[0])
	})

	t.Run("GenerateSummaryWithAudit", func(t *testing.T) {
		ctx := context.Background()
		jobID := "integration-test-job-001"
		executionDuration := 125*time.Second + 750*time.Millisecond
		slurmJobID := "54321"
		outputFiles := []string{
			strings.Replace(modelPath, ".mod", ".lst", 1),
			strings.Replace(modelPath, ".mod", ".err", 1),
		}

		// Generate summary with audit logging
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

		// Verify comprehensive summary content
		assert.Equal(t, modelPath, summary.ModelPath)
		assert.NotEmpty(t, summary.RunID)
		assert.Equal(t, "janus-nonmem-summarizer", summary.ProcessedBy)

		// Verify estimation details
		assert.Equal(t, "FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION", summary.Estimation.Method)
		assert.Equal(t, 72, summary.Estimation.Subjects)
		assert.Equal(t, 360, summary.Estimation.Observations)
		assert.Equal(t, 4, summary.Estimation.SignificantDigits)
		assert.True(t, summary.Estimation.Minimized)

		// Verify parameter estimates
		assert.Equal(t, 6, summary.Parameters.TotalParameters)
		assert.Equal(t, 6, summary.Parameters.EstimatedParameters)
		assert.Equal(t, 0, summary.Parameters.FixedParameters)
		assert.Len(t, summary.Parameters.Thetas, 3)
		assert.Len(t, summary.Parameters.Omegas, 2)
		assert.Len(t, summary.Parameters.Sigmas, 1)

		// Verify THETA estimates
		theta1 := summary.Parameters.Thetas[0]
		assert.Equal(t, "THETA1", theta1.Name)
		require.NotNil(t, theta1.Estimate)
		assert.InDelta(t, 0.982, *theta1.Estimate, 0.001)

		// Verify goodness of fit
		require.NotNil(t, summary.GoodnessOfFit.ObjectiveFunctionValue)
		assert.InDelta(t, -2456.789, *summary.GoodnessOfFit.ObjectiveFunctionValue, 0.001)
		require.NotNil(t, summary.GoodnessOfFit.AIC)
		assert.InDelta(t, 4925.578, *summary.GoodnessOfFit.AIC, 0.001)
		require.NotNil(t, summary.GoodnessOfFit.BIC)
		assert.InDelta(t, 4959.123, *summary.GoodnessOfFit.BIC, 0.001)

		// Verify diagnostics
		require.NotNil(t, summary.Diagnostics.ConditionNumber)
		assert.InDelta(t, 67.8, *summary.Diagnostics.ConditionNumber, 0.1)
		assert.True(t, summary.Diagnostics.CovarianceStepSuccess)
		assert.False(t, summary.Diagnostics.LargeConditionNumber)
		assert.False(t, summary.Diagnostics.ParametersNearBoundary)

		// Verify runtime
		require.NotNil(t, summary.RunTime)
		assert.Equal(t, int64(125750), *summary.RunTime) // 125.75 seconds in ms

		// Verify source files were collected
		assert.Len(t, summary.SourceFiles, 4) // control, output, ext, data files

		t.Logf("Complete summary generated:")
		t.Logf("  Model: %s", summary.ModelPath)
		t.Logf("  Run ID: %s", summary.RunID)
		t.Logf("  Method: %s", summary.Estimation.Method)
		t.Logf("  Subjects: %d", summary.Estimation.Subjects)
		t.Logf("  OFV: %.3f", *summary.GoodnessOfFit.ObjectiveFunctionValue)
		t.Logf("  Parameters: %d total", summary.Parameters.TotalParameters)
		t.Logf("  Runtime: %d ms", *summary.RunTime)
	})

	t.Run("VerifyAuditTrail", func(t *testing.T) {
		// Read and verify audit log
		auditEntries, err := runlog.ReadRunEntries(auditLogPath)
		require.NoError(t, err)
		require.Len(t, auditEntries, 1)

		entry := auditEntries[0]
		assert.Equal(t, "integration-test-job-001", entry.JobID)
		assert.Equal(t, "model-summarizer", entry.Binary)
		assert.Equal(t, []string{modelPath}, entry.Arguments)
		assert.Equal(t, 0, entry.ExitCode)
		assert.Equal(t, "54321", entry.SLURMJobID)
		assert.Contains(t, entry.STDOUT, "Generated summary for run")

		// Verify the complete model summary is embedded
		require.NotNil(t, entry.ModelSummary)

		// Since ModelSummary is interface{}, it will be a map[string]interface{} after JSON unmarshaling
		summaryMap, ok := entry.ModelSummary.(map[string]interface{})
		require.True(t, ok, "ModelSummary should be a map after JSON unmarshaling")

		// Validate key summary fields are preserved in audit log
		assert.Equal(t, modelPath, summaryMap["model_path"])

		estimationMap, ok := summaryMap["estimation"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION", estimationMap["method"])
		assert.Equal(t, float64(72), estimationMap["subjects"])

		parametersMap, ok := summaryMap["parameters"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(6), parametersMap["total_parameters"])

		gofMap, ok := summaryMap["goodness_of_fit"].(map[string]interface{})
		require.True(t, ok)
		require.NotNil(t, gofMap["ofv"])
		assert.InDelta(t, -2456.789, gofMap["ofv"].(float64), 0.001)

		// Verify the audit entry can be serialized back to JSON properly
		jsonData, err := json.MarshalIndent(entry, "", "  ")
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), "model_summary")
		assert.Contains(t, string(jsonData), "THETA1")
		assert.Contains(t, string(jsonData), "OMEGA(1,1)")

		t.Logf("Audit trail verified - summary properly embedded")
		t.Logf("JSON size: %d bytes", len(jsonData))
	})

	t.Run("TestSummaryRetrievalFromAudit", func(t *testing.T) {
		// Demonstrate how to extract model summaries from audit logs
		auditEntries, err := runlog.ReadRunEntries(auditLogPath)
		require.NoError(t, err)

		var modelSummaries []map[string]interface{}
		for _, entry := range auditEntries {
			if entry.ModelSummary != nil {
				if summaryMap, ok := entry.ModelSummary.(map[string]interface{}); ok {
					modelSummaries = append(modelSummaries, summaryMap)
				}
			}
		}

		require.Len(t, modelSummaries, 1)
		summaryMap := modelSummaries[0]

		// Verify we can work with the summary extracted from audit log
		assert.NotEmpty(t, summaryMap["run_id"])

		estimationMap, ok := summaryMap["estimation"].(map[string]interface{})
		require.True(t, ok)
		assert.True(t, estimationMap["minimized"].(bool))

		parametersMap, ok := summaryMap["parameters"].(map[string]interface{})
		require.True(t, ok)
		thetasArray, ok := parametersMap["thetas"].([]interface{})
		require.True(t, ok)
		assert.Len(t, thetasArray, 3)

		t.Logf("Successfully extracted %d model summaries from audit log", len(modelSummaries))
	})
}

// createCompleteModelSetup creates a realistic NONMEM model with all associated files
func createCompleteModelSetup(t *testing.T, dir, baseName string) string {
	// Create control file
	controlContent := `
$PROBLEM Population pharmacokinetic model for Drug X
$DATA "pkdata.csv" IGNORE=@
$INPUT ID TIME DV AMT EVID CMT RATE WT AGE SEX
$SUBROUTINES ADVAN4 TRANS4

$THETA
(0, 0.982, 5)      ; CL/F (L/h)
(0, 15.6, 100)     ; V2/F (L)
(0, 2.45, 50)      ; Q/F (L/h)

$OMEGA BLOCK(2)
0.123              ; IIV CL
0.0156 0.089       ; IIV V2

$SIGMA
0.0234             ; Proportional error

$ESTIMATION METHOD=1 INTER MAXEVAL=9999 POSTHOC
$COVARIANCE PRINT=E
$TABLE ID TIME DV PRED IPRED IWRES CWRES ONEHEADER NOPRINT FILE=pkmodel001.tab
`

	controlPath := filepath.Join(dir, baseName+".mod")
	err := os.WriteFile(controlPath, []byte(controlContent), 0644)
	require.NoError(t, err)

	// Create output file
	outputContent := `
 NONMEM 7.5.0

 1NONLINEAR MIXED EFFECTS MODEL PROGRAM (NONMEM) VERSION 7.5.0

 $PROBLEM Population pharmacokinetic model for Drug X

 ESTIMATION METHOD: FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION

 TOTAL NO. OF INDIVIDUALS: 72
 TOTAL NO. OF OBSERVATIONS: 360
 SIGNIFICANT DIGITS IN FINAL RESULTS: 4

 CONDITION NUMBER = 67.8

 COVARIANCE STEP COMPLETED SUCCESSFULLY

 MINIMIZATION SUCCESSFUL

 #OBJV:*******************  -2456.789

 AIC = 4925.578
 BIC = 4959.123

 TOTAL NO. OF ELAPSED SECONDS: 125.75
`

	outputPath := filepath.Join(dir, baseName+".lst")
	err = os.WriteFile(outputPath, []byte(outputContent), 0644)
	require.NoError(t, err)

	// Create ext file with final parameter estimates
	extContent := `
; NONMEM 7.5.0
; Final parameter estimates
 ITERATION    THETA1    THETA2    THETA3    OMEGA(1,1)    OMEGA(1,2)    OMEGA(2,2)    SIGMA(1,1)
     -1000    0.982     15.6      2.45      0.123         0.0156        0.089         0.0234
`

	extPath := filepath.Join(dir, baseName+".ext")
	err = os.WriteFile(extPath, []byte(extContent), 0644)
	require.NoError(t, err)

	// Create a minimal data file reference (just for file discovery)
	dataContent := `ID,TIME,DV,AMT,EVID,CMT,RATE,WT,AGE,SEX
1,0,0,100,1,1,0,70,35,1
1,0.5,2.3,0,0,2,0,70,35,1
1,1.0,4.1,0,0,2,0,70,35,1
`

	dataPath := filepath.Join(dir, "pkdata.csv")
	err = os.WriteFile(dataPath, []byte(dataContent), 0644)
	require.NoError(t, err)

	return controlPath
}
