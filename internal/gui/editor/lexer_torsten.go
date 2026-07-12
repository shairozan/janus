package editor

import (
	"strings"
	"unicode"
)

// TorstenLexer implements the Lexer interface for Torsten model files.
// Torsten is an extension to Stan that adds PK/PD modeling capabilities.
// It provides all Stan syntax highlighting plus Torsten-specific PK functions.
type TorstenLexer struct{}

// NewTorstenLexer creates a new Torsten lexer instance.
func NewTorstenLexer() *TorstenLexer {
	return &TorstenLexer{}
}

// torstenLexer holds the state of the Torsten lexical analysis.
type torstenLexer struct {
	input      []rune
	pos        int
	start      int
	tokens     []Token
	iterations int
}

const torstenMaxIterations = 100000

// newTorstenLexer creates a new Torsten lexer for the given input.
func newTorstenLexer(input []rune) *torstenLexer {
	return &torstenLexer{
		input:      input,
		pos:        0,
		start:      0,
		tokens:     []Token{},
		iterations: 0,
	}
}

// LexLine tokenizes a single line of Torsten code.
func (l *TorstenLexer) LexLine(line []rune) []Token {
	tl := newTorstenLexer(line)

	return tl.lex()
}

// Name returns the display name for this lexer.
func (l *TorstenLexer) Name() string {
	return "Torsten"
}

func (tl *torstenLexer) lex() []Token {
	for tl.pos < len(tl.input) {
		tl.iterations++
		if tl.iterations > torstenMaxIterations {
			tl.tokens = append(tl.tokens, Token{
				Type:  TokenError,
				Value: "lexer exceeded iteration limit",
			})

			break
		}

		tl.start = tl.pos
		r := tl.input[tl.pos]

		switch {
		case r == '/' && tl.peek(1) == '/':
			// Single-line comment
			tl.lexComment()
		case r == '/' && tl.peek(1) == '*':
			// Multi-line comment
			tl.lexBlockComment()
		case r == '~':
			// Sampling operator
			tl.pos++
			tl.emit(TokenDirective)
		case unicode.IsSpace(r):
			tl.lexWhitespace()
		case unicode.IsDigit(r) || (r == '.' && tl.isDigit(tl.peek(1))):
			tl.lexNumber()
		case r == '"' || r == '\'':
			tl.lexString(r)
		case unicode.IsLetter(r) || r == '_':
			tl.lexIdentifier()
		default:
			tl.pos++
			tl.emit(TokenIdentifier)
		}
	}

	return tl.tokens
}

func (tl *torstenLexer) peek(offset int) rune {
	pos := tl.pos + offset
	if pos >= len(tl.input) {
		return 0
	}

	return tl.input[pos]
}

func (tl *torstenLexer) isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (tl *torstenLexer) emit(tokenType TokenType) {
	if tl.pos <= tl.start {
		return
	}

	start := tl.start
	end := tl.pos

	if start > len(tl.input) {
		start = len(tl.input)
	}
	if end > len(tl.input) {
		end = len(tl.input)
	}
	if start > end {
		start = end
	}

	tl.tokens = append(tl.tokens, Token{
		Type:     tokenType,
		Value:    string(tl.input[start:end]),
		StartPos: start,
		EndPos:   end,
	})
	tl.start = tl.pos
}

func (tl *torstenLexer) lexComment() {
	for tl.pos < len(tl.input) {
		tl.pos++
	}
	tl.emit(TokenComment)
}

func (tl *torstenLexer) lexBlockComment() {
	for tl.pos < len(tl.input) {
		tl.pos++
	}
	tl.emit(TokenComment)
}

func (tl *torstenLexer) lexWhitespace() {
	for tl.pos < len(tl.input) && unicode.IsSpace(tl.input[tl.pos]) {
		tl.pos++
	}
	tl.emit(TokenWhitespace)
}

func (tl *torstenLexer) lexNumber() {
	for tl.pos < len(tl.input) && unicode.IsDigit(tl.input[tl.pos]) {
		tl.pos++
	}

	if tl.pos < len(tl.input) && tl.input[tl.pos] == '.' {
		tl.pos++
		for tl.pos < len(tl.input) && unicode.IsDigit(tl.input[tl.pos]) {
			tl.pos++
		}
	}

	if tl.pos < len(tl.input) && (tl.input[tl.pos] == 'e' || tl.input[tl.pos] == 'E') {
		tl.pos++
		if tl.pos < len(tl.input) && (tl.input[tl.pos] == '+' || tl.input[tl.pos] == '-') {
			tl.pos++
		}
		for tl.pos < len(tl.input) && unicode.IsDigit(tl.input[tl.pos]) {
			tl.pos++
		}
	}

	tl.emit(TokenLiteralNumber)
}

func (tl *torstenLexer) lexString(quote rune) {
	tl.pos++
	for tl.pos < len(tl.input) {
		r := tl.input[tl.pos]
		if r == quote {
			tl.pos++

			break
		}
		if r == '\\' && tl.pos+1 < len(tl.input) {
			tl.pos += 2

			continue
		}
		tl.pos++
	}
	tl.emit(TokenIdentifier)
}

func (tl *torstenLexer) lexIdentifier() {
	for tl.pos < len(tl.input) {
		r := tl.input[tl.pos]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			tl.pos++
		} else {
			break
		}
	}

	word := string(tl.input[tl.start:tl.pos])
	tokenType := classifyTorstenWord(word)
	tl.emit(tokenType)
}

// classifyTorstenWord determines the token type for a Torsten identifier.
// It recognizes both Stan and Torsten-specific keywords.
func classifyTorstenWord(word string) TokenType {
	lower := strings.ToLower(word)

	// Block keywords (Stan)
	blockKeywords := map[string]bool{
		"data": true, "transformed": true, "parameters": true,
		"model": true, "generated": true, "quantities": true,
		"functions": true,
	}

	if blockKeywords[lower] {
		return TokenSectionKeyword
	}

	// Type keywords (Stan)
	typeKeywords := map[string]bool{
		"int": true, "real": true, "vector": true, "row_vector": true,
		"matrix": true, "simplex": true, "ordered": true,
		"positive_ordered": true, "corr_matrix": true, "cov_matrix": true,
		"cholesky_factor_corr": true, "cholesky_factor_cov": true,
		"unit_vector": true, "array": true, "complex": true,
	}

	if typeKeywords[lower] {
		return TokenDataKeyword
	}

	// Control flow (Stan)
	controlKeywords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true,
		"return": true, "break": true, "continue": true,
		"in": true, "target": true, "print": true, "reject": true,
	}

	if controlKeywords[lower] {
		return TokenModelKeyword
	}

	// Torsten-specific PK/PD functions (these get special highlighting)
	torstenFunctions := map[string]bool{
		// One-compartment models
		"pmx_solve_onecpt": true, "PKModelOneCpt": true,
		// Two-compartment models
		"pmx_solve_twocpt": true, "PKModelTwoCpt": true,
		// General linear ODE
		"pmx_solve_linode": true, "linOdeModel": true,
		// General ODE solvers
		"pmx_solve_rk45": true, "pmx_solve_bdf": true, "pmx_solve_adams": true,
		"generalOdeModel": true, "generalOdeModel_rk45": true, "generalOdeModel_bdf": true,
		// Mixed ODE solvers
		"mixOde1CptModel_rk45": true, "mixOde1CptModel_bdf": true,
		"mixOde2CptModel_rk45": true, "mixOde2CptModel_bdf": true,
		// Effect compartment models
		"effCptModel": true,
		// Steady state solvers
		"pmx_solve_onecpt_ss": true, "pmx_solve_twocpt_ss": true,
		// Group ODE solvers
		"pmx_solve_group_rk45": true, "pmx_solve_group_bdf": true, "pmx_solve_group_adams": true,
		// Event data handling
		"pmx_to_event_array": true,
		// Interpolation
		"pmx_linear_interpolation": true,
	}

	if torstenFunctions[lower] || torstenFunctions[word] {
		return TokenDirective // Special highlighting for Torsten functions
	}

	// Stan distributions
	distributions := map[string]bool{
		"normal": true, "lognormal": true, "gamma": true, "beta": true,
		"exponential": true, "poisson": true, "binomial": true,
		"categorical": true, "dirichlet": true, "multinomial": true,
		"uniform": true, "cauchy": true, "student_t": true,
		"inv_gamma": true, "multi_normal": true, "wishart": true,
		"bernoulli": true, "neg_binomial": true,
	}

	if distributions[lower] {
		return TokenParameterID
	}

	// Stan math functions
	mathFunctions := map[string]bool{
		"exp": true, "log": true, "log10": true, "sqrt": true, "pow": true,
		"abs": true, "fabs": true, "sin": true, "cos": true, "tan": true,
		"floor": true, "ceil": true, "round": true, "sum": true, "prod": true,
		"mean": true, "sd": true, "variance": true, "min": true, "max": true,
		"inv": true, "transpose": true, "trace": true, "determinant": true,
		"rows": true, "cols": true, "size": true, "softmax": true, "log_sum_exp": true,
	}

	if mathFunctions[lower] {
		return TokenParameterID
	}

	return TokenIdentifier
}
