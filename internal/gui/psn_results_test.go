package gui

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPercentile(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5}
	assert.Equal(t, 3.0, percentile(v, 50))
	assert.Equal(t, 1.0, percentile(v, 0))
	assert.Equal(t, 5.0, percentile(v, 100))
	assert.InDelta(t, 1.2, percentile(v, 5), 1e-9)
	assert.True(t, math.IsNaN(percentile(nil, 50)))
}

func TestParseBootstrapCIs(t *testing.T) {
	// A label column (model) plus two numeric parameter columns over 5 samples.
	csv := "model,CL,V\n" +
		"run1,1.0,10\n" +
		"run2,2.0,20\n" +
		"run3,3.0,30\n" +
		"run4,4.0,40\n" +
		"run5,5.0,50\n"

	cis, err := parseBootstrapCIs([]byte(csv))
	require.NoError(t, err)
	require.Len(t, cis, 2) // model column skipped (non-numeric)

	assert.Equal(t, "CL", cis[0].Name)
	assert.Equal(t, 3.0, cis[0].Median)
	assert.Equal(t, "V", cis[1].Name)
	assert.Equal(t, 30.0, cis[1].Median)
}

func TestParseBootstrapCIsNoNumeric(t *testing.T) {
	_, err := parseBootstrapCIs([]byte("a,b\nx,y\n"))
	assert.Error(t, err)
}

func TestParseScmSummary(t *testing.T) {
	log := `Starting scm
Parameter-covariate relation chosen in this forward step: CL-WT-2
Some noise line
Step OFV dropped by 12.3
nothing here`

	out := parseScmSummary(log)
	require.Len(t, out, 2)
	assert.Contains(t, out[0], "chosen")
	assert.Contains(t, out[1], "OFV")
}
