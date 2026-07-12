package gui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/pirana"
)

// importFromPirana runs Pirana detection and, on success, shows a migration
// report. When the user confirms the import, apply is invoked with the detected
// settings so the caller can prefill its own form widgets. If no Pirana config
// is found, it offers a manual file picker instead.
//
// It never writes the Janus config itself: persistence stays with the caller's
// existing save path, per the orthogonality rules in CLAUDE.md.
func importFromPirana(win fyne.Window, apply func(*pirana.Settings)) {
	settings, err := pirana.Detect()
	if err != nil {
		if errors.Is(err, pirana.ErrNotFound) {
			promptForPiranaFile(win, apply)

			return
		}

		dialog.ShowError(fmt.Errorf("could not read Pirana configuration: %w", err), win)

		return
	}

	showMigrationReport(win, settings, apply)
}

// promptForPiranaFile lets the user point Janus at a Pirana settings file
// (settings.db or pirana.ini) when auto-detection finds nothing.
func promptForPiranaFile(win fyne.Window, apply func(*pirana.Settings)) {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, win)

			return
		}

		if reader == nil {
			// User cancelled the picker.
			return
		}

		path := reader.URI().Path()
		_ = reader.Close()

		settings, err := pirana.Load(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("could not read %s: %w", path, err), win)

			return
		}

		showMigrationReport(win, settings, apply)
	}, win)
}

// showMigrationReport presents what will be imported and, on confirmation,
// applies the settings.
func showMigrationReport(win fyne.Window, s *pirana.Settings, apply func(*pirana.Settings)) {
	report := widget.NewRichTextFromMarkdown(migrationReportMarkdown(s))
	report.Wrapping = fyne.TextWrapWord

	scroll := container.NewVScroll(report)
	scroll.SetMinSize(fyne.NewSize(460, 320))

	dialog.NewCustomConfirm("Import from Pirana", "Import", "Cancel", scroll,
		func(ok bool) {
			if ok {
				apply(s)
			}
		}, win).Show()
}

// migrationReportMarkdown renders a human-readable summary of the detected
// Pirana settings: what will be imported (with on-disk warnings for missing
// paths) and what was detected but has no Janus destination yet.
func migrationReportMarkdown(s *pirana.Settings) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**Source:** %s\n\n", s.Source)
	b.WriteString("Your Pirana installation will not be modified.\n\n")
	b.WriteString("### Will import\n\n")

	imported := false

	line := func(label, val string) {
		if val == "" {
			return
		}

		imported = true

		warn := ""
		if isPathLike(val) && !pathExists(val) {
			warn = "  ⚠ not found on disk"
		}

		fmt.Fprintf(&b, "- **%s:** %s%s\n", label, val, warn)
	}

	line("NONMEM path", s.NonmemPath)
	line("NONMEM binary", s.NonmemBinary)
	line("Default directory", s.DefaultDir)
	line("Scheduler", s.Scheduler)
	line("Researcher → Organization", s.Researcher)

	if !imported {
		b.WriteString("_No mappable settings were found._\n")
	}

	if len(s.Detected) > 0 {
		b.WriteString("\n### Detected but not imported\n\n")

		for _, d := range s.Detected {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}

	return b.String()
}

// isPathLike reports whether a value looks like a filesystem path (and is thus
// worth checking for existence).
func isPathLike(val string) bool {
	return strings.ContainsAny(val, `/\`) || strings.Contains(val, ":\\")
}

// pathExists reports whether path exists on disk.
func pathExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
