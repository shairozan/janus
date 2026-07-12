//go:build unit
// +build unit

package editor

import (
	"testing"
)

func TestStanLexer_Name(t *testing.T) {
	lexer := NewStanLexer()
	if lexer.Name() != "Stan" {
		t.Errorf("expected name 'Stan', got '%s'", lexer.Name())
	}
}

func TestStanLexer_EmptyLine(t *testing.T) {
	lexer := NewStanLexer()
	tokens := lexer.LexLine([]rune(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty line, got %d", len(tokens))
	}
}

func TestStanLexer_BlockKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"data keyword", "data", TokenSectionKeyword},
		{"parameters keyword", "parameters", TokenSectionKeyword},
		{"model keyword", "model", TokenSectionKeyword},
		{"transformed keyword", "transformed", TokenSectionKeyword},
		{"generated keyword", "generated", TokenSectionKeyword},
		{"functions keyword", "functions", TokenSectionKeyword},
	}

	lexer := NewStanLexer()
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

func TestStanLexer_TypeKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"int type", "int", TokenDataKeyword},
		{"real type", "real", TokenDataKeyword},
		{"vector type", "vector", TokenDataKeyword},
		{"matrix type", "matrix", TokenDataKeyword},
		{"simplex type", "simplex", TokenDataKeyword},
	}

	lexer := NewStanLexer()
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

func TestStanLexer_ControlFlow(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"if keyword", "if", TokenModelKeyword},
		{"else keyword", "else", TokenModelKeyword},
		{"for keyword", "for", TokenModelKeyword},
		{"while keyword", "while", TokenModelKeyword},
		{"return keyword", "return", TokenModelKeyword},
	}

	lexer := NewStanLexer()
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

func TestStanLexer_Distributions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"normal distribution", "normal", TokenParameterID},
		{"gamma distribution", "gamma", TokenParameterID},
		{"beta distribution", "beta", TokenParameterID},
		{"uniform distribution", "uniform", TokenParameterID},
	}

	lexer := NewStanLexer()
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

func TestStanLexer_Comment(t *testing.T) {
	lexer := NewStanLexer()
	tokens := lexer.LexLine([]rune("// This is a comment"))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenComment {
		t.Errorf("expected TokenComment, got %v", tokens[0].Type)
	}
}

func TestStanLexer_SamplingOperator(t *testing.T) {
	lexer := NewStanLexer()
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

func TestStanLexer_Numbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"integer", "123", "123"},
		{"decimal", "0.5", "0.5"},
		{"scientific", "1e-3", "1e-3"},
		{"scientific positive", "2.5E+2", "2.5E+2"},
	}

	lexer := NewStanLexer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := lexer.LexLine([]rune(tt.input))
			if len(tokens) != 1 {
				t.Fatalf("expected 1 token, got %d", len(tokens))
			}
			if tokens[0].Type != TokenLiteralNumber {
				t.Errorf("expected TokenLiteralNumber, got %v", tokens[0].Type)
			}
			if tokens[0].Value != tt.expected {
				t.Errorf("expected value %s, got %s", tt.expected, tokens[0].Value)
			}
		})
	}
}

func TestStanLexer_ComplexLine(t *testing.T) {
	lexer := NewStanLexer()
	tokens := lexer.LexLine([]rune("real mu = 0.5; // mean"))

	// Should have tokens for: real, whitespace, mu, whitespace, =, whitespace, 0.5, ;, whitespace, // mean
	if len(tokens) < 5 {
		t.Fatalf("expected at least 5 tokens, got %d", len(tokens))
	}

	// Check first token is 'real' (type keyword)
	if tokens[0].Type != TokenDataKeyword {
		t.Errorf("expected first token to be TokenDataKeyword, got %v", tokens[0].Type)
	}

	// Check there's a comment at the end
	lastToken := tokens[len(tokens)-1]
	if lastToken.Type != TokenComment {
		t.Errorf("expected last token to be TokenComment, got %v", lastToken.Type)
	}
}
