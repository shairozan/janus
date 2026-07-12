package editor

import (
	"image/color"

	"fyne.io/fyne/v2/theme"
)

// colorForToken returns the appropriate color for a token type.
// Colors are theme-aware and adapt to light/dark mode.
func colorForToken(tokenType TokenType) color.Color {
	switch tokenType {
	case TokenDirective:
		// Directives ($PROBLEM, $DATA, etc.) - Bright and prominent
		return theme.Color(theme.ColorNamePrimary)

	case TokenSectionKeyword:
		// Section keywords ($PK, $ERROR, etc.) - Secondary prominence
		return theme.Color(theme.ColorNameFocus)

	case TokenDataKeyword:
		// Data column names (ID, TIME, DV, etc.) - Info color
		return color.NRGBA{R: 100, G: 150, B: 200, A: 255} // Light blue

	case TokenModelKeyword:
		// Model keywords (ADVAN1, FOCE, etc.) - Warning color
		return color.NRGBA{R: 200, G: 150, B: 50, A: 255} // Gold/amber

	case TokenParameterID:
		// Parameters (THETA, CL, V, etc.) - Success color
		return color.NRGBA{R: 100, G: 200, B: 100, A: 255} // Light green

	case TokenComment:
		// Comments - Muted/disabled color
		return theme.Color(theme.ColorNameDisabled)

	case TokenLiteralNumber:
		// Numbers - Distinct color
		return color.NRGBA{R: 200, G: 100, B: 200, A: 255} // Light purple

	case TokenWhitespace:
		// Whitespace - invisible (use background color)
		return theme.Color(theme.ColorNameInputBackground)

	case TokenIdentifier:
		// Generic identifiers - Default text color
		return theme.Color(theme.ColorNameForeground)

	case TokenError:
		// Lexing errors - Error color
		return theme.Color(theme.ColorNameError)

	default:
		// Unknown token types - Default to foreground color
		return theme.Color(theme.ColorNameForeground)
	}
}

// Color scheme documentation for NONMEM syntax:
//
// TokenDirective (Primary):
//   - $PROBLEM, $DATA, $INPUT, $ESTIMATION, $TABLE, $THETA, $OMEGA, $SIGMA
//   - These are the main structural directives of a NONMEM control stream
//
// TokenSectionKeyword (Focus):
//   - $PK, $ERROR, $DES, $PRED, $MODEL, $SUBROUTINES
//   - These define code sections where users write equations
//
// TokenDataKeyword (Blue):
//   - ID, TIME, DV, AMT, EVID, CMT, MDV, RATE, SS, II, ADDL
//   - Standard NONMEM data column names
//
// TokenModelKeyword (Gold):
//   - ADVAN1, ADVAN2, TRANS1, FOCE, FOCEI, SAEM, LAPLACE
//   - Subroutine and estimation method names
//
// TokenParameterID (Green):
//   - THETA, ETA, EPS, CL, V, KA, Q
//   - Pharmacokinetic parameters and random effects
//
// TokenComment (Disabled):
//   - ; This is a comment
//   - All text after semicolon to end of line
//
// TokenLiteralNumber (Purple):
//   - 0.1, 1E-3, 2.5E+2
//   - Numeric values in initial estimates or data
