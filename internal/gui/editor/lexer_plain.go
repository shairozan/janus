package editor

// PlainLexer implements the Lexer interface for plain text files.
// It provides no syntax highlighting - all text is treated as plain identifiers.
// This is useful as a fallback when no specific lexer is available for a file type.
type PlainLexer struct{}

// NewPlainLexer creates a new plain text lexer instance.
func NewPlainLexer() *PlainLexer {
	return &PlainLexer{}
}

// LexLine tokenizes a single line of plain text.
// For plain text, the entire line is returned as a single identifier token.
func (l *PlainLexer) LexLine(line []rune) []Token {
	if len(line) == 0 {
		return nil
	}

	return []Token{
		{
			Type:     TokenIdentifier,
			Value:    string(line),
			StartPos: 0,
			EndPos:   len(line),
		},
	}
}

// Name returns the display name for this lexer.
func (l *PlainLexer) Name() string {
	return "Plain Text"
}
