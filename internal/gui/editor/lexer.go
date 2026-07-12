// Package editor provides a syntax-highlighting text editor widget for model files.
// It supports multiple modeling languages through a pluggable lexer system.
package editor

// Lexer defines the interface for language-specific lexers.
// Each supported modeling language implements this interface to provide
// syntax highlighting for its specific syntax.
//
// The editor widget uses this interface to delegate tokenization to
// language-specific implementations (NONMEM, Stan, Monolix, etc.).
type Lexer interface {
	// LexLine tokenizes a single line of text (as runes) and returns the tokens.
	// The implementation should handle all language-specific syntax rules.
	// Each token includes its type, value, and position within the line.
	LexLine(line []rune) []Token

	// Name returns a human-readable name for this lexer (e.g., "NONMEM", "Stan").
	Name() string
}
