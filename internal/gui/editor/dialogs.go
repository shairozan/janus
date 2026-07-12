package editor

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// showFindDialog displays the find dialog.
func (e *ModelEditor) showFindDialog() {
	if e.window == nil {
		return
	}

	// Create find input
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search for...")

	// Pre-fill with selected text if any
	if e.HasSelection() {
		searchEntry.SetText(e.GetSelectedText())
	} else if e.searchTerm != "" {
		searchEntry.SetText(e.searchTerm)
	}

	// Create case-sensitive checkbox
	caseSensitiveCheck := widget.NewCheck("Case sensitive", func(checked bool) {
		e.caseSensitive = checked
	})
	caseSensitiveCheck.SetChecked(e.caseSensitive)

	// Create match counter label
	matchLabel := widget.NewLabel("")

	// Declare button variables first
	var nextButton, prevButton *widget.Button

	// Update match count and show/hide navigation buttons
	updateMatchCount := func() {
		if len(e.searchMatches) == 0 {
			matchLabel.SetText("No matches")
			if nextButton != nil {
				nextButton.Hide()
			}
			if prevButton != nil {
				prevButton.Hide()
			}
		} else {
			matchLabel.SetText(fmt.Sprintf("Match %d of %d", e.GetCurrentMatchIndex(), e.GetSearchMatchCount()))
			if nextButton != nil {
				nextButton.Show()
			}
			if prevButton != nil {
				prevButton.Show()
			}
		}
	}

	// Find Next and Previous buttons (initially hidden)
	nextButton = widget.NewButton("Next", func() {
		if len(e.searchMatches) > 0 {
			e.FindNext()
			updateMatchCount()
		}
	})
	nextButton.Hide()

	prevButton = widget.NewButton("Previous", func() {
		if len(e.searchMatches) > 0 {
			e.FindPrevious()
			updateMatchCount()
		}
	})
	prevButton.Hide()

	// Navigation container (starts hidden)
	navContainer := container.NewHBox(nextButton, prevButton)

	// Find button
	findButton := widget.NewButton("Find", func() {
		searchText := searchEntry.Text
		if searchText == "" {
			e.ClearSearch()
			matchLabel.SetText("")
			nextButton.Hide()
			prevButton.Hide()

			return
		}

		e.Find(searchText, caseSensitiveCheck.Checked)
		updateMatchCount()
	})

	// Search input row with Find button inline
	searchRow := container.NewBorder(nil, nil, nil, findButton, searchEntry)

	// Create form with minimum width
	content := container.NewVBox(
		widget.NewLabel("Find"),
		searchRow,
		caseSensitiveCheck,
		container.NewHBox(matchLabel, navContainer),
	)

	// Create dialog
	d := dialog.NewCustom("Find", "Close", content, e.window)
	d.Resize(fyne.NewSize(1000, 200))

	// Auto-find when Enter is pressed in the search entry
	searchEntry.OnSubmitted = func(text string) {
		findButton.OnTapped()
	}

	// Clear search when dialog is closed
	d.SetOnClosed(func() {
		// Keep the search active but don't clear it
		// User can still use F3/Shift+F3 to navigate
	})

	// Show dialog
	d.Show()

	// Focus the search entry
	e.window.Canvas().Focus(searchEntry)
}

// showReplaceDialog displays the find and replace dialog.
func (e *ModelEditor) showReplaceDialog() {
	if e.window == nil {
		return
	}

	// Create find input
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Find...")

	// Pre-fill with selected text if any
	if e.HasSelection() {
		searchEntry.SetText(e.GetSelectedText())
	} else if e.searchTerm != "" {
		searchEntry.SetText(e.searchTerm)
	}

	// Create replace input
	replaceEntry := widget.NewEntry()
	replaceEntry.SetPlaceHolder("Replace with...")

	// Create case-sensitive checkbox
	caseSensitiveCheck := widget.NewCheck("Case sensitive", func(checked bool) {
		e.caseSensitive = checked
	})
	caseSensitiveCheck.SetChecked(e.caseSensitive)

	// Create match counter label
	matchLabel := widget.NewLabel("")

	// Declare button variables first
	var nextButton, prevButton, replaceButton, replaceAllButton *widget.Button

	// Update match count and show/hide navigation buttons
	updateMatchCount := func() {
		if len(e.searchMatches) == 0 {
			matchLabel.SetText("No matches")
			if nextButton != nil {
				nextButton.Hide()
			}
			if prevButton != nil {
				prevButton.Hide()
			}
			if replaceButton != nil {
				replaceButton.Hide()
			}
			if replaceAllButton != nil {
				replaceAllButton.Hide()
			}
		} else {
			matchLabel.SetText(fmt.Sprintf("Match %d of %d", e.GetCurrentMatchIndex(), e.GetSearchMatchCount()))
			if nextButton != nil {
				nextButton.Show()
			}
			if prevButton != nil {
				prevButton.Show()
			}
			if replaceButton != nil {
				replaceButton.Show()
			}
			if replaceAllButton != nil {
				replaceAllButton.Show()
			}
		}
	}

	// Find Next and Previous buttons (initially hidden)
	nextButton = widget.NewButton("Next", func() {
		if len(e.searchMatches) > 0 {
			e.FindNext()
			updateMatchCount()
		}
	})
	nextButton.Hide()

	prevButton = widget.NewButton("Previous", func() {
		if len(e.searchMatches) > 0 {
			e.FindPrevious()
			updateMatchCount()
		}
	})
	prevButton.Hide()

	// Replace buttons (initially hidden)
	replaceButton = widget.NewButton("Replace", func() {
		if len(e.searchMatches) > 0 && e.currentMatch >= 0 {
			e.Replace(replaceEntry.Text)
			updateMatchCount()
		}
	})
	replaceButton.Hide()

	replaceAllButton = widget.NewButton("Replace All", func() {
		if len(e.searchMatches) > 0 {
			count := len(e.searchMatches)
			e.ReplaceAll(replaceEntry.Text)
			matchLabel.SetText(fmt.Sprintf("Replaced %d matches", count))
			if replaceButton != nil {
				replaceButton.Hide()
			}
			if replaceAllButton != nil {
				replaceAllButton.Hide()
			}
			if nextButton != nil {
				nextButton.Hide()
			}
			if prevButton != nil {
				prevButton.Hide()
			}
		}
	})
	replaceAllButton.Hide()

	// Find button
	findButton := widget.NewButton("Find", func() {
		searchText := searchEntry.Text
		if searchText == "" {
			e.ClearSearch()
			matchLabel.SetText("")
			nextButton.Hide()
			prevButton.Hide()
			replaceButton.Hide()
			replaceAllButton.Hide()

			return
		}

		e.Find(searchText, caseSensitiveCheck.Checked)
		updateMatchCount()
	})

	// Search input row with Find button inline
	searchRow := container.NewBorder(nil, nil, nil, findButton, searchEntry)

	// Navigation container
	navContainer := container.NewHBox(nextButton, prevButton)

	// Replace container
	replaceContainer := container.NewHBox(replaceButton, replaceAllButton)

	// Create form with minimum width
	content := container.NewVBox(
		widget.NewLabel("Find and Replace"),
		searchRow,
		replaceEntry,
		caseSensitiveCheck,
		container.NewHBox(matchLabel, navContainer),
		replaceContainer,
	)

	// Create dialog
	d := dialog.NewCustom("Find and Replace", "Close", content, e.window)
	d.Resize(fyne.NewSize(1000, 250))

	// Auto-find when Enter is pressed in the search entry
	searchEntry.OnSubmitted = func(text string) {
		findButton.OnTapped()
	}

	// Auto-replace when Enter is pressed in the replace entry
	replaceEntry.OnSubmitted = func(text string) {
		replaceButton.OnTapped()
	}

	// Clear search when dialog is closed
	d.SetOnClosed(func() {
		// Keep the search active but don't clear it
		// User can still use F3/Shift+F3 to navigate
	})

	// Show dialog
	d.Show()

	// Focus the search entry
	e.window.Canvas().Focus(searchEntry)
}
