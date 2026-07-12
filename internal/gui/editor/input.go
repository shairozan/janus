package editor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Ensure ModelEditor implements fyne.Focusable for keyboard input.
var _ fyne.Focusable = (*ModelEditor)(nil)

// Ensure ModelEditor implements fyne.Tappable for mouse click focus.
var _ fyne.Tappable = (*ModelEditor)(nil)

// Ensure ModelEditor implements desktop.Hoverable for mouse interaction.
var _ desktop.Hoverable = (*ModelEditor)(nil)

// Ensure ModelEditor implements fyne.Draggable for text selection.
var _ fyne.Draggable = (*ModelEditor)(nil)

// FocusGained is called when the editor receives focus.
func (e *ModelEditor) FocusGained() {
	// Show cursor and ensure it's visible
	if r := e.getRenderer(); r != nil {
		r.resetCursorBlink()
	}

	e.Refresh()
}

// FocusLost is called when the editor loses focus.
func (e *ModelEditor) FocusLost() {
	// Hide cursor when not focused
	if r := e.getRenderer(); r != nil {
		r.stopCursorBlink()
	}

	e.Refresh()
}

// MouseIn is called when the mouse enters the widget area.
func (e *ModelEditor) MouseIn(*desktop.MouseEvent) {
	// Mouse entered widget area
}

// MouseOut is called when the mouse leaves the widget area.
func (e *ModelEditor) MouseOut() {
	// Mouse left widget area
}

// MouseMoved is called when the mouse moves within the widget.
func (e *ModelEditor) MouseMoved(*desktop.MouseEvent) {
	// Mouse moved within widget (not logging to avoid spam)
}

// Tapped handles mouse click events to gain focus and position cursor.
func (e *ModelEditor) Tapped(ev *fyne.PointEvent) {
	// Request focus when tapped
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}

	// Clear any existing selection (unless shift-clicking for extend selection in future)
	e.ClearSelection()

	// Position cursor at click location
	e.positionCursorAtPoint(ev.Position)
}

// TypedRune handles character input.
func (e *ModelEditor) TypedRune(r rune) {
	// Ignore input when disabled
	if e.disabled {
		return
	}
	// Insert rune at cursor position
	e.insertRune(r)
}

// TypedKey handles structural keyboard events (arrows, enter, backspace).
func (e *ModelEditor) TypedKey(key *fyne.KeyEvent) {
	// Handle regular keys (non-shift versions)
	// Shift+Arrow is handled in TypedShortcut via CustomShortcut
	//exhaustive:ignore
	switch key.Name {
	case fyne.KeyBackspace:
		if e.disabled {
			return
		}
		e.handleBackspace()
	case fyne.KeyDelete:
		if e.disabled {
			return
		}
		e.handleDelete()
	case fyne.KeyReturn, fyne.KeyEnter:
		if e.disabled {
			return
		}
		e.handleEnter()
	case fyne.KeyLeft:
		e.moveCursorLeft()
	case fyne.KeyRight:
		e.moveCursorRight()
	case fyne.KeyUp:
		e.moveCursorUp()
	case fyne.KeyDown:
		e.moveCursorDown()
	case fyne.KeyHome:
		e.moveCursorHome()
	case fyne.KeyEnd:
		e.moveCursorEnd()
	default:
	}
}

// insertRune inserts a rune at the current cursor position.
func (e *ModelEditor) insertRune(r rune) {
	// Delete any selected text first (typing replaces selection)
	if e.HasSelection() {
		e.DeleteSelection()
	}

	// Save undo state before modification
	e.pushUndoState()

	// Insert rune into TextContent slice
	e.TextContent = append(e.TextContent[:e.CursorPos], append([]rune{r}, e.TextContent[e.CursorPos:]...)...)
	e.CursorPos++

	// Reset cursor blink to make it visible immediately after typing
	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after modification
	e.updateBracketMatching()
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// handleBackspace deletes the character before the cursor.
func (e *ModelEditor) handleBackspace() {
	// If there's a selection, delete it instead of backspacing
	if e.HasSelection() {
		e.DeleteSelection()

		return
	}

	if e.CursorPos == 0 {
		return
	}

	// Save undo state before modification
	e.pushUndoState()

	// Remove character before cursor
	e.TextContent = append(e.TextContent[:e.CursorPos-1], e.TextContent[e.CursorPos:]...)
	e.CursorPos--

	// Reset cursor blink to make it visible immediately after deletion
	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after modification
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// handleDelete deletes the character at the cursor.
func (e *ModelEditor) handleDelete() {
	// If there's a selection, delete it instead
	if e.HasSelection() {
		e.DeleteSelection()

		return
	}

	if e.CursorPos >= len(e.TextContent) {
		return
	}

	// Save undo state before modification
	e.pushUndoState()

	// Remove character at cursor
	e.TextContent = append(e.TextContent[:e.CursorPos], e.TextContent[e.CursorPos+1:]...)

	// Reset cursor blink to make it visible immediately after deletion
	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after modification
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// handleEnter inserts a newline at the cursor position.
func (e *ModelEditor) handleEnter() {
	e.insertRune('\n')
}

// moveCursorLeft moves the cursor one position to the left.
func (e *ModelEditor) moveCursorLeft() {
	// Clear selection when moving without shift
	e.ClearSelection()

	if e.CursorPos > 0 {
		e.CursorPos--

		if renderer := e.getRenderer(); renderer != nil {
			renderer.resetCursorBlink()
		}

		e.updateBracketMatching()
		e.Refresh()
	}
}

// moveCursorRight moves the cursor one position to the right.
func (e *ModelEditor) moveCursorRight() {
	// Clear selection when moving without shift
	e.ClearSelection()

	if e.CursorPos < len(e.TextContent) {
		e.CursorPos++

		if renderer := e.getRenderer(); renderer != nil {
			renderer.resetCursorBlink()
		}

		e.updateBracketMatching()
		e.Refresh()
	}
}

// moveCursorUp moves the cursor up one line.
func (e *ModelEditor) moveCursorUp() {
	// Clear selection when moving without shift
	e.ClearSelection()

	line, col := e.GetLineCol(e.CursorPos)
	if line > 0 {
		e.CursorPos = e.GetPosFromLineCol(line-1, col)

		if renderer := e.getRenderer(); renderer != nil {
			renderer.resetCursorBlink()
		}

		e.updateBracketMatching()
		e.Refresh()
	}
}

// moveCursorDown moves the cursor down one line.
func (e *ModelEditor) moveCursorDown() {
	// Clear selection when moving without shift
	e.ClearSelection()

	line, col := e.GetLineCol(e.CursorPos)
	if line < len(e.lineBreaks) {
		e.CursorPos = e.GetPosFromLineCol(line+1, col)

		if renderer := e.getRenderer(); renderer != nil {
			renderer.resetCursorBlink()
		}

		e.updateBracketMatching()
		e.Refresh()
	}
}

// moveCursorHome moves the cursor to the start of the current line.
func (e *ModelEditor) moveCursorHome() {
	// Clear selection when moving without shift
	e.ClearSelection()

	line, _ := e.GetLineCol(e.CursorPos)
	e.CursorPos = e.GetPosFromLineCol(line, 0)

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.updateBracketMatching()
	e.Refresh()
}

// moveCursorEnd moves the cursor to the end of the current line.
func (e *ModelEditor) moveCursorEnd() {
	// Clear selection when moving without shift
	e.ClearSelection()

	line, _ := e.GetLineCol(e.CursorPos)

	// Find end of line
	var lineEnd int
	if line < len(e.lineBreaks) {
		lineEnd = e.lineBreaks[line]
	} else {
		lineEnd = len(e.TextContent)
	}

	e.CursorPos = lineEnd

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.updateBracketMatching()
	e.Refresh()
}

// Ensure ModelEditor implements desktop.Keyable for advanced keyboard handling.
var _ desktop.Keyable = (*ModelEditor)(nil)

// Ensure ModelEditor implements fyne.Shortcutable for keyboard shortcuts.
var _ fyne.Shortcutable = (*ModelEditor)(nil)

// KeyDown handles key down events for keyboard shortcuts.
func (e *ModelEditor) KeyDown(key *fyne.KeyEvent) {
	// Reserved for future use
	// Note: Shift state tracking doesn't work reliably in Fyne
	// Using TypedShortcut with CustomShortcut instead
}

// KeyUp handles key up events (for future use).
func (e *ModelEditor) KeyUp(key *fyne.KeyEvent) {
	// Reserved for future use
}

// TypedShortcut handles keyboard shortcuts (Ctrl+S, Ctrl+Z, etc.)
func (e *ModelEditor) TypedShortcut(shortcut fyne.Shortcut) {
	// Handle built-in Fyne shortcuts
	switch shortcut.(type) {
	case *fyne.ShortcutUndo:
		if e.disabled {
			return
		}
		e.Undo()

		return
	case *fyne.ShortcutRedo:
		if e.disabled {
			return
		}
		e.Redo()

		return
	case *fyne.ShortcutCopy:
		// Allow copy even when disabled
		e.copyToClipboard()

		return
	case *fyne.ShortcutPaste:
		if e.disabled {
			return
		}
		e.pasteFromClipboard()

		return
	case *fyne.ShortcutCut:
		if e.disabled {
			return
		}
		e.cutToClipboard()

		return
	}

	// Handle custom shortcuts
	if typed, ok := shortcut.(*desktop.CustomShortcut); ok {
		// Ctrl+S / Cmd+S: Save
		if typed.KeyName == fyne.KeyS && (typed.Modifier&fyne.KeyModifierControl != 0 || typed.Modifier&fyne.KeyModifierSuper != 0) {
			if e.OnSave != nil {
				e.OnSave()
			}

			return
		}

		// Ctrl+F / Cmd+F: Find
		if typed.KeyName == fyne.KeyF && (typed.Modifier&fyne.KeyModifierControl != 0 || typed.Modifier&fyne.KeyModifierSuper != 0) {
			e.showFindDialog()

			return
		}

		// Ctrl+H / Cmd+H: Replace
		if typed.KeyName == fyne.KeyH && (typed.Modifier&fyne.KeyModifierControl != 0 || typed.Modifier&fyne.KeyModifierSuper != 0) {
			e.showReplaceDialog()

			return
		}

		// F3: Find next
		if typed.KeyName == fyne.KeyF3 {
			e.FindNext()

			return
		}

		// Shift+F3: Find previous
		if typed.KeyName == fyne.KeyF3 && (typed.Modifier&fyne.KeyModifierShift != 0) {
			e.FindPrevious()

			return
		}

		// Shift+Arrow: Extend selection
		if typed.Modifier&fyne.KeyModifierShift != 0 {
			//exhaustive:ignore
			switch typed.KeyName {
			case fyne.KeyLeft:
				e.extendSelectionLeft()

				return
			case fyne.KeyRight:
				e.extendSelectionRight()

				return
			case fyne.KeyUp:
				e.extendSelectionUp()

				return
			case fyne.KeyDown:
				e.extendSelectionDown()

				return
			case fyne.KeyHome:
				e.extendSelectionHome()

				return
			case fyne.KeyEnd:
				e.extendSelectionEnd()

				return
			}
		}
	}
}

// calculateGutterWidth calculates the width of the line number gutter.
func (e *ModelEditor) calculateGutterWidth() float32 {
	e.linesMutex.RLock()
	lineCount := len(e.Lines)
	e.linesMutex.RUnlock()

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

// positionCursorAtPointFast positions the cursor without triggering expensive operations.
// Used during drag operations where we need speed over features like bracket matching.
func (e *ModelEditor) positionCursorAtPointFast(pos fyne.Position) {
	gutterWidth := e.calculateGutterWidth()
	charWidth := float32(8.5)
	lineHeight := float32(20)
	leftMargin := gutterWidth + 5

	e.linesMutex.RLock()

	line := int(pos.Y / lineHeight)
	if line < 0 {
		line = 0
	}
	if line >= len(e.Lines) {
		line = len(e.Lines) - 1
	}

	col := int((pos.X - leftMargin) / charWidth)
	if col < 0 {
		col = 0
	}

	if line >= 0 && line < len(e.Lines) {
		lineContent := e.Lines[line].Content
		maxCol := len(lineContent)
		if maxCol > 0 && lineContent[maxCol-1] == '\n' {
			maxCol--
		}
		if col > maxCol {
			col = maxCol
		}
	}

	e.linesMutex.RUnlock()

	e.CursorPos = e.GetPosFromLineCol(line, col)
}

// positionCursorAtPoint positions the cursor at the given click position.
func (e *ModelEditor) positionCursorAtPoint(pos fyne.Position) {
	// Character dimensions (from renderer.go)
	gutterWidth := e.calculateGutterWidth()
	charWidth := float32(8.5)
	lineHeight := float32(20)
	leftMargin := gutterWidth + 5 // Gutter width + text margin

	// Acquire read lock for thread-safe access to Lines
	e.linesMutex.RLock()

	// Calculate line from Y position
	line := int(pos.Y / lineHeight)
	if line < 0 {
		line = 0
	}
	if line >= len(e.Lines) {
		line = len(e.Lines) - 1
	}

	// Calculate column from X position
	col := int((pos.X - leftMargin) / charWidth)
	if col < 0 {
		col = 0
	}

	// Clamp column to line length
	if line >= 0 && line < len(e.Lines) {
		lineContent := e.Lines[line].Content
		// Don't count the newline character in column calculation
		maxCol := len(lineContent)
		if maxCol > 0 && lineContent[maxCol-1] == '\n' {
			maxCol--
		}
		if col > maxCol {
			col = maxCol
		}
	}

	e.linesMutex.RUnlock()

	// Convert line/col to linear position
	newPos := e.GetPosFromLineCol(line, col)
	e.CursorPos = newPos

	// Reset cursor blink to make it visible immediately after clicking
	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.updateBracketMatching()
	e.Refresh()
}

// Dragged handles mouse drag events for text selection.
func (e *ModelEditor) Dragged(ev *fyne.DragEvent) {
	// Start selection on first drag if not already selecting
	if !e.selecting {
		e.selecting = true
		e.selectionStart = e.CursorPos
	}

	// Update selection end position based on drag position
	// Use lightweight cursor positioning (no bracket matching or full refresh)
	e.positionCursorAtPointFast(ev.Position)
	e.selectionEnd = e.CursorPos

	// Only update selection rectangles, not full refresh
	if r := e.getRenderer(); r != nil {
		r.updateSelectionOnly()
	}
}

// DragEnd handles the end of a drag operation.
func (e *ModelEditor) DragEnd() {
	// Mark selection as complete
	e.selecting = false

	// If selection is empty (start == end), clear it
	if e.selectionStart == e.selectionEnd {
		e.ClearSelection()
	}
}

// extendSelectionLeft extends selection one character to the left.
func (e *ModelEditor) extendSelectionLeft() {
	if e.CursorPos == 0 {
		return
	}

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Move cursor left and update selection end
	e.CursorPos--
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// extendSelectionRight extends selection one character to the right.
func (e *ModelEditor) extendSelectionRight() {
	if e.CursorPos >= len(e.TextContent) {
		return
	}

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Move cursor right and update selection end
	e.CursorPos++
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// extendSelectionUp extends selection one line up.
func (e *ModelEditor) extendSelectionUp() {
	line, col := e.GetLineCol(e.CursorPos)
	if line == 0 {
		return
	}

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Move cursor up and update selection end
	e.CursorPos = e.GetPosFromLineCol(line-1, col)
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// extendSelectionDown extends selection one line down.
func (e *ModelEditor) extendSelectionDown() {
	line, col := e.GetLineCol(e.CursorPos)
	if line >= len(e.lineBreaks) {
		return
	}

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Move cursor down and update selection end
	e.CursorPos = e.GetPosFromLineCol(line+1, col)
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// extendSelectionHome extends selection to start of current line.
func (e *ModelEditor) extendSelectionHome() {
	line, _ := e.GetLineCol(e.CursorPos)

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Move cursor to line start and update selection end
	e.CursorPos = e.GetPosFromLineCol(line, 0)
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// extendSelectionEnd extends selection to end of current line.
func (e *ModelEditor) extendSelectionEnd() {
	line, _ := e.GetLineCol(e.CursorPos)

	// Start new selection if none exists
	if !e.HasSelection() {
		e.selectionStart = e.CursorPos
	}

	// Find end of line
	var lineEnd int
	if line < len(e.lineBreaks) {
		lineEnd = e.lineBreaks[line]
	} else {
		lineEnd = len(e.TextContent)
	}

	// Move cursor to line end and update selection end
	e.CursorPos = lineEnd
	e.selectionEnd = e.CursorPos

	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	e.Refresh()
}

// copyToClipboard copies the selected text to the clipboard.
func (e *ModelEditor) copyToClipboard() {
	if !e.HasSelection() {
		return
	}

	selectedText := e.GetSelectedText()

	// Use App.Clipboard() for clipboard access
	fyne.CurrentApp().Clipboard().SetContent(selectedText)
}

// pasteFromClipboard pastes text from the clipboard at the cursor position.
func (e *ModelEditor) pasteFromClipboard() {
	// Use App.Clipboard() for clipboard access
	clipboardContent := fyne.CurrentApp().Clipboard().Content()

	if clipboardContent == "" {
		return
	}

	// Delete any selected text first (paste replaces selection)
	if e.HasSelection() {
		e.DeleteSelection()
	}

	// Save undo state before modification
	e.pushUndoState()

	// Insert clipboard content at cursor position
	clipboardRunes := []rune(clipboardContent)
	e.TextContent = append(e.TextContent[:e.CursorPos], append(clipboardRunes, e.TextContent[e.CursorPos:]...)...)
	e.CursorPos += len(clipboardRunes)

	// Reset cursor blink to make it visible immediately after pasting
	if renderer := e.getRenderer(); renderer != nil {
		renderer.resetCursorBlink()
	}

	// Update line breaks and re-lex
	e.updateLineBreaks()
	e.lexContent()
	e.checkDirtyState() // Update dirty state after modification
	e.Refresh()

	if e.OnChanged != nil {
		e.OnChanged(e.GetText())
	}
}

// cutToClipboard cuts the selected text to the clipboard.
func (e *ModelEditor) cutToClipboard() {
	if !e.HasSelection() {
		return
	}

	// Copy to clipboard first
	e.copyToClipboard()

	// Then delete the selection
	e.DeleteSelection()
}
