//go:build unit
// +build unit

package editor

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestBracketMatching_SimpleParentheses(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("(test)")

	// Position cursor after opening bracket
	editor.CursorPos = 1
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 5 {
		t.Errorf("Expected matching bracket at position 5, got %d", editor.matchingBracketPos)
	}

	// Position cursor before closing bracket
	editor.CursorPos = 5
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 0 {
		t.Errorf("Expected matching bracket at position 0, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_NestedParentheses(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("THETA(1) * EXP(ETA(1))")

	// Position cursor after first opening bracket (THETA(
	editor.CursorPos = 6 // After '(' at position 5
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 7 {
		t.Errorf("Expected matching bracket at position 7, got %d", editor.matchingBracketPos)
	}

	// Position cursor after EXP opening bracket
	editor.CursorPos = 15 // After 'EXP(' at position 14
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 21 {
		t.Errorf("Expected matching bracket at position 21, got %d", editor.matchingBracketPos)
	}

	// Position cursor after nested ETA opening bracket
	editor.CursorPos = 19 // After 'ETA(' at position 18
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 20 {
		t.Errorf("Expected matching bracket at position 20, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_SquareBrackets(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("array[index]")

	// Position cursor after opening square bracket
	editor.CursorPos = 6
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 11 {
		t.Errorf("Expected matching bracket at position 11, got %d", editor.matchingBracketPos)
	}

	// Position cursor before closing square bracket
	editor.CursorPos = 11
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 5 {
		t.Errorf("Expected matching bracket at position 5, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_CurlyBraces(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("{block}")

	// Position cursor after opening brace
	editor.CursorPos = 1
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 6 {
		t.Errorf("Expected matching bracket at position 6, got %d", editor.matchingBracketPos)
	}

	// Position cursor before closing brace
	editor.CursorPos = 6
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 0 {
		t.Errorf("Expected matching bracket at position 0, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_NoMatch(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("no brackets here")

	// Position cursor in middle
	editor.CursorPos = 5
	editor.updateBracketMatching()

	if editor.matchingBracketPos != -1 {
		t.Errorf("Expected no matching bracket (-1), got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_UnmatchedBracket(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("(unclosed")

	// Position cursor after opening bracket
	editor.CursorPos = 1
	editor.updateBracketMatching()

	// Should not find a match for unclosed bracket
	if editor.matchingBracketPos != -1 {
		t.Errorf("Expected no matching bracket for unclosed parenthesis, got %d", editor.matchingBracketPos)
	}

	// Test closing bracket without opening
	editor.SetText("unclosed)")
	editor.CursorPos = 8 // Before ')'
	editor.updateBracketMatching()

	if editor.matchingBracketPos != -1 {
		t.Errorf("Expected no matching bracket for unmatched closing parenthesis, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_ComplexNONMEMExpression(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	// Typical NONMEM expression with multiple nested brackets
	editor.SetText("CL = THETA(1) * (WT/70)**THETA(2) * EXP(ETA(1))")

	// Test THETA(1) opening bracket
	editor.CursorPos = 11 // After 'THETA(' at position 10
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 12 {
		t.Errorf("Expected THETA(1) closing bracket at 12, got %d", editor.matchingBracketPos)
	}

	// Test (WT/70) opening bracket
	editor.CursorPos = 17 // After '(' at position 16
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 22 {
		t.Errorf("Expected (WT/70) closing bracket at 22, got %d", editor.matchingBracketPos)
	}

	// Test THETA(2) opening bracket
	editor.CursorPos = 31 // After 'THETA(' at position 30
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 32 {
		t.Errorf("Expected THETA(2) closing bracket at 32, got %d", editor.matchingBracketPos)
	}

	// Test EXP(ETA(1)) - outer opening bracket
	editor.CursorPos = 40 // After 'EXP(' at position 39
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 46 {
		t.Errorf("Expected EXP closing bracket at 46, got %d", editor.matchingBracketPos)
	}

	// Test EXP(ETA(1)) - inner opening bracket
	editor.CursorPos = 44 // After 'ETA(' at position 43
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 45 {
		t.Errorf("Expected ETA closing bracket at 45, got %d", editor.matchingBracketPos)
	}
}

func TestBracketMatching_CursorBeforeBracket(t *testing.T) {
	test.NewApp() // Initialize test Fyne app
	editor := NewModelEditor()
	editor.SetText("(test)")

	// Position cursor ON the opening bracket (should match character at cursor)
	editor.CursorPos = 0
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 5 {
		t.Errorf("Expected matching bracket at position 5, got %d", editor.matchingBracketPos)
	}

	// Position cursor AFTER opening bracket (should check previous char)
	editor.CursorPos = 1
	editor.updateBracketMatching()

	if editor.matchingBracketPos != 5 {
		t.Errorf("Expected matching bracket at position 5 (checking prev char), got %d", editor.matchingBracketPos)
	}
}
