//go:build unit
// +build unit

package editor

import (
	"testing"
)

func TestMonolixLexer_Name(t *testing.T) {
	lexer := NewMonolixLexer()
	if lexer.Name() != "Monolix" {
		t.Errorf("expected name 'Monolix', got '%s'", lexer.Name())
	}
}

func TestMonolixLexer_EmptyLine(t *testing.T) {
	lexer := NewMonolixLexer()
	tokens := lexer.LexLine([]rune(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty line, got %d", len(tokens))
	}
}

func TestMonolixLexer_SectionHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"LONGITUDINAL section", "[LONGITUDINAL]", TokenSectionKeyword},
		{"INDIVIDUAL section", "[INDIVIDUAL]", TokenSectionKeyword},
		{"COVARIATE section", "[COVARIATE]", TokenSectionKeyword},
		{"POPULATION section", "[POPULATION]", TokenSectionKeyword},
		{"FIT section", "[FIT]", TokenSectionKeyword},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_Directives(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"MODEL directive", "<MODEL>", TokenDirective},
		{"DESIGN directive", "<DESIGN>", TokenDirective},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_PKMacros(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"compartment macro", "compartment", TokenDirective},
		{"elimination macro", "elimination", TokenDirective},
		{"absorption macro", "absorption", TokenDirective},
		{"distribution macro", "distribution", TokenDirective},
		{"transfer macro", "transfer", TokenDirective},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_DataKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"ID keyword", "ID", TokenDataKeyword},
		{"TIME keyword", "TIME", TokenDataKeyword},
		{"AMT keyword", "AMT", TokenDataKeyword},
		{"DV keyword", "DV", TokenDataKeyword},
		{"EVID keyword", "EVID", TokenDataKeyword},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_ModelKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"equation keyword", "equation", TokenModelKeyword},
		{"ode keyword", "ode", TokenModelKeyword},
		{"ddt keyword", "ddt", TokenModelKeyword},
		{"if keyword", "if", TokenModelKeyword},
		{"else keyword", "else", TokenModelKeyword},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_ParameterKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TokenType
	}{
		{"THETA keyword", "THETA", TokenParameterID},
		{"ETA keyword", "ETA", TokenParameterID},
		{"OMEGA keyword", "OMEGA", TokenParameterID},
		{"SIGMA keyword", "SIGMA", TokenParameterID},
		{"logNormal distribution", "logNormal", TokenParameterID},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_Comment(t *testing.T) {
	lexer := NewMonolixLexer()
	tokens := lexer.LexLine([]rune("; This is a Monolix comment"))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenComment {
		t.Errorf("expected TokenComment, got %v", tokens[0].Type)
	}
}

func TestMonolixLexer_Numbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"integer", "123", "123"},
		{"decimal", "0.5", "0.5"},
		{"scientific", "1e-3", "1e-3"},
	}

	lexer := NewMonolixLexer()
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

func TestMonolixLexer_ComplexLine(t *testing.T) {
	lexer := NewMonolixLexer()
	tokens := lexer.LexLine([]rune("Cl = THETA(1) * exp(ETA(1)) ; clearance"))

	// Should contain: Cl, whitespace, =, whitespace, THETA, (, 1, ), whitespace, *, etc.
	if len(tokens) < 5 {
		t.Fatalf("expected at least 5 tokens, got %d", len(tokens))
	}

	// Check there's a comment at the end
	lastToken := tokens[len(tokens)-1]
	if lastToken.Type != TokenComment {
		t.Errorf("expected last token to be TokenComment, got %v", lastToken.Type)
	}
}
