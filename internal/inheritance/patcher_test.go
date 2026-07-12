//go:build unit
// +build unit

package inheritance

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/model"
)

func TestPatchControlStream_BasicTheta(t *testing.T) {
	targetContent := `$PROBLEM Test Model
$DATA data.csv
$THETA
0.5 ; CL
1.0 ; V
$OMEGA
0.1 ; IIV_CL
$SIGMA
0.05
$EST METHOD=1
`
	clEst := 1.5
	vEst := 25.0
	sourceSummary := &model.ModelSummary{
		RunID: "run001",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CL", Estimate: &clEst},
				{Name: "THETA2", Label: "V", Estimate: &vEst},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	opts := DefaultPatchOptions()
	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Check that estimates were patched
	assert.Contains(t, result.ModifiedContent, "1.5")
	assert.Contains(t, result.ModifiedContent, "25")

	// Check provenance comment was added
	assert.Contains(t, result.ModifiedContent, "Parameters inherited from: run001")

	// Check mappings
	assert.Len(t, result.Mappings, 2)
}

func TestPatchControlStream_BoundedTheta(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
(0, 0.5, 10) ; CL
$EST METHOD=1
`
	clEst := 2.5
	sourceSummary := &model.ModelSummary{
		RunID: "run002",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CL", Estimate: &clEst},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL"},
	}

	opts := DefaultPatchOptions()
	opts.PreserveBounds = true

	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)

	// Should preserve bounds but update estimate
	assert.Contains(t, result.ModifiedContent, "(0, 2.5, 10)")
}

func TestPatchControlStream_FixedTheta(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
0.5 FIX ; CL
1.0 ; V
$EST METHOD=1
`
	clEst := 2.0
	vEst := 20.0
	sourceSummary := &model.ModelSummary{
		RunID: "run003",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CL", Estimate: &clEst},
				{Name: "THETA2", Label: "V", Estimate: &vEst},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	opts := DefaultPatchOptions()
	opts.InheritFixed = false // Don't inherit FIX status

	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)

	// FIX should be preserved
	assert.Contains(t, result.ModifiedContent, "FIX")
	assert.Contains(t, result.ModifiedContent, "2 FIX")
}

func TestPatchControlStream_OmegaPatching(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
0.5 ; CL
$OMEGA
0.1 ; IIV_CL
0.2 ; IIV_V
$EST METHOD=1
`
	clEst := 1.5
	iivCL := 0.25
	iivV := 0.36
	sourceSummary := &model.ModelSummary{
		RunID: "run004",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CL", Estimate: &clEst},
			},
			Omegas: []model.ParameterEstimate{
				{Name: "OMEGA(1,1)", Label: "IIV_CL", Estimate: &iivCL},
				{Name: "OMEGA(2,2)", Label: "IIV_V", Estimate: &iivV},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL"},
		Omegas: map[string]string{"1,1": "IIV_CL", "2,2": "IIV_V"},
	}

	opts := DefaultPatchOptions()
	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)
	assert.Contains(t, result.ModifiedContent, "0.25")
	assert.Contains(t, result.ModifiedContent, "0.36")
}

func TestPatchControlStream_SigmaPatching(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
0.5
$SIGMA
0.1 ; PROP
$EST METHOD=1
`
	propErr := 0.15
	sourceSummary := &model.ModelSummary{
		RunID: "run005",
		Parameters: model.ParameterSummary{
			Sigmas: []model.ParameterEstimate{
				{Name: "SIGMA(1,1)", Label: "PROP", Estimate: &propErr},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Sigmas: map[string]string{"1,1": "PROP"},
	}

	opts := DefaultPatchOptions()
	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)
	assert.Contains(t, result.ModifiedContent, "0.15")
}

func TestPatchControlStream_NoMatchingLabels_Conservative(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
0.5 ; CL
1.0 ; V
$EST METHOD=1
`
	kaEst := 0.8
	sourceSummary := &model.ModelSummary{
		RunID: "run006",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "KA", Estimate: &kaEst}, // No match in target
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	opts := DefaultPatchOptions()
	opts.Strategy = StrategyConservative

	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)

	// Original values should be preserved
	assert.Contains(t, result.ModifiedContent, "0.5")
	assert.Contains(t, result.ModifiedContent, "1.0")

	// No mappings should be created
	assert.Len(t, result.Mappings, 0)

	// Source param should be unmatched
	assert.Contains(t, result.UnmatchedSource, "THETA1")
}

func TestPatchControlStream_PositionalStrategy(t *testing.T) {
	targetContent := `$PROBLEM Test
$THETA
0.5 ; CL
1.0 ; V
$EST METHOD=1
`
	// Source has different labels but same positions
	param1 := 2.0
	param2 := 30.0
	sourceSummary := &model.ModelSummary{
		RunID: "run007",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CLEARANCE", Estimate: &param1},
				{Name: "THETA2", Label: "VOLUME", Estimate: &param2},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	opts := DefaultPatchOptions()
	opts.Strategy = StrategyPositional

	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, opts)

	require.NoError(t, err)

	// Should match by position
	assert.Contains(t, result.ModifiedContent, "2")
	assert.Contains(t, result.ModifiedContent, "30")
	assert.Len(t, result.Mappings, 2)
	assert.Equal(t, "position", result.Mappings[0].MatchedBy)
}

func TestPatchControlStream_NilSourceSummary(t *testing.T) {
	_, err := PatchControlStream("$PROBLEM Test", nil, nil, DefaultPatchOptions())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source summary is required")
}

func TestPatchControlStream_PreservesComments(t *testing.T) {
	targetContent := `$PROBLEM Test Model
; This is a comment
$THETA
; Parameter comments
0.5 ; CL - Clearance
; Another comment
1.0 ; V - Volume
$EST METHOD=1
`
	clEst := 1.5
	vEst := 25.0
	sourceSummary := &model.ModelSummary{
		RunID: "run008",
		Parameters: model.ParameterSummary{
			Thetas: []model.ParameterEstimate{
				{Name: "THETA1", Label: "CL", Estimate: &clEst},
				{Name: "THETA2", Label: "V", Estimate: &vEst},
			},
		},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	result, err := PatchControlStream(targetContent, sourceSummary, targetLabels, DefaultPatchOptions())

	require.NoError(t, err)

	// All comments should be preserved
	assert.Contains(t, result.ModifiedContent, "; This is a comment")
	assert.Contains(t, result.ModifiedContent, "; Parameter comments")
	assert.Contains(t, result.ModifiedContent, "; CL - Clearance")
	assert.Contains(t, result.ModifiedContent, "; Another comment")
	assert.Contains(t, result.ModifiedContent, "; V - Volume")
}

func TestPatchThetaLine_SimpleValue(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		newEstimate float64
		expected    string
	}{
		{
			name:        "Simple value",
			line:        "0.5",
			newEstimate: 1.5,
			expected:    "1.5",
		},
		{
			name:        "With leading whitespace",
			line:        "  0.5",
			newEstimate: 1.5,
			expected:    "  1.5",
		},
		{
			name:        "With comment",
			line:        "0.5 ; CL",
			newEstimate: 1.5,
			expected:    "1.5 ; CL",
		},
		{
			name:        "With FIX",
			line:        "0.5 FIX",
			newEstimate: 1.5,
			expected:    "1.5 FIX",
		},
		{
			name:        "With FIX and comment",
			line:        "0.5 FIX ; CL",
			newEstimate: 1.5,
			expected:    "1.5 FIX ; CL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultPatchOptions()
			result := patchThetaLine(tt.line, tt.newEstimate, opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatchThetaLine_BoundedValue(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		newEstimate    float64
		preserveBounds bool
		expected       string
	}{
		{
			name:           "Bounded with preserve",
			line:           "(0, 0.5, 10)",
			newEstimate:    2.5,
			preserveBounds: true,
			expected:       "(0, 2.5, 10)",
		},
		{
			name:           "Bounded without preserve",
			line:           "(0, 0.5, 10)",
			newEstimate:    2.5,
			preserveBounds: false,
			expected:       "2.5",
		},
		{
			name:           "Bounded with comment",
			line:           "(0, 0.5, 10) ; CL",
			newEstimate:    2.5,
			preserveBounds: true,
			expected:       "(0, 2.5, 10) ; CL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultPatchOptions()
			opts.PreserveBounds = tt.preserveBounds
			result := patchThetaLine(tt.line, tt.newEstimate, opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTargetThetas(t *testing.T) {
	content := `$PROBLEM Test
$THETA
0.5 ; CL
(0, 1.0, 10) ; V
2.0 FIX ; KA
$OMEGA
0.1
`
	thetas := parseTargetThetas(content)

	require.Len(t, thetas, 3)
	assert.Equal(t, "CL", thetas[0].Label)
	assert.Equal(t, 0.5, thetas[0].Estimate)
	assert.Equal(t, "V", thetas[1].Label)
	assert.Equal(t, 1.0, thetas[1].Estimate)
	assert.Equal(t, "KA", thetas[2].Label)
	assert.Equal(t, 2.0, thetas[2].Estimate)
}

func TestParseTargetOmegas(t *testing.T) {
	content := `$PROBLEM Test
$THETA
0.5
$OMEGA
0.1 ; IIV_CL
0.2 ; IIV_V
$SIGMA
0.05
`
	omegas := parseTargetOmegas(content)

	require.Len(t, omegas, 2)
	assert.Equal(t, "IIV_CL", omegas[0].Label)
	assert.Equal(t, 0.1, omegas[0].Estimate)
	assert.Equal(t, "IIV_V", omegas[1].Label)
	assert.Equal(t, 0.2, omegas[1].Estimate)
}

func TestParseTargetSigmas(t *testing.T) {
	content := `$PROBLEM Test
$OMEGA
0.1
$SIGMA
0.05 ; PROP
0.01 ; ADD
$EST METHOD=1
`
	sigmas := parseTargetSigmas(content)

	require.Len(t, sigmas, 2)
	assert.Equal(t, "PROP", sigmas[0].Label)
	assert.Equal(t, 0.05, sigmas[0].Estimate)
	assert.Equal(t, "ADD", sigmas[1].Label)
	assert.Equal(t, 0.01, sigmas[1].Estimate)
}

func TestIsCommentOrEmpty(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"", true},
		{"  ", true},
		{"; comment", true},
		{"  ; comment", true},
		{"0.5", false},
		{"  0.5 ; CL", false},
	}

	for _, tt := range tests {
		result := isCommentOrEmpty(tt.line)
		assert.Equal(t, tt.expected, result, "line: %q", tt.line)
	}
}

func TestAddProvenanceComment(t *testing.T) {
	content := `$PROBLEM Original Problem
$DATA data.csv
$THETA
0.5
`
	source := &model.ModelSummary{RunID: "run123"}

	result := addProvenanceComment(content, source)

	// Check provenance is after $PROBLEM
	lines := strings.Split(result, "\n")
	foundProblem := false
	foundProvenance := false
	for i, line := range lines {
		if strings.Contains(line, "$PROBLEM") {
			foundProblem = true
		} else if strings.Contains(line, "inherited from: run123") {
			foundProvenance = true
			// Provenance should be right after $PROBLEM
			assert.True(t, foundProblem, "Provenance should be after $PROBLEM")
			assert.Greater(t, i, 0, "Provenance should not be first line")
		}
	}

	assert.True(t, foundProblem, "Should find $PROBLEM")
	assert.True(t, foundProvenance, "Should find provenance comment")
}

func TestDefaultPatchOptions(t *testing.T) {
	opts := DefaultPatchOptions()

	assert.Equal(t, StrategyConservative, opts.Strategy)
	assert.False(t, opts.InheritFixed)
	assert.True(t, opts.PreserveBounds)
	assert.True(t, opts.WarnOutOfBounds)
}
