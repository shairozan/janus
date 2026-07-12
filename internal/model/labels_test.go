package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseParameterLabels_BasicThetas(t *testing.T) {
	content := `$PROBLEM Test Model

$THETA
(0, 2)  ; KA
(0, 3)  ; CL
(0, 10) ; V2
(0.02)  ; RUVp
(1)     ; RUVa

$OMEGA
0.05    ; iiv CL
0.2     ; iiv V2

$SIGMA
1 FIX
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Len(t, labels.Thetas, 5)
	assert.Equal(t, "KA", labels.Thetas[1])
	assert.Equal(t, "CL", labels.Thetas[2])
	assert.Equal(t, "V2", labels.Thetas[3])
	assert.Equal(t, "RUVp", labels.Thetas[4])
	assert.Equal(t, "RUVa", labels.Thetas[5])
}

func TestParseParameterLabels_Omegas(t *testing.T) {
	content := `$PROBLEM Test

$THETA
(0, 2)  ; KA

$OMEGA
0.05    ; iiv CL
0.2     ; iiv V2
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Len(t, labels.Omegas, 2)
	assert.Equal(t, "iiv CL", labels.Omegas["1,1"])
	assert.Equal(t, "iiv V2", labels.Omegas["2,2"])
}

func TestParseParameterLabels_Sigmas(t *testing.T) {
	content := `$PROBLEM Test

$SIGMA
0.1     ; prop
0.05    ; add
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Len(t, labels.Sigmas, 2)
	assert.Equal(t, "prop", labels.Sigmas["1,1"])
	assert.Equal(t, "add", labels.Sigmas["2,2"])
}

func TestParseParameterLabels_NoLabels(t *testing.T) {
	content := `$PROBLEM Test

$THETA
(0, 2)
(0, 3)
1 FIX

$OMEGA
0.05
0.2
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Empty(t, labels.Thetas)
	assert.Empty(t, labels.Omegas)
	assert.Empty(t, labels.Sigmas)
}

func TestParseParameterLabels_MixedLabels(t *testing.T) {
	content := `$PROBLEM Test

$THETA
(0, 2)  ; KA
(0, 3)
(0, 10) ; V2
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Len(t, labels.Thetas, 2)
	assert.Equal(t, "KA", labels.Thetas[1])
	assert.Equal(t, "V2", labels.Thetas[3])
	_, hasTheta2 := labels.Thetas[2]
	assert.False(t, hasTheta2, "Theta 2 should not have a label")
}

func TestParseParameterLabels_InlineThetas(t *testing.T) {
	// Case where $THETA has inline values
	content := `$PROBLEM Test
$THETA 1
$OMEGA 0.1
$SIGMA 0.1
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	// Inline values without a comment have no labels
	assert.Empty(t, labels.Thetas)
}

func TestParseParameterLabels_InlineThetaWithLabel(t *testing.T) {
	// A label on the $THETA definition line itself must be captured (issue #115).
	content := `$PROBLEM Test
$THETA (0,1) ; CL
$OMEGA 0.1
$SIGMA 0.1
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Equal(t, "CL", labels.Thetas[1])
}

func TestParseParameterLabels_InlineThetaMultipleWithLabel(t *testing.T) {
	// With multiple inline values, the trailing comment labels the last one.
	content := `$PROBLEM Test
$THETA (0,1) (0,2) ; V
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Empty(t, labels.Thetas[1])
	assert.Equal(t, "V", labels.Thetas[2])
}

func TestParseParameterLabels_NumberedComments(t *testing.T) {
	// Some models use numbered comments like ";1 prop"
	content := `$PROBLEM Test

$THETA
(0, 0.5) ;1 prop
(0 0.1)  ;2 add
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Equal(t, "1 prop", labels.Thetas[1])
	assert.Equal(t, "2 add", labels.Thetas[2])
}

func TestParseParameterLabels_WhitespaceVariations(t *testing.T) {
	content := `$PROBLEM Test

$THETA
(0, 2);KA
(0, 3)    ;    CL
(0, 10)	;	V2
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Equal(t, "KA", labels.Thetas[1])
	assert.Equal(t, "CL", labels.Thetas[2])
	assert.Equal(t, "V2", labels.Thetas[3])
}

func TestParseParameterLabels_RealControlStream(t *testing.T) {
	// Based on testdata/acop.mod
	content := `$PROBLEM PK model 1 cmt base

$INPUT ID TIME MDV EVID DV AMT  SEX WT ETN
$DATA acop.csv IGNORE=@
$SUBROUTINES ADVAN2 TRANS2

$PK
ET=1
IF(ETN.EQ.3) ET=1.3
KA = THETA(1)
CL = THETA(2)*((WT/70)**0.75)* EXP(ETA(1))
V = THETA(3)*EXP(ETA(2))
SC=V


$THETA
(0, 2)  ; KA
(0, 3)  ; CL
(0, 10) ; V2
(0.02)  ; RUVp
(1)     ; RUVa

$OMEGA
0.05    ; iiv CL
0.2     ; iiv V2

$SIGMA
1 FIX

$ERROR
IPRED = F
IRES = DV-IPRED
W = IPRED*THETA(4) + THETA(5)
IF (W.EQ.0) W = 1
IWRES = IRES/W
Y= IPRED+W*ERR(1)

$EST METHOD=1 INTERACTION MAXEVAL=9999 SIG=3 PRINT=5 NOABORT POSTHOC
$COV
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)

	// Check thetas
	assert.Equal(t, "KA", labels.Thetas[1])
	assert.Equal(t, "CL", labels.Thetas[2])
	assert.Equal(t, "V2", labels.Thetas[3])
	assert.Equal(t, "RUVp", labels.Thetas[4])
	assert.Equal(t, "RUVa", labels.Thetas[5])

	// Check omegas
	assert.Equal(t, "iiv CL", labels.Omegas["1,1"])
	assert.Equal(t, "iiv V2", labels.Omegas["2,2"])

	// Sigma has FIX but no label
	assert.Empty(t, labels.Sigmas)
}

func TestExtractLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"basic", "(0, 2)  ; KA", "KA"},
		{"no_space", "(0, 2);KA", "KA"},
		{"extra_spaces", "(0, 2)    ;    CL", "CL"},
		{"multi_word", "0.05    ; iiv CL", "iiv CL"},
		{"no_comment", "1 FIX", ""},
		{"empty_comment", "(0, 2)  ;", ""},
		{"numbered", "(0, 0.5) ;1 prop", "1 prop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeForComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CL", "cl"},
		{"  CL  ", "cl"},
		{"iiv CL", "iiv cl"},
		{"", ""},
		{"Ka", "ka"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeForComparison(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetThetaLabel(t *testing.T) {
	labels := &ParameterLabels{
		Thetas: map[int]string{1: "KA", 2: "CL"},
	}

	assert.Equal(t, "KA", labels.GetThetaLabel(1))
	assert.Equal(t, "CL", labels.GetThetaLabel(2))
	assert.Equal(t, "", labels.GetThetaLabel(3))

	// Nil safety
	var nilLabels *ParameterLabels
	assert.Equal(t, "", nilLabels.GetThetaLabel(1))
}

func TestGetOmegaLabel(t *testing.T) {
	labels := &ParameterLabels{
		Omegas: map[string]string{"1,1": "iiv CL", "2,2": "iiv V"},
	}

	assert.Equal(t, "iiv CL", labels.GetOmegaLabel("1,1"))
	assert.Equal(t, "iiv V", labels.GetOmegaLabel("2,2"))
	assert.Equal(t, "", labels.GetOmegaLabel("3,3"))
}

func TestFormatMatrixPosition(t *testing.T) {
	assert.Equal(t, "1,1", formatMatrixPosition(1))
	assert.Equal(t, "2,2", formatMatrixPosition(2))
	assert.Equal(t, "10,10", formatMatrixPosition(10))
}

func TestParseParameterLabels_EmptyContent(t *testing.T) {
	labels := ParseParameterLabels("")

	require.NotNil(t, labels)
	assert.Empty(t, labels.Thetas)
	assert.Empty(t, labels.Omegas)
	assert.Empty(t, labels.Sigmas)
}

func TestParseParameterLabels_OnlyComments(t *testing.T) {
	content := `; This is a comment
; Another comment
$PROBLEM Test
; More comments
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	assert.Empty(t, labels.Thetas)
}

func TestParseParameterLabels_MultiValueOmega(t *testing.T) {
	// Test case where multiple OMEGA values are on a single line
	content := `$PROBLEM Test

$OMEGA 0.04 0.04
0.05 ; iiv V2
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	// First line has 2 values (no labels), second line has 1 value with label
	// So positions should be: 1,1 (no label), 2,2 (no label), 3,3 (iiv V2)
	assert.Len(t, labels.Omegas, 1)
	assert.Equal(t, "iiv V2", labels.Omegas["3,3"])
}

func TestParseParameterLabels_MultiValueSigma(t *testing.T) {
	content := `$PROBLEM Test

$SIGMA 0.1 0.05 ; combined
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	// 2 values on one line, label applies to last one (position 2,2)
	assert.Len(t, labels.Sigmas, 1)
	assert.Equal(t, "combined", labels.Sigmas["2,2"])
}

func TestParseParameterLabels_OmegaBlock(t *testing.T) {
	// BLOCK syntax should not count as values
	content := `$PROBLEM Test

$OMEGA BLOCK(2)
0.04       ; iiv CL
0.01 0.09  ; iiv V
`

	labels := ParseParameterLabels(content)

	require.NotNil(t, labels)
	// BLOCK(2) line has 0 values
	// Next line has 1 value -> position 1,1
	// Last line has 2 values -> positions 2,2 and 3,3, label on 3,3
	assert.Equal(t, "iiv CL", labels.Omegas["1,1"])
	assert.Equal(t, "iiv V", labels.Omegas["3,3"])
}

func TestNormalizeLabel_DoubleSemicolon(t *testing.T) {
	// Test that labels starting with semicolon are handled
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"double_semicolon", "; KA", "KA"},
		{"triple_semicolon", ";; KA", "KA"},
		{"semicolon_space", ";  Label", "Label"},
		{"normal", "KA", "KA"},
		{"trailing_comment", "KA ; extra", "KA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountMatrixValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"single_value", "0.05", 1},
		{"two_values", "0.04 0.04", 2},
		{"three_values", "0.1 0.2 0.3", 3},
		{"with_fix", "0.05 FIX", 1},
		{"with_comment", "0.05 ; label", 1},
		{"block_syntax", "BLOCK(2)", 0},
		{"same_syntax", "SAME", 0},
		{"empty", "", 0},
		{"negative", "-0.05", 1},
		{"parens", "(0.05)", 1},
		{"decimal_start", ".05", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countMatrixValues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
