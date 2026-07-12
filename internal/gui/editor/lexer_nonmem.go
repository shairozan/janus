package editor

import (
	"strings"
	"unicode"
)

// lexerStateFn represents a lexer state function.
// It processes input and returns the next state function.
type lexerStateFn func(*lexer) lexerStateFn

// lexer holds the state of the lexical analysis.
type lexer struct {
	input      []rune  // Input text as runes
	pos        int     // Current position in input
	start      int     // Start position of current token
	tokens     []Token // Collected tokens
	iterations int     // Safety counter to prevent infinite loops
}

const maxIterations = 100000 // Safety limit

// newLexer creates a new lexer for the given input.
func newLexer(input []rune) *lexer {
	return &lexer{
		input:      input,
		pos:        0,
		start:      0,
		tokens:     []Token{},
		iterations: 0,
	}
}

// LexLine runs the lexer state machine and returns all tokens for a line.
// Exported for testing purposes.
func LexLine(input []rune) []Token {
	l := newLexer(input)

	// Run state machine until completion with safety limit
	for state := lexText; state != nil; {
		l.iterations++
		if l.iterations > maxIterations {
			// Emergency exit - emit error token
			l.tokens = append(l.tokens, Token{
				Type:  TokenError,
				Value: "lexer exceeded iteration limit - possible infinite loop",
			})

			break
		}
		state = state(l)
	}

	return l.tokens
}

// emit creates a token and adds it to the token list.
func (l *lexer) emit(tokenType TokenType) {
	// Don't emit empty tokens
	if l.pos <= l.start {
		return
	}

	// Bounds checking to prevent panics
	start := l.start
	end := l.pos

	if start > len(l.input) {
		start = len(l.input)
	}
	if end > len(l.input) {
		end = len(l.input)
	}
	if start > end {
		start = end
	}

	token := Token{
		Type:     tokenType,
		Value:    string(l.input[start:end]),
		StartPos: start,
		EndPos:   end,
	}
	l.tokens = append(l.tokens, token)
	l.start = l.pos
}

// next consumes and returns the next rune.
func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		l.pos++ // Increment even at EOF to prevent stuck position

		return 0 // EOF
	}
	r := l.input[l.pos]
	l.pos++

	return r
}

// peek returns the next rune without consuming it.
func (l *lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}

	return l.input[l.pos]
}

// backup steps back one rune.
func (l *lexer) backup() {
	if l.pos > 0 {
		l.pos--
	}
}

// accept consumes the next rune if it's in the valid set.
func (l *lexer) accept(valid string) bool {
	r := l.next()
	if r == 0 {
		// EOF - don't accept
		return false
	}
	if strings.ContainsRune(valid, r) {
		return true
	}
	l.backup()

	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for {
		r := l.next()
		if r == 0 || !strings.ContainsRune(valid, r) {
			l.backup()

			return
		}
	}
}

// acceptWhile consumes runes while the predicate is true.
func (l *lexer) acceptWhile(predicate func(rune) bool) {
	for {
		r := l.next()
		if r == 0 || !predicate(r) {
			l.backup()

			return
		}
	}
}

// lexText is the main text processing state.
func lexText(l *lexer) lexerStateFn {
	// Safety check - if we're at or past EOF, we're done
	if l.pos >= len(l.input) {
		// Emit any remaining content
		if l.start < len(l.input) {
			l.pos = len(l.input)
			l.emit(TokenIdentifier)
		}

		return nil
	}

	r := l.next()

	switch {
	case r == 0:
		// EOF
		return nil

	case r == ';':
		// Comment - emit any text before comment
		l.backup()
		if l.pos > l.start {
			l.emit(TokenIdentifier)
		}
		l.next()        // Consume semicolon
		l.start = l.pos // CRITICAL: Update start position

		return lexComment

	case r == '$':
		// NONMEM directive - emit any text before directive
		l.backup()
		if l.pos > l.start {
			l.emit(TokenIdentifier)
		}
		l.next()        // Consume $
		l.start = l.pos // CRITICAL: Update start position

		return lexDirective

	case unicode.IsSpace(r):
		// Whitespace - emit any text before whitespace
		l.backup()
		if l.pos > l.start {
			l.emit(TokenIdentifier)
		}
		l.next()        // Consume first space
		l.start = l.pos // CRITICAL: Update start position

		return lexWhitespace

	case unicode.IsDigit(r) || (r == '.' && unicode.IsDigit(l.peek())):
		// Number - emit any text before number
		l.backup()
		if l.pos > l.start {
			l.emit(TokenIdentifier)
		}
		l.next()        // Consume first digit
		l.start = l.pos // CRITICAL: Update start position

		return lexNumber

	default:
		// Regular text - continue accumulating in lexText
		return lexText
	}
}

// lexComment processes semicolon-based comments.
func lexComment(l *lexer) lexerStateFn {
	// Consume everything until end of line or EOF
	for {
		r := l.next()
		if r == 0 || r == '\n' {
			l.backup() // Don't include newline in comment
			l.emit(TokenComment)

			return nil // Comments go to end of line, so we're done
		}
	}
}

// lexDirective processes NONMEM directives ($PROBLEM, $DATA, etc.)
func lexDirective(l *lexer) lexerStateFn {
	// We're positioned after the '$', so back up to include it
	l.backup()
	l.start = l.pos
	l.next() // Move past $ again

	// Consume directive name (letters only)
	l.acceptWhile(unicode.IsLetter)

	directiveName := strings.ToUpper(string(l.input[l.start:l.pos]))

	// Categorize directive
	switch directiveName {
	case "$PROBLEM", "$PROB", "$DATA", "$INPUT", "$INPT",
		"$ESTIMATION", "$EST", "$COVARIANCE", "$COV",
		"$TABLE", "$TAB", "$SIMULATION", "$SIM",
		"$THETA", "$OMEGA", "$SIGMA":
		l.emit(TokenDirective)

	case "$PK", "$PRED", "$ERROR", "$ERR", "$DES", "$AES", "$AESINITIAL", "$INFN",
		"$MODEL", "$ABBREVIATED", "$ABBR", "$SUBROUTINES", "$SUBROUTINE", "$SUB":
		l.emit(TokenSectionKeyword)

	default:
		// Unknown directive - treat as identifier
		l.emit(TokenIdentifier)
	}

	return lexText
}

// lexWhitespace processes whitespace runs.
func lexWhitespace(l *lexer) lexerStateFn {
	// We've already consumed one space character, now get the rest
	l.backup()
	l.start = l.pos

	l.acceptWhile(unicode.IsSpace)
	l.emit(TokenWhitespace)

	return lexText
}

// lexNumber processes numeric literals.
func lexNumber(l *lexer) lexerStateFn {
	// We've already consumed first digit, back up to include it
	l.backup()
	l.start = l.pos

	// Accept digits before decimal point
	l.acceptRun("0123456789")

	// Accept decimal point and digits after
	if l.accept(".") {
		l.acceptRun("0123456789")
	}

	// Accept scientific notation (1E-3, 2.5E+2)
	if l.accept("eE") {
		l.accept("+-")
		l.acceptRun("0123456789")
	}

	l.emit(TokenLiteralNumber)

	return lexText
}

// NONMEMLexer implements the Lexer interface for NONMEM control stream files.
// It provides syntax highlighting for NONMEM directives, sections, keywords,
// parameters, comments, and numeric literals.
type NONMEMLexer struct{}

// NewNONMEMLexer creates a new NONMEM lexer instance.
func NewNONMEMLexer() *NONMEMLexer {
	return &NONMEMLexer{}
}

// LexLine tokenizes a single line of NONMEM control stream text.
func (l *NONMEMLexer) LexLine(line []rune) []Token {
	return LexLine(line)
}

// Name returns the display name for this lexer.
func (l *NONMEMLexer) Name() string {
	return "NONMEM"
}
