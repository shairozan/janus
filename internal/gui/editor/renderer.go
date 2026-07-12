package editor

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// modelRenderer implements fyne.WidgetRenderer for ModelEditor
//
// Responsibilities:
//   - Render visible text lines using virtualization
//   - Manage cursor position and blinking
//   - Recycle canvas primitives for performance
//   - Render tokens with syntax-specific colors
type modelRenderer struct {
	editor *ModelEditor

	// Canvas primitives (recycled for performance)
	LineObjects          []*canvas.Text      // Text primitives for visible lines (deprecated - using token rendering)
	TokenObjects         [][]*canvas.Text    // Text primitives per token per line (for syntax highlighting)
	LineNumbers          []*canvas.Text      // Line number text primitives
	GutterBackground     *canvas.Rectangle   // Background for line number gutter
	Cursor               *canvas.Rectangle   // Cursor indicator
	Background           *canvas.Rectangle   // Background
	SelectionRects       []*canvas.Rectangle // Selection highlight rectangles
	SearchHighlightRects []*canvas.Rectangle // Search match highlight rectangles
	MatchingBracketRect  *canvas.Rectangle   // Matching bracket highlight

	// Scroll offset from containing scroll container
	ScrollOffset fyne.Position

	// Visible line range (for virtualization)
	firstVisibleLine int
	lastVisibleLine  int

	// Cursor blinking state
	cursorVisible bool
	blinkTicker   *time.Ticker
	blinkStop     chan bool
}

// newModelRenderer creates a renderer for the editor.
func newModelRenderer(editor *ModelEditor) *modelRenderer {
	r := &modelRenderer{
		editor:        editor,
		LineObjects:   []*canvas.Text{},
		TokenObjects:  [][]*canvas.Text{},
		LineNumbers:   []*canvas.Text{},
		cursorVisible: true,
		blinkStop:     make(chan bool, 1),
	}

	// Create cursor primitive (make it more visible for debugging)
	r.Cursor = canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	r.Cursor.Resize(fyne.NewSize(3, 20)) // Slightly larger for visibility

	// Create background (use a distinct color to ensure widget is visible)
	r.Background = canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))

	// Create gutter background (slightly darker than main background)
	r.GutterBackground = canvas.NewRectangle(theme.Color(theme.ColorNameShadow))

	// Create matching bracket highlight (subtle outline)
	r.MatchingBracketRect = canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))

	// Start cursor blinking animation
	r.startCursorBlink()

	return r
}

// calculateGutterWidth calculates the width of the line number gutter based on line count.
func (r *modelRenderer) calculateGutterWidth() float32 {
	r.editor.linesMutex.RLock()
	lineCount := len(r.editor.Lines)
	r.editor.linesMutex.RUnlock()

	// Calculate number of digits needed
	digits := 1
	temp := lineCount
	for temp >= 10 {
		digits++
		temp /= 10
	}

	// Minimum 2 digits, add padding
	if digits < 2 {
		digits = 2
	}

	// Each digit is ~8.5px wide (monospace), plus padding on both sides
	charWidth := float32(8.5)
	padding := float32(10) // 5px padding on each side

	return float32(digits)*charWidth + padding
}

// Layout positions all visible canvas objects.
func (r *modelRenderer) Layout(size fyne.Size) {
	// Position background
	r.Background.Resize(size)

	// Calculate gutter width
	gutterWidth := r.calculateGutterWidth()

	// Position gutter background
	r.GutterBackground.Move(fyne.NewPos(0, 0))
	r.GutterBackground.Resize(fyne.NewSize(gutterWidth, size.Height))

	// Acquire read lock for thread-safe access to Lines during async lexing
	r.editor.linesMutex.RLock()
	defer r.editor.linesMutex.RUnlock()

	// Calculate visible line range based on scroll offset
	// For Phase 2, we render all lines (virtualization optimization in Phase 4)
	r.firstVisibleLine = 0
	r.lastVisibleLine = len(r.editor.Lines) - 1

	// Ensure we have token object arrays for all lines
	for len(r.TokenObjects) < len(r.editor.Lines) {
		r.TokenObjects = append(r.TokenObjects, []*canvas.Text{})
	}

	// Ensure we have line number primitives for all lines
	for len(r.LineNumbers) < len(r.editor.Lines) {
		lineNumText := canvas.NewText("", theme.Color(theme.ColorNameDisabled))
		lineNumText.TextSize = 14
		lineNumText.TextStyle = fyne.TextStyle{Monospace: true}
		r.LineNumbers = append(r.LineNumbers, lineNumText)
	}

	// Render visible lines using token-based rendering
	for i := r.firstVisibleLine; i <= r.lastVisibleLine && i < len(r.editor.Lines); i++ {
		lineData := r.editor.Lines[i]

		// Render line number in gutter
		if i < len(r.LineNumbers) {
			lineNum := fmt.Sprintf("%d", i+1) // Line numbers start at 1
			r.LineNumbers[i].Text = lineNum
			r.LineNumbers[i].Color = theme.Color(theme.ColorNameDisabled)

			// Right-align line numbers in gutter
			numWidth := float32(len(lineNum)) * float32(8.5)
			lineNumX := gutterWidth - numWidth - 5 // 5px padding from right edge of gutter
			lineNumY := lineData.YOffset
			r.LineNumbers[i].Move(fyne.NewPos(lineNumX, lineNumY))
			r.LineNumbers[i].Refresh()
		}

		// Calculate current X position for token placement (after gutter)
		xPos := gutterWidth + 5 // Gutter width + left margin
		yPos := lineData.YOffset

		// Ensure we have enough token primitives for this line
		tokensNeeded := len(lineData.Tokens)
		for len(r.TokenObjects[i]) < tokensNeeded {
			text := canvas.NewText("", theme.Color(theme.ColorNameForeground))
			text.TextSize = 14
			text.TextStyle = fyne.TextStyle{Monospace: true}
			r.TokenObjects[i] = append(r.TokenObjects[i], text)
		}

		// Render each token with appropriate color
		for j, token := range lineData.Tokens {
			if j >= len(r.TokenObjects[i]) {
				break
			}

			textObj := r.TokenObjects[i][j]
			textObj.Text = token.Value
			textObj.Color = colorForToken(token.Type)
			textObj.TextSize = 14
			textObj.TextStyle = fyne.TextStyle{Monospace: true}

			// Position token
			textObj.Move(fyne.NewPos(xPos, yPos))
			textObj.Refresh()

			// Advance X position (approximate monospace width)
			charWidth := float32(8.5)
			xPos += float32(len(token.Value)) * charWidth
		}

		// Hide any extra token primitives for this line
		for j := len(lineData.Tokens); j < len(r.TokenObjects[i]); j++ {
			r.TokenObjects[i][j].Text = ""
			r.TokenObjects[i][j].Refresh()
		}
	}

	// Position cursor, selection, and search highlights
	r.updateCursorPosition()
	r.updateMatchingBracketPosition()
	r.updateSelectionRects()
	r.updateSearchHighlightRects()
}

// MinSize calculates the minimum size needed for the entire document.
// This is critical for scroll container sizing.
func (r *modelRenderer) MinSize() fyne.Size {
	r.editor.linesMutex.RLock()
	defer r.editor.linesMutex.RUnlock()

	if len(r.editor.Lines) == 0 {
		return fyne.NewSize(400, 300) // Minimum sensible size
	}

	// Calculate total height
	lastLine := r.editor.Lines[len(r.editor.Lines)-1]
	totalHeight := lastLine.YOffset + lastLine.LineHeight

	// Calculate max width (for horizontal scrolling)
	// For Phase 1, use a reasonable default
	maxWidth := float32(800)

	return fyne.NewSize(maxWidth, totalHeight)
}

// Refresh updates all visual elements.
func (r *modelRenderer) Refresh() {
	// Update colors from theme
	r.Background.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.GutterBackground.FillColor = theme.Color(theme.ColorNameShadow)
	r.Cursor.FillColor = theme.Color(theme.ColorNamePrimary)

	// Acquire read lock for thread-safe access to Lines during async lexing
	r.editor.linesMutex.RLock()
	defer r.editor.linesMutex.RUnlock()

	// Update line number colors
	for i := 0; i < len(r.LineNumbers); i++ {
		r.LineNumbers[i].Color = theme.Color(theme.ColorNameDisabled)
	}

	// CRITICAL FIX: Update token text AND colors when content changes
	// This was the bug - we were only updating colors, not text values
	for lineIdx := 0; lineIdx < len(r.editor.Lines) && lineIdx < len(r.TokenObjects); lineIdx++ {
		lineData := r.editor.Lines[lineIdx]

		// Ensure we have enough token primitives for this line
		tokensNeeded := len(lineData.Tokens)
		for len(r.TokenObjects[lineIdx]) < tokensNeeded {
			text := canvas.NewText("", theme.Color(theme.ColorNameForeground))
			text.TextSize = 14
			text.TextStyle = fyne.TextStyle{Monospace: true}
			r.TokenObjects[lineIdx] = append(r.TokenObjects[lineIdx], text)
		}

		// Update token text content AND colors
		for tokenIdx, token := range lineData.Tokens {
			if tokenIdx >= len(r.TokenObjects[lineIdx]) {
				break
			}
			tokenObj := r.TokenObjects[lineIdx][tokenIdx]
			tokenObj.Text = token.Value // UPDATE TEXT CONTENT
			tokenObj.Color = colorForToken(token.Type)
			tokenObj.Refresh()
		}

		// Hide any extra token primitives for this line
		for tokenIdx := len(lineData.Tokens); tokenIdx < len(r.TokenObjects[lineIdx]); tokenIdx++ {
			r.TokenObjects[lineIdx][tokenIdx].Text = ""
			r.TokenObjects[lineIdx][tokenIdx].Refresh()
		}
	}

	// Ensure we have token arrays for all lines
	for len(r.TokenObjects) < len(r.editor.Lines) {
		r.TokenObjects = append(r.TokenObjects, []*canvas.Text{})
	}

	r.updateCursorPosition()
	r.updateMatchingBracketPosition()
	r.updateSelectionRects()
	r.updateSearchHighlightRects()
	r.Background.Refresh()
	r.Cursor.Refresh()
	canvas.Refresh(r.editor)
}

// updateSelectionRects creates rectangles to highlight selected text.
// NOTE: Caller must hold r.editor.linesMutex.RLock().
func (r *modelRenderer) updateSelectionRects() {
	// Clear existing selection rectangles
	r.SelectionRects = []*canvas.Rectangle{}

	if !r.editor.HasSelection() {
		return
	}

	start, end := r.editor.GetSelection()
	startLine, startCol := r.editor.GetLineCol(start)
	endLine, endCol := r.editor.GetLineCol(end)

	gutterWidth := r.calculateGutterWidth()
	charWidth := float32(8.5)
	lineHeight := float32(20)
	leftMargin := gutterWidth + 5 // Gutter width + text margin

	// Selection color (light blue with transparency)
	selectionColor := theme.Color(theme.ColorNameSelection)

	// Single-line selection
	if startLine == endLine {
		x := leftMargin + float32(startCol)*charWidth
		y := float32(startLine) * lineHeight
		width := float32(endCol-startCol) * charWidth
		height := lineHeight

		rect := canvas.NewRectangle(selectionColor)
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(width, height))
		r.SelectionRects = append(r.SelectionRects, rect)

		return
	}

	// Multi-line selection
	// First line (from startCol to end of line)
	if startLine < len(r.editor.Lines) {
		lineContent := r.editor.Lines[startLine].Content
		lineLen := len(lineContent)
		if lineLen > 0 && lineContent[lineLen-1] == '\n' {
			lineLen--
		}

		x := leftMargin + float32(startCol)*charWidth
		y := float32(startLine) * lineHeight
		width := float32(lineLen-startCol) * charWidth
		height := lineHeight

		rect := canvas.NewRectangle(selectionColor)
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(width, height))
		r.SelectionRects = append(r.SelectionRects, rect)
	}

	// Middle lines (entire line width)
	for line := startLine + 1; line < endLine && line < len(r.editor.Lines); line++ {
		lineContent := r.editor.Lines[line].Content
		lineLen := len(lineContent)
		if lineLen > 0 && lineContent[lineLen-1] == '\n' {
			lineLen--
		}

		x := leftMargin
		y := float32(line) * lineHeight
		width := float32(lineLen) * charWidth
		height := lineHeight

		rect := canvas.NewRectangle(selectionColor)
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(width, height))
		r.SelectionRects = append(r.SelectionRects, rect)
	}

	// Last line (from start to endCol)
	if endLine < len(r.editor.Lines) {
		x := leftMargin
		y := float32(endLine) * lineHeight
		width := float32(endCol) * charWidth
		height := lineHeight

		rect := canvas.NewRectangle(selectionColor)
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(width, height))
		r.SelectionRects = append(r.SelectionRects, rect)
	}
}

// updateSearchHighlightRects creates rectangles to highlight all search matches.
// NOTE: Caller must hold r.editor.linesMutex.RLock().
func (r *modelRenderer) updateSearchHighlightRects() {
	// Clear existing search highlight rectangles
	r.SearchHighlightRects = []*canvas.Rectangle{}

	if r.editor.searchTerm == "" || len(r.editor.searchMatches) == 0 || !r.editor.searchHighlight {
		return
	}

	gutterWidth := r.calculateGutterWidth()
	charWidth := float32(8.5)
	lineHeight := float32(20)
	leftMargin := gutterWidth + 5 // Gutter width + text margin
	searchLen := len([]rune(r.editor.searchTerm))

	// Use different colors for current match vs other matches
	currentMatchColor := theme.Color(theme.ColorNamePrimary) // Bright color for current match
	otherMatchColor := theme.Color(theme.ColorNameFocus)     // Dimmer color for other matches

	// Create highlight rectangles for each match
	for i, matchPos := range r.editor.searchMatches {
		matchEnd := matchPos + searchLen

		// Determine color (current match gets brighter highlight)
		highlightColor := otherMatchColor
		if i == r.editor.currentMatch {
			highlightColor := currentMatchColor
			// Make the current match color semi-transparent
			_ = highlightColor // Use the color
		}

		// Get line and column positions
		startLine, startCol := r.editor.GetLineCol(matchPos)
		endLine, endCol := r.editor.GetLineCol(matchEnd)

		// Single-line match (most common case)
		if startLine == endLine {
			x := leftMargin + float32(startCol)*charWidth
			y := float32(startLine) * lineHeight
			width := float32(endCol-startCol) * charWidth
			height := lineHeight

			rect := canvas.NewRectangle(highlightColor)
			rect.Move(fyne.NewPos(x, y))
			rect.Resize(fyne.NewSize(width, height))
			r.SearchHighlightRects = append(r.SearchHighlightRects, rect)

			continue
		}

		// Multi-line match (rare, but possible with regex or newline searches)
		// First line (from startCol to end of line)
		if startLine < len(r.editor.Lines) {
			lineContent := r.editor.Lines[startLine].Content
			lineLen := len(lineContent)
			if lineLen > 0 && lineContent[lineLen-1] == '\n' {
				lineLen--
			}

			x := leftMargin + float32(startCol)*charWidth
			y := float32(startLine) * lineHeight
			width := float32(lineLen-startCol) * charWidth
			height := lineHeight

			rect := canvas.NewRectangle(highlightColor)
			rect.Move(fyne.NewPos(x, y))
			rect.Resize(fyne.NewSize(width, height))
			r.SearchHighlightRects = append(r.SearchHighlightRects, rect)
		}

		// Middle lines (entire line width)
		for line := startLine + 1; line < endLine && line < len(r.editor.Lines); line++ {
			lineContent := r.editor.Lines[line].Content
			lineLen := len(lineContent)
			if lineLen > 0 && lineContent[lineLen-1] == '\n' {
				lineLen--
			}

			x := leftMargin
			y := float32(line) * lineHeight
			width := float32(lineLen) * charWidth
			height := lineHeight

			rect := canvas.NewRectangle(highlightColor)
			rect.Move(fyne.NewPos(x, y))
			rect.Resize(fyne.NewSize(width, height))
			r.SearchHighlightRects = append(r.SearchHighlightRects, rect)
		}

		// Last line (from start to endCol)
		if endLine < len(r.editor.Lines) {
			x := leftMargin
			y := float32(endLine) * lineHeight
			width := float32(endCol) * charWidth
			height := lineHeight

			rect := canvas.NewRectangle(highlightColor)
			rect.Move(fyne.NewPos(x, y))
			rect.Resize(fyne.NewSize(width, height))
			r.SearchHighlightRects = append(r.SearchHighlightRects, rect)
		}
	}
}

// Objects returns all canvas primitives to render.
func (r *modelRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{r.Background}

	// Add gutter background (above main background, behind gutter text)
	objects = append(objects, r.GutterBackground)

	// Add search highlights (behind selection and text)
	for _, rect := range r.SearchHighlightRects {
		objects = append(objects, rect)
	}

	// Add selection highlights (behind text, but above search highlights)
	for _, rect := range r.SelectionRects {
		objects = append(objects, rect)
	}

	// Add matching bracket highlight (if present)
	if r.editor.matchingBracketPos != -1 {
		objects = append(objects, r.MatchingBracketRect)
	}

	// Add line numbers (in gutter, before main text)
	for i := r.firstVisibleLine; i <= r.lastVisibleLine && i < len(r.LineNumbers); i++ {
		if r.LineNumbers[i].Text != "" {
			objects = append(objects, r.LineNumbers[i])
		}
	}

	// Add visible token objects (for syntax highlighting)
	for i := r.firstVisibleLine; i <= r.lastVisibleLine && i < len(r.TokenObjects); i++ {
		for _, tokenObj := range r.TokenObjects[i] {
			if tokenObj.Text != "" {
				objects = append(objects, tokenObj)
			}
		}
	}

	// Only show cursor when visible (for blinking animation)
	if r.cursorVisible {
		objects = append(objects, r.Cursor)
	}

	return objects
}

// Destroy cleans up renderer resources.
func (r *modelRenderer) Destroy() {
	// Stop cursor blink ticker to prevent goroutine leak
	if r.blinkTicker != nil {
		r.blinkStop <- true
		r.blinkTicker.Stop()
	}

	// Canvas primitives are garbage collected
}

// updateCursorPosition calculates and sets cursor visual position.
func (r *modelRenderer) updateCursorPosition() {
	line, col := r.editor.GetLineCol(r.editor.CursorPos)

	// Calculate Y position from line offset
	var yPos float32
	if line < len(r.editor.Lines) {
		yPos = r.editor.Lines[line].YOffset
	}

	// Calculate X position from column (fixed-width font), accounting for gutter
	gutterWidth := r.calculateGutterWidth()
	charWidth := float32(8.5) // Approximate monospace character width
	xPos := gutterWidth + 5 + float32(col)*charWidth

	r.Cursor.Move(fyne.NewPos(xPos, yPos))
	r.Cursor.Refresh()
}

// updateMatchingBracketPosition calculates and sets matching bracket highlight position.
func (r *modelRenderer) updateMatchingBracketPosition() {
	if r.editor.matchingBracketPos == -1 {
		return
	}

	line, col := r.editor.GetLineCol(r.editor.matchingBracketPos)

	// Calculate Y position from line offset
	var yPos float32
	if line < len(r.editor.Lines) {
		yPos = r.editor.Lines[line].YOffset
	}

	// Calculate X position from column (fixed-width font), accounting for gutter
	gutterWidth := r.calculateGutterWidth()
	charWidth := float32(8.5) // Approximate monospace character width
	lineHeight := float32(20)
	xPos := gutterWidth + 5 + float32(col)*charWidth

	// Position and size the matching bracket highlight
	r.MatchingBracketRect.Move(fyne.NewPos(xPos, yPos))
	r.MatchingBracketRect.Resize(fyne.NewSize(charWidth, lineHeight))

	// Use a subtle outline effect (semi-transparent primary color)
	primaryColor := theme.Color(theme.ColorNamePrimary)
	r.MatchingBracketRect.FillColor = primaryColor
	r.MatchingBracketRect.StrokeColor = primaryColor
	r.MatchingBracketRect.StrokeWidth = 2

	r.MatchingBracketRect.Refresh()
}

// startCursorBlink starts the cursor blinking animation.
func (r *modelRenderer) startCursorBlink() {
	r.blinkTicker = time.NewTicker(500 * time.Millisecond)

	go func() {
		for {
			select {
			case <-r.blinkTicker.C:
				// Toggle cursor visibility
				r.cursorVisible = !r.cursorVisible

				// Refresh widget to update cursor visibility on main thread
				fyne.Do(func() {
					canvas.Refresh(r.editor)
				})

			case <-r.blinkStop:
				return
			}
		}
	}()
}

// stopCursorBlink stops the cursor blinking (when editor loses focus).
func (r *modelRenderer) stopCursorBlink() {
	r.cursorVisible = false
	canvas.Refresh(r.editor)
}

// resetCursorBlink resets cursor to visible state (when editor gains focus or user types).
func (r *modelRenderer) resetCursorBlink() {
	r.cursorVisible = true
	canvas.Refresh(r.editor)
}

// updateSelectionOnly updates only the selection rectangles and cursor position.
// This is a lightweight refresh used during drag operations for better performance.
func (r *modelRenderer) updateSelectionOnly() {
	r.editor.linesMutex.RLock()
	defer r.editor.linesMutex.RUnlock()

	r.updateCursorPosition()
	r.updateSelectionRects()
	canvas.Refresh(r.editor)
}
