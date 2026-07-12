package editor

import (
	"strings"
	"unicode"
)

// MonolixLexer implements the Lexer interface for Monolix model files (mlxtran).
// Monolix is a pharmacometric software for population PK/PD analysis.
// It provides syntax highlighting for mlxtran sections, keywords, macros,
// mathematical functions, comments, and numeric literals.
type MonolixLexer struct{}

// NewMonolixLexer creates a new Monolix lexer instance.
func NewMonolixLexer() *MonolixLexer {
	return &MonolixLexer{}
}

// monolixLexer holds the state of the Monolix lexical analysis.
type monolixLexer struct {
	input      []rune
	pos        int
	start      int
	tokens     []Token
	iterations int
}

const monolixMaxIterations = 100000

// newMonolixLexer creates a new Monolix lexer for the given input.
func newMonolixLexer(input []rune) *monolixLexer {
	return &monolixLexer{
		input:      input,
		pos:        0,
		start:      0,
		tokens:     []Token{},
		iterations: 0,
	}
}

// LexLine tokenizes a single line of Monolix mlxtran code.
func (l *MonolixLexer) LexLine(line []rune) []Token {
	ml := newMonolixLexer(line)

	return ml.lex()
}

// Name returns the display name for this lexer.
func (l *MonolixLexer) Name() string {
	return "Monolix"
}

func (ml *monolixLexer) lex() []Token {
	for ml.pos < len(ml.input) {
		ml.iterations++
		if ml.iterations > monolixMaxIterations {
			ml.tokens = append(ml.tokens, Token{
				Type:  TokenError,
				Value: "lexer exceeded iteration limit",
			})

			break
		}

		ml.start = ml.pos
		r := ml.input[ml.pos]

		switch {
		case r == ';':
			// Comment (semicolon to end of line)
			ml.lexComment()
		case r == '[':
			// Section header [SECTION]
			ml.lexSection()
		case r == '<':
			// Directive/keyword like <MODEL>, <DESIGN>
			ml.lexDirective()
		case unicode.IsSpace(r):
			ml.lexWhitespace()
		case unicode.IsDigit(r) || (r == '.' && ml.isDigit(ml.peek(1))):
			ml.lexNumber()
		case r == '"' || r == '\'':
			ml.lexString(r)
		case unicode.IsLetter(r) || r == '_':
			ml.lexIdentifier()
		default:
			ml.pos++
			ml.emit(TokenIdentifier)
		}
	}

	return ml.tokens
}

func (ml *monolixLexer) peek(offset int) rune {
	pos := ml.pos + offset
	if pos >= len(ml.input) {
		return 0
	}

	return ml.input[pos]
}

func (ml *monolixLexer) isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (ml *monolixLexer) emit(tokenType TokenType) {
	if ml.pos <= ml.start {
		return
	}

	start := ml.start
	end := ml.pos

	if start > len(ml.input) {
		start = len(ml.input)
	}
	if end > len(ml.input) {
		end = len(ml.input)
	}
	if start > end {
		start = end
	}

	ml.tokens = append(ml.tokens, Token{
		Type:     tokenType,
		Value:    string(ml.input[start:end]),
		StartPos: start,
		EndPos:   end,
	})
	ml.start = ml.pos
}

func (ml *monolixLexer) lexComment() {
	for ml.pos < len(ml.input) {
		ml.pos++
	}
	ml.emit(TokenComment)
}

func (ml *monolixLexer) lexSection() {
	// Consume [SECTION_NAME]
	ml.pos++ // Skip [
	for ml.pos < len(ml.input) {
		r := ml.input[ml.pos]
		ml.pos++
		if r == ']' {
			break
		}
	}
	ml.emit(TokenSectionKeyword)
}

func (ml *monolixLexer) lexDirective() {
	// Consume <DIRECTIVE>
	ml.pos++ // Skip <
	for ml.pos < len(ml.input) {
		r := ml.input[ml.pos]
		ml.pos++
		if r == '>' {
			break
		}
	}
	ml.emit(TokenDirective)
}

func (ml *monolixLexer) lexWhitespace() {
	for ml.pos < len(ml.input) && unicode.IsSpace(ml.input[ml.pos]) {
		ml.pos++
	}
	ml.emit(TokenWhitespace)
}

func (ml *monolixLexer) lexNumber() {
	for ml.pos < len(ml.input) && unicode.IsDigit(ml.input[ml.pos]) {
		ml.pos++
	}

	if ml.pos < len(ml.input) && ml.input[ml.pos] == '.' {
		ml.pos++
		for ml.pos < len(ml.input) && unicode.IsDigit(ml.input[ml.pos]) {
			ml.pos++
		}
	}

	if ml.pos < len(ml.input) && (ml.input[ml.pos] == 'e' || ml.input[ml.pos] == 'E') {
		ml.pos++
		if ml.pos < len(ml.input) && (ml.input[ml.pos] == '+' || ml.input[ml.pos] == '-') {
			ml.pos++
		}
		for ml.pos < len(ml.input) && unicode.IsDigit(ml.input[ml.pos]) {
			ml.pos++
		}
	}

	ml.emit(TokenLiteralNumber)
}

func (ml *monolixLexer) lexString(quote rune) {
	ml.pos++
	for ml.pos < len(ml.input) {
		r := ml.input[ml.pos]
		if r == quote {
			ml.pos++

			break
		}
		if r == '\\' && ml.pos+1 < len(ml.input) {
			ml.pos += 2

			continue
		}
		ml.pos++
	}

	ml.emit(TokenIdentifier)
}

func (ml *monolixLexer) lexIdentifier() {
	for ml.pos < len(ml.input) {
		r := ml.input[ml.pos]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			ml.pos++
		} else {
			break
		}
	}

	word := string(ml.input[ml.start:ml.pos])
	tokenType := classifyMonolixWord(word)
	ml.emit(tokenType)
}

// classifyMonolixWord determines the token type for a Monolix identifier.
func classifyMonolixWord(word string) TokenType {
	upper := strings.ToUpper(word)

	// mlxtran section keywords
	sectionKeywords := map[string]bool{
		"LONGITUDINAL": true, "INDIVIDUAL": true, "COVARIATE": true,
		"POPULATION": true, "FIT": true, "DESIGN": true,
		"OBSERVATION": true, "DEFINITION": true, "MODEL": true,
		"INPUT": true, "OUTPUT": true, "STRUCTURAL": true,
		"VARIABILITY": true, "CORRELATION": true, "PARAMETER": true,
		"FILE": true, "CONTENT": true, "DATA": true,
		"TASKS": true, "MONOLIX": true,
	}

	if sectionKeywords[upper] {
		return TokenSectionKeyword
	}

	// PK macros and compartment keywords
	pkMacros := map[string]bool{
		"compartment":   true,
		"elimination":   true,
		"absorption":    true,
		"distribution":  true,
		"transfer":      true,
		"peripheral":    true,
		"oral":          true,
		"iv":            true,
		"depot":         true,
		"effect":        true,
		"concentration": true,
		"amount":        true,
		"volume":        true,
		"clearance":     true,
		"Tlag":          true,
		"ka":            true,
		"ke":            true,
		"Tk0":           true,
		"p":             true,
		"k":             true,
		"ktr":           true,
		"Mtt":           true,
		"Ktr":           true,
		"V":             true,
		"Cl":            true,
		"Q":             true,
		"V1":            true,
		"V2":            true,
		"V3":            true,
	}

	if pkMacros[word] || pkMacros[upper] {
		return TokenDirective
	}

	// Data column types
	dataKeywords := map[string]bool{
		"ID": true, "TIME": true, "AMT": true, "DV": true, "EVID": true,
		"MDV": true, "CMT": true, "RATE": true, "SS": true, "II": true,
		"ADDL": true, "OCC": true, "YTYPE": true, "CENS": true, "LIMIT": true,
		"ADM": true, "TINF": true,
		"use": true, "define": true, "type": true, "regressor": true,
	}

	if dataKeywords[upper] || dataKeywords[word] {
		return TokenDataKeyword
	}

	// Model definition keywords
	modelKeywords := map[string]bool{
		"input":    true,
		"PK":       true,
		"PRED":     true,
		"EQUATION": true,
		"equation": true,
		"ode":      true,
		"ODE":      true,
		"ddt":      true,
		"delay":    true,
		"if":       true,
		"else":     true,
		"elseif":   true,
		"end":      true,
		"while":    true,
		"for":      true,
		"sequence": true,
		"looplim":  true,
		"when":     true,
	}

	if modelKeywords[word] || modelKeywords[upper] {
		return TokenModelKeyword
	}

	// Parameter types
	paramKeywords := map[string]bool{
		"THETA": true, "theta": true,
		"ETA": true, "eta": true,
		"OMEGA": true, "omega": true,
		"SIGMA": true, "sigma": true,
		"EPS": true, "eps": true,
		"sd": true, "var": true, "cv": true,
		"logNormal": true, "normal": true, "logitNormal": true,
		"probitNormal": true, "uniform": true, "loguniform": true,
	}

	if paramKeywords[word] || paramKeywords[upper] {
		return TokenParameterID
	}

	// Built-in math functions
	mathFunctions := map[string]bool{
		"exp": true, "log": true, "log10": true, "log2": true,
		"sqrt": true, "pow": true, "abs": true,
		"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true,
		"sinh": true, "cosh": true, "tanh": true,
		"floor": true, "ceil": true, "round": true, "trunc": true,
		"min": true, "max": true,
		"sum": true, "prod": true, "mean": true,
		"factorial": true, "lgamma": true, "psi": true,
		"normcdf": true, "norminv": true,
		"erf": true, "erfc": true,
	}

	if mathFunctions[word] {
		return TokenParameterID
	}

	return TokenIdentifier
}
