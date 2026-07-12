//go:build unit
// +build unit

package inheritance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/model"
)

func TestCorrelationStrategy_String(t *testing.T) {
	tests := []struct {
		strategy CorrelationStrategy
		expected string
	}{
		{StrategyConservative, "conservative"},
		{StrategyInferred, "inferred"},
		{StrategyPositional, "positional"},
		{CorrelationStrategy(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.strategy.String())
	}
}

func TestParseCorrelationStrategy(t *testing.T) {
	tests := []struct {
		input    string
		expected CorrelationStrategy
	}{
		{"conservative", StrategyConservative},
		{"Conservative", StrategyConservative},
		{"CONSERVATIVE", StrategyConservative},
		{"inferred", StrategyInferred},
		{"positional", StrategyPositional},
		{"unknown", StrategyConservative}, // Default
		{"", StrategyConservative},        // Default
		{"  conservative  ", StrategyConservative},
	}

	for _, tt := range tests {
		result := ParseCorrelationStrategy(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}

func TestMatcher_MatchThetas_Conservative_ExactMatch(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CL"},
		{Name: "THETA2", Label: "V"},
		{Name: "THETA3", Label: "KA"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
		{Index: 2, Label: "KA", Estimate: 0.8},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V", 3: "KA"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	require.Len(t, mappings, 3)

	// Check all matched by label
	for _, m := range mappings {
		assert.Equal(t, "label", m.MatchedBy)
	}

	// Check correct pairings
	assert.Equal(t, 0, mappings[0].SourceIndex)
	assert.Equal(t, 0, mappings[0].TargetIndex)
	assert.Equal(t, 1, mappings[1].SourceIndex)
	assert.Equal(t, 1, mappings[1].TargetIndex)
	assert.Equal(t, 2, mappings[2].SourceIndex)
	assert.Equal(t, 2, mappings[2].TargetIndex)
}

func TestMatcher_MatchThetas_Conservative_NoMatch(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CL"},
		{Name: "THETA2", Label: "V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "KA", Estimate: 0.8},
		{Index: 1, Label: "F1", Estimate: 0.9},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "KA", 2: "F1"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	// No matches should be found
	assert.Len(t, mappings, 0)
}

func TestMatcher_MatchThetas_Conservative_PartialMatch(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CL"},
		{Name: "THETA2", Label: "V"},
		{Name: "THETA3", Label: "KA"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "Q", Estimate: 2.0}, // Different
		{Index: 2, Label: "KA", Estimate: 0.8},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "Q", 3: "KA"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	// Only CL and KA should match
	require.Len(t, mappings, 2)

	// CL
	assert.Equal(t, 0, mappings[0].SourceIndex)
	assert.Equal(t, 0, mappings[0].TargetIndex)
	assert.Equal(t, "CL", mappings[0].SourceLabel)

	// KA
	assert.Equal(t, 2, mappings[1].SourceIndex)
	assert.Equal(t, 2, mappings[1].TargetIndex)
	assert.Equal(t, "KA", mappings[1].SourceLabel)
}

func TestMatcher_MatchThetas_Conservative_NoLabels(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: ""}, // No label
		{Name: "THETA2", Label: "V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	// Only V should match (THETA1 has no label)
	require.Len(t, mappings, 1)
	assert.Equal(t, 1, mappings[0].SourceIndex)
	assert.Equal(t, "V", mappings[0].SourceLabel)
}

func TestMatcher_MatchThetas_Positional(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CLEARANCE"},
		{Name: "THETA2", Label: "VOLUME"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	matcher := NewMatcher(StrategyPositional)
	mappings := matcher.MatchThetas(source, target, nil)

	require.Len(t, mappings, 2)

	// Both should match by position
	assert.Equal(t, "position", mappings[0].MatchedBy)
	assert.Equal(t, "position", mappings[1].MatchedBy)

	// Indices should match directly
	assert.Equal(t, 0, mappings[0].SourceIndex)
	assert.Equal(t, 0, mappings[0].TargetIndex)
	assert.Equal(t, 1, mappings[1].SourceIndex)
	assert.Equal(t, 1, mappings[1].TargetIndex)
}

func TestMatcher_MatchThetas_Positional_DifferentLengths(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CL"},
		{Name: "THETA2", Label: "V"},
		{Name: "THETA3", Label: "KA"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	matcher := NewMatcher(StrategyPositional)
	mappings := matcher.MatchThetas(source, target, nil)

	// Only 2 matches (limited by target length)
	require.Len(t, mappings, 2)
}

func TestMatcher_MatchThetas_Inferred_ExactFirst(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "CL"},
		{Name: "THETA2", Label: "V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	matcher := NewMatcher(StrategyInferred)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	require.Len(t, mappings, 2)

	// Should match by label first
	assert.Equal(t, "label", mappings[0].MatchedBy)
	assert.Equal(t, "label", mappings[1].MatchedBy)
}

func TestMatcher_MatchThetas_Inferred_FallbackToPosition(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: ""}, // No label
		{Name: "THETA2", Label: "V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	matcher := NewMatcher(StrategyInferred)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	require.Len(t, mappings, 2)

	// Check that we have both a label match and a position match
	hasLabelMatch := false
	hasPositionMatch := false
	for _, m := range mappings {
		if m.MatchedBy == "label" {
			hasLabelMatch = true
		}
		if m.MatchedBy == "position" {
			hasPositionMatch = true
		}
	}
	assert.True(t, hasLabelMatch, "Should have at least one label match")
	assert.True(t, hasPositionMatch, "Should have at least one position match")
}

func TestMatcher_MatchOmegas_Conservative(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "OMEGA(1,1)", Label: "IIV_CL"},
		{Name: "OMEGA(2,2)", Label: "IIV_V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "IIV_CL", Estimate: 0.1},
		{Index: 1, Label: "IIV_V", Estimate: 0.2},
	}

	targetLabels := &model.ParameterLabels{
		Omegas: map[string]string{"1,1": "IIV_CL", "2,2": "IIV_V"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchOmegas(source, target, targetLabels)

	require.Len(t, mappings, 2)
	assert.Equal(t, "label", mappings[0].MatchedBy)
}

func TestMatcher_MatchSigmas_Positional(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "SIGMA(1,1)", Label: "PROP"},
		{Name: "SIGMA(2,2)", Label: "ADD"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "PROPORTIONAL", Estimate: 0.05},
		{Index: 1, Label: "ADDITIVE", Estimate: 0.01},
	}

	matcher := NewMatcher(StrategyPositional)
	mappings := matcher.MatchSigmas(source, target, nil)

	require.Len(t, mappings, 2)
	assert.Equal(t, "position", mappings[0].MatchedBy)
	assert.Equal(t, "position", mappings[1].MatchedBy)
}

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CL", "CL"},
		{"cl", "CL"},
		{"  CL  ", "CL"},
		{"TVCL", "CL"},
		{"TV_CL", "_CL"}, // "TV" prefix is removed
		{"LOG_CL", "CL"},
		{"LN_CL", "CL"},
		{"ETA_CL", "CL"},
		{"EPS_ADD", "ADD"},
		{"", ""},
	}

	for _, tt := range tests {
		result := normalizeLabel(tt.input)
		assert.Equal(t, tt.expected, result, "input: %q", tt.input)
	}
}

func TestFuzzyLabelScore(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		minScore float64
	}{
		{"CL", "CL", 1.0},
		{"CLEARANCE", "CLEARANCE", 1.0},
		{"CL", "CLEARANCE", 0.2}, // Low but not zero
		{"", "CL", 0.0},
		{"CL", "", 0.0},
	}

	for _, tt := range tests {
		score := fuzzyLabelScore(tt.a, tt.b)
		assert.GreaterOrEqual(t, score, tt.minScore, "a=%q b=%q", tt.a, tt.b)
	}
}

func TestParseMatrixIndex(t *testing.T) {
	tests := []struct {
		name        string
		expectedRow int
		expectedCol int
	}{
		{"OMEGA(1,1)", 0, 0},
		{"OMEGA(2,2)", 1, 1},
		{"OMEGA(1,2)", 0, 1},
		{"OMEGA(3,1)", 2, 0},
		{"SIGMA(1,1)", 0, 0},
		{"THETA1", 0, 0}, // No indices
	}

	for _, tt := range tests {
		row, col := parseMatrixIndex(tt.name)
		assert.Equal(t, tt.expectedRow, row, "name: %s row", tt.name)
		assert.Equal(t, tt.expectedCol, col, "name: %s col", tt.name)
	}
}

func TestBuildTargetLabelMap(t *testing.T) {
	target := []TargetParameter{
		{Index: 0, Label: "CL"},
		{Index: 1, Label: "V"},
		{Index: 2, Label: ""}, // No label
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "VOLUME", 3: "KA"}, // Overrides V with VOLUME
	}

	result := buildTargetLabelMap(target, targetLabels, "THETA")

	assert.Equal(t, "CL", result[0])
	assert.Equal(t, "VOLUME", result[1]) // Overridden
	assert.Equal(t, "KA", result[2])     // From targetLabels
}

func TestBuildTargetLabelMap_NilTargetLabels(t *testing.T) {
	target := []TargetParameter{
		{Index: 0, Label: "CL"},
		{Index: 1, Label: "V"},
	}

	result := buildTargetLabelMap(target, nil, "THETA")

	assert.Equal(t, "CL", result[0])
	assert.Equal(t, "V", result[1])
}

func TestMatcher_MatchThetas_CaseInsensitive(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "cl"},
		{Name: "THETA2", Label: "V"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5},
		{Index: 1, Label: "v", Estimate: 1.0},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "v"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	// Should match case-insensitively
	require.Len(t, mappings, 2)
}

func TestMatcher_MatchThetas_PrefixNormalization(t *testing.T) {
	source := []model.ParameterEstimate{
		{Name: "THETA1", Label: "TVCL"}, // TV prefix
		{Name: "THETA2", Label: "TVV"},
	}

	target := []TargetParameter{
		{Index: 0, Label: "CL", Estimate: 0.5}, // No TV prefix
		{Index: 1, Label: "V", Estimate: 1.0},
	}

	targetLabels := &model.ParameterLabels{
		Thetas: map[int]string{1: "CL", 2: "V"},
	}

	matcher := NewMatcher(StrategyConservative)
	mappings := matcher.MatchThetas(source, target, targetLabels)

	// Should match after normalizing TV prefix
	require.Len(t, mappings, 2)
	assert.Equal(t, "TVCL", mappings[0].SourceLabel)
	assert.Equal(t, "CL", mappings[0].TargetLabel)
}
