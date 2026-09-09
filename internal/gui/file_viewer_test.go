package gui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/shairozan/janus/internal/runlog"
)

func TestNewFileViewerDialog(t *testing.T) {
	app := test.NewApp()
	window := app.NewWindow("Test")

	entry := &runlog.RunEntry{
		JobID:       "test-job-123",
		Timestamp:   time.Now(),
		ExitCode:    0,
		Duration:    1500,
		OutputFiles: []string{"/path/to/model.lst"},
	}

	viewer := NewFileViewerDialog(entry, window)

	if viewer == nil {
		t.Fatal("NewFileViewerDialog returned nil")
	}

	if viewer.entry != entry {
		t.Error("FileViewerDialog entry not set correctly")
	}

	if viewer.window != window {
		t.Error("FileViewerDialog window not set correctly")
	}
}

func TestFileViewerDialog_EmptyEntry(_ *testing.T) {
	app := test.NewApp()
	window := app.NewWindow("Test")

	entry := &runlog.RunEntry{
		JobID:       "test-job-empty",
		Timestamp:   time.Now(),
		OutputFiles: []string{}, // No files
	}

	viewer := NewFileViewerDialog(entry, window)

	// This should not panic and should show an information dialog
	// In a real GUI test, we would verify the dialog appears
	viewer.Show()
}

func TestFileViewerDialog_MultipleFiles(_ *testing.T) {
	app := test.NewApp()
	window := app.NewWindow("Test")

	entry := &runlog.RunEntry{
		JobID:     "test-job-multi",
		Timestamp: time.Now(),
		ExitCode:  0,
		Duration:  2000,
		OutputFiles: []string{
			"/path/to/model.lst",
			"/path/to/model.ext",
			"/path/to/model.cov",
		},
	}

	viewer := NewFileViewerDialog(entry, window)
	viewer.Show()

	// In a real GUI test, we would:
	// - Verify the file list shows 3 items
	// - Verify the metadata section shows correct job info
	// - Verify buttons are present and functional
}

func TestShowRunLogFilesDialog(_ *testing.T) {
	app := test.NewApp()
	window := app.NewWindow("Test")

	entry := &runlog.RunEntry{
		JobID:       "test-job-convenience",
		Timestamp:   time.Now(),
		OutputFiles: []string{"/path/to/test.txt"},
	}

	// This should not panic
	ShowRunLogFilesDialog(entry, window)
}
