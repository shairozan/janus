package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fynecontainer "fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/gui/editor"
	"github.com/pharmalytica/janus/internal/inheritance"
	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
)

// showInheritParametersDialog displays the parameter inheritance dialog.
// This allows users to inherit parameter estimates from a previous run
// into the currently loaded model.
func (a *App) showInheritParametersDialog(sourceRun *runlog.RunRecord) {
	// Validate prerequisites
	if a.currentFilePath == "" {
		dialog.ShowError(fmt.Errorf("no model file loaded"), a.window)

		return
	}

	if sourceRun == nil || sourceRun.Summary == nil {
		dialog.ShowError(fmt.Errorf("selected run has no parameter estimates"), a.window)

		return
	}

	// Load correlation strategy from model config
	strategy := inheritance.StrategyConservative
	if modelCfg, err := config.LoadModelConfig(a.currentFilePath); err == nil && modelCfg != nil {
		strategy = inheritance.ParseCorrelationStrategy(modelCfg.CorrelationStrategy)
	}

	// Get current model content from editor
	content := a.getEditorContent()
	if content == "" {
		dialog.ShowError(fmt.Errorf("no model content to update"), a.window)

		return
	}
	targetLabels := model.ParseParameterLabels(content)

	// Build options
	opts := inheritance.DefaultPatchOptions()
	opts.Strategy = strategy

	// Preview the patch to show user what will change
	preview, err := inheritance.PatchControlStream(content, sourceRun.Summary, targetLabels, opts)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to preview parameter inheritance: %w", err), a.window)

		return
	}

	// Show confirmation dialog with preview
	a.showInheritPreviewDialog(sourceRun, preview, opts)
}

// showInheritPreviewDialog shows a preview of parameter changes before applying.
func (a *App) showInheritPreviewDialog(sourceRun *runlog.RunRecord, preview *inheritance.PatchResult, opts inheritance.PatchOptions) {
	// Build mappings summary
	var mappingsSummary strings.Builder
	if len(preview.Mappings) == 0 {
		mappingsSummary.WriteString("No parameters matched.\n\n")
		mappingsSummary.WriteString("Try changing the correlation strategy in .janus.config.json")
	} else {
		fmt.Fprintf(&mappingsSummary, "Found %d parameter(s) to inherit:\n\n", len(preview.Mappings))
		for _, m := range preview.Mappings {
			matchInfo := ""
			if m.MatchedBy != "" {
				matchInfo = fmt.Sprintf(" (by %s)", m.MatchedBy)
			}
			fmt.Fprintf(&mappingsSummary, "  %s -> %s: %g%s\n",
				m.SourceName, m.TargetName, m.SourceEstimate, matchInfo)
		}
	}

	// Add warnings if any
	if len(preview.Warnings) > 0 {
		mappingsSummary.WriteString("\nWarnings:\n")
		for _, w := range preview.Warnings {
			fmt.Fprintf(&mappingsSummary, "  - %s\n", w)
		}
	}

	// Add unmatched info
	if len(preview.UnmatchedSource) > 0 {
		fmt.Fprintf(&mappingsSummary, "\nUnmatched source parameters: %s\n",
			strings.Join(preview.UnmatchedSource, ", "))
	}

	// Create info labels
	sourceLabel := widget.NewLabel(fmt.Sprintf("Source Run: #%s (%s)",
		shortRunID(sourceRun.ID),
		sourceRun.Timestamp.Format(time.RFC3339)))

	strategyLabel := widget.NewLabel(fmt.Sprintf("Strategy: %s", opts.Strategy.String()))

	// Create scrollable preview of mappings
	mappingsText := widget.NewRichTextWithText(mappingsSummary.String())
	mappingsText.Wrapping = fyne.TextWrapWord
	mappingsScroll := fynecontainer.NewVScroll(mappingsText)
	mappingsScroll.SetMinSize(fyne.NewSize(500, 200))

	// Assemble content
	content := fynecontainer.NewVBox(
		sourceLabel,
		strategyLabel,
		widget.NewSeparator(),
		widget.NewLabel("Parameter Changes:"),
		mappingsScroll,
	)

	// Create dialog
	d := dialog.NewCustom("Inherit Parameters", "Cancel", content, a.window)

	// Create Apply button - only enable if there are matches
	applyButton := widget.NewButton("Apply", func() {
		d.Hide()
		a.applyParameterInheritance(sourceRun, preview)
	})
	applyButton.Importance = widget.HighImportance
	if len(preview.Mappings) == 0 {
		applyButton.Disable()
	}

	d.SetButtons([]fyne.CanvasObject{
		applyButton,
		widget.NewButton("Cancel", func() {
			d.Hide()
		}),
	})

	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}

// applyParameterInheritance applies the parameter inheritance to the model file.
func (a *App) applyParameterInheritance(sourceRun *runlog.RunRecord, preview *inheritance.PatchResult) {
	// Create backup file
	backupPath := a.currentFilePath + ".bak"
	if err := a.createBackupFile(backupPath); err != nil {
		dialog.ShowError(fmt.Errorf("failed to create backup: %w", err), a.window)

		return
	}

	// Update the editor with the new content
	a.setEditorContent(preview.ModifiedContent)

	// Save the file
	if err := a.saveModelFile(); err != nil {
		// Restore from backup on failure
		if backupContent, readErr := os.ReadFile(backupPath); readErr == nil {
			a.setEditorContent(string(backupContent))
		}
		dialog.ShowError(fmt.Errorf("failed to save file: %w", err), a.window)

		return
	}

	// Show success message
	a.showSuccessToast(fmt.Sprintf("Inherited %d parameters from run #%s (backup: %s)",
		len(preview.Mappings),
		shortRunID(sourceRun.ID),
		filepath.Base(backupPath)))
}

// getEditorContent returns the current content of the model editor.
func (a *App) getEditorContent() string {
	if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
		return modelEditor.GetText()
	}

	// Fallback to legacy editor
	if a.textEditor != nil {
		return a.textEditor.Text
	}

	return ""
}

// setEditorContent sets the content of the model editor.
func (a *App) setEditorContent(content string) {
	if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
		modelEditor.SetText(content)

		return
	}

	// Fallback to legacy editor
	if a.textEditor != nil {
		a.textEditor.SetText(content)
	}
}

// createBackupFile creates a backup of the current model file.
func (a *App) createBackupFile(backupPath string) error {
	content, err := os.ReadFile(a.currentFilePath)
	if err != nil {
		return fmt.Errorf("failed to read original file: %w", err)
	}

	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}
