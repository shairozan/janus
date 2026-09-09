//go:build unit
// +build unit

package editor

import (
	"testing"

	"github.com/shairozan/janus/internal/execution/category"
)

func TestLexerForCategory_NONMEM(t *testing.T) {
	lexer := LexerForCategory(category.CategoryNONMEM)
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "NONMEM" {
		t.Errorf("expected NONMEM lexer, got %s", lexer.Name())
	}
}

func TestLexerForCategory_Stan(t *testing.T) {
	lexer := LexerForCategory(category.CategoryStan)
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "Stan" {
		t.Errorf("expected Stan lexer, got %s", lexer.Name())
	}
}

func TestLexerForCategory_Torsten(t *testing.T) {
	lexer := LexerForCategory(category.CategoryTorsten)
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "Torsten" {
		t.Errorf("expected Torsten lexer, got %s", lexer.Name())
	}
}

func TestLexerForCategory_Monolix(t *testing.T) {
	lexer := LexerForCategory(category.CategoryMonolix)
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "Monolix" {
		t.Errorf("expected Monolix lexer, got %s", lexer.Name())
	}
}

func TestLexerForCategory_Unknown(t *testing.T) {
	lexer := LexerForCategory(category.CategoryUnknown)
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "Plain Text" {
		t.Errorf("expected Plain Text lexer for unknown category, got %s", lexer.Name())
	}
}

func TestLexerForCategory_EmptyCategory(t *testing.T) {
	lexer := LexerForCategory("")
	if lexer == nil {
		t.Fatal("expected non-nil lexer")
	}
	if lexer.Name() != "Plain Text" {
		t.Errorf("expected Plain Text lexer for empty category, got %s", lexer.Name())
	}
}

func TestLexerForCategory_ImplementsInterface(t *testing.T) {
	categories := []category.CategoryType{
		category.CategoryNONMEM,
		category.CategoryStan,
		category.CategoryTorsten,
		category.CategoryMonolix,
		category.CategoryUnknown,
	}

	for _, cat := range categories {
		t.Run(string(cat), func(t *testing.T) {
			lexer := LexerForCategory(cat)
			if lexer == nil {
				t.Fatal("expected non-nil lexer")
			}

			// Verify it implements Lexer interface by calling methods
			name := lexer.Name()
			if name == "" {
				t.Error("expected non-empty name")
			}

			// Test that LexLine works without panicking
			tokens := lexer.LexLine([]rune("test input"))
			if tokens == nil {
				t.Error("expected non-nil tokens slice")
			}
		})
	}
}
