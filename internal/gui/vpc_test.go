//go:build gui
// +build gui

package gui

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseVPCResults parses the real PsN 5.7.1 vpc_results.csv fixture (acop, 4
// auto bins, 100 samples) and checks the continuous block is decoded correctly.
func TestParseVPCResults(t *testing.T) {
	data, err := os.ReadFile("testdata/vpc_results.csv")
	require.NoError(t, err)

	vpc, err := parseVPCResults(data)
	require.NoError(t, err)

	// Variable names come from the run-info block.
	assert.Equal(t, "TIME", vpc.IDVName)
	assert.Equal(t, "DV", vpc.DVName)

	// Four bins, x-coordinates = median.idv.
	require.Len(t, vpc.Bins, 4)
	assert.InDelta(t, 1.625, vpc.Bins[0].MedianIDV, 1e-9)
	assert.InDelta(t, 8.5, vpc.Bins[1].MedianIDV, 1e-9)
	assert.InDelta(t, 19.0, vpc.Bins[3].MedianIDV, 1e-9)
	assert.InDelta(t, -2.14285714285714, vpc.Bins[0].Lower, 1e-9)
	assert.Equal(t, 560, vpc.Bins[0].NObs)

	// Percentiles present and ordered ascending, "mean" last.
	byLabel := map[string]*vpcPercentile{}
	for _, p := range vpc.Percentiles {
		byLabel[p.Label] = p
	}
	for _, want := range []string{"2.5", "5", "10", "30", "50", "70", "90", "95", "97.5", "mean"} {
		assert.Contains(t, byLabel, want, "missing percentile %s", want)
	}
	assert.Equal(t, "mean", vpc.Percentiles[len(vpc.Percentiles)-1].Label, "mean sorts last")
	assert.Equal(t, "2.5", vpc.Percentiles[0].Label, "lowest percentile sorts first")

	// Spot-check the 50% series against bin 1 (row 9): real=15.4245, sim CI
	// [512.615, 638.37].
	p50 := byLabel["50"]
	require.NotNil(t, p50)
	require.Len(t, p50.Real, 4)
	assert.InDelta(t, 15.4245, p50.Real[0], 1e-6)
	assert.InDelta(t, 512.615, p50.CIFrom[0], 1e-6)
	assert.InDelta(t, 638.37, p50.CITo[0], 1e-6)

	// And the 90% band on bin 1: from 836.80 to 1048.0 (8.3680E+02 / 1.0480E+03).
	p90 := byLabel["90"]
	require.NotNil(t, p90)
	assert.InDelta(t, 836.80, p90.CIFrom[0], 1e-1)
	assert.InDelta(t, 1048.0, p90.CITo[0], 1e-1)
}

func TestPSNToolFromCommand(t *testing.T) {
	cases := map[string]string{
		"vpc acop.mod -samples=200":         "vpc",
		"bootstrap acop.mod -samples=200":   "bootstrap",
		"scm acop.mod -config_file=x.scm":   "scm",
		"/usr/local/bin/vpc run1.mod":       "vpc",
		"nonmem acop.mod acop.lst":          "",
		"/opt/NONMEM/nm75/run/nmfe75 m.mod": "",
		"":                                  "",
	}

	for cmd, want := range cases {
		assert.Equal(t, want, psnToolFromCommand(cmd), "command %q", cmd)
	}
}

func TestParseVPCResultsErrors(t *testing.T) {
	_, err := parseVPCResults([]byte("not a vpc file\njust text\n"))
	require.Error(t, err)

	_, err = parseVPCResults(nil)
	require.Error(t, err)
}
