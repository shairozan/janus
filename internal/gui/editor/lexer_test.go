//go:build unit
// +build unit

package editor

import (
	"testing"
)

func TestLexDirectives(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		expectedType  TokenType
	}{
		{
			name:          "$PROBLEM directive",
			input:         "$PROBLEM Test Model",
			expectedCount: 3, // $PROBLEM, whitespace, "Test Model"
			expectedType:  TokenDirective,
		},
		{
			name:          "$DATA directive",
			input:         "$DATA data.csv",
			expectedCount: 3, // $DATA, whitespace, "data.csv"
			expectedType:  TokenDirective,
		},
		{
			name:          "$PK section keyword",
			input:         "$PK",
			expectedCount: 1,
			expectedType:  TokenSectionKeyword,
		},
		{
			name:          "$ERROR section keyword",
			input:         "$ERROR",
			expectedCount: 1,
			expectedType:  TokenSectionKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := LexLine([]rune(tt.input))

			if len(tokens) < 1 {
				t.Fatalf("expected at least 1 token, got %d", len(tokens))
			}

			if tokens[0].Type != tt.expectedType {
				t.Errorf("expected token type %v, got %v", tt.expectedType, tokens[0].Type)
			}

			isExpectedRoot := tokens[0].Value == "$PROBLEM" || tokens[0].Value == "$DATA" ||
				tokens[0].Value == "$PK" || tokens[0].Value == "$ERROR"

			if !isExpectedRoot {
				t.Errorf("unexpected token value: %s", tokens[0].Value)
			}
		})
	}
}

func TestLexComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Simple comment",
			input: "; This is a comment",
		},
		{
			name:  "Comment after code",
			input: "$PROBLEM Test ; inline comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := LexLine([]rune(tt.input))

			// Find comment token
			var commentFound bool
			for _, token := range tokens {
				if token.Type == TokenComment {
					commentFound = true
					if len(token.Value) == 0 {
						t.Error("comment token should not be empty")
					}
				}
			}

			if !commentFound {
				t.Error("expected to find comment token")
			}
		})
	}
}

func TestLexNumbers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Integer",
			input:    "123",
			expected: "123",
		},
		{
			name:     "Decimal",
			input:    "0.5",
			expected: "0.5",
		},
		{
			name:     "Scientific notation",
			input:    "1E-3",
			expected: "1E-3",
		},
		{
			name:     "Scientific with plus",
			input:    "2.5E+2",
			expected: "2.5E+2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := LexLine([]rune(tt.input))

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

func TestLexWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Spaces",
			input: "   ",
		},
		{
			name:  "Tabs",
			input: "\t\t",
		},
		{
			name:  "Mixed whitespace",
			input: "  \t  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := LexLine([]rune(tt.input))

			if len(tokens) != 1 {
				t.Fatalf("expected 1 token, got %d", len(tokens))
			}

			if tokens[0].Type != TokenWhitespace {
				t.Errorf("expected TokenWhitespace, got %v", tokens[0].Type)
			}
		})
	}
}

func TestLexComplexLine(t *testing.T) {
	input := "$PROBLEM Test Model ; This is a comment"
	tokens := LexLine([]rune(input))

	// Expected tokens:
	// 1. $PROBLEM (TokenDirective)
	// 2. " " (TokenWhitespace)
	// 3. "Test" (TokenIdentifier)
	// 4. " " (TokenWhitespace)
	// 5. "Model" (TokenIdentifier)
	// 6. " " (TokenWhitespace)
	// 7. "; This is a comment" (TokenComment)

	expectedTypes := []TokenType{
		TokenDirective,
		TokenWhitespace,
		TokenIdentifier,
		TokenWhitespace,
		TokenIdentifier,
		TokenWhitespace,
		TokenComment,
	}

	if len(tokens) != len(expectedTypes) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTypes), len(tokens))
	}

	for i, expectedType := range expectedTypes {
		if tokens[i].Type != expectedType {
			t.Errorf("token %d: expected type %v, got %v (value: %q)",
				i, expectedType, tokens[i].Type, tokens[i].Value)
		}
	}
}

func TestLexNONMEMCode(t *testing.T) {
	// Test realistic NONMEM code line
	input := "CL = THETA(1) * EXP(ETA(1))"
	tokens := LexLine([]rune(input))

	// Should have multiple tokens including identifiers and numbers
	if len(tokens) < 3 {
		t.Fatalf("expected at least 3 tokens, got %d", len(tokens))
	}

	// Check that we have some identifiers
	var hasIdentifier bool
	for _, token := range tokens {
		if token.Type == TokenIdentifier {
			hasIdentifier = true
			break
		}
	}

	if !hasIdentifier {
		t.Error("expected to find at least one identifier token")
	}
}

func TestLexEmptyLine(t *testing.T) {
	tokens := LexLine([]rune(""))

	// Empty line should produce no tokens or just EOF
	if len(tokens) > 0 {
		t.Errorf("expected 0 tokens for empty line, got %d", len(tokens))
	}
}
