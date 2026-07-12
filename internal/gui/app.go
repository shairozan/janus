package gui

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/audit"
	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
	"github.com/pharmalytica/janus/internal/license/validator"
)

type App struct {
	fyneApp             fyne.App
	window              fyne.Window
	settingsOpen        bool
	needsRunDetailsTab  bool
	pendingModelPath    string
	config              *config.Config
	licenseClaims       *validator.Claims

	// Current loaded file
	currentFilePath string
	fileContent     string
	fileHash        [32]byte

	// File watching
	watchCtx    context.Context //nolint:containedctx // Long-lived app context for file watching
	watchCancel context.CancelFunc

	// Error handling
	errorCh     chan error
	errorCtx    context.Context //nolint:containedctx // Long-lived app context for error handling
	errorCancel context.CancelFunc

	// UI components that need to be enabled/disabled
	modelEntry             *widget.Entry
	dataEntry              *widget.Entry
	syncRadio              *widget.RadioGroup
	coresEntry             *widget.Entry
	coresContainer         *fyne.Container
	nonmemOptionsEntry     *widget.Entry
	nonmemOptionsContainer *fyne.Container
	descriptionCheck       *widget.Check
	runHereBtn             *widget.Button
	runGridBtn             *widget.Button
	textEditor             *widget.Entry
	modelSubTabs           *container.AppTabs
	saveBtn                *widget.Button

	// Run details components
	runDetailsTab   *container.TabItem
	outputTabs      *container.AppTabs
	stdoutDisplay   *widget.Entry
	stderrDisplay   *widget.Entry
	runHistoryTable *widget.Table
	runHistory      *audit.RunHistory

	// Active runs tracking
	activeRuns        map[int]context.CancelFunc         // runID -> cancel function
	activeStreams     map[int]*execution.StreamingOutput // runID -> streaming output
	liveOutputWindows map[int]fyne.Window                // runID -> live output window
	activeRunsTable   *widget.Table
	mainLayout        *fyne.Container
	updateTicker      *time.Ticker

	// SLURM monitoring
	slurmMonitor *SLURMMonitor

	// Dialog tracking
	cancelJobDialog *widget.PopUp
	errorDialog     *widget.PopUp
}

func NewApp(ctx context.Context) *App {
	fyneApp := app.New()
	window := fyneApp.NewWindow("Janus")
	window.Resize(fyne.NewSize(1200, 800))

	// Initialize empty run history (will be set when model is loaded)
	runHistory := &audit.RunHistory{
		Runs:        []audit.RunRecord{},
		HistoryFile: "", // Will be set when model is loaded
	}

	// Create error handling context and channel
	errorCtx, errorCancel := context.WithCancel(ctx)
	errorCh := make(chan error, 100) // Buffered channel for async errors

	app := &App{
		fyneApp:           fyneApp,
		window:            window,
		runHistory:        runHistory,
		errorCh:           errorCh,
		errorCtx:          errorCtx,
		errorCancel:       errorCancel,
		activeRuns:        make(map[int]context.CancelFunc),
		activeStreams:     make(map[int]*execution.StreamingOutput),
		liveOutputWindows: make(map[int]fyne.Window),
	}

	// Start error handling goroutine
	go app.handleErrors()

	// Start periodic updates for active runs
	app.startActiveRunsUpdater()

	return app
}

func (a *App) SetConfiguration(cfg *config.Config) {
	// Stop existing SLURM monitoring if any
	a.stopSLURMMonitoring()

	a.config = cfg

	// Set up SLURM monitoring if configuration supports it
	a.setupSLURMMonitoring()
}

// SetLicenseClaims stores the validated license claims for feature gating.
func (a *App) SetLicenseClaims(claims *validator.Claims) {
	a.licenseClaims = claims
}

// HasFeature checks if a feature is enabled in the license.
func (a *App) HasFeature(feature string) bool {
	if a.licenseClaims == nil {
		return false
	}

	return a.licenseClaims.HasFeature(feature)
}

// ShowLicenseError displays a license validation error and exits the application.
func (a *App) ShowLicenseError(err error) {
	// Create a simple window to show the error
	window := a.fyneApp.NewWindow("License Error")
	window.Resize(fyne.NewSize(500, 200))

	errorMsg := fmt.Sprintf("Failed to validate license:\n\n%v\n\nPlease ensure a valid license file exists at the configured path.", err)

	content := container.NewVBox(
		widget.NewLabel("License Validation Failed"),
		widget.NewLabel(""),
		widget.NewLabel(errorMsg),
		widget.NewLabel(""),
		widget.NewButton("Exit", func() {
			a.fyneApp.Quit()
		}),
	)

	window.SetContent(container.NewPadded(content))
	window.CenterOnScreen()
	window.Show()

	// Run the app to show the error dialog
	a.fyneApp.Run()
}

// handleErrors processes errors from the error channel and displays them as toast notifications.
func (a *App) handleErrors() {
	for {
		select {
		case <-a.errorCtx.Done():
			// Context cancelled, stop processing errors
			return
		case err := <-a.errorCh:
			if err != nil {
				// Display error as toast notification
				a.showToast("ERROR", err.Error(), 6*time.Second)
			}
		}
	}
}

// sendError sends an error to the error channel with context cancellation support.
func (a *App) sendError(err error) {
	if err == nil {
		return
	}

	select {
	case a.errorCh <- err:
		// Error sent successfully
	case <-a.errorCtx.Done():
		// Context cancelled, don't send error
		return
	}
}

// startActiveRunsUpdater starts a goroutine that periodically refreshes the active runs display.
func (a *App) startActiveRunsUpdater() {
	a.updateTicker = time.NewTicker(2 * time.Second) // Update every 2 seconds

	go func() {
		for {
			select {
			case <-a.errorCtx.Done():
				// Context cancelled, stop updating
				return
			case <-a.updateTicker.C:
				// Only update if there are active runs
				if len(a.activeRuns) > 0 && a.activeRunsTable != nil {
					a.activeRunsTable.Refresh()
				}
			}
		}
	}()
}

// Cleanup should be called when the app is shutting down.
func (a *App) Cleanup() {
	// Stop SLURM monitoring
	a.stopSLURMMonitoring()

	// Stop the update ticker
	if a.updateTicker != nil {
		a.updateTicker.Stop()
	}

	// Cancel error handling context
	if a.errorCancel != nil {
		a.errorCancel()
	}

	// Cancel file watching if active
	if a.watchCancel != nil {
		a.watchCancel()
	}

	// Cancel any active runs
	for runID, cancel := range a.activeRuns {
		cancel()
		a.updateRunRecord(runID, -1, "", "Run cancelled due to application shutdown")
	}

	// Clear active runs map
	a.activeRuns = make(map[int]context.CancelFunc)

	// Close error channel
	close(a.errorCh)
}

// getNextRunID calculates the next available run ID by finding the highest existing ID and adding 1.
func (a *App) getNextRunID() int {
	maxID := 0
	for _, run := range a.runHistory.Runs {
		if run.ID > maxID {
			maxID = run.ID
		}
	}

	return maxID + 1
}

// GetFyneApp returns the underlying fyne application.
func (a *App) GetFyneApp() fyne.App {
	return a.fyneApp
}

// ShowMainWindow shows the main application window and takes over app lifecycle.
func (a *App) ShowMainWindow() {
	content := a.buildMainInterface()
	a.window.SetContent(content)
	a.window.Show()

	// Set close intercept to trigger graceful shutdown (same as signal handling)
	a.window.SetCloseIntercept(func() {
		// Trigger the same graceful shutdown sequence as CTRL+C
		a.Cleanup()
		a.fyneApp.Quit()
	})
}

func (a *App) Run() {
	content := a.buildMainInterface()
	a.window.SetContent(content)

	a.window.ShowAndRun()
}

// RunWithoutShowing runs the app but doesn't show the main window initially.
func (a *App) RunWithoutShowing() {
	// Don't show the main window - it will be shown later by ShowMainWindow()
	a.fyneApp.Run()
}

func (a *App) buildMainInterface() fyne.CanvasObject {
	// Settings button in top right
	settingsBtn := widget.NewButton("⚙️ Settings", func() {
		a.showSettingsPanel()
	})

	// Load logo resource
	logoResource, err := fyne.LoadResourceFromPath("assets/logo.png")
	if err != nil {
		// Fallback if logo can't be loaded
		logoResource = nil
	}

	// Create logo icon
	var logoWidget fyne.CanvasObject
	if logoResource != nil {
		logoIcon := widget.NewIcon(logoResource)
		logoIcon.Resize(fyne.NewSize(32, 32)) // Set logo size
		logoWidget = logoIcon
	} else {
		// Fallback to text if logo fails to load
		logoWidget = widget.NewLabel("🏛️")
	}

	// Top bar with logo and settings
	topBar := container.NewBorder(
		nil, nil,
		container.NewHBox(logoWidget, widget.NewLabel("Janus")), // Left: logo and title
		settingsBtn, // Right: settings button
	)

	// Main horizontal tabs (Model Run only for now)
	mainTabs := container.NewAppTabs(
		container.NewTabItem("Model Run", a.buildModelRunTab()),
	)

	// Top section contains top bar and main tabs
	topSection := container.NewVBox(topBar, mainTabs)

	// Conditionally add grid details if licensed for "grid" feature
	if a.HasFeature("grid") {
		// Bottom grid details section
		gridDetails := a.buildGridDetails()

		// Main layout: Use border layout to give grid details more space
		a.mainLayout = container.NewBorder(
			topSection, // Top - takes minimum needed space
			nil,        // Bottom
			nil,        // Left
			nil,        // Right
			gridDetails, // Center - takes all remaining space
		)
	} else {
		// No grid details - main tabs take full space
		a.mainLayout = container.NewBorder(
			topSection, // Top - takes minimum needed space
			nil,        // Bottom
			nil,        // Left
			nil,        // Right
			nil, // Center - empty
		)
	}

	return a.mainLayout
}

func (a *App) buildModelRunTab() fyne.CanvasObject {
	// Sub-tabs within the Model Run tab (Model only, Run Details added dynamically)
	modelTab := a.buildModelContent()

	a.modelSubTabs = container.NewAppTabs(
		container.NewTabItem("Model", modelTab),
	)

	// If we need to create run details tab (from early model loading), create it now
	if a.needsRunDetailsTab {
		a.createRunDetailsTab()
		a.needsRunDetailsTab = false
	}

	return a.modelSubTabs
}

func (a *App) buildModelContent() fyne.CanvasObject {
	// Left side: Load Model section
	a.modelEntry = widget.NewEntry()
	a.modelEntry.SetText("/Path/to/file.mod")
	a.modelEntry.Disable()

	browseButton := widget.NewButton("📁", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()

			filePath := reader.URI().Path()
			a.modelEntry.SetText(filePath)

			// Read and load the file content
			if err := a.loadModelFile(filePath); err != nil {
				dialog.ShowError(fmt.Errorf("failed to load model file: %w", err), a.window)

				return
			}

			// Enable components when model is loaded
			a.setModelLoaded(true)
		}, a.window)
	})

	loadModelBox := container.NewBorder(nil, nil, nil, browseButton,
		container.NewVBox(
			widget.NewLabel("Load Model"),
			a.modelEntry,
		),
	)

	// $DATA section
	dataLabel := widget.NewLabel("$DATA")

	// Store reference to components that need to be disabled
	a.dataEntry = widget.NewEntry()
	a.dataEntry.SetText("data.csv")
	a.dataEntry.Disable() // Always disabled since it's read from file

	dataSection := container.NewVBox(
		dataLabel,
		a.dataEntry,
	)

	// Execution mode
	a.syncRadio = widget.NewRadioGroup([]string{"Synchronous", "Parallel"}, func(selected string) {
		a.updateCoresVisibility(selected == "Parallel")
	})
	a.syncRadio.SetSelected("Synchronous")

	// Cores input (initially hidden)
	a.coresEntry = widget.NewEntry()
	a.coresEntry.SetText("4") // Default cores
	a.coresEntry.SetPlaceHolder("Number of cores")

	// Add validation on text change for immediate feedback
	a.coresEntry.OnChanged = func(text string) {
		a.validateCoresInputSilent(text)
	}

	// Create increment/decrement buttons
	decrementBtn := widget.NewButton("-", func() {
		a.adjustCores(-1)
	})
	incrementBtn := widget.NewButton("+", func() {
		a.adjustCores(1)
	})

	coresLabel := widget.NewLabel("Cores:")
	coresInputBox := container.NewHBox(decrementBtn, a.coresEntry, incrementBtn)
	a.coresContainer = container.NewVBox(coresLabel, coresInputBox)
	a.coresContainer.Hide() // Initially hidden since Synchronous is default

	// Additional NONMEM options (only visible for NONMEM execution mode)
	a.nonmemOptionsEntry = widget.NewEntry()
	a.nonmemOptionsEntry.SetPlaceHolder("Additional NONMEM options (e.g., -maxeval=9999 -files=100)")
	a.nonmemOptionsEntry.MultiLine = false

	nonmemOptionsLabel := widget.NewLabel("Additional NONMEM Options:")
	a.nonmemOptionsContainer = container.NewVBox(nonmemOptionsLabel, a.nonmemOptionsEntry)
	a.nonmemOptionsContainer.Hide() // Initially hidden, shown only for NONMEM mode

	// Show NONMEM options input only when execution mode is NONMEM
	if a.config != nil && a.config.ExecutionMode == "NONMEM" {
		a.nonmemOptionsContainer.Show()
	}

	// Optional description checkbox for audit trail
	a.descriptionCheck = widget.NewCheck("Record message for audit trail", nil)

	// Action buttons
	a.runHereBtn = widget.NewButton("Run Here", func() {
		a.executeRun(false) // local execution
	})
	a.runHereBtn.Importance = widget.HighImportance

	a.runGridBtn = widget.NewButton("Run on Grid", func() {
		a.showGridConfigurationModal() // Show grid configuration modal
	})

	// Initially disable all components since no model is loaded yet
	// This must happen after button creation but before pending model loading
	a.setModelLoaded(false)

	leftPanel := container.NewVBox(
		loadModelBox,
		widget.NewSeparator(),
		dataSection,
		widget.NewSeparator(),
		a.syncRadio,
		a.coresContainer,
		widget.NewSeparator(),
		a.nonmemOptionsContainer,
		widget.NewSeparator(),
		a.descriptionCheck,
		widget.NewSeparator(),
		container.NewHBox(a.runHereBtn, a.runGridBtn),
	)

	// Right side: Text editor with toolbar
	textEditorWidget := a.buildTextEditor()

	// Create HSplit and store reference for resizing
	split := container.NewHSplit(leftPanel, textEditorWidget)

	// Set initial split ratio - left panel takes less space when file is loaded
	split.SetOffset(0.25) // 25% left panel, 75% text editor

	return split
}

func (a *App) buildTextEditor() fyne.CanvasObject {
	// Save button for the toolbar
	a.saveBtn = widget.NewButton("💾 Save", func() {
		if err := a.saveModelFile(); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save file: %w", err), a.window)
		} else {
			a.showSuccessToast("Model file saved successfully!")
		}
	})
	a.saveBtn.Importance = widget.HighImportance

	// Toolbar with formatting buttons and save
	toolbar := container.NewHBox(
		a.saveBtn,
		widget.NewSeparator(),
		widget.NewButton("B", func() {}), // Bold
		widget.NewButton("I", func() {}), // Italic
		widget.NewButton("U", func() {}), // Underline
		widget.NewSeparator(),
		widget.NewButton("📋", func() {}), // Copy
		widget.NewButton("📄", func() {}), // Paste
		widget.NewSeparator(),
		widget.NewButton("↶", func() {}), // Undo
		widget.NewButton("↷", func() {}), // Redo
	)

	// Large text editor - store reference for enabling/disabling
	a.textEditor = widget.NewEntry()
	a.textEditor.MultiLine = true
	a.textEditor.Wrapping = fyne.TextWrapWord
	a.textEditor.SetText("// Load a model file to begin editing...")

	// Make the text editor expand to fill available space
	editorContainer := container.NewBorder(toolbar, nil, nil, nil, a.textEditor)

	// Check if there's a pending model to load now that UI is ready
	if a.pendingModelPath != "" {
		if err := a.loadModelFile(a.pendingModelPath); err != nil {
			// TODO: Show error dialog to user about model loading failure
			log.Printf("Error loading pending model file: %v", err)
		}
		a.pendingModelPath = "" // Clear the pending path
	}

	return editorContainer
}

func (a *App) buildRunDetailsTab() fyne.CanvasObject {
	// Create run history table with command and arguments columns
	a.runHistoryTable = widget.NewTable(
		func() (int, int) {
			return len(a.runHistory.Runs), 7 // 7 columns: Run#, Status, Time, Binary, Arguments, Type, Description
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row >= len(a.runHistory.Runs) {
				if label, ok := obj.(*widget.Label); ok {
					label.SetText("")
				}

				return
			}

			run := a.runHistory.Runs[id.Row]
			label, ok := obj.(*widget.Label)
			if !ok {
				return
			}

			switch id.Col {
			case 0: // Run #
				label.SetText(fmt.Sprintf("#%d", run.ID))
			case 1: // Status
				label.SetText(strings.ToUpper(run.Status))
				// Color code status
				switch run.Status {
				case "completed":
					label.Importance = widget.SuccessImportance
				case "failed":
					label.Importance = widget.DangerImportance
				case "running":
					label.Importance = widget.MediumImportance
				default:
					label.Importance = widget.LowImportance
				}
			case 2: // Time
				label.SetText(run.Timestamp.Format("15:04:05"))
			case 3: // Binary (extracted from command)
				binary, _ := a.splitCommand(run.Command)
				label.SetText(binary)
			case 4: // Arguments (extracted from command)
				_, args := a.splitCommand(run.Command)
				// Truncate very long arguments for better display
				if len(args) > 100 {
					args = args[:100] + "..."
				}
				label.SetText(args)
			case 5: // Type (Parallel/Grid indicators)
				runType := "Local"
				if run.IsGrid {
					runType = "Grid"
				} else if run.IsParallel {
					runType = fmt.Sprintf("Parallel (%d)", run.Cores)
				}
				label.SetText(runType)
			case 6: // Description
				description, err := run.GetDescription()
				if err != nil {
					log.Printf("Failed to decompress description: %v", err)
					description = run.Description // Fallback to legacy field
				}
				if description != "" {
					label.SetText(description)
				} else {
					label.SetText("--")
				}
			}
		},
	)

	// Set column widths for better display - optimized for readability
	a.runHistoryTable.SetColumnWidth(0, 50)  // Run #
	a.runHistoryTable.SetColumnWidth(1, 80)  // Status
	a.runHistoryTable.SetColumnWidth(2, 70)  // Time
	a.runHistoryTable.SetColumnWidth(3, 110) // Binary
	a.runHistoryTable.SetColumnWidth(4, 300) // Arguments (increased for better readability)
	a.runHistoryTable.SetColumnWidth(5, 90)  // Type
	a.runHistoryTable.SetColumnWidth(6, 120) // Description (reduced to save space)

	// Add selection handler
	a.runHistoryTable.OnSelected = func(id widget.TableCellID) {
		if id.Row < len(a.runHistory.Runs) {
			// Pass by pointer to avoid copying large compressed data
			a.displayRunDetails(&a.runHistory.Runs[id.Row])
		}
	}

	// Create table headers
	headerTable := widget.NewTable(
		func() (int, int) { return 1, 7 }, // 1 row, 7 columns for headers
		func() fyne.CanvasObject { return widget.NewRichTextFromMarkdown("**Header**") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row != 0 {
				return
			}

			richText, ok := obj.(*widget.RichText)
			if !ok {
				return
			}

			headers := []string{"**Run#**", "**Status**", "**Time**", "**Binary**", "**Arguments**", "**Type**", "**Description**"}
			if id.Col < len(headers) {
				richText.ParseMarkdown(headers[id.Col])
			}
		},
	)

	// Set the same column widths as the data table for perfect alignment
	headerTable.SetColumnWidth(0, 50)  // Run #
	headerTable.SetColumnWidth(1, 80)  // Status
	headerTable.SetColumnWidth(2, 70)  // Time
	headerTable.SetColumnWidth(3, 110) // Binary
	headerTable.SetColumnWidth(4, 300) // Arguments (increased for better readability)
	headerTable.SetColumnWidth(5, 90)  // Type
	headerTable.SetColumnWidth(6, 120) // Description (reduced to save space)

	// Right side: Output displays
	a.stdoutDisplay = widget.NewEntry()
	a.stdoutDisplay.MultiLine = true
	a.stdoutDisplay.Wrapping = fyne.TextWrapWord
	a.stdoutDisplay.SetText("No run selected...")

	a.stderrDisplay = widget.NewEntry()
	a.stderrDisplay.MultiLine = true
	a.stderrDisplay.Wrapping = fyne.TextWrapWord
	a.stderrDisplay.SetText("No run selected...")

	// Create tabbed output view
	a.outputTabs = container.NewAppTabs(
		container.NewTabItem("STDOUT", container.NewScroll(a.stdoutDisplay)),
		container.NewTabItem("STDERR", container.NewScroll(a.stderrDisplay)),
	)

	// Split layout: run table on left, output on right
	split := container.NewHSplit(
		container.NewBorder(
			container.NewVBox(widget.NewLabel("Run History"), headerTable), // Header table for column alignment
			nil, nil, nil,
			a.runHistoryTable,
		),
		container.NewBorder(
			widget.NewLabel("Output"),
			nil, nil, nil,
			a.outputTabs,
		),
	)
	split.SetOffset(0.5) // 50% for history table, 50% for output (balanced layout for wider Arguments column)

	return split
}

func (a *App) displayRunDetails(run *audit.RunRecord) {
	// Clear current displays immediately
	a.stdoutDisplay.SetText("")
	a.stderrDisplay.SetText("")

	// Do ALL decompression work in background
	go func(runPtr *audit.RunRecord) {
		// Decompress stdout/stderr
		stdout, err := runPtr.GetStdout()
		if err != nil {
			log.Printf("Failed to decompress stdout: %v", err)
			stdout = runPtr.Stdout // Fallback to legacy field
		}

		stderr, err := runPtr.GetStderr()
		if err != nil {
			log.Printf("Failed to decompress stderr: %v", err)
			stderr = runPtr.Stderr // Fallback to legacy field
		}

		// Update displays
		a.stdoutDisplay.SetText(stdout)
		a.stderrDisplay.SetText(stderr)

		// Build buttons container for embedded files
		var buttons []fyne.CanvasObject

		// Add "Run Message" button if description exists
		hasDescription := runPtr.DescriptionCompressed != "" || runPtr.Description != ""
		if hasDescription {
			messageBtn := widget.NewButton("Run Message", func() {
				a.showFileWindow(runPtr, "Run Message", "description")
			})
			messageBtn.Importance = widget.MediumImportance
			buttons = append(buttons, messageBtn)
		}

		// Add buttons for embedded output files
		extensions := audit.GetEmbeddedFileExtensions(runPtr)
		for _, ext := range extensions {
			extCopy := ext // Capture for closure
			fileBtn := widget.NewButton("."+ext, func() {
				a.showFileWindow(runPtr, "."+extCopy, extCopy)
			})
			buttons = append(buttons, fileBtn)
		}

		// Create buttons container if we have any buttons
		if len(buttons) > 0 {
			// Add export button
			exportBtn := widget.NewButton("Export (.zip)", func() {
				a.exportRunFiles(runPtr)
			})
			exportBtn.Importance = widget.HighImportance

			buttonsLabel := widget.NewLabel("Embedded Files:")
			buttonsGrid := container.NewGridWithColumns(4, buttons...)
			buttonsContainer := container.NewVBox(
				buttonsLabel,
				buttonsGrid,
				widget.NewSeparator(),
				exportBtn,
			)

			// Add to output tabs (replacing any existing "Files" tab)
			for i := len(a.outputTabs.Items) - 1; i >= 0; i-- {
				if a.outputTabs.Items[i].Text == "Files" {
					a.outputTabs.RemoveIndex(i)
				}
			}

			filesTab := container.NewTabItem("Files", buttonsContainer)
			a.outputTabs.Append(filesTab)
		}
	}(run)
}

// showFileWindow opens a separate window to display a file from the run record.
func (a *App) showFileWindow(run *audit.RunRecord, title, fileType string) {
	// Create new window for file display
	window := a.fyneApp.NewWindow(fmt.Sprintf("%s - Run #%d", title, run.ID))
	window.Resize(fyne.NewSize(800, 600))

	// Create loading label
	loadingLabel := widget.NewLabel("Loading...")
	window.SetContent(container.NewCenter(loadingLabel))
	window.Show()

	// Load content in background
	go func() {
		var content string
		var err error

		// Load content based on file type
		if fileType == "description" {
			content, err = run.GetDescription()
			if err != nil {
				log.Printf("Failed to decompress description: %v", err)
				content = run.Description
			}
		} else {
			// It's an embedded file
			data, extractErr := audit.ExtractEmbeddedFile(run, fileType)
			if extractErr != nil {
				err = extractErr
				content = fmt.Sprintf("Error: %v", err)
			} else {
				content = string(data)
			}
		}

		// Create read-only text display using RichText for better performance
		richText := widget.NewRichTextFromMarkdown("```\n" + content + "\n```")
		richText.Wrapping = fyne.TextWrapWord

		// Create scrollable container
		scroll := container.NewScroll(richText)

		// Add close button
		closeBtn := widget.NewButton("Close", func() {
			window.Close()
		})

		// Main layout
		windowContent := container.NewBorder(
			nil,
			container.NewHBox(widget.NewLabel(""), closeBtn), // Right-aligned close button
			nil,
			nil,
			scroll,
		)

		window.SetContent(windowContent)
	}()
}

// exportRunFiles exports all embedded files from a run record to a zip archive.
func (a *App) exportRunFiles(run *audit.RunRecord) {
	// Get model name from model file path
	modelName := strings.TrimSuffix(filepath.Base(run.ModelFile), filepath.Ext(run.ModelFile))
	defaultFileName := fmt.Sprintf("%s_run%d.zip", modelName, run.ID)

	// Show directory picker dialog
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}

		// Create full path for the zip file
		zipPath := filepath.Join(dir.Path(), defaultFileName)

		// Create zip archive in background
		go func() {
			// Create the file
			file, err := os.Create(zipPath)
			if err != nil {
				a.sendError(fmt.Errorf("failed to create zip file: %w", err))

				return
			}
			defer file.Close()

			// Write zip contents
			if err := a.createRunFilesZip(run, modelName, file); err != nil {
				a.sendError(fmt.Errorf("failed to create zip archive: %w", err))
			} else {
				a.showSuccessToast(fmt.Sprintf("Exported run #%d files to %s", run.ID, zipPath))
			}
		}()
	}, a.window)
}

// createRunFilesZip creates a zip archive containing all embedded files from a run.
func (a *App) createRunFilesZip(run *audit.RunRecord, modelName string, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Add description if available
	hasDescription := run.DescriptionCompressed != "" || run.Description != ""
	if hasDescription {
		description, err := run.GetDescription()
		if err != nil {
			log.Printf("Failed to decompress description: %v", err)
			description = run.Description
		}

		if description != "" {
			fileName := fmt.Sprintf("%s_run_message.txt", modelName)
			fileWriter, err := zipWriter.Create(fileName)
			if err != nil {
				return fmt.Errorf("failed to create zip entry for description: %w", err)
			}

			if _, err := fileWriter.Write([]byte(description)); err != nil {
				return fmt.Errorf("failed to write description to zip: %w", err)
			}
		}
	}

	// Add all embedded files
	extensions := audit.GetEmbeddedFileExtensions(run)
	for _, ext := range extensions {
		content, err := audit.ExtractEmbeddedFile(run, ext)
		if err != nil {
			log.Printf("Failed to extract embedded file %s: %v", ext, err)

			continue
		}

		fileName := fmt.Sprintf("%s.%s", modelName, ext)
		fileWriter, err := zipWriter.Create(fileName)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", fileName, err)
		}

		if _, err := fileWriter.Write(content); err != nil {
			return fmt.Errorf("failed to write %s to zip: %w", fileName, err)
		}
	}

	return nil
}

func (a *App) showSettingsPanel() {
	if a.settingsOpen {
		return // Settings already open
	}

	// Create settings window
	settingsWindow := a.fyneApp.NewWindow("Settings")
	settingsWindow.Resize(fyne.NewSize(400, 300))

	// Settings content from configuration
	appVersion := "dev"
	user := "unknown"
	organization := "BigPharma LLC"
	defaultDir := "~/models"
	nonmemPath := "/opt/NONMEM/nm76/run"
	nonmemBinary := "nmfe76"
	scheduler := "SLURM"
	executionMode := "NONMEM"
	validationIQ := "~/.config/janus/validation/iq-report.json"
	validationOQ := "~/.config/janus/validation/oq-report.json"

	if a.config != nil {
		appVersion = a.config.Version
		user = a.config.User
		organization = a.config.Organization
		defaultDir = a.config.DefaultDirectory
		nonmemPath = a.config.NonmemPath
		nonmemBinary = a.config.NonmemBinary
		scheduler = a.config.Scheduler
		executionMode = a.config.ExecutionMode
		validationIQ = a.config.Validation.IQ
		validationOQ = a.config.Validation.OQ
	}

	versionLabel := widget.NewLabel(fmt.Sprintf("Version: %s", appVersion))

	validationSection := container.NewVBox(
		widget.NewLabel("CFR 21 Part 11 Validation:"),
		widget.NewLabel(fmt.Sprintf("IQ Report: %s", validationIQ)),
		widget.NewLabel(fmt.Sprintf("OQ Report: %s", validationOQ)),
	)

	userSection := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("User: %s", user)),
		widget.NewLabel(fmt.Sprintf("Organization: %s", organization)),
		widget.NewLabel(fmt.Sprintf("Default Directory: %s", defaultDir)),
	)

	nonmemSection := container.NewVBox(
		widget.NewLabel("NONMEM Configuration:"),
		widget.NewLabel(fmt.Sprintf("Path: %s", nonmemPath)),
		widget.NewLabel(fmt.Sprintf("Binary: %s", nonmemBinary)),
		widget.NewLabel(fmt.Sprintf("Execution Mode: %s", executionMode)),
		widget.NewLabel(fmt.Sprintf("Scheduler: %s", scheduler)),
	)

	settingsContent := container.NewVBox(
		versionLabel,
		widget.NewSeparator(),
		validationSection,
		widget.NewSeparator(),
		userSection,
		widget.NewSeparator(),
		nonmemSection,
	)

	settingsWindow.SetContent(settingsContent)
	settingsWindow.SetOnClosed(func() {
		a.settingsOpen = false
	})

	a.settingsOpen = true
	settingsWindow.Show()
}

func (a *App) setModelLoaded(loaded bool) {
	// Note: dataEntry is always disabled since it's read-only from file

	if a.syncRadio != nil {
		if loaded {
			a.syncRadio.Enable()
		} else {
			a.syncRadio.Disable()
		}
	}

	if a.coresEntry != nil {
		if loaded {
			a.coresEntry.Enable()
		} else {
			a.coresEntry.Disable()
		}
	}

	if a.nonmemOptionsEntry != nil {
		if loaded {
			a.nonmemOptionsEntry.Enable()
		} else {
			a.nonmemOptionsEntry.Disable()
		}
	}

	if a.descriptionCheck != nil {
		if loaded {
			a.descriptionCheck.Enable()
		} else {
			a.descriptionCheck.Disable()
		}
	}

	if a.runHereBtn != nil {
		if loaded {
			a.runHereBtn.Enable()
		} else {
			a.runHereBtn.Disable()
		}
	}

	if a.runGridBtn != nil {
		if loaded {
			a.runGridBtn.Enable()
		} else {
			a.runGridBtn.Disable()
		}
	}

	if a.saveBtn != nil {
		if loaded {
			a.saveBtn.Enable()
		} else {
			a.saveBtn.Disable()
		}
	}

	if a.textEditor != nil {
		if loaded {
			a.textEditor.Enable()
			// Display the loaded file content
			a.textEditor.SetText(a.fileContent)
		} else {
			a.textEditor.Disable()
			a.textEditor.SetText("// Load a model file to begin editing...")
		}
	}

	// Note: Fyne doesn't support disabling individual tabs
	// The Run History tab will remain available but could show a message
	// when accessed without a loaded model
}

// LoadModelFile is a public method that loads a model file and sets up the GUI accordingly.
// This is used by the command line interface to auto-load models.
func (a *App) LoadModelFile(filePath string) error {
	// If UI components aren't ready yet, store the path for later loading
	if a.modelEntry == nil || a.textEditor == nil {
		a.pendingModelPath = filePath

		return nil
	}

	return a.loadModelFile(filePath)
}

func (a *App) loadModelFile(filePath string) error {
	// Cancel any existing file watcher
	if a.watchCancel != nil {
		a.watchCancel()
	}

	// Read the file content
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Store the file information
	a.currentFilePath = filePath
	a.fileContent = string(content)
	a.fileHash = sha256.Sum256(content)

	// Set up model-specific run history
	a.setupModelRunHistory(filePath)

	// Create the Run Details tab when model is loaded but don't switch to it
	// This ensures run history is available if the user wants to view it
	a.createRunDetailsTab()

	// Parse and update the $DATA variable
	dataValue, hasData := a.parseDataVariableWithValidation(a.fileContent)
	if a.dataEntry != nil {
		a.dataEntry.SetText(dataValue)
	}

	// Show warning if this doesn't appear to be a NONMEM model
	if !hasData {
		a.showWarningToast("This does not appear to be a NONMEM model - no $DATA section found.")
	}

	// Start file watching
	a.startFileWatcher()

	// Update the model entry field with the loaded file path
	if a.modelEntry != nil {
		a.modelEntry.SetText(filePath)
	}

	// Enable components when model is loaded
	a.setModelLoaded(true)

	return nil
}

func (a *App) saveModelFile() error {
	if a.currentFilePath == "" {
		return fmt.Errorf("no file loaded")
	}

	// Get the current content from the text editor
	content := a.textEditor.Text

	// Write the content to the file
	err := os.WriteFile(a.currentFilePath, []byte(content), 0600)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update the stored content and parse $DATA again
	a.fileContent = content
	a.fileHash = sha256.Sum256([]byte(content)) // Update hash after save
	dataValue, _ := a.parseDataVariableWithValidation(a.fileContent)
	if a.dataEntry != nil {
		a.dataEntry.SetText(dataValue)
	}

	return nil
}

func (a *App) parseDataVariableWithValidation(content string) (string, bool) {
	// Look for $DATA line in the NONMEM control file
	// Pattern matches: $DATA filename or $DATA filename IGNORE=...
	dataRegex := regexp.MustCompile(`(?i)\$DATA\s+([^\s]+)`)

	matches := dataRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), true
	}

	// Default value if no $DATA found
	return "data.csv", false
}

func (a *App) startFileWatcher() {
	// Create a new context for this file watcher
	a.watchCtx, a.watchCancel = context.WithCancel(a.errorCtx)

	go a.watchFileChanges()
}

func (a *App) watchFileChanges() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()

	for {
		select {
		case <-a.watchCtx.Done():
			// Context cancelled, stop watching
			return
		case <-ticker.C:
			// Check if file has changed
			if a.currentFilePath == "" {
				continue
			}

			// Read current file content
			content, err := os.ReadFile(a.currentFilePath)
			if err != nil {
				// File might have been deleted or moved, continue watching
				continue
			}

			// Compare hash
			currentHash := sha256.Sum256(content)
			if currentHash != a.fileHash {
				// File has changed, reload it
				a.reloadFileContent(string(content))
			}
		}
	}
}

func (a *App) reloadFileContent(newContent string) {
	// Update stored content and hash
	a.fileContent = newContent
	a.fileHash = sha256.Sum256([]byte(newContent))

	// Update UI on the main thread
	if a.textEditor != nil {
		a.textEditor.SetText(newContent)
	}

	// Parse and update the $DATA variable
	dataValue, _ := a.parseDataVariableWithValidation(newContent)
	if a.dataEntry != nil {
		a.dataEntry.SetText(dataValue)
	}
}

func (a *App) updateCoresVisibility(showCores bool) {
	if a.coresContainer != nil {
		if showCores {
			a.coresContainer.Show()
		} else {
			a.coresContainer.Hide()
		}
	}
}

func (a *App) adjustCores(delta int) {
	if a.coresEntry == nil {
		return
	}

	// Parse current value
	currentText := a.coresEntry.Text
	currentValue := 4 // Default if parsing fails

	if val, err := strconv.Atoi(currentText); err == nil {
		currentValue = val
	}

	// Adjust value with bounds checking
	newValue := currentValue + delta
	if newValue < 1 {
		newValue = 1
	}
	if newValue > 64 { // Reasonable upper limit
		newValue = 64
	}

	// Update the entry
	a.coresEntry.SetText(strconv.Itoa(newValue))
}

func (a *App) showDescriptionDialog(onComplete func(string)) {
	// Create description entry
	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Enter a description for this run (like a git commit message)...\n\nExample:\n- Fix population model parameters\n- Initial dose-response analysis\n- Updated covariate structure")
	descEntry.Resize(fyne.NewSize(400, 150))

	// Create dialog
	dialog := dialog.NewCustomConfirm(
		"Run Description",
		"Continue",
		"Cancel",
		descEntry,
		func(confirmed bool) {
			if confirmed {
				onComplete(strings.TrimSpace(descEntry.Text))
			}
		},
		a.window,
	)

	dialog.Resize(fyne.NewSize(500, 300))
	dialog.Show()
}

func (a *App) validateCoresInput() error {
	if a.coresEntry == nil {
		return fmt.Errorf("cores input not initialized")
	}

	coresText := strings.TrimSpace(a.coresEntry.Text)

	// Check if empty
	if coresText == "" {
		return fmt.Errorf("number of cores cannot be empty")
	}

	// Check if it's a valid integer
	coresVal, err := strconv.Atoi(coresText)
	if err != nil {
		return fmt.Errorf("number of cores must be a valid integer, got: %q", coresText)
	}

	// Check bounds
	if coresVal < 1 {
		return fmt.Errorf("number of cores must be at least 1, got: %d", coresVal)
	}

	if coresVal > 64 {
		return fmt.Errorf("number of cores cannot exceed 64, got: %d", coresVal)
	}

	return nil
}

func (a *App) validateCoresInputSilent(text string) {
	// This provides visual feedback without blocking the user
	// We could change text color, add visual indicators, etc.
	text = strings.TrimSpace(text)

	if text == "" {
		return // Allow empty during typing
	}

	if _, err := strconv.Atoi(text); err != nil {
		// Invalid integer - could add visual feedback here
		// For now, we just validate silently and let the blocking validation
		// handle the error display when they try to submit
		return
	}
}

func (a *App) executeRun(isGrid bool) {
	if a.currentFilePath == "" {
		dialog.ShowError(fmt.Errorf("no model file loaded"), a.window)

		return
	}

	// Check if parallel mode is selected
	isParallel := a.syncRadio.Selected == "Parallel"

	// Get number of cores if parallel
	cores := 1
	if isParallel {
		if err := a.validateCoresInput(); err != nil {
			dialog.ShowError(err, a.window)

			return
		}

		// Parse cores value (we know it's valid from validation)
		cores, _ = strconv.Atoi(strings.TrimSpace(a.coresEntry.Text))
	}

	// Check if user wants to add a description
	if a.descriptionCheck.Checked {
		a.showDescriptionDialog(func(description string) {
			a.executeRunWithDescription(isGrid, isParallel, cores, description)
		})
	} else {
		a.executeRunWithDescription(isGrid, isParallel, cores, "")
	}
}

func (a *App) executeRunWithDescription(isGrid, isParallel bool, cores int, description string) {
	// Get additional NONMEM options if execution mode is NONMEM
	var nonmemOptions *string
	if a.config != nil && a.config.ExecutionMode == "NONMEM" && a.nonmemOptionsEntry != nil {
		if opts := strings.TrimSpace(a.nonmemOptionsEntry.Text); opts != "" {
			nonmemOptions = &opts
		}
	}
	// Generate .pnm file if parallel mode
	if isParallel {
		if err := a.generatePnmFile(cores); err != nil {
			a.sendError(fmt.Errorf("failed to generate .pnm file: %w", err))

			return
		}
	}

	// Create run record with description and NONMEM options
	var desc *string
	if description != "" {
		desc = &description
	}
	runRecord := a.createRunRecord(isGrid, isParallel, cores, desc, nonmemOptions)

	// Execute the run
	if isGrid {
		a.executeGridRun(runRecord)
	} else {
		a.executeLocalRun(runRecord)
	}

	// Show Run Details tab if not already shown
	a.showRunDetailsTab()
}

func (a *App) generatePnmFile(cores int) error {
	if a.currentFilePath == "" {
		return fmt.Errorf("no model file loaded")
	}

	// Generate .pnm file path (same directory, same name but .pnm extension)
	modelDir := filepath.Dir(a.currentFilePath)
	modelName := strings.TrimSuffix(filepath.Base(a.currentFilePath), filepath.Ext(a.currentFilePath))
	pnmPath := filepath.Join(modelDir, modelName+".pnm")

	// Create .pnm file content (NONMEM parallel configuration)
	// Following BBI's approach with ORTE-based parallel execution
	pnmContent := fmt.Sprintf(`$GENERAL
NODES=%d PARSE_TYPE=2 TIMEOUTI=100 TIMEOUT=2400 PARAPRINT=0 TRANSFER_TYPE=1
`, cores)

	// Write .pnm file
	if err := os.WriteFile(pnmPath, []byte(pnmContent), 0600); err != nil {
		return fmt.Errorf("failed to write .pnm file: %w", err)
	}

	return nil
}

func (a *App) showToast(title, message string, duration time.Duration) {
	// Skip toast if no window (e.g., in tests)
	if a.window == nil {
		return
	}

	// Create toast content with title and message
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	messageLabel := widget.NewLabel(message)
	messageLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		titleLabel,
		messageLabel,
	)

	// Create popup with padding
	paddedContent := container.NewPadded(content)
	popup := widget.NewPopUp(paddedContent, a.window.Canvas())

	// Position at top-right corner
	canvasSize := a.window.Canvas().Size()
	toastWidth := float32(300)
	toastHeight := float32(80)
	popup.Resize(fyne.NewSize(toastWidth, toastHeight))
	popup.ShowAtPosition(fyne.NewPos(canvasSize.Width-toastWidth-20, 20))

	// Auto-dismiss after duration
	time.AfterFunc(duration, func() {
		popup.Hide()
	})
}

func (a *App) showSuccessToast(message string) {
	a.showToast("SUCCESS", message, 3*time.Second)
}

func (a *App) showInfoToast(title, message string) {
	a.showToast(strings.ToUpper(title), message, 4*time.Second)
}

func (a *App) showWarningToast(message string) {
	a.showToast("WARNING", message, 5*time.Second)
}

// buildActualExecutionCommand builds the actual command string that will be executed
// using the same logic as the executor to ensure audit trail accuracy.
func (a *App) buildActualExecutionCommand(isGrid, isParallel bool, cores int, nonmemOptions *string) string {
	// Handle different execution modes
	if a.config == nil {
		return fmt.Sprintf("nonmem %s", filepath.Base(a.currentFilePath))
	}

	var additionalOptions []string
	if nonmemOptions != nil && *nonmemOptions != "" {
		// Parse additional options (simple split for now)
		additionalOptions = strings.Fields(*nonmemOptions)
	}

	switch a.config.ExecutionMode {
	case "NONMEM":
		// Use the same logic as NONMEMExecutor
		return a.buildNONMEMCommandString(isParallel, cores, isGrid, additionalOptions)
	case "BBI":
		// Use BBI command format
		return a.buildBBICommandString(isParallel, cores, isGrid, additionalOptions)
	case "PSN":
		// Use PSN command format
		return a.buildPSNCommandString(isParallel, cores, isGrid, additionalOptions)
	default:
		return fmt.Sprintf("nonmem %s", filepath.Base(a.currentFilePath))
	}
}

// buildNONMEMCommandString builds the command string for NONMEM execution.
func (a *App) buildNONMEMCommandString(isParallel bool, cores int, isGrid bool, additionalOptions []string) string {
	// Build the full binary path
	nonmemPath := a.config.NonmemPath
	nonmemBinary := a.config.NonmemBinary
	if nonmemBinary == "" {
		nonmemBinary = "nmfe75" // Default
	}

	// Join path and convert to absolute path
	binaryPath := filepath.Join(nonmemPath, nonmemBinary)
	if absPath, err := filepath.Abs(binaryPath); err == nil {
		binaryPath = absPath
	}

	// Build arguments - use absolute paths for grid execution
	var modelPath string
	if isGrid {
		// For grid execution, ensure model path is absolute for NFS shared storage
		if absModelPath, err := filepath.Abs(a.currentFilePath); err == nil {
			modelPath = absModelPath
		} else {
			modelPath = a.currentFilePath // Fallback to relative path
		}
	} else {
		modelPath = a.currentFilePath
	}

	args := []string{modelPath}

	// Handle parallel execution
	if isParallel && cores > 1 {
		parafilePath := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".pnm"
		if isGrid {
			// For grid execution, ensure parafile path is also absolute
			if absPnmPath, err := filepath.Abs(parafilePath); err == nil {
				args = append(args, fmt.Sprintf("-parafile=%s", absPnmPath))
			} else {
				args = append(args, fmt.Sprintf("-parafile=%s", parafilePath))
			}
		} else {
			args = append(args, fmt.Sprintf("-parafile=%s", parafilePath))
		}
	}

	// Add additional options
	if additionalOptions != nil {
		args = append(args, additionalOptions...)
	}

	// Return full command string
	return fmt.Sprintf("%s %s", binaryPath, strings.Join(args, " "))
}

// buildBBICommandString builds the command string for BBI execution.
func (a *App) buildBBICommandString(isParallel bool, cores int, isGrid bool, additionalOptions []string) string {
	// Use absolute path for grid execution
	var modelPath string
	if isGrid {
		if absModelPath, err := filepath.Abs(a.currentFilePath); err == nil {
			modelPath = absModelPath
		} else {
			modelPath = a.currentFilePath
		}
	} else {
		modelPath = a.currentFilePath
	}

	args := []string{"bbi", "nonmem", "run", "local", modelPath}

	// Add parallel execution options
	if isParallel && cores > 1 {
		args = append(args, "--parallel", fmt.Sprintf("--threads=%d", cores))
	}

	// Add grid execution options
	if isGrid {
		// Change "local" to "sge" for grid execution
		args[3] = "sge"
	}

	// Add additional options
	if additionalOptions != nil {
		args = append(args, additionalOptions...)
	}

	return strings.Join(args, " ")
}

// buildPSNCommandString builds the command string for PSN execution.
func (a *App) buildPSNCommandString(isParallel bool, cores int, isGrid bool, additionalOptions []string) string {
	// Use absolute path for grid execution
	var modelPath string
	if isGrid {
		if absModelPath, err := filepath.Abs(a.currentFilePath); err == nil {
			modelPath = absModelPath
		} else {
			modelPath = a.currentFilePath
		}
	} else {
		modelPath = a.currentFilePath
	}

	args := []string{"execute", modelPath}

	// Add parallel execution options
	if isParallel && cores > 1 {
		args = append(args, fmt.Sprintf("-threads=%d", cores))
	}

	// Add grid execution options
	if isGrid {
		args = append(args, "-slurm")
	}

	// Add additional options
	if additionalOptions != nil {
		args = append(args, additionalOptions...)
	}

	return strings.Join(args, " ")
}

// splitCommand splits a command string into binary and arguments for table display.
func (a *App) splitCommand(command string) (binary string, args string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", ""
	}

	binary = filepath.Base(parts[0]) // Just show the binary name, not full path
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}

	return binary, args
}

func (a *App) createRunRecord(isGrid, isParallel bool, cores int, description *string, nonmemOptions *string) *audit.RunRecord {
	// Build the actual command that will be executed using the same logic as the executor
	command := a.buildActualExecutionCommand(isGrid, isParallel, cores, nonmemOptions)

	runRecord := &audit.RunRecord{
		ID:         a.getNextRunID(),
		Timestamp:  time.Now(),
		ModelFile:  a.currentFilePath,
		Command:    command,
		ExitCode:   -1, // Not completed yet
		Stdout:     "",
		Stderr:     "",
		IsParallel: isParallel,
		Cores:      cores,
		IsGrid:     isGrid,
		Status:     "running",
	}

	// Only set description if provided (use compressed storage)
	if description != nil && *description != "" {
		if err := runRecord.SetDescription(*description); err != nil {
			log.Printf("Failed to compress description: %v", err)
			runRecord.Description = *description // Fallback to uncompressed
		}
	}

	// Only set NONMEM options if provided and execution mode is NONMEM
	if nonmemOptions != nil && *nonmemOptions != "" && a.config != nil && a.config.ExecutionMode == "NONMEM" {
		runRecord.NonmemOptions = nonmemOptions
	}

	// Add to history
	a.runHistory.Runs = append(a.runHistory.Runs, *runRecord)

	// Save to JSON file
	a.saveRunHistory()

	return runRecord
}

func (a *App) showRunDetailsTab() {
	// Check if Run Details tab already exists
	for _, item := range a.modelSubTabs.Items {
		if item.Text == "Run Details" {
			a.modelSubTabs.Select(item)

			return
		}
	}

	// Create and add Run Details tab
	a.runDetailsTab = container.NewTabItem("Run Details", a.buildRunDetailsTab())
	a.modelSubTabs.Append(a.runDetailsTab)
	a.modelSubTabs.Select(a.runDetailsTab)

	// Refresh the run history table
	a.runHistoryTable.Refresh()
}

func (a *App) createRunDetailsTab() {
	// Check if modelSubTabs is initialized yet
	if a.modelSubTabs == nil {
		// UI not built yet, mark that we need to create the tab later
		a.needsRunDetailsTab = true

		return
	}

	// Check if Run Details tab already exists
	for _, item := range a.modelSubTabs.Items {
		if item.Text == "Run Details" {
			return // Tab already exists, don't create another
		}
	}

	// Create and add Run Details tab without selecting it
	a.runDetailsTab = container.NewTabItem("Run Details", a.buildRunDetailsTab())
	a.modelSubTabs.Append(a.runDetailsTab)

	// Refresh the run history table
	a.runHistoryTable.Refresh()
}

func (a *App) updateRunRecord(runID int, exitCode int, stdout, stderr string) {
	for i, run := range a.runHistory.Runs {
		if run.ID == runID {
			a.runHistory.Runs[i].ExitCode = exitCode

			// Use compressed storage for stdout/stderr
			if err := a.runHistory.Runs[i].SetStdout(stdout); err != nil {
				log.Printf("Failed to compress stdout for run %d: %v", runID, err)
				a.runHistory.Runs[i].Stdout = stdout // Fallback to uncompressed
			}
			if err := a.runHistory.Runs[i].SetStderr(stderr); err != nil {
				log.Printf("Failed to compress stderr for run %d: %v", runID, err)
				a.runHistory.Runs[i].Stderr = stderr // Fallback to uncompressed
			}

			if exitCode == 0 {
				a.runHistory.Runs[i].Status = "completed"

				// Embed output files for successful runs
				if err := audit.EmbedOutputFiles(&a.runHistory.Runs[i], run.ModelFile); err != nil {
					log.Printf("Failed to embed output files for run %d: %v", runID, err)
					// Don't fail the run, just log the error
				}
			} else {
				a.runHistory.Runs[i].Status = "failed"
			}

			break
		}
	}

	// Save updated history
	a.saveRunHistory()

	// Refresh UI if Run Details tab is visible
	if a.runHistoryTable != nil {
		a.runHistoryTable.Refresh()
	}
}

func (a *App) setupModelRunHistory(modelFilePath string) {
	// Create history file path: same directory as model, same name with .janus_history.json extension
	modelDir := filepath.Dir(modelFilePath)
	modelName := strings.TrimSuffix(filepath.Base(modelFilePath), filepath.Ext(modelFilePath))
	historyFile := filepath.Join(modelDir, modelName+".janus_history.json")

	// Update run history file path
	a.runHistory.HistoryFile = historyFile

	// Load existing history if file exists
	a.loadRunHistory()

	// If run history table exists, refresh it to show loaded data
	if a.runHistoryTable != nil {
		a.runHistoryTable.Refresh()
	}
}

func (a *App) loadRunHistory() {
	if a.runHistory.HistoryFile == "" {
		return
	}

	// Check if history file exists
	if _, err := os.Stat(a.runHistory.HistoryFile); os.IsNotExist(err) {
		// File doesn't exist, start with empty history
		a.runHistory.Runs = []audit.RunRecord{}

		return
	}

	// Read and parse existing history file
	data, err := os.ReadFile(a.runHistory.HistoryFile)
	if err != nil {
		// Failed to read, start with empty history
		a.runHistory.Runs = []audit.RunRecord{}

		return
	}

	// Parse JSON
	var loadedHistory audit.RunHistory
	if err := json.Unmarshal(data, &loadedHistory); err != nil {
		// Failed to parse, start with empty history
		a.runHistory.Runs = []audit.RunRecord{}

		return
	}

	// Update current history with loaded data
	a.runHistory.Runs = loadedHistory.Runs
}

func (a *App) saveRunHistory() {
	if a.runHistory.HistoryFile == "" {
		a.sendError(fmt.Errorf("cannot save run history: no file path set"))

		return
	}

	data, err := json.MarshalIndent(a.runHistory, "", "  ")
	if err != nil {
		a.sendError(fmt.Errorf("failed to marshal run history: %w", err))

		return
	}

	if err := os.WriteFile(a.runHistory.HistoryFile, data, 0600); err != nil {
		a.sendError(fmt.Errorf("failed to save run history to %s: %w", a.runHistory.HistoryFile, err))
	}
}

func (a *App) buildActiveRunsTable() *fyne.Container {
	// Get currently running local executions
	runningLocalRuns := a.getRunningLocalRuns()

	if len(runningLocalRuns) == 0 {
		return nil // No active runs, don't show table
	}

	// Create table for active runs
	a.activeRunsTable = widget.NewTable(
		func() (int, int) {
			return len(runningLocalRuns), 6 // rows, columns: Run, Model, Type, Elapsed, Live Output, Cancel
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel(""))
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row >= len(runningLocalRuns) {
				return
			}

			run := runningLocalRuns[id.Row]
			container, ok := obj.(*fyne.Container)
			if !ok {
				return
			}
			container.Objects = nil // Clear existing objects

			elapsed := time.Since(run.Timestamp)

			switch id.Col {
			case 0: // Run ID
				container.Add(widget.NewLabel(fmt.Sprintf("Run #%d", run.ID)))
			case 1: // Model name
				modelName := filepath.Base(run.ModelFile)
				container.Add(widget.NewLabel(modelName))
			case 2: // Execution type
				execType := "Synchronous"
				if run.IsParallel {
					execType = fmt.Sprintf("Parallel (%d cores)", run.Cores)
				}
				container.Add(widget.NewLabel(execType))
			case 3: // Elapsed time
				container.Add(widget.NewLabel(formatDuration(elapsed)))
			case 4: // Live Output button
				liveBtn := widget.NewButton("Live Output", func() {
					a.showLiveOutput(run.ID)
				})
				liveBtn.Importance = widget.MediumImportance
				// Only enable if streaming is available for this run
				if _, hasStream := a.activeStreams[run.ID]; !hasStream {
					liveBtn.Disable()
				}
				container.Add(liveBtn)
			case 5: // Cancel button
				cancelBtn := widget.NewButton("Cancel", func() {
					a.cancelRun(run.ID)
				})
				cancelBtn.Importance = widget.DangerImportance
				container.Add(cancelBtn)
			}
		},
	)

	// Set column headers
	a.activeRunsTable.SetColumnWidth(0, 80)  // Run ID
	a.activeRunsTable.SetColumnWidth(1, 200) // Model name
	a.activeRunsTable.SetColumnWidth(2, 150) // Execution type
	a.activeRunsTable.SetColumnWidth(3, 100) // Elapsed time
	a.activeRunsTable.SetColumnWidth(4, 100) // Live Output button
	a.activeRunsTable.SetColumnWidth(5, 80)  // Cancel button

	// Create header row
	headerContainer := container.NewHBox(
		widget.NewLabelWithStyle("Active Local Executions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	return container.NewVBox(
		headerContainer,
		a.activeRunsTable,
		widget.NewSeparator(),
		widget.NewLabel(""), // Extra spacing
	)
}

func (a *App) getRunningLocalRuns() []audit.RunRecord {
	var runningRuns []audit.RunRecord
	for _, run := range a.runHistory.Runs {
		if run.Status == "running" && !run.IsGrid {
			runningRuns = append(runningRuns, run)
		}
	}

	return runningRuns
}

func (a *App) updateActiveRunsDisplay() {
	if a.mainLayout == nil {
		return
	}

	// Remove existing active runs table if present
	for i, obj := range a.mainLayout.Objects {
		if container, ok := obj.(*fyne.Container); ok {
			// Check if this is our active runs container by looking for the header
			if len(container.Objects) > 0 {
				if label, ok := container.Objects[0].(*fyne.Container); ok {
					if len(label.Objects) > 0 {
						if headerLabel, ok := label.Objects[0].(*widget.Label); ok {
							if headerLabel.Text == "Active Local Executions" {
								a.mainLayout.Objects = append(a.mainLayout.Objects[:i], a.mainLayout.Objects[i+1:]...)

								break
							}
						}
					}
				}
			}
		}
	}

	// Build new active runs table
	activeRunsContainer := a.buildActiveRunsTable()
	if activeRunsContainer != nil {
		// Insert before the last item (grid details)
		if len(a.mainLayout.Objects) > 0 {
			// Insert before grid details (last item)
			lastItem := a.mainLayout.Objects[len(a.mainLayout.Objects)-1]
			a.mainLayout.Objects = a.mainLayout.Objects[:len(a.mainLayout.Objects)-1]
			a.mainLayout.Objects = append(a.mainLayout.Objects, activeRunsContainer, lastItem)
		} else {
			a.mainLayout.Objects = append(a.mainLayout.Objects, activeRunsContainer)
		}
	}

	a.mainLayout.Refresh()
}

func (a *App) showLiveOutput(runID int) {
	// Check if window already exists
	if window, exists := a.liveOutputWindows[runID]; exists {
		window.RequestFocus()

		return
	}

	// Check if streaming is available
	streaming, exists := a.activeStreams[runID]
	if !exists {
		a.sendError(fmt.Errorf("no live output available for run #%d", runID))

		return
	}

	// Create new window for live output
	window := a.fyneApp.NewWindow(fmt.Sprintf("Live Output - Run #%d", runID))
	window.Resize(fyne.NewSize(800, 600))

	// Create output displays
	stdoutDisplay := widget.NewEntry()
	stdoutDisplay.MultiLine = true
	stdoutDisplay.Wrapping = fyne.TextWrapWord
	stdoutDisplay.SetText("")

	stderrDisplay := widget.NewEntry()
	stderrDisplay.MultiLine = true
	stderrDisplay.Wrapping = fyne.TextWrapWord
	stderrDisplay.SetText("")

	// Create scrollable containers
	stdoutScroll := container.NewScroll(stdoutDisplay)
	stderrScroll := container.NewScroll(stderrDisplay)

	// Create tabs for stdout and stderr
	outputTabs := container.NewAppTabs(
		container.NewTabItem("STDOUT", stdoutScroll),
		container.NewTabItem("STDERR", stderrScroll),
	)

	// Add close button
	closeBtn := widget.NewButton("Close", func() {
		window.Close()
	})

	// Create status label
	statusLabel := widget.NewLabel("Status: Running...")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Main layout
	content := container.NewBorder(
		container.NewVBox(statusLabel, widget.NewSeparator()),
		container.NewHBox(widget.NewLabel(""), closeBtn), // Right-aligned close button
		nil,
		nil,
		outputTabs,
	)

	window.SetContent(content)

	// Store window reference
	a.liveOutputWindows[runID] = window

	// Set up cleanup when window is closed
	window.SetOnClosed(func() {
		delete(a.liveOutputWindows, runID)
	})

	// Start goroutine to handle streaming output
	go func() {
		var stdoutLines, stderrLines []string

		for {
			select {
			case line, ok := <-streaming.Stdout:
				if ok {
					stdoutLines = append(stdoutLines, line)
					stdoutDisplay.SetText(strings.Join(stdoutLines, "\n"))
					// Auto-scroll to bottom
					stdoutScroll.ScrollToBottom()
				}
			case line, ok := <-streaming.Stderr:
				if ok {
					stderrLines = append(stderrLines, line)
					stderrDisplay.SetText(strings.Join(stderrLines, "\n"))
					// Auto-scroll to bottom
					stderrScroll.ScrollToBottom()
				}
			case <-streaming.Done:
				statusLabel.SetText("Status: Completed")

				return
			}
		}
	}()

	window.Show()
}

func (a *App) cancelRun(runID int) {
	if cancelFunc, exists := a.activeRuns[runID]; exists {
		cancelFunc() // Cancel the context
		delete(a.activeRuns, runID)

		// Update run status to failed with cancellation message
		a.updateRunRecord(runID, -1, "", "Run cancelled by user")

		a.showInfoToast("Run Cancelled", fmt.Sprintf("Run #%d has been cancelled", runID))

		// Update the display
		a.updateActiveRunsDisplay()
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// parseNonmemOptions parses a string of NONMEM options into a slice of individual options.
// Example: "-maxeval=9999 -files=100" becomes []string{"-maxeval=9999", "-files=100"}
//
//nolint:godot
func parseNonmemOptions(options string) []string {
	if options == "" {
		return nil
	}

	// Split by spaces, but keep quoted arguments together
	// For now, use simple space splitting - could be enhanced later for quoted args
	parts := strings.Fields(strings.TrimSpace(options))

	// Filter out empty strings
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

func (a *App) executeLocalRun(runRecord *audit.RunRecord) {
	modeText := "synchronous"
	if runRecord.IsParallel {
		modeText = fmt.Sprintf("parallel (%d cores)", runRecord.Cores)
	}

	a.showInfoToast("Run Started",
		fmt.Sprintf("Starting local %s NONMEM run...\nModel: %s",
			modeText, filepath.Base(a.currentFilePath)))

	// Create executor factory and get the appropriate executor
	factory := execution.NewExecutorFactory(a.config)
	if concreteFactory, ok := factory.(*execution.DefaultExecutorFactory); ok {
		concreteFactory.SetAuditEnabled(a.HasFeature("audit"))
	}
	executor, err := factory.CreateExecutor(a.config.ExecutionMode)
	if err != nil {
		a.sendError(fmt.Errorf("failed to create executor: %w", err))
		a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Executor creation failed: %s", err.Error()))

		return
	}

	// Show the active runs table immediately
	a.updateActiveRunsDisplay()

	// Execute in background goroutine
	go func() {
		// Use error context for execution timeout
		ctx, cancel := context.WithTimeout(a.errorCtx, 30*time.Minute)
		defer func() {
			cancel()
			// Remove from active runs when complete
			delete(a.activeRuns, runRecord.ID)
			// Update display to hide table if no more active runs
			a.updateActiveRunsDisplay()
		}()

		// Store cancel function for this run
		a.activeRuns[runRecord.ID] = cancel

		// Check if executor supports streaming
		if streamingExecutor, ok := executor.(execution.StreamingExecutor); ok {
			// Parse additional NONMEM options
			var additionalOptions []string
			if runRecord.NonmemOptions != nil {
				additionalOptions = parseNonmemOptions(*runRecord.NonmemOptions)
			}

			// Use streaming execution
			streaming, result, err := streamingExecutor.ExecuteWithStreaming(ctx, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, false, additionalOptions)
			if err != nil {
				a.sendError(fmt.Errorf("execution failed: %w", err))
				a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Execution failed: %s", err.Error()))

				return
			}

			// Store streaming output for live viewing
			a.activeStreams[runRecord.ID] = streaming

			// Wait for completion and get final result
			<-streaming.Done

			// Update run record with results
			stdout := string(result.Stdout)
			stderr := string(result.Stderr)
			a.updateRunRecord(runRecord.ID, result.ExitCode, stdout, stderr)

			// Clean up streaming data
			delete(a.activeStreams, runRecord.ID)

			// Show completion notification
			if result.ExitCode == 0 {
				a.showSuccessToast(fmt.Sprintf("NONMEM run #%d completed successfully", runRecord.ID))
			} else {
				a.sendError(fmt.Errorf("NONMEM run #%d failed with exit code %d", runRecord.ID, result.ExitCode))
			}

			return
		}

		// Parse additional NONMEM options
		var additionalOptions []string
		if runRecord.NonmemOptions != nil {
			additionalOptions = parseNonmemOptions(*runRecord.NonmemOptions)
		}

		// Fall back to regular execution if streaming not supported
		result, err := executor.Execute(ctx, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, false, additionalOptions)
		if err != nil {
			a.sendError(fmt.Errorf("execution failed: %w", err))
			a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Execution failed: %s", err.Error()))

			return
		}

		// Convert []byte to string for storage/display
		stdout := string(result.Stdout)
		stderr := string(result.Stderr)

		// Update run record with results
		a.updateRunRecord(runRecord.ID, result.ExitCode, stdout, stderr)

		// Show completion notification
		if result.ExitCode == 0 {
			a.showSuccessToast(fmt.Sprintf("NONMEM run #%d completed successfully", runRecord.ID))
		} else {
			a.sendError(fmt.Errorf("NONMEM run #%d failed with exit code %d", runRecord.ID, result.ExitCode))
		}
	}()
}

func (a *App) showGridConfigurationModal() {
	a.ShowGridConfigModal(func(result *GridConfigResult) {
		if result.Cancelled {
			return
		}

		// Save settings if requested
		if result.SaveAsTemplate {
			if err := a.saveGridSettings(result.Settings); err != nil {
				a.sendError(fmt.Errorf("failed to save grid settings: %w", err))
				// Continue with execution even if save fails
			}

			// TODO: Add audit trail for settings save
			a.showSuccessToast("Grid settings saved for this model")
		}

		// Execute with grid settings
		a.executeGridRunWithSettings(result.Settings)
	})
}

func (a *App) executeGridRunWithSettings(gridSettings *GridSettings) {
	// Check if user wants to add a description
	if a.descriptionCheck.Checked {
		a.showDescriptionDialog(func(description string) {
			a.executeGridRunWithDescription(gridSettings, description)
		})
	} else {
		a.executeGridRunWithDescription(gridSettings, "")
	}
}

func (a *App) executeGridRunWithDescription(gridSettings *GridSettings, description string) {
	// Create run record using grid settings
	runRecord := a.createGridRunRecord(gridSettings, description)

	// Execute the grid run
	a.executeGridRun(runRecord)
}

func (a *App) createGridRunRecord(gridSettings *GridSettings, description string) *audit.RunRecord {
	// Build command based on grid settings
	var additionalOptions []string
	if len(gridSettings.NONMEM.AdditionalOptions) > 0 {
		additionalOptions = gridSettings.NONMEM.AdditionalOptions
	}

	var nonmemOptions *string
	if len(additionalOptions) > 0 {
		optionsStr := strings.Join(additionalOptions, " ")
		nonmemOptions = &optionsStr
	}

	command := a.buildActualExecutionCommand(
		true, // isGrid
		gridSettings.NONMEM.Parallel,
		gridSettings.NONMEM.Threads,
		nonmemOptions,
	)

	runRecord := &audit.RunRecord{
		ID:         a.getNextRunID(),
		Timestamp:  time.Now(),
		ModelFile:  a.currentFilePath,
		Command:    command,
		ExitCode:   -1, // Not completed yet
		Stdout:     "",
		Stderr:     "",
		IsParallel: gridSettings.NONMEM.Parallel,
		Cores:      gridSettings.NONMEM.Threads,
		IsGrid:     true,
		Status:     "running",
	}

	// Add description if provided (use compressed storage)
	if description != "" {
		if err := runRecord.SetDescription(description); err != nil {
			log.Printf("Failed to compress description: %v", err)
			runRecord.Description = description // Fallback to uncompressed
		}
	}

	// Add NONMEM options if provided
	if len(gridSettings.NONMEM.AdditionalOptions) > 0 {
		optionsStr := strings.Join(gridSettings.NONMEM.AdditionalOptions, " ")
		runRecord.NonmemOptions = &optionsStr
	}

	// Add to history and save
	a.runHistory.Runs = append(a.runHistory.Runs, *runRecord)
	a.saveRunHistory()

	return runRecord
}

func (a *App) executeGridRun(runRecord *audit.RunRecord) {
	modeText := "synchronous"
	if runRecord.IsParallel {
		modeText = fmt.Sprintf("parallel (%d cores)", runRecord.Cores)
	}

	a.showInfoToast("Run Started",
		fmt.Sprintf("Submitting %s NONMEM job to grid...\nModel: %s",
			modeText, filepath.Base(a.currentFilePath)))

	// Create executor factory and get the appropriate executor
	factory := execution.NewExecutorFactory(a.config)
	if concreteFactory, ok := factory.(*execution.DefaultExecutorFactory); ok {
		concreteFactory.SetAuditEnabled(a.HasFeature("audit"))
	}
	executor, err := factory.CreateExecutor(a.config.ExecutionMode)
	if err != nil {
		a.sendError(fmt.Errorf("failed to create executor: %w", err))
		a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Executor creation failed: %s", err.Error()))

		return
	}

	// Execute in background goroutine
	go func() {
		// Use error context for execution timeout (longer for grid jobs)
		ctx, cancel := context.WithTimeout(a.errorCtx, 2*time.Hour)
		defer cancel()

		// Parse additional NONMEM options
		var additionalOptions []string
		if runRecord.NonmemOptions != nil {
			additionalOptions = parseNonmemOptions(*runRecord.NonmemOptions)
		}

		// Execute the model on grid with job ID for better job naming
		var result *execution.ExecutionResult
		var err error

		// Check if this is a NONMEM executor and use ExecuteWithJobID for better SLURM job naming
		if nonmemExecutor, ok := executor.(*execution.NONMEMExecutor); ok {
			result, err = nonmemExecutor.ExecuteWithJobID(ctx, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, true, additionalOptions, runRecord.ID)
		} else {
			// Fall back to standard Execute method for other executors
			result, err = executor.Execute(ctx, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, true, additionalOptions)
		}
		if err != nil {
			a.sendError(fmt.Errorf("grid execution failed: %w", err))
			a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Grid execution failed: %s", err.Error()))

			return
		}

		// Convert []byte to string for storage/display
		stdout := string(result.Stdout)
		stderr := string(result.Stderr)

		// Update run record with results
		a.updateRunRecord(runRecord.ID, result.ExitCode, stdout, stderr)

		// Show completion notification
		if result.ExitCode == 0 {
			a.showSuccessToast(fmt.Sprintf("Grid job #%d completed successfully", runRecord.ID))
		} else {
			a.sendError(fmt.Errorf("grid job #%d failed with exit code %d", runRecord.ID, result.ExitCode))
		}
	}()
}
