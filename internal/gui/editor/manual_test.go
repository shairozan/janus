//go:build ignore
// +build ignore

// Manual test program for lexer - run with: go run manual_test.go
// This is safer than automated tests during debugging

package main

import (
	"fmt"
)

// Copy of token types for standalone test
type TokenType int

const (
	TokenIdentifier TokenType = iota
	TokenDirective
	TokenSectionKeyword
	TokenDataKeyword
	TokenModelKeyword
	TokenParameterID
	TokenComment
	TokenLiteralNumber
	TokenWhitespace
	TokenError
)

func (t TokenType) String() string {
	names := []string{
		"Identifier",
		"Directive",
		"SectionKeyword",
		"DataKeyword",
		"ModelKeyword",
		"ParameterID",
		"Comment",
		"LiteralNumber",
		"Whitespace",
		"Error",
	}
	if int(t) < len(names) {
		return names[t]
	}
	return "Unknown"
}

type Token struct {
	Type     TokenType
	Value    string
	StartPos int
	EndPos   int
}

// Import the lexer functions directly
// (In real use, would import from editor package)

func main() {
	tests := []struct {
		name  string
		input string
	}{
		{"Empty string", ""},
		{"Single character", "A"},
		{"Simple directive", "$PROBLEM"},
		{"Directive with text", "$PROBLEM Test"},
		{"Comment", "; This is a comment"},
		{"Number", "123"},
		{"Decimal", "0.5"},
		{"Scientific notation", "1E-3"},
		{"Whitespace", "   "},
		{"Mixed content", "$PROBLEM Test ; comment"},
		{"No newline at end", "$DATA data.csv"},
	}

	fmt.Println("Manual Lexer Test")
	fmt.Println("=================\n")

	for _, tt := range tests {
		fmt.Printf("Test: %s\n", tt.name)
		fmt.Printf("Input: %q\n", tt.input)

		// This would call lexLine from the actual package
		// For now, just show we'd call it
		fmt.Printf("Would call: lexLine([]rune(%q))\n", tt.input)
		fmt.Printf("Expected: Should complete without hanging\n\n")
	}

	fmt.Println("All tests defined. To actually run:")
	fmt.Println("1. Import this package's lexLine function")
	fmt.Println("2. Call lexLine for each test")
	fmt.Println("3. Print results")
	fmt.Println("\nIf any test hangs, press Ctrl+C immediately!")
}
