package editor

import (
	"sort"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// ModelEditor is a custom Fyne widget providing syntax-highlighted editing
// for NONMEM control stream files.
//
// Architecture follows the Fyne custom widget pattern:
//   - ModelEditor manages state (text content, cursor position, tokens)
//   - modelRenderer manages visual representation (canvas primitives)
//
// This separation ensures clean testing and follows idiomatic Fyne patterns.
type ModelEditor struct {
	widget.BaseWidget

	// Text content stored as rune slice for efficient editing
	// (O(1) insertion/deletion vs O(n) string manipulation)
	TextContent []rune

	// Pre-computed line data for efficient rendering
	// Each LineData contains tokens and vertical offset for virtualization
	Lines []LineData

	// Cursor position as linear index into TextContent
	// Renderer maps this to 2D (line, col) coordinates
	CursorPos int

	// Line break index for fast cursor position mapping
	// Stores TextContent indices where \n occurs
	lineBreaks []int

	// Viewport size for virtualization calculations
	Viewport fyne.Size

	// Renderer reference for cursor blink control
	renderer *modelRenderer

	// Mutex to protect Lines data during async lexing
	linesMutex sync.RWMutex

	// Async lexing control
	lexVersion int  // Incremented on each text change to cancel stale lexing
	lexPending bool // True when async lex is in progress

	// Undo/redo stacks
	undoStack        []editorState
	redoStack        []editorState
	maxUndoStackSize int

	// Text selection
	selectionStart int  // Start position of selection (-1 if no selection)
	selectionEnd   int  // End position of selection
	selecting      bool // True when actively selecting with mouse

	// Search state
	searchTerm      string // Current search term
	searchMatches   []int  // Positions of all matches (starting positions)
	currentMatch    int    // Index of current match in searchMatches (-1 if none)
	caseSensitive   bool   // Case-sensitive search flag
	searchHighlight bool   // Whether to highlight all matches

	// Window reference for showing dialogs
	window fyne.Window

	// Dirty state tracking (for unsaved changes indicator)
	savedContent []rune // Content as it was when last saved/loaded
	isDirty      bool   // True when current content differs from saved content

	// Bracket matching
	matchingBracketPos int // Position of matching bracket (-1 if none)

	// Widget state
	disabled bool // True when editor is disabled (read-only)

	// Callbacks
	OnChanged func(string) // Called when text content changes
	OnSave    func()       // Called when user presses Ctrl+S/Cmd+S

	// Pluggable lexer for multi-modal support
	// If nil, uses default NONMEM lexer (LexLine function)
	lexer Lexer
}

// editorState captures the editor state for undo/redo.
type editorState struct {
	content   []rune
	cursorPos int
}

// LineData stores pre-computed data for a single line of text.
type LineData struct {
	// Line content as runes
	Content []rune

	// Lexed tokens for this line (for syntax highlighting)
	Tokens []Token

	// Vertical offset from top of document (for virtualization)
	YOffset float32

	// Height of this line in pixels
	LineHeight float32
}

// Token represents a lexed segment with syntax classification.
type Token struct {
	Type     TokenType
	Value    string
	StartPos int // Position within line
	EndPos   int
}

// TokenType classifies tokens for syntax highlighting.
type TokenType int

const (
	TokenDirective      TokenType = iota // $PROBLEM, $DATA, $ESTIMATION
	TokenSectionKeyword                  // $PK, $ERROR, $OMEGA, $SIGMA
	TokenDataKeyword                     // ID, TIME, DV, AMT, EVID
	TokenModelKeyword                    // ADVAN1, FOCE, PREDPP
	TokenParameterID                     // THETA, ETA, EPSILON, CL, V
	TokenComment                         // ; comment text
	TokenLiteralNumber                   // 0.1, 1E-3
	TokenIdentifier                      // Generic identifiers
	TokenWhitespace                      // Spaces, tabs
	TokenError                           // Lexing errors
)

// NewModelEditor creates a new NONMEM editor widget.
func NewModelEditor() *ModelEditor {
	e := &ModelEditor{
		TextContent:        []rune{},
		Lines:              []LineData{},
		CursorPos:          0,
		lineBreaks:         []int{},
		undoStack:          []editorState{},
		redoStack:          []editorState{},
		maxUndoStackSize:   100, // Limit undo history to 100 states
		selectionStart:     -1,  // No selection initially
		selectionEnd:       -1,
		selecting:          false,
		searchTerm:         "",
		searchMatches:      []int{},
		currentMatch:       -1,
		caseSensitive:      false, // Default to case-insensitive search
		searchHighlight:    true,  // Default to highlighting all matches
		matchingBracketPos: -1,    // No matching bracket initially
	}

	e.ExtendBaseWidget(e)

	return e
}

// CreateRenderer creates the visual representation of the editor.
// This is called by Fyne's rendering system.
func (e *ModelEditor) CreateRenderer() fyne.WidgetRenderer {
	e.renderer = newModelRenderer(e)

	return e.renderer
}

// getRenderer safely retrieves the renderer as *modelRenderer.
func (e *ModelEditor) getRenderer() *modelRenderer {
	return e.renderer
}

// SetWindow sets the window reference for showing dialogs.
func (e *ModelEditor) SetWindow(window fyne.Window) {
	e.window = window
}

// SetLexer sets the lexer to use for syntax highlighting.
// If set to nil, the editor will use the default NONMEM lexer.
// The lexer is used when re-lexing content, so call Refresh() after
// changing the lexer if the editor already has content.
func (e *ModelEditor) SetLexer(lexer Lexer) {
	e.lexer = lexer
	// Re-lex content with new lexer
	if len(e.TextContent) > 0 {
		e.lexContent()
		e.Refresh()
	}
}

// GetLexer returns the current lexer, or nil if using default NONMEM lexer.
func (e *ModelEditor) GetLexer() Lexer {
	return e.lexer
}

// lexLine tokenizes a single line using the configured lexer or default.
func (e *ModelEditor) lexLine(line []rune) []Token {
	if e.lexer != nil {
		return e.lexer.LexLine(line)
	}
	// Default to built-in NONMEM lexer
	return LexLine(line)
}

// SetText updates the editor content.
func (e *ModelEditor) SetText(text string) {
	e.TextContent = []rune(text)
	e.CursorPos = 0

	// Initialize saved content to match current content (used for dirty tracking)
	// When a file is loaded, it starts in a "clean" state
	e.savedContent = make([]rune, len(e.TextContent))
	copy(e.savedContent, e.TextContent)
	e.isDirty = false

	e.updateLineBreaks()
	e.lexContent()
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(text)
	}
}

// GetText returns the current editor content as a string.
func (e *ModelEditor) GetText() string {
	return string(e.TextContent)
}

// Enable makes the editor editable.
func (e *ModelEditor) Enable() {
	e.disabled = false
	e.Refresh()
}

// Disable makes the editor read-only.
func (e *ModelEditor) Disable() {
	e.disabled = true
	e.Refresh()
}

// Disabled returns true if the editor is disabled (read-only).
func (e *ModelEditor) Disabled() bool {
	return e.disabled
}

// IsDirty returns true if the editor has unsaved changes.
func (e *ModelEditor) IsDirty() bool {
	return e.isDirty
}

// ClearDirtyState marks the current content as saved.
// This should be called after a successful file save.
func (e *ModelEditor) ClearDirtyState() {
	e.savedContent = make([]rune, len(e.TextContent))
	copy(e.savedContent, e.TextContent)
	e.isDirty = false

	// Trigger OnChanged to update UI (save button state, etc.)
	if e.OnChanged != nil {
		e.OnChanged(string(e.TextContent))
	}
}

// checkDirtyState compares current content to saved content.
// This should be called after any text modification.
func (e *ModelEditor) checkDirtyState() {
	// Compare current content to saved content
	if len(e.TextContent) != len(e.savedContent) {
		e.isDirty = true

		return
	}

	for i, r := range e.TextContent {
		if r != e.savedContent[i] {
			e.isDirty = true

			return
		}
	}

	e.isDirty = false
}

// updateLineBreaks rebuilds the line break index.
// This enables fast cursor position to (line, col) mapping.
func (e *ModelEditor) updateLineBreaks() {
	e.lineBreaks = []int{}
	for i, r := range e.TextContent {
		if r == '\n' {
			e.lineBreaks = append(e.lineBreaks, i)
		}
	}
}

// GetLineCol converts linear cursor position to (line, col) coordinates.
func (e *ModelEditor) GetLineCol(pos int) (line, col int) {
	if pos < 0 || pos > len(e.TextContent) {
		return 0, 0
	}

	// Binary search for line number
	line = sort.SearchInts(e.lineBreaks, pos)

	// Calculate column offset
	if line == 0 {
		col = pos
	} else {
		col = pos - e.lineBreaks[line-1] - 1
	}

	return line, col
}

// GetPosFromLineCol converts (line, col) to linear position.
func (e *ModelEditor) GetPosFromLineCol(line, col int) int {
	if line < 0 {
		return 0
	}

	var lineStart int

	switch {
	case line == 0:
		lineStart = 0
	case line <= len(e.lineBreaks):
		lineStart = e.lineBreaks[line-1] + 1
	default:
		return len(e.TextContent)
	}

	pos := lineStart + col
	if pos > len(e.TextContent) {
		return len(e.TextContent)
	}

	return pos
}

// lexContent performs lexical analysis on the current content.
// Uses functional state machine lexer to tokenize each line.
// For large files (>1000 lines), uses async lexing to avoid blocking UI.
func (e *ModelEditor) lexContent() {
	// Count approximate lines for async decision
	lineCount := 0
	for _, r := range e.TextContent {
		if r == '\n' {
			lineCount++
		}
	}

	// Use async lexing for large files (>1000 lines)
	if lineCount > 1000 && !e.lexPending {
		e.lexContentAsync()
	} else {
		// Synchronous lexing for small files
		e.lexContentSync()
	}
}

// lexContentSync performs synchronous lexical analysis (used for small files).
func (e *ModelEditor) lexContentSync() {
	e.linesMutex.Lock()
	defer e.linesMutex.Unlock()

	// Split content into lines
	e.Lines = []LineData{}

	var currentLine []rune

	for i, r := range e.TextContent {
		currentLine = append(currentLine, r)

		if r == '\n' || i == len(e.TextContent)-1 {
			// Lex the line to generate tokens
			tokens := e.lexLine(currentLine)

			// Create line data with tokens
			lineData := LineData{
				Content:    currentLine,
				Tokens:     tokens,
				YOffset:    float32(len(e.Lines)) * 20,
				LineHeight: 20,
			}
			e.Lines = append(e.Lines, lineData)

			// Reset for next line
			currentLine = []rune{}
		}
	}

	// Handle empty document or document without trailing newline
	if len(e.Lines) == 0 {
		e.Lines = append(e.Lines, LineData{
			Content:    []rune{},
			Tokens:     []Token{},
			YOffset:    0,
			LineHeight: 20,
		})
	}
}

// lexContentAsync performs asynchronous lexical analysis (used for large files).
func (e *ModelEditor) lexContentAsync() {
	// Increment version to cancel any pending lexing operations
	e.lexVersion++
	currentVersion := e.lexVersion
	e.lexPending = true

	// Capture current content for lexing (copy to avoid race conditions)
	contentCopy := make([]rune, len(e.TextContent))
	copy(contentCopy, e.TextContent)

	// Lex in background goroutine
	go func() {
		// Split content into lines
		var newLines []LineData
		var currentLine []rune

		for i, r := range contentCopy {
			currentLine = append(currentLine, r)

			if r == '\n' || i == len(contentCopy)-1 {
				// Lex the line to generate tokens
				tokens := e.lexLine(currentLine)

				// Create line data with tokens
				lineData := LineData{
					Content:    currentLine,
					Tokens:     tokens,
					YOffset:    float32(len(newLines)) * 20,
					LineHeight: 20,
				}
				newLines = append(newLines, lineData)

				// Reset for next line
				currentLine = []rune{}
			}

			// Check if lexing was cancelled by newer edit
			if e.lexVersion != currentVersion {
				// Stale lexing - abort
				e.lexPending = false

				return
			}
		}

		// Handle empty document
		if len(newLines) == 0 {
			newLines = append(newLines, LineData{
				Content:    []rune{},
				Tokens:     []Token{},
				YOffset:    0,
				LineHeight: 20,
			})
		}

		// Update Lines atomically
		e.linesMutex.Lock()
		e.Lines = newLines
		e.linesMutex.Unlock()

		e.lexPending = false

		// Refresh UI on main thread
		canvas.Refresh(e)
	}()
}

// pushUndoState saves the current editor state to the undo stack.
func (e *ModelEditor) pushUndoState() {
	// Create a copy of current state
	contentCopy := make([]rune, len(e.TextContent))
	copy(contentCopy, e.TextContent)

	state := editorState{
		content:   contentCopy,
		cursorPos: e.CursorPos,
	}

	// Add to undo stack
	e.undoStack = append(e.undoStack, state)

	// Limit stack size
	if len(e.undoStack) > e.maxUndoStackSize {
		e.undoStack = e.undoStack[1:]
	}

	// Clear redo stack when new change is made
	e.redoStack = []editorState{}
}

// Undo reverts to the previous editor state.
func (e *ModelEditor) Undo() {
	if len(e.undoStack) == 0 {
		return
	}

	// Save current state to redo stack
	currentState := editorState{
		content:   make([]rune, len(e.TextContent)),
		cursorPos: e.CursorPos,
	}
	copy(currentState.content, e.TextContent)
	e.redoStack = append(e.redoStack, currentState)

	// Restore previous state
	prevState := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]

	e.TextContent = prevState.content
	e.CursorPos = prevState.cursorPos

	// Update UI
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after undo
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// Redo restores a previously undone state.
func (e *ModelEditor) Redo() {
	if len(e.redoStack) == 0 {
		return
	}

	// Save current state to undo stack
	currentState := editorState{
		content:   make([]rune, len(e.TextContent)),
		cursorPos: e.CursorPos,
	}
	copy(currentState.content, e.TextContent)
	e.undoStack = append(e.undoStack, currentState)

	// Restore next state
	nextState := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]

	e.TextContent = nextState.content
	e.CursorPos = nextState.cursorPos

	// Update UI
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after redo
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// HasSelection returns true if there is an active text selection.
func (e *ModelEditor) HasSelection() bool {
	return e.selectionStart != -1 && e.selectionEnd != -1 && e.selectionStart != e.selectionEnd
}

// GetSelection returns the normalized selection range (start <= end).
func (e *ModelEditor) GetSelection() (start, end int) {
	if !e.HasSelection() {
		return -1, -1
	}

	if e.selectionStart < e.selectionEnd {
		return e.selectionStart, e.selectionEnd
	}

	return e.selectionEnd, e.selectionStart
}

// GetSelectedText returns the currently selected text as a string.
func (e *ModelEditor) GetSelectedText() string {
	if !e.HasSelection() {
		return ""
	}

	start, end := e.GetSelection()

	return string(e.TextContent[start:end])
}

// ClearSelection clears the current selection and refreshes the UI.
func (e *ModelEditor) ClearSelection() {
	e.selectionStart = -1
	e.selectionEnd = -1
	e.selecting = false
	e.Refresh()
}

// SetSelection sets the selection range and refreshes the UI.
func (e *ModelEditor) SetSelection(start, end int) {
	// Clamp to valid range
	if start < 0 {
		start = 0
	}
	if end > len(e.TextContent) {
		end = len(e.TextContent)
	}

	e.selectionStart = start
	e.selectionEnd = end
	e.selecting = false
	e.Refresh()
}

// DeleteSelection deletes the currently selected text (used when typing over selection).
func (e *ModelEditor) DeleteSelection() {
	if !e.HasSelection() {
		return
	}

	start, end := e.GetSelection()

	// Save undo state before modification
	e.pushUndoState()

	// Delete selected text
	e.TextContent = append(e.TextContent[:start], e.TextContent[end:]...)

	// Position cursor at start of deleted selection
	e.CursorPos = start

	// Clear selection
	e.selectionStart = -1
	e.selectionEnd = -1

	// Update UI
	e.updateLineBreaks()
	e.lexContent()
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// updateBracketMatching finds and highlights the matching bracket/parenthesis
// for the character at or before the cursor position.
func (e *ModelEditor) updateBracketMatching() {
	// Reset matching bracket position
	e.matchingBracketPos = -1

	if e.CursorPos < 0 || e.CursorPos > len(e.TextContent) {
		return
	}

	// Check character at cursor position
	var targetChar rune
	var searchPos int

	switch {
	case e.CursorPos < len(e.TextContent):
		targetChar = e.TextContent[e.CursorPos]
		searchPos = e.CursorPos
	case e.CursorPos > 0:
		// Check character before cursor if at end
		targetChar = e.TextContent[e.CursorPos-1]
		searchPos = e.CursorPos - 1
	default:
		return
	}

	// Determine bracket type and search direction
	var matchChar rune
	var searchForward bool

	switch targetChar {
	case '(':
		matchChar = ')'
		searchForward = true
	case ')':
		matchChar = '('
		searchForward = false
	case '[':
		matchChar = ']'
		searchForward = true
	case ']':
		matchChar = '['
		searchForward = false
	case '{':
		matchChar = '}'
		searchForward = true
	case '}':
		matchChar = '{'
		searchForward = false
	default:
		// Not a bracket character, try character before cursor
		if searchPos > 0 {
			targetChar = e.TextContent[searchPos-1]
			searchPos--

			switch targetChar {
			case '(':
				matchChar = ')'
				searchForward = true
			case ')':
				matchChar = '('
				searchForward = false
			case '[':
				matchChar = ']'
				searchForward = true
			case ']':
				matchChar = '['
				searchForward = false
			case '{':
				matchChar = '}'
				searchForward = true
			case '}':
				matchChar = '{'
				searchForward = false
			default:
				return
			}
		} else {
			return
		}
	}

	// Search for matching bracket using stack-based algorithm
	matchPos := e.findMatchingBracket(searchPos, targetChar, matchChar, searchForward)
	if matchPos != -1 {
		e.matchingBracketPos = matchPos
	}
}

// findMatchingBracket searches for the matching bracket using a stack-based algorithm.
// currentChar is the bracket at startPos, matchChar is what we're looking for.
func (e *ModelEditor) findMatchingBracket(startPos int, currentChar, matchChar rune, searchForward bool) int {
	depth := 1
	step := 1
	endPos := len(e.TextContent)

	if !searchForward {
		step = -1
		endPos = -1
	}

	for pos := startPos + step; pos != endPos && pos >= 0 && pos < len(e.TextContent); pos += step {
		ch := e.TextContent[pos]

		switch ch {
		case currentChar:
			// If we find another bracket of the same type as where we started, increment depth
			depth++
		case matchChar:
			// If we find the matching bracket type, decrement depth
			depth--
			if depth == 0 {
				return pos
			}
		}
	}

	return -1
}
