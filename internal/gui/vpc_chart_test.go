//go:build gui
// +build gui

package gui

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildVPCChart renders the chart from the real fixture and confirms the
// selected percentile series and the bin x-coordinates made it into the output.
func TestBuildVPCChart(t *testing.T) {
	data, err := os.ReadFile("testdata/vpc_results.csv")
	require.NoError(t, err)

	vpc, err := parseVPCResults(data)
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, buildVPCChart(vpc).Render(&buf))
	html := buf.String()

	// The 5/50/95 triple is fully present in the fixture, so it's the selection.
	for _, s := range []string{
		"5% observed", "50% observed", "95% observed",
		"5% sim CI low", "50% sim CI high",
	} {
		assert.Contains(t, html, s, "missing series %q", s)
	}

	// Bin x-coordinates (median.idv) and axis names are rendered.
	assert.Contains(t, html, "1.625")
	assert.Contains(t, html, "TIME")
	assert.Contains(t, html, "DV")
}

func TestSelectVPCPercentiles(t *testing.T) {
	mk := func(labels ...string) []*vpcPercentile {
		ps := make([]*vpcPercentile, len(labels))
		for i, l := range labels {
			ps[i] = &vpcPercentile{Label: l, Pct: vpcPctValue(l)}
		}

		return ps
	}

	// Preferred 5/50/95 wins when present.
	got := selectVPCPercentiles(mk("2.5", "5", "10", "50", "90", "95", "97.5"))
	require.Len(t, got, 3)
	assert.Equal(t, []string{"5", "50", "95"}, []string{got[0].Label, got[1].Label, got[2].Label})

	// Falls back to lowest/median/highest numeric when no preferred set is whole.
	got = selectVPCPercentiles(mk("20", "40", "60", "80", "mean"))
	require.Len(t, got, 3)
	assert.Equal(t, "20", got[0].Label)
	assert.Equal(t, "80", got[2].Label)
	for _, p := range got {
		assert.NotEqual(t, "mean", p.Label, "mean must not be selected")
	}
}
