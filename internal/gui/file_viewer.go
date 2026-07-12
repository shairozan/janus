package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/runlog"
)

// FileViewerDialog creates a dialog for viewing and exporting files from a run log entry.
// TODO: This needs to be updated to work with the current RunEntry structure which only
// stores file paths in OutputFiles[], not embedded file content. For now, it shows file paths.
type FileViewerDialog struct {
	entry  *runlog.RunEntry
	window fyne.Window
}

// NewFileViewerDialog creates a new file viewer dialog for the given run log entry.
func NewFileViewerDialog(entry *runlog.RunEntry, parent fyne.Window) *FileViewerDialog {
	return &FileViewerDialog{
		entry:  entry,
		window: parent,
	}
}

// Show displays the file viewer dialog.
func (fvd *FileViewerDialog) Show() {
	// Get list of output file paths
	files := fvd.entry.OutputFiles

	if len(files) == 0 {
		dialog.ShowInformation("No Files", "This run log entry contains no output files.", fvd.window)

		return
	}

	// Create file list widget
	fileList := widget.NewList(
		func() int {
			return len(files)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if label, ok := obj.(*widget.Label); ok {
				label.SetText(files[i])
			}
		},
	)

	// Job metadata section
	metadata := widget.NewRichTextFromMarkdown(fmt.Sprintf(`## Job: %s

**Timestamp:** %s
**Exit Code:** %d
**Duration:** %d ms
**Files:** %d

Output files from this run (file paths only):`,
		fvd.entry.JobID,
		fvd.entry.Timestamp.Format("2006-01-02 15:04:05 MST"),
		fvd.entry.ExitCode,
		fvd.entry.Duration,
		len(files),
	))

	// Layout
	content := container.NewBorder(
		metadata, // Top: metadata
		nil,      // Bottom
		nil,      // Left
		nil,      // Right
		fileList, // Center: file list
	)

	// Create and show dialog
	d := dialog.NewCustom("Run Log Files", "Close", content, fvd.window)
	d.Resize(fyne.NewSize(600, 500))
	d.Show()
}

// ShowRunLogFilesDialog is a convenience function to show the file viewer dialog.
func ShowRunLogFilesDialog(entry *runlog.RunEntry, parent fyne.Window) {
	viewer := NewFileViewerDialog(entry, parent)
	viewer.Show()
}
