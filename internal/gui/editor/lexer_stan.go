package editor

import (
	"strings"
	"unicode"
)

// StanLexer implements the Lexer interface for Stan model files.
// Stan is a probabilistic programming language for Bayesian statistical inference.
// It provides syntax highlighting for block keywords, types, distributions,
// control flow, built-in functions, comments, and numeric literals.
type StanLexer struct{}

// NewStanLexer creates a new Stan lexer instance.
func NewStanLexer() *StanLexer {
	return &StanLexer{}
}

// stanLexer holds the state of the Stan lexical analysis.
type stanLexer struct {
	input      []rune
	pos        int
	start      int
	tokens     []Token
	iterations int
}

const stanMaxIterations = 100000

// newStanLexer creates a new Stan lexer for the given input.
func newStanLexer(input []rune) *stanLexer {
	return &stanLexer{
		input:      input,
		pos:        0,
		start:      0,
		tokens:     []Token{},
		iterations: 0,
	}
}

// LexLine tokenizes a single line of Stan code.
func (l *StanLexer) LexLine(line []rune) []Token {
	sl := newStanLexer(line)

	return sl.lex()
}

// Name returns the display name for this lexer.
func (l *StanLexer) Name() string {
	return "Stan"
}

func (sl *stanLexer) lex() []Token {
	for sl.pos < len(sl.input) {
		sl.iterations++
		if sl.iterations > stanMaxIterations {
			sl.tokens = append(sl.tokens, Token{
				Type:  TokenError,
				Value: "lexer exceeded iteration limit",
			})

			break
		}

		sl.start = sl.pos
		r := sl.input[sl.pos]

		switch {
		case r == '/' && sl.peek(1) == '/':
			// Single-line comment
			sl.lexComment()
		case r == '/' && sl.peek(1) == '*':
			// Multi-line comment (treat rest of line as comment)
			sl.lexBlockComment()
		case r == '~':
			// Sampling operator - special highlighting
			sl.pos++
			sl.emit(TokenDirective)
		case unicode.IsSpace(r):
			sl.lexWhitespace()
		case unicode.IsDigit(r) || (r == '.' && sl.isDigit(sl.peek(1))):
			sl.lexNumber()
		case r == '"' || r == '\'':
			sl.lexString(r)
		case unicode.IsLetter(r) || r == '_':
			sl.lexIdentifier()
		default:
			// Single character token (operators, punctuation)
			sl.pos++
			sl.emit(TokenIdentifier)
		}
	}

	return sl.tokens
}

func (sl *stanLexer) peek(offset int) rune {
	pos := sl.pos + offset
	if pos >= len(sl.input) {
		return 0
	}

	return sl.input[pos]
}

func (sl *stanLexer) isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func (sl *stanLexer) emit(tokenType TokenType) {
	if sl.pos <= sl.start {
		return
	}

	start := sl.start
	end := sl.pos

	if start > len(sl.input) {
		start = len(sl.input)
	}
	if end > len(sl.input) {
		end = len(sl.input)
	}
	if start > end {
		start = end
	}

	sl.tokens = append(sl.tokens, Token{
		Type:     tokenType,
		Value:    string(sl.input[start:end]),
		StartPos: start,
		EndPos:   end,
	})
	sl.start = sl.pos
}

func (sl *stanLexer) lexComment() {
	// Consume // and everything after
	for sl.pos < len(sl.input) {
		sl.pos++
	}
	sl.emit(TokenComment)
}

func (sl *stanLexer) lexBlockComment() {
	// Consume /* and rest of line (multi-line comments span lines)
	for sl.pos < len(sl.input) {
		sl.pos++
	}
	sl.emit(TokenComment)
}

func (sl *stanLexer) lexWhitespace() {
	for sl.pos < len(sl.input) && unicode.IsSpace(sl.input[sl.pos]) {
		sl.pos++
	}
	sl.emit(TokenWhitespace)
}

func (sl *stanLexer) lexNumber() {
	// Integer part
	for sl.pos < len(sl.input) && unicode.IsDigit(sl.input[sl.pos]) {
		sl.pos++
	}

	// Decimal part
	if sl.pos < len(sl.input) && sl.input[sl.pos] == '.' {
		sl.pos++
		for sl.pos < len(sl.input) && unicode.IsDigit(sl.input[sl.pos]) {
			sl.pos++
		}
	}

	// Scientific notation
	if sl.pos < len(sl.input) && (sl.input[sl.pos] == 'e' || sl.input[sl.pos] == 'E') {
		sl.pos++
		if sl.pos < len(sl.input) && (sl.input[sl.pos] == '+' || sl.input[sl.pos] == '-') {
			sl.pos++
		}
		for sl.pos < len(sl.input) && unicode.IsDigit(sl.input[sl.pos]) {
			sl.pos++
		}
	}

	sl.emit(TokenLiteralNumber)
}

func (sl *stanLexer) lexString(quote rune) {
	sl.pos++ // Skip opening quote
	for sl.pos < len(sl.input) {
		r := sl.input[sl.pos]
		if r == quote {
			sl.pos++

			break
		}
		if r == '\\' && sl.pos+1 < len(sl.input) {
			sl.pos += 2 // Skip escape sequence

			continue
		}
		sl.pos++
	}
	sl.emit(TokenIdentifier) // String literals as identifiers
}

func (sl *stanLexer) lexIdentifier() {
	for sl.pos < len(sl.input) {
		r := sl.input[sl.pos]
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sl.pos++
		} else {
			break
		}
	}

	word := string(sl.input[sl.start:sl.pos])
	tokenType := classifyStanWord(word)
	sl.emit(tokenType)
}

// classifyStanWord determines the token type for a Stan identifier.
func classifyStanWord(word string) TokenType {
	lower := strings.ToLower(word)

	// Block keywords (major sections)
	blockKeywords := map[string]bool{
		"data": true, "transformed": true, "parameters": true,
		"model": true, "generated": true, "quantities": true,
		"functions": true,
	}

	if blockKeywords[lower] {
		return TokenSectionKeyword
	}

	// Type keywords
	typeKeywords := map[string]bool{
		"int": true, "real": true, "vector": true, "row_vector": true,
		"matrix": true, "simplex": true, "ordered": true,
		"positive_ordered": true, "corr_matrix": true, "cov_matrix": true,
		"cholesky_factor_corr": true, "cholesky_factor_cov": true,
		"unit_vector": true, "array": true, "complex": true,
		"complex_vector": true, "complex_matrix": true, "complex_row_vector": true,
	}

	if typeKeywords[lower] {
		return TokenDataKeyword
	}

	// Control flow
	controlKeywords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true,
		"return": true, "break": true, "continue": true,
		"in": true, "target": true, "print": true, "reject": true,
	}

	if controlKeywords[lower] {
		return TokenModelKeyword
	}

	// Common distributions and functions
	distributions := map[string]bool{
		"normal": true, "lognormal": true, "gamma": true, "beta": true,
		"exponential": true, "poisson": true, "binomial": true,
		"categorical": true, "dirichlet": true, "multinomial": true,
		"uniform": true, "cauchy": true, "student_t": true,
		"inv_gamma": true, "inv_chi_square": true, "scaled_inv_chi_square": true,
		"pareto": true, "weibull": true, "frechet": true, "gumbel": true,
		"multi_normal": true, "multi_student_t": true, "wishart": true,
		"inv_wishart": true, "lkj_corr": true, "bernoulli": true,
		"bernoulli_logit": true, "neg_binomial": true, "ordered_logistic": true,
		"normal_lpdf": true, "normal_lpmf": true, "normal_rng": true,
	}

	if distributions[lower] {
		return TokenParameterID
	}

	// Built-in math functions
	mathFunctions := map[string]bool{
		"exp": true, "log": true, "log10": true, "log2": true,
		"sqrt": true, "pow": true, "abs": true, "fabs": true,
		"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true,
		"sinh": true, "cosh": true, "tanh": true,
		"floor": true, "ceil": true, "round": true, "trunc": true,
		"sum": true, "prod": true, "mean": true, "sd": true, "variance": true,
		"min": true, "max": true, "dot_product": true, "dot_self": true,
		"inv": true, "inverse": true, "transpose": true, "trace": true,
		"determinant": true, "log_determinant": true,
		"rows": true, "cols": true, "size": true, "dims": true, "num_elements": true,
		"head": true, "tail": true, "segment": true, "rep_vector": true, "rep_matrix": true,
		"append_row": true, "append_col": true,
		"softmax": true, "log_softmax": true, "log_sum_exp": true,
		"lgamma": true, "tgamma": true, "digamma": true, "trigamma": true,
		"lbeta": true, "binomial_coefficient_log": true,
		"fma": true, "fdim": true, "fmod": true, "fmax": true, "fmin": true,
	}

	if mathFunctions[lower] {
		return TokenParameterID
	}

	return TokenIdentifier
}
