// Simple standalone lexer test with timeout protection
// Run with: go run cmd/test-lexer/main.go
//
//nolint:forbidigo // Test utility with intentional console output
package main

import (
	"fmt"
	"time"

	"github.com/shairozan/janus/internal/gui/editor"
)

//nolint:unparam // timeout is intentionally configurable for test flexibility
func testWithTimeout(name, input string, timeout time.Duration) {
	fmt.Printf("Testing: %s\n", name)
	fmt.Printf("  Input: %q\n", input)

	done := make(chan bool, 1)
	var tokens []editor.Token

	go func() {
		tokens = editor.LexLine([]rune(input))
		done <- true
	}()

	select {
	case <-done:
		fmt.Printf("  ✓ Completed successfully\n")
		fmt.Printf("  Tokens: %d\n", len(tokens))
		for i, token := range tokens {
			fmt.Printf("    [%d] %v: %q\n", i, token.Type, token.Value)
		}
	case <-time.After(timeout):
		fmt.Printf("  ✗ TIMEOUT after %v - likely infinite loop!\n", timeout)
		fmt.Printf("  This test would have hung the system.\n")

		return
	}

	fmt.Println()
}

func main() {
	fmt.Println("NONMEM Lexer Safety Test")
	fmt.Println("========================")
	fmt.Println("Each test has a 2-second timeout to prevent system hangs.")

	// Test cases with 2-second timeout (should be instant if working)
	timeout := 2 * time.Second

	testWithTimeout("Empty string", "", timeout)
	testWithTimeout("Single character", "A", timeout)
	testWithTimeout("Simple directive", "$PROBLEM", timeout)
	testWithTimeout("Directive with space", "$PROBLEM ", timeout)
	testWithTimeout("Directive with text", "$PROBLEM Test Model", timeout)
	testWithTimeout("Comment only", "; This is a comment", timeout)
	testWithTimeout("Text with comment", "Test ; comment", timeout)
	testWithTimeout("Number integer", "123", timeout)
	testWithTimeout("Number decimal", "0.5", timeout)
	testWithTimeout("Scientific notation", "1E-3", timeout)
	testWithTimeout("Whitespace only", "   ", timeout)
	testWithTimeout("Mixed content", "$PROBLEM Test ; comment", timeout)
	testWithTimeout("No trailing newline", "$DATA data.csv", timeout)
	testWithTimeout("Section keyword", "$PK", timeout)
	testWithTimeout("Multiple directives", "$THETA $OMEGA", timeout)

	fmt.Println("========================")
	fmt.Println("All tests completed!")
	fmt.Println("If you see any TIMEOUT messages, the lexer still has bugs.")
}
