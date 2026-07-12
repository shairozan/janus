//go:build unit
// +build unit

package editor

import (
	"testing"
)

func TestTorstenLexer_Name(t *testing.T) {
	lexer := NewTorstenLexer()
	if lexer.Name() != "Torsten" {
		t.Errorf("expected name 'Torsten', got '%s'", lexer.Name())
	}
}

func TestTorstenLexer_EmptyLine(t *testing.T) {
	lexer := NewTorstenLexer()
	tokens := lexer.LexLine([]rune(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty line, got %d", len(tokens))
	}
}

func TestTorstenLexer_StanKeywords(t *testing.T) {
	// Torsten should support all Stan keywords
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"data keyword", "data", TokenSectionKeyword},
		{"parameters keyword", "parameters", TokenSectionKeyword},
		{"model keyword", "model", TokenSectionKeyword},
		{"int type", "int", TokenDataKeyword},
		{"real type", "real", TokenDataKeyword},
		{"if keyword", "if", TokenModelKeyword},
		{"for keyword", "for", TokenModelKeyword},
	}

	lexer := NewTorstenLexer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := lexer.LexLine([]rune(tt.input))
			if len(tokens) < 1 {
				t.Fatalf("expected at least 1 token, got %d", len(tokens))
			}
			if tokens[0].Type != tt.expected {
				t.Errorf("expected token type %v, got %v", tt.expected, tokens[0].Type)
			}
		})
	}
}

func TestTorstenLexer_TorstenPKFunctions(t *testing.T) {
	// Torsten-specific PK functions should get special highlighting
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"pmx_solve_onecpt", "pmx_solve_onecpt", TokenDirective},
		{"pmx_solve_twocpt", "pmx_solve_twocpt", TokenDirective},
		{"pmx_solve_rk45", "pmx_solve_rk45", TokenDirective},
		{"pmx_solve_bdf", "pmx_solve_bdf", TokenDirective},
		{"pmx_solve_adams", "pmx_solve_adams", TokenDirective},
		{"PKModelOneCpt", "PKModelOneCpt", TokenDirective},
		{"PKModelTwoCpt", "PKModelTwoCpt", TokenDirective},
		{"generalOdeModel", "generalOdeModel", TokenDirective},
	}

	lexer := NewTorstenLexer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := lexer.LexLine([]rune(tt.input))
			if len(tokens) < 1 {
				t.Fatalf("expected at least 1 token, got %d", len(tokens))
			}
			if tokens[0].Type != tt.expected {
				t.Errorf("expected token type %v for %s, got %v", tt.expected, tt.input, tokens[0].Type)
			}
		})
	}
}

func TestTorstenLexer_Comment(t *testing.T) {
	lexer := NewTorstenLexer()
	tokens := lexer.LexLine([]rune("// Torsten comment"))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenComment {
		t.Errorf("expected TokenComment, got %v", tokens[0].Type)
	}
}

func TestTorstenLexer_SamplingOperator(t *testing.T) {
	lexer := NewTorstenLexer()
	tokens := lexer.LexLine([]rune("y ~ normal(mu, sigma)"))

	// Find the sampling operator
	var foundSampling bool
	for _, token := range tokens {
		if token.Value == "~" && token.Type == TokenDirective {
			foundSampling = true
			break
		}
	}

	if !foundSampling {
		t.Error("expected to find sampling operator (~) with TokenDirective type")
	}
}

func TestTorstenLexer_Numbers(t *testing.T) {
	lexer := NewTorstenLexer()
	tokens := lexer.LexLine([]rune("123.456"))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenLiteralNumber {
		t.Errorf("expected TokenLiteralNumber, got %v", tokens[0].Type)
	}
}

func TestTorstenLexer_ComplexPKExpression(t *testing.T) {
	lexer := NewTorstenLexer()
	// Typical Torsten expression
	tokens := lexer.LexLine([]rune("cmt = pmx_solve_twocpt(time, amt, rate, ii, evid, cmt, addl, ss, theta, biovar, tlag)"))

	// Check that pmx_solve_twocpt is recognized as a Torsten function
	var foundPKFunction bool
	for _, token := range tokens {
		if token.Value == "pmx_solve_twocpt" && token.Type == TokenDirective {
			foundPKFunction = true
			break
		}
	}

	if !foundPKFunction {
		t.Error("expected to find pmx_solve_twocpt with TokenDirective type")
	}
}
