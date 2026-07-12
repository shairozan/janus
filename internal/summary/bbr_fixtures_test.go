//go:build unit
// +build unit

package summary

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test using realistic BBR fixtures from metrumresearchgroup/bbr repository
func TestWithBBRFixtures(t *testing.T) {
	// Use committed testdata files from BBR
	testdataDir := "testdata/bbr"
	modelPath := filepath.Join(testdataDir, "1.ctl")
	outputPath := filepath.Join(testdataDir, "1.lst")
	extPath := filepath.Join(testdataDir, "1.ext")

	// Test NONMEM parser with realistic BBR data
	t.Run("ParseBBREstimationSummary", func(t *testing.T) {
		summarizer := NewNONMEMSummarizer()
		var estimation EstimationSummary

		err := summarizer.parseEstimationSummary(outputPath, &estimation)
		require.NoError(t, err)

		// Verify realistic BBR values
		assert.Equal(t, "FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION", estimation.Method)
		assert.Equal(t, 39, estimation.Subjects)
		assert.Equal(t, 741, estimation.Observations)
		assert.Equal(t, 3, estimation.SignificantDigits)
		assert.Equal(t, "MINIMIZATION SUCCESSFUL", estimation.TerminationReason)
		assert.True(t, estimation.Minimized)
	})

	t.Run("ParseBBRGoodnessOfFit", func(t *testing.T) {
		summarizer := NewNONMEMSummarizer()
		var gof GoodnessOfFitSummary

		err := summarizer.parseGoodnessOfFit(outputPath, &gof)
		require.NoError(t, err)

		// Verify realistic BBR goodness of fit values
		require.NotNil(t, gof.ObjectiveFunctionValue)
		assert.InDelta(t, 2583.311, *gof.ObjectiveFunctionValue, 0.001)

		// Calculate expected log-likelihood
		require.NotNil(t, gof.LogLikelihood)
		assert.InDelta(t, -1291.6555, *gof.LogLikelihood, 0.001)
	})

	t.Run("ParseBBRParameterSummary", func(t *testing.T) {
		summarizer := NewNONMEMSummarizer()
		var parameters ParameterSummary

		err := summarizer.parseParameterSummary(extPath, &parameters)
		require.NoError(t, err)

		// Verify BBR parameter structure
		assert.Equal(t, 7, parameters.TotalParameters) // 5 THETAs + 2 OMEGAs
		assert.Equal(t, 7, parameters.EstimatedParameters)
		assert.Equal(t, 0, parameters.FixedParameters)

		// Check THETA parameters (BBR model has 5 THETAs)
		require.Len(t, parameters.Thetas, 5)

		// KA (THETA1)
		assert.Equal(t, "THETA1", parameters.Thetas[0].Name)
		require.NotNil(t, parameters.Thetas[0].Estimate)
		assert.InDelta(t, 2.31716, *parameters.Thetas[0].Estimate, 0.001)

		// CL (THETA2)
		assert.Equal(t, "THETA2", parameters.Thetas[1].Name)
		require.NotNil(t, parameters.Thetas[1].Estimate)
		assert.InDelta(t, 54.6151, *parameters.Thetas[1].Estimate, 0.001)

		// V (THETA3)
		assert.Equal(t, "THETA3", parameters.Thetas[2].Name)
		require.NotNil(t, parameters.Thetas[2].Estimate)
		assert.InDelta(t, 462.514, *parameters.Thetas[2].Estimate, 0.001)

		// Check OMEGA parameters (BBR model has 2 OMEGAs)
		require.Len(t, parameters.Omegas, 2)

		// IIV CL (OMEGA(1,1))
		assert.Equal(t, "OMEGA(1,1)", parameters.Omegas[0].Name)
		require.NotNil(t, parameters.Omegas[0].Estimate)
		assert.InDelta(t, 0.0985328, *parameters.Omegas[0].Estimate, 0.0001)

		// IIV V (OMEGA(2,2))
		assert.Equal(t, "OMEGA(2,2)", parameters.Omegas[1].Name)
		require.NotNil(t, parameters.Omegas[1].Estimate)
		assert.InDelta(t, 0.156825, *parameters.Omegas[1].Estimate, 0.0001)
	})

	t.Run("CompleteBBRWorkflow", func(t *testing.T) {
		// Test complete summarization workflow with BBR data
		summarizer := NewNONMEMSummarizer()
		options := DefaultSummaryOptions()

		// Use explicit file paths
		files := ModelFiles{
			ControlFile: modelPath,
			OutputFile:  outputPath,
			ExtFile:     extPath,
			DataFiles:   []string{"../../../../extdata/acop.csv"},
		}

		ctx := context.Background()
		summary, err := summarizer.SummarizeModelWithFiles(ctx, files, options)
		require.NoError(t, err)
		require.NotNil(t, summary)

		// Verify the complete BBR summary
		assert.Equal(t, modelPath, summary.ModelPath)
		assert.Equal(t, outputPath, summary.OutputPath)
		assert.NotEmpty(t, summary.RunID)
		assert.Equal(t, "janus-nonmem-summarizer", summary.ProcessedBy)

		// Verify BBR estimation details
		assert.Equal(t, "FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION", summary.Estimation.Method)
		assert.Equal(t, 39, summary.Estimation.Subjects)
		assert.Equal(t, 741, summary.Estimation.Observations)
		assert.True(t, summary.Estimation.Minimized)

		// Verify BBR parameter summary
		assert.Equal(t, 7, summary.Parameters.TotalParameters)
		assert.Len(t, summary.Parameters.Thetas, 5)
		assert.Len(t, summary.Parameters.Omegas, 2)
		assert.Len(t, summary.Parameters.Sigmas, 0) // SIGMA is fixed at 1, not included in .ext

		// Verify BBR goodness of fit
		require.NotNil(t, summary.GoodnessOfFit.ObjectiveFunctionValue)
		assert.InDelta(t, 2583.311, *summary.GoodnessOfFit.ObjectiveFunctionValue, 0.001)

		t.Logf("BBR Summary Results:")
		t.Logf("  Model: %s", summary.ModelPath)
		t.Logf("  Method: %s", summary.Estimation.Method)
		t.Logf("  Subjects: %d", summary.Estimation.Subjects)
		t.Logf("  Observations: %d", summary.Estimation.Observations)
		t.Logf("  OFV: %.3f", *summary.GoodnessOfFit.ObjectiveFunctionValue)
		t.Logf("  Total Parameters: %d", summary.Parameters.TotalParameters)
		t.Logf("  KA (THETA1): %.3f", *summary.Parameters.Thetas[0].Estimate)
		t.Logf("  CL (THETA2): %.3f", *summary.Parameters.Thetas[1].Estimate)
		t.Logf("  V (THETA3): %.3f", *summary.Parameters.Thetas[2].Estimate)
	})
}
