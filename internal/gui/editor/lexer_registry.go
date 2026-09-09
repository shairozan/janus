package editor

import (
	"github.com/shairozan/janus/internal/execution/category"
)

// LexerForCategory returns the appropriate lexer for a given model category.
// This function maps model categories (NONMEM, Stan, Torsten, Monolix) to their
// corresponding lexer implementations.
//
// For unknown categories, a PlainLexer is returned to provide basic text editing
// without syntax highlighting.
func LexerForCategory(cat category.CategoryType) Lexer {
	switch cat {
	case category.CategoryNONMEM:
		return NewNONMEMLexer()
	case category.CategoryStan:
		return NewStanLexer()
	case category.CategoryTorsten:
		return NewTorstenLexer()
	case category.CategoryMonolix:
		return NewMonolixLexer()
	case category.CategoryUnknown:
		// Unknown category - use plain text
		return NewPlainLexer()
	default:
		// Any other category - use plain text
		return NewPlainLexer()
	}
}
