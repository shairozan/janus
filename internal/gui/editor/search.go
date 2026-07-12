package editor

import (
	"strings"
)

// Find searches for the given term and highlights all matches.
func (e *ModelEditor) Find(searchTerm string, caseSensitive bool) {
	e.searchTerm = searchTerm
	e.caseSensitive = caseSensitive
	e.searchMatches = []int{}
	e.currentMatch = -1

	if searchTerm == "" {
		e.Refresh()

		return
	}

	// Prepare search strings
	content := string(e.TextContent)
	search := searchTerm

	if !caseSensitive {
		content = strings.ToLower(content)
		search = strings.ToLower(search)
	}

	// Find all matches
	pos := 0
	for {
		index := strings.Index(content[pos:], search)
		if index == -1 {
			break
		}

		matchPos := pos + index
		e.searchMatches = append(e.searchMatches, matchPos)
		pos = matchPos + 1 // Move past this match to find overlapping matches
	}

	// If we found matches, jump to the first one
	if len(e.searchMatches) > 0 {
		e.currentMatch = 0
		e.jumpToCurrentMatch()
	}

	e.Refresh()
}

// FindNext jumps to the next search match.
func (e *ModelEditor) FindNext() {
	if len(e.searchMatches) == 0 {
		return
	}

	e.currentMatch = (e.currentMatch + 1) % len(e.searchMatches)
	e.jumpToCurrentMatch()
}

// FindPrevious jumps to the previous search match.
func (e *ModelEditor) FindPrevious() {
	if len(e.searchMatches) == 0 {
		return
	}

	e.currentMatch--
	if e.currentMatch < 0 {
		e.currentMatch = len(e.searchMatches) - 1
	}

	e.jumpToCurrentMatch()
}

// jumpToCurrentMatch moves the cursor to the current search match and selects it.
func (e *ModelEditor) jumpToCurrentMatch() {
	if e.currentMatch < 0 || e.currentMatch >= len(e.searchMatches) {
		return
	}

	matchPos := e.searchMatches[e.currentMatch]
	matchEnd := matchPos + len([]rune(e.searchTerm))

	// Move cursor and select the match
	e.CursorPos = matchPos
	e.SetSelection(matchPos, matchEnd)

	// Refresh to update UI
	e.Refresh()
}

// ClearSearch clears the current search and removes highlighting.
func (e *ModelEditor) ClearSearch() {
	e.searchTerm = ""
	e.searchMatches = []int{}
	e.currentMatch = -1
	e.Refresh()
}

// GetSearchMatchCount returns the number of search matches found.
func (e *ModelEditor) GetSearchMatchCount() int {
	return len(e.searchMatches)
}

// GetCurrentMatchIndex returns the current match index (1-based for display).
func (e *ModelEditor) GetCurrentMatchIndex() int {
	if e.currentMatch < 0 {
		return 0
	}

	return e.currentMatch + 1
}

// Replace replaces the current match with the given replacement text.
func (e *ModelEditor) Replace(replacement string) {
	if e.currentMatch < 0 || e.currentMatch >= len(e.searchMatches) {
		return
	}

	// Save undo state
	e.pushUndoState()

	matchPos := e.searchMatches[e.currentMatch]
	matchEnd := matchPos + len([]rune(e.searchTerm))

	// Delete the match
	replacementRunes := []rune(replacement)
	e.TextContent = append(e.TextContent[:matchPos], append(replacementRunes, e.TextContent[matchEnd:]...)...)

	// Update cursor position
	e.CursorPos = matchPos + len(replacementRunes)

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()

	// Re-run the search to update match positions
	e.Find(e.searchTerm, e.caseSensitive)

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// ReplaceAll replaces all matches with the given replacement text.
func (e *ModelEditor) ReplaceAll(replacement string) {
	if len(e.searchMatches) == 0 {
		return
	}

	// Save undo state
	e.pushUndoState()

	// Replace in reverse order to maintain position validity
	replacementRunes := []rune(replacement)
	searchLen := len([]rune(e.searchTerm))

	for i := len(e.searchMatches) - 1; i >= 0; i-- {
		matchPos := e.searchMatches[i]
		matchEnd := matchPos + searchLen

		e.TextContent = append(e.TextContent[:matchPos], append(replacementRunes, e.TextContent[matchEnd:]...)...)
	}

	// Update cursor position to the last replacement
	if len(e.searchMatches) > 0 {
		e.CursorPos = e.searchMatches[0] + len(replacementRunes)
	}

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()

	// Clear search
	e.ClearSearch()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}
