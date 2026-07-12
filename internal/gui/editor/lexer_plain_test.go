//go:build unit
// +build unit

package editor

import (
	"testing"
)

func TestPlainLexer_Name(t *testing.T) {
	lexer := NewPlainLexer()
	if lexer.Name() != "Plain Text" {
		t.Errorf("expected name 'Plain Text', got '%s'", lexer.Name())
	}
}

func TestPlainLexer_EmptyLine(t *testing.T) {
	lexer := NewPlainLexer()
	tokens := lexer.LexLine([]rune(""))
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty line, got %d", len(tokens))
	}
}

func TestPlainLexer_SingleWord(t *testing.T) {
	lexer := NewPlainLexer()
	tokens := lexer.LexLine([]rune("hello"))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Type != TokenIdentifier {
		t.Errorf("expected TokenIdentifier, got %v", tokens[0].Type)
	}
	if tokens[0].Value != "hello" {
		t.Errorf("expected value 'hello', got '%s'", tokens[0].Value)
	}
}

func TestPlainLexer_EntireLine(t *testing.T) {
	lexer := NewPlainLexer()
	input := "This is a complete line with special characters: @#$%^&*()"
	tokens := lexer.LexLine([]rune(input))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token for entire line, got %d", len(tokens))
	}
	if tokens[0].Type != TokenIdentifier {
		t.Errorf("expected TokenIdentifier, got %v", tokens[0].Type)
	}
	if tokens[0].Value != input {
		t.Errorf("expected value '%s', got '%s'", input, tokens[0].Value)
	}
}

func TestPlainLexer_TokenPositions(t *testing.T) {
	lexer := NewPlainLexer()
	input := "test line"
	tokens := lexer.LexLine([]rune(input))

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].StartPos != 0 {
		t.Errorf("expected StartPos 0, got %d", tokens[0].StartPos)
	}
	if tokens[0].EndPos != len(input) {
		t.Errorf("expected EndPos %d, got %d", len(input), tokens[0].EndPos)
	}
}

func TestPlainLexer_NoSyntaxHighlighting(t *testing.T) {
	lexer := NewPlainLexer()

	// Test that nothing gets special highlighting
	tests := []string{
		"$PROBLEM Test",     // NONMEM directive
		"data {",            // Stan block
		"[LONGITUDINAL]",    // Monolix section
		"; comment",         // Comment
		"123.456",           // Number
		"if else for while", // Keywords
		"pmx_solve_onecpt",  // Torsten function
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			tokens := lexer.LexLine([]rune(input))
			if len(tokens) != 1 {
				t.Fatalf("expected 1 token, got %d", len(tokens))
			}
			if tokens[0].Type != TokenIdentifier {
				t.Errorf("expected TokenIdentifier for plain text, got %v", tokens[0].Type)
			}
		})
	}
}
