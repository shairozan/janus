package gui

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/appsetup"
	"github.com/pharmalytica/janus/internal/comparison"
	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution"
	"github.com/pharmalytica/janus/internal/execution/category"
	"github.com/pharmalytica/janus/internal/gui/editor"
	"github.com/pharmalytica/janus/internal/license/validator"
	"github.com/pharmalytica/janus/internal/mcp"
	"github.com/pharmalytica/janus/internal/mcpservice"
	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/signing"
	"github.com/pharmalytica/janus/internal/visualization"
)

type App struct {
	fyneApp            fyne.App
	window             fyne.Window
	settingsOpen       bool
	needsRunDetailsTab bool
	pendingModelPath   string
	config             *config.Config
	licenseClaims      *validator.Claims

	// Current loaded file
	currentFilePath string
	fileContent     string
	fileHash        [32]byte

	// Model category and license state (for multi-modal support)
	currentModelCategory   category.CategoryType
	modelingLicenseFound   bool
	modelingLicenseMessage string

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
	versionSelect          *widget.Select // per-run NONMEM installation picker
	versionContainer       *fyne.Container
	psnPresetSelect        *widget.Select // PSN analysis preset (execute / vpc / bootstrap / …)
	psnPresetContainer     *fyne.Container
	psnForm                *psnFunctionForm // per-function argument controls
	runRemote              bool             // true when the current run targets SSH (direct remote)
	descriptionCheck       *widget.Check
	runBtn                 *widget.Button     // single Run button
	targetRadio            *widget.RadioGroup // Here / Scheduler / SSH
	hermesConfigBtn        *widget.Button
	retainInfoBox          *fyne.Container // read-only "retained files" summary for the current model (Hermes mode)
	retainGlobsLabel       *widget.Label
	commandPreview         *widget.Label     // live read-only preview of the command to run
	textEditor             *widget.Entry     // Legacy - will be replaced with nonmemEditor
	nonmemEditor           fyne.CanvasObject // Custom NONMEM syntax-highlighting editor
	modelSubTabs           *container.AppTabs
	saveBtn                *widget.Button

	// Run details components
	runDetailsTab       *container.TabItem
	outputTabs          *container.AppTabs
	stdoutDisplay       *widget.Entry
	stderrDisplay       *widget.Entry
	parametersContainer *fyne.Container
	runHistoryTable     *widget.Table
	runLogStore         *runlog.RunLogStore
	cachedRuns          []runlog.RunRecord // Local cache of runs for table display

	// Run comparison components
	selectedRunsForCompare map[string]bool // Run IDs selected for comparison
	compareButton          *widget.Button  // "Compare Selected" button
	compareButtonContainer *fyne.Container // Container to show/hide compare button

	// Run visualization components
	visualizeButton *widget.Button // "Visualize Selected" button

	// Single-run diagnostics
	selectedRun             *runlog.RunRecord // Currently selected run for display
	diagnosticsButton       *widget.Button    // "View GOF Plots" button for single-run diagnostics
	inheritParametersButton *widget.Button    // "Inherit Parameters" button for parameter inheritance

	// Active runs tracking
	activeRuns        map[string]context.CancelFunc         // runID -> cancel function
	activeStreams     map[string]*execution.StreamingOutput // runID -> streaming output
	liveOutputWindows map[string]fyne.Window                // runID -> live output window
	activeRunsTable   *widget.Table
	mainLayout        *fyne.Container
	activeRunsArea    *fyne.Container // persistent bottom slot for the active-runs / saga card

	// Live bootstrap-saga progress, keyed by run ID: published by the saga as it
	// runs and read by the active-runs display to render the progress card.
	// sagaMu guards the map.
	sagaProgress map[string]execution.SagaProgress
	sagaMu       sync.Mutex
	mainTabs     *container.AppTabs // top-level tabs (Model Run, Models)
	modelBrowser *modelBrowser      // "Models" tab state
	updateTicker *time.Ticker

	// SLURM monitoring
	slurmMonitor *SLURMMonitor

	// Dialog tracking
	cancelJobDialog *widget.PopUp
	errorDialog     *widget.PopUp

	// Run log signing (nil if signing not configured)
	signer *signing.Signer

	// trustStore is the run-log verification trust anchor, constructed at the
	// highest layer (internal/appsetup) from the license's signing key and handed
	// down here. A nil trustStore (signing not configured, or no license key)
	// degrades verification to Unverifiable — never Valid.
	trustStore runlog.TrustStore

	// modelMu guards currentFilePath and runLogStore against concurrent access
	// from MCP HTTP goroutines while the UI thread reassigns them in LoadModelFile.
	modelMu sync.Mutex

	// MCP server (nil unless enabled). ephemeralResolver builds/caches run-log
	// stores for models other than the currently-loaded one; it is shared with
	// the fyne-free mcpservice that backs both the GUI and the daemon.
	mcpMu             sync.Mutex
	mcpServer         *mcp.Server
	ephemeralResolver *mcpservice.EphemeralResolver
}

func NewApp(ctx context.Context) *App {
	fyneApp := app.New()
	window := fyneApp.NewWindow("Janus")
	window.Resize(fyne.NewSize(1200, 800))

	// Create error handling context and channel
	errorCtx, errorCancel := context.WithCancel(ctx)
	errorCh := make(chan error, 100) // Buffered channel for async errors

	app := &App{
		fyneApp:                fyneApp,
		window:                 window,
		cachedRuns:             []runlog.RunRecord{},
		errorCh:                errorCh,
		errorCtx:               errorCtx,
		errorCancel:            errorCancel,
		activeRuns:             make(map[string]context.CancelFunc),
		sagaProgress:           make(map[string]execution.SagaProgress),
		activeStreams:          make(map[string]*execution.StreamingOutput),
		liveOutputWindows:      make(map[string]fyne.Window),
		selectedRunsForCompare: make(map[string]bool),
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

	// Initialize run log signer if signing is configured
	a.initSigner()

	// Construct the verification trust anchor from the license's signing key.
	a.initTrustStore()

	// One-shot startup belt: clear any Hermes pods orphaned by a previously killed
	// run. Called once at startup (both the wizard and normal paths land here;
	// settings saves use reloadAppConfig instead), so it never fires mid-run.
	a.sweepOrphanedHermesPodsAsync()
}

// sweepOrphanedHermesPodsAsync fires a one-shot, best-effort background sweep of
// any Janus Hermes pods orphaned in the configured Kubernetes namespace. It is
// gated on a configured namespace, runs off the UI thread, and only logs — a
// failure (e.g. the cluster is unreachable) must never block startup.
func (a *App) sweepOrphanedHermesPodsAsync() {
	if a.config == nil || strings.TrimSpace(a.config.Hermes.Kubernetes.Namespace) == "" {
		return
	}

	cfg := a.config
	namespace := cfg.Hermes.Kubernetes.Namespace

	go func() {
		ctx, cancel := context.WithTimeout(a.errorCtx, 60*time.Second)
		defer cancel()

		if err := execution.SweepOrphanedHermesPods(ctx, cfg); err != nil {
			log.Printf("Startup Hermes pod sweep failed (continuing): %v", err)

			return
		}

		log.Printf("Startup Hermes pod sweep complete (namespace %q)", namespace)
	}()
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

// initSigner initializes the run log signer if signing is configured.
// This should be called after SetConfiguration and SetLicenseClaims.
func (a *App) initSigner() {
	// Reset existing signer and the resolver that captured it.
	a.signer = nil
	a.ephemeralResolver = nil

	signer, err := appsetup.BuildSigner(a.config, a.licenseClaims)
	if err != nil {
		log.Printf("ERROR: run log signing setup failed: %v", err)
		// Surface compliance/signing failures to the user.
		if a.window != nil {
			dialog.ShowError(fmt.Errorf("CFR 21 Part 11 compliance error: %w", err), a.window)
		}

		return
	}

	a.signer = signer

	if signer != nil {
		log.Printf("Run log signing enabled")
	}
}

// initTrustStore constructs the run-log verification trust anchor from the
// license claims. This must be constructed here (the highest layer that knows
// about both config and license) and handed down — internal/runlog must never
// construct its own trust anchor, and the record being verified must never
// supply it either.
//
// This should be called after SetConfiguration and SetLicenseClaims.
func (a *App) initTrustStore() {
	trust, err := appsetup.BuildTrustStore(a.licenseClaims)
	if err != nil {
		log.Printf("ERROR: run log trust store setup failed: %v", err)

		if a.window != nil {
			dialog.ShowError(fmt.Errorf("CFR 21 Part 11 compliance error: %w", err), a.window)
		}

		a.trustStore = nil

		return
	}

	a.trustStore = trust

	if a.runLogStore != nil {
		a.runLogStore.SetTrustStore(a.trustStore)
	}
}

// checkModelingLicense checks if the required modeling software license exists for the given category.
// This does not validate the license contents, just verifies it exists at one of the expected locations.
func (a *App) checkModelingLicense(cat category.CategoryType) (bool, string) {
	switch cat {
	case category.CategoryNONMEM:
		// If execution mode requires NONMEM license, check using consistent logic
		if a.config != nil && config.RequiresNONMEMLicense(a.config.ExecutionMode) {
			licensePath, err := config.ValidateNONMEMLicenseForExecution(a.config)
			if err != nil {
				return false, err.Error()
			}

			return true, fmt.Sprintf("NONMEM license found at: %s", licensePath)
		}

		// For non-NONMEM execution modes with NONMEM models, check default locations
		locations := []string{}

		// Priority 1: ~/nonmem.lic
		if homeDir, err := os.UserHomeDir(); err == nil {
			locations = append(locations, filepath.Join(homeDir, "nonmem.lic"))
		}

		// Priority 2: ./nonmem.lic
		locations = append(locations, "nonmem.lic")

		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				return true, fmt.Sprintf("NONMEM license found at: %s", loc)
			}
		}

		return false, "NONMEM license not found. Please place nonmem.lic in your home directory or configure the path in settings."

	case category.CategoryMonolix:
		// Monolix license check would go here when implemented
		// For now, assume Monolix doesn't require a separate license file
		return true, "Monolix execution ready"

	case category.CategoryStan, category.CategoryTorsten:
		// Stan/Torsten don't require software licenses
		return true, "Stan/Torsten execution ready"

	case category.CategoryUnknown:
		// Unknown models can't be executed
		return false, "Unknown model type - cannot determine execution requirements"

	default:
		return true, ""
	}
}

// updateRunButtonState updates the run button styling based on license availability.
func (a *App) updateRunButtonState() {
	if a.runBtn == nil {
		return
	}

	if !a.modelingLicenseFound && a.currentModelCategory != "" {
		// License missing - show warning state
		a.runBtn.Importance = widget.DangerImportance
	} else {
		// License found or no model loaded - normal state
		a.runBtn.Importance = widget.HighImportance
	}
	a.runBtn.Refresh()
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
				// Only update if there are active runs (UI refresh must be on main thread).
				if len(a.activeRuns) == 0 {
					continue
				}

				a.sagaMu.Lock()
				sagaActive := len(a.sagaProgress) > 0
				a.sagaMu.Unlock()

				// A running saga needs a full rebuild so its progress card picks up
				// the latest snapshot (stage, k/N, live pods); a plain run only needs
				// the table's elapsed column refreshed.
				if sagaActive {
					a.updateActiveRunsDisplay()
				} else if a.activeRunsTable != nil {
					fyne.Do(func() {
						a.activeRunsTable.Refresh()
					})
				}
			}
		}
	}()
}

// Cleanup should be called when the app is shutting down.
func (a *App) Cleanup() {
	// Stop the MCP server first: its handlers touch app state (run-log stores,
	// execution), so it must be quiesced before the error channel closes.
	if err := a.StopMCPServer(); err != nil {
		log.Printf("Warning: failed to stop MCP server cleanly: %v", err)
	}

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
		a.updateRunRecord(runID, -1, "", "Run cancelled due to application shutdown", nil)
	}

	// Clear active runs map
	a.activeRuns = make(map[string]context.CancelFunc)

	// Close error channel
	close(a.errorCh)
}

// shortRunID returns a shortened version of a run ID for display purposes.
// Shows the last 6 characters of the UUID for better uniqueness.
// UUIDv7 has timestamp-based prefix, so the suffix is more distinctive.
func shortRunID(id string) string {
	if len(id) >= 6 {
		return id[len(id)-6:]
	}

	return id
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

	// Main horizontal tabs. "Model Run" stays the default landing tab; "Models"
	// is the per-directory model browser.
	mainTabs := container.NewAppTabs(
		container.NewTabItem("Model Run", a.buildModelRunTab()),
		container.NewTabItem("Models", a.buildModelBrowserTab()),
	)
	a.mainTabs = mainTabs

	// Re-scan the model browser each time the Models tab is opened so status,
	// run counts, and OFV reflect the latest runs.
	mainTabs.OnSelected = func(item *container.TabItem) {
		if item.Text == "Models" && a.modelBrowser != nil {
			a.modelBrowser.refresh()
		}
	}

	// Conditionally add grid details if licensed for "grid" feature AND scheduler is available
	showGridDetails := false
	if a.HasFeature("grid") && a.config != nil && a.config.Scheduler == "SLURM" {
		// Check if SLURM is actually available on the system
		if IsSLURMAvailable() {
			showGridDetails = true
		} else {
			log.Printf("SLURM scheduler configured but not available on system - hiding grid details panel")
		}
	}

	// Persistent bottom slot for active runs (and the live saga progress card). It
	// lives in the main layout's Bottom so it shows across tabs (e.g. while on the
	// run log) and never overlaps the tab bar. updateActiveRunsDisplay repopulates
	// it rather than appending to the Border, which can't place extra children.
	a.activeRunsArea = container.NewVBox()

	if showGridDetails {
		// Bottom grid details section
		gridDetails := a.buildGridDetails()

		// Use VSplit to divide space between main tabs and grid details
		splitContent := container.NewVSplit(mainTabs, gridDetails)
		splitContent.SetOffset(0.7) // 70% for main tabs, 30% for grid details

		a.mainLayout = container.NewBorder(
			topBar,           // Top - just the top bar
			a.activeRunsArea, // Bottom - active runs / saga progress card
			nil,              // Left
			nil,              // Right
			splitContent,     // Center - split between tabs and grid, expands to fill
		)
	} else {
		// No grid details - main tabs take full space
		a.mainLayout = container.NewBorder(
			topBar,           // Top - just the top bar
			a.activeRunsArea, // Bottom - active runs / saga progress card
			nil,              // Left
			nil,              // Right
			mainTabs,         // Center - tabs expand to fill all available space
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
		a.refreshCommandPreview()
	})
	a.syncRadio.SetSelected("Synchronous")

	// Cores input (initially hidden)
	a.coresEntry = widget.NewEntry()
	a.coresEntry.SetText("4") // Default cores
	a.coresEntry.SetPlaceHolder("Number of cores")

	// Add validation on text change for immediate feedback
	a.coresEntry.OnChanged = func(text string) {
		a.validateCoresInputSilent(text)
		a.refreshCommandPreview()
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
	a.nonmemOptionsEntry.OnChanged = func(string) { a.refreshCommandPreview() }

	nonmemOptionsLabel := widget.NewLabel("Additional NONMEM Options:")
	a.nonmemOptionsContainer = container.NewVBox(nonmemOptionsLabel, a.nonmemOptionsEntry)
	a.nonmemOptionsContainer.Hide() // Initially hidden, shown only for NONMEM mode

	// Show NONMEM options input only when execution mode is NONMEM
	if a.config != nil && a.config.ExecutionMode == "NONMEM" {
		a.nonmemOptionsContainer.Show()
	}

	// Per-run NONMEM installation/version picker (only when multiple
	// installations are configured, for NONMEM-style modes).
	a.versionSelect = widget.NewSelect(nil, func(string) { a.refreshCommandPreview() })
	a.versionContainer = container.NewVBox(widget.NewLabel("NONMEM Version:"), a.versionSelect)
	a.refreshVersionSelect()

	// PSN analysis preset picker + per-function parameter form (only in PSN mode).
	a.psnForm = newPSNFunctionForm()
	a.psnForm.onChanged = a.refreshCommandPreview
	a.psnPresetSelect = widget.NewSelect(nil, func(string) {
		a.psnForm.show(a.selectedPSNFunction())
		a.refreshCommandPreview()
	})
	a.psnPresetContainer = container.NewVBox(widget.NewLabel("PSN Analysis:"), a.psnPresetSelect)
	a.refreshPSNPresets()

	// Optional description checkbox for run log
	a.descriptionCheck = widget.NewCheck("Record message in run log", nil)

	// Execution target: Here (local) / Scheduler (grid) / SSH (remote-direct).
	// Options are gated by execution mode + remote config in refreshTargetOptions.
	a.targetRadio = widget.NewRadioGroup([]string{"Here"}, func(string) { a.refreshCommandPreview() })
	a.targetRadio.Horizontal = true
	a.targetRadio.SetSelected("Here")

	// Live, read-only preview of the command that will be run/logged, assembled
	// from the current engine, target, and run options. Mirrors what
	// createRunRecord records.
	a.commandPreview = widget.NewLabel("(load a model to preview the command)")
	a.commandPreview.Wrapping = fyne.TextWrapWord
	a.commandPreview.TextStyle = fyne.TextStyle{Monospace: true}
	commandPreviewBox := container.NewVBox(
		widget.NewLabelWithStyle("Command preview:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		a.commandPreview,
	)

	a.runBtn = widget.NewButton("Run", func() {
		a.onRunClicked()
	})
	a.runBtn.Importance = widget.HighImportance

	// Hermes config button for execution panel
	a.hermesConfigBtn = widget.NewButton("⚙️ Hermes Config", func() {
		a.showEditHermesConfigDialog()
	})

	// Summary of the current model's output files of interest — the files
	// collected after a run and embedded in the run log (its own retain, else the
	// inherited global/category default). Retain is a model-wide property, so this
	// is shown for any loaded model, with an Edit button to change it in place.
	a.retainGlobsLabel = widget.NewLabel("")
	a.retainGlobsLabel.Wrapping = fyne.TextWrapWord
	retainEditBtn := widget.NewButton("Edit", func() {
		a.showEditRetainDialog()
	})
	a.retainInfoBox = container.NewVBox(
		widget.NewSeparator(),
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Output files of interest", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			retainEditBtn,
		),
		a.retainGlobsLabel,
	)
	a.retainInfoBox.Hide()

	// Initially disable all components since no model is loaded yet
	// This must happen after button creation but before pending model loading
	a.setModelLoaded(false)
	a.refreshTargetOptions()

	leftPanel := container.NewVBox(
		loadModelBox,
		widget.NewSeparator(),
		dataSection,
		widget.NewSeparator(),
		a.syncRadio,
		a.coresContainer,
		widget.NewSeparator(),
		a.nonmemOptionsContainer,
		a.versionContainer,
		a.psnPresetContainer,
		a.psnForm.widget(),
		widget.NewSeparator(),
		a.descriptionCheck,
		widget.NewSeparator(),
		widget.NewLabel("Target:"),
		a.targetRadio,
		widget.NewSeparator(),
		commandPreviewBox,
		container.NewHBox(a.runBtn, a.hermesConfigBtn),
		a.retainInfoBox,
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

	// Create custom syntax-highlighting editor (supports multiple modeling languages)
	modelEditor := editor.NewModelEditor()
	modelEditor.SetText("// Load a model file to begin editing...")
	modelEditor.Disable() // Start disabled until a model is loaded

	// Set window reference for find/replace dialogs
	modelEditor.SetWindow(a.window)

	// Wire up OnSave callback (Ctrl+S/Cmd+S)
	modelEditor.OnSave = func() {
		if err := a.saveModelFile(); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save file: %w", err), a.window)
		} else {
			a.showSuccessToast("Model file saved successfully!")
		}
	}

	// Wire up OnChanged callback for dirty state tracking
	modelEditor.OnChanged = func(content string) {
		// Update save button state based on dirty flag
		if a.saveBtn != nil {
			if modelEditor.IsDirty() {
				a.saveBtn.Importance = widget.DangerImportance // Red to indicate unsaved changes
			} else {
				a.saveBtn.Importance = widget.HighImportance // Normal state
			}
			a.saveBtn.Refresh()
		}
	}

	// Store reference to the editor for direct access
	a.nonmemEditor = modelEditor

	// Wrap editor in scroll container for large files
	// The editor widget itself handles focus/tapping, scroll just provides viewport
	scrolledEditor := container.NewScroll(modelEditor)

	// Legacy textEditor kept for backward compatibility during transition
	a.textEditor = widget.NewEntry()
	a.textEditor.MultiLine = true
	a.textEditor.Wrapping = fyne.TextWrapWord
	a.textEditor.Hide() // Hidden - using nonmemEditor instead

	// Make the text editor expand to fill available space
	editorContainer := container.NewBorder(toolbar, nil, nil, nil, scrolledEditor)

	// Check if there's a pending model to load now that UI is ready
	if a.pendingModelPath != "" {
		if err := a.loadModelFile(a.pendingModelPath); err != nil {
			log.Printf("Failed to load model file %s: %v", a.pendingModelPath, err)
		}
		a.pendingModelPath = "" // Clear the pending path
	}

	return editorContainer
}

func (a *App) buildRunDetailsTab() fyne.CanvasObject {
	// Create run history table with checkbox column for comparison selection
	a.runHistoryTable = widget.NewTable(
		func() (int, int) {
			return len(a.cachedRuns), 11 // 11 columns: Checkbox, Run#, Status, OFV, Min, Verified, Time, Binary, Arguments, Type, Description
		},
		func() fyne.CanvasObject {
			// Create a container that can hold either a checkbox or a label
			check := widget.NewCheck("", nil)
			label := widget.NewLabel("")
			// Stack them - we'll show/hide based on column
			return container.NewStack(check, label)
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row >= len(a.cachedRuns) {
				return
			}

			stack, ok := obj.(*fyne.Container)
			if !ok || len(stack.Objects) < 2 {
				return
			}

			check, checkOk := stack.Objects[0].(*widget.Check)
			label, labelOk := stack.Objects[1].(*widget.Label)
			if !checkOk || !labelOk {
				return
			}

			run := a.cachedRuns[id.Row]

			if id.Col == 0 {
				// Checkbox column
				check.Show()
				label.Hide()

				// Update checkbox state without triggering callback
				check.OnChanged = nil
				check.SetChecked(a.selectedRunsForCompare[run.ID])

				// Set callback for this specific run
				check.OnChanged = func(checked bool) {
					a.toggleRunSelection(run.ID, checked)
				}

				return
			}

			// All other columns use label
			check.Hide()
			label.Show()

			// Reset importance for every cell first. Table cells are recycled, so a
			// label that previously rendered a green "COMPLETED" or red "FAILED"
			// status would otherwise leak that color into whichever column reuses it
			// (the colored noise in #197). Columns that intentionally color override
			// this below.
			label.Importance = widget.MediumImportance

			switch id.Col {
			case 1: // Run #
				label.SetText(fmt.Sprintf("#%s", shortRunID(run.ID)))
			case 2: // Status
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
			case 3: // OFV (Objective Function Value)
				if run.Summary != nil && run.Summary.GoodnessOfFit.ObjectiveFunctionValue != nil {
					label.SetText(fmt.Sprintf("%.2f", *run.Summary.GoodnessOfFit.ObjectiveFunctionValue))
				} else {
					label.SetText("-")
				}
				label.Importance = widget.MediumImportance
			case 4: // Minimized status
				if run.Summary != nil {
					if run.Summary.Estimation.Minimized {
						label.SetText("✓")
						label.Importance = widget.SuccessImportance
					} else {
						label.SetText("✗")
						label.Importance = widget.DangerImportance
					}
				} else {
					label.SetText("-")
					label.Importance = widget.LowImportance
				}
			case 5: // Verified (signature verification status)
				result := runlog.VerifyRecordStatus(&run, a.trustStore)
				switch result.Status {
				case runlog.VerificationValid:
					label.SetText("✓")
					label.Importance = widget.SuccessImportance
				case runlog.VerificationInvalid, runlog.VerificationUntrusted, runlog.VerificationChainBroken:
					// Untrusted is cryptographically sound but signed by an
					// unauthorized key — it must render exactly like Invalid,
					// never green, or a forged record would be indistinguishable
					// from a genuine one at a glance.
					label.SetText("✗")
					label.Importance = widget.DangerImportance
				case runlog.VerificationUnsigned, runlog.VerificationUnverifiable:
					label.SetText("?")
					label.Importance = widget.LowImportance
				}
			case 6: // Date/Time
				label.SetText(run.Timestamp.Format("2006-01-02 15:04:05"))
			case 7: // Binary (extracted from command)
				binary, _ := a.splitCommand(run.Command)
				label.SetText(binary)
			case 8: // Arguments (extracted from command)
				_, args := a.splitCommand(run.Command)
				// Truncate very long arguments for better display
				if len(args) > 100 {
					args = args[:100] + "..."
				}
				label.SetText(args)
			case 9: // Type (Parallel/Grid indicators)
				runType := "Local"
				if run.IsGrid {
					runType = "Grid"
				} else if run.IsParallel {
					runType = fmt.Sprintf("Parallel (%d)", run.Cores)
				}
				label.SetText(runType)
			case 10: // Description
				description, err := run.GetDescription()
				if err != nil {
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
	a.runHistoryTable.SetColumnWidth(0, 32)   // Checkbox (selection for comparison)
	a.runHistoryTable.SetColumnWidth(1, 65)   // Run # (7 chars: # + 6 UUID chars)
	a.runHistoryTable.SetColumnWidth(2, 95)   // Status (COMPLETED, FAILED, RUNNING)
	a.runHistoryTable.SetColumnWidth(3, 90)   // OFV (objective function value)
	a.runHistoryTable.SetColumnWidth(4, 40)   // Min (minimized status icon)
	a.runHistoryTable.SetColumnWidth(5, 40)   // Verified (icon only)
	a.runHistoryTable.SetColumnWidth(6, 145)  // Date/Time (YYYY-MM-DD HH:MM:SS)
	a.runHistoryTable.SetColumnWidth(7, 80)   // Binary (nonmem, psn, etc)
	a.runHistoryTable.SetColumnWidth(8, 250)  // Arguments (model paths) - reduced to fit checkbox column
	a.runHistoryTable.SetColumnWidth(9, 100)  // Type (Parallel (8), Grid, Local)
	a.runHistoryTable.SetColumnWidth(10, 150) // Description

	// Add selection handler
	a.runHistoryTable.OnSelected = func(id widget.TableCellID) {
		// Ensure valid row index (non-negative and within bounds)
		if id.Row >= 0 && id.Row < len(a.cachedRuns) {
			// Pass by pointer to avoid copying large compressed data
			a.displayRunDetails(&a.cachedRuns[id.Row])
		}
	}

	// Create table headers
	headerTable := widget.NewTable(
		func() (int, int) { return 1, 11 }, // 1 row, 11 columns for headers
		func() fyne.CanvasObject { return widget.NewRichTextFromMarkdown("**Header**") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row != 0 {
				return
			}

			richText, ok := obj.(*widget.RichText)
			if !ok {
				return
			}

			headers := []string{"**☐**", "**Run#**", "**Status**", "**OFV**", "**Min**", "**Sig**", "**Date/Time**", "**Binary**", "**Arguments**", "**Type**", "**Description**"}
			if id.Col < len(headers) {
				richText.ParseMarkdown(headers[id.Col])
			}
		},
	)

	// Set the same column widths as the data table for perfect alignment
	headerTable.SetColumnWidth(0, 32)   // Checkbox
	headerTable.SetColumnWidth(1, 65)   // Run #
	headerTable.SetColumnWidth(2, 95)   // Status
	headerTable.SetColumnWidth(3, 90)   // OFV
	headerTable.SetColumnWidth(4, 40)   // Min
	headerTable.SetColumnWidth(5, 40)   // Sig (Verified)
	headerTable.SetColumnWidth(6, 145)  // Date/Time
	headerTable.SetColumnWidth(7, 80)   // Binary
	headerTable.SetColumnWidth(8, 250)  // Arguments
	headerTable.SetColumnWidth(9, 100)  // Type
	headerTable.SetColumnWidth(10, 150) // Description

	// Right side: Output displays
	a.stdoutDisplay = widget.NewEntry()
	a.stdoutDisplay.MultiLine = true
	a.stdoutDisplay.Wrapping = fyne.TextWrapWord
	a.stdoutDisplay.SetText("No run selected...")

	a.stderrDisplay = widget.NewEntry()
	a.stderrDisplay.MultiLine = true
	a.stderrDisplay.Wrapping = fyne.TextWrapWord
	a.stderrDisplay.SetText("No run selected...")

	// Parameters container - will be populated when a run is selected
	a.parametersContainer = container.NewVBox(
		widget.NewLabel("No run selected..."),
	)

	// Create tabbed output view
	a.outputTabs = container.NewAppTabs(
		container.NewTabItem("STDOUT", container.NewScroll(a.stdoutDisplay)),
		container.NewTabItem("STDERR", container.NewScroll(a.stderrDisplay)),
		container.NewTabItem("Parameters", container.NewScroll(a.parametersContainer)),
	)

	// Create "Compare Selected" button (initially hidden)
	a.compareButton = widget.NewButton("Compare Selected (0)", func() {
		a.showComparisonDialog()
	})
	a.compareButton.Importance = widget.HighImportance
	a.compareButton.Hide() // Hidden until 2+ runs selected

	// Create "Visualize Selected" button (initially hidden)
	a.visualizeButton = widget.NewButton("Visualize Selected (0)", func() {
		a.showVisualizationDialog()
	})
	a.visualizeButton.Hide() // Hidden until 2+ runs selected

	// Create "View GOF Plots" button for single-run diagnostics (initially disabled)
	a.diagnosticsButton = widget.NewButton("View GOF Plots", func() {
		if a.selectedRun != nil {
			ShowDiagnosticsDialog(a.selectedRun, a.window)
		}
	})
	a.diagnosticsButton.Disable() // Disabled until a run is selected

	// Create "Inherit Parameters" button for inheriting estimates from a run (initially disabled)
	a.inheritParametersButton = widget.NewButton("Inherit Parameters", func() {
		if a.selectedRun != nil {
			a.showInheritParametersDialog(a.selectedRun)
		}
	})
	a.inheritParametersButton.Disable() // Disabled until a run with parameters is selected

	// Container for action buttons (allows show/hide)
	a.compareButtonContainer = container.NewHBox(a.inheritParametersButton, a.diagnosticsButton, a.compareButton, a.visualizeButton)

	// Header area with title, header table, and compare button
	headerArea := container.NewVBox(
		container.NewBorder(
			nil, nil,
			widget.NewLabel("Run History"),
			a.compareButtonContainer,
		),
		headerTable,
	)

	// Split layout: run table on left, output on right
	split := container.NewHSplit(
		container.NewBorder(
			headerArea,
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

// toggleRunSelection handles checkbox changes for run comparison selection.
func (a *App) toggleRunSelection(runID string, selected bool) {
	if selected {
		// Limit to 4 selections
		if len(a.selectedRunsForCompare) >= 4 {
			// Don't allow more than 4 selections
			// Refresh the table to reset the checkbox
			if a.runHistoryTable != nil {
				a.runHistoryTable.Refresh()
			}

			return
		}
		a.selectedRunsForCompare[runID] = true
	} else {
		delete(a.selectedRunsForCompare, runID)
	}

	a.updateCompareButton()
}

// updateCompareButton updates the compare and visualize button visibility and text.
func (a *App) updateCompareButton() {
	// Buttons may not exist yet during initial model loading
	if a.compareButton == nil {
		return
	}

	count := len(a.selectedRunsForCompare)

	if count >= 2 {
		a.compareButton.SetText(fmt.Sprintf("Compare Selected (%d)", count))
		a.compareButton.Show()

		if a.visualizeButton != nil {
			a.visualizeButton.SetText(fmt.Sprintf("Visualize Selected (%d)", count))
			a.visualizeButton.Show()
		}
	} else {
		a.compareButton.Hide()

		if a.visualizeButton != nil {
			a.visualizeButton.Hide()
		}
	}
}

// clearRunSelections clears all run selections (e.g., when switching models).
func (a *App) clearRunSelections() {
	a.selectedRunsForCompare = make(map[string]bool)
	a.updateCompareButton()

	if a.runHistoryTable != nil {
		a.runHistoryTable.Refresh()
	}
}

// showComparisonDialog displays the run comparison dialog.
func (a *App) showComparisonDialog() {
	// Collect selected run IDs
	var selectedIDs []string
	for id := range a.selectedRunsForCompare {
		selectedIDs = append(selectedIDs, id)
	}

	if len(selectedIDs) < 2 {
		return
	}

	// Fetch run records from store
	var records []*runlog.RunRecord
	for _, id := range selectedIDs {
		rec, err := a.runLogStore.GetRun(id)
		if err != nil {
			a.sendError(fmt.Errorf("failed to get run %s: %w", id, err))

			return
		}
		records = append(records, rec)
	}

	// Determine correlation strategy from the model config that lives alongside
	// the runs being compared. We resolve it from the most recent RunRecord rather
	// than the file currently open in the editor, which may be a different model
	// (or none at all) than the runs the user selected.
	strategy := comparison.StrategyConservative
	if modelFile := mostRecentModelFile(records); modelFile != "" {
		if modelConfig, err := config.LoadModelConfig(modelFile); err == nil {
			strategy = comparison.ParseCorrelationStrategy(modelConfig.CorrelationStrategy)
		}
		// If config doesn't exist or can't be loaded, use default (conservative)
	}

	// Perform comparison with the configured strategy
	result, err := comparison.CompareRunsWithStrategy(records, strategy)
	if err != nil {
		a.sendError(fmt.Errorf("comparison failed: %w", err))

		return
	}

	// Build comparison dialog content
	content := a.buildComparisonContent(result)

	// Create custom dialog
	comparisonDialog := dialog.NewCustom(
		fmt.Sprintf("Compare Runs (%d)", len(selectedIDs)),
		"Close",
		content,
		a.window,
	)
	comparisonDialog.Resize(fyne.NewSize(800, 600))
	comparisonDialog.Show()
}

// mostRecentModelFile returns the ModelFile of the most recently timestamped
// record, used to locate the .janus.config.json that governs the comparison.
// Records with no ModelFile are ignored; it returns "" if none qualify.
func mostRecentModelFile(records []*runlog.RunRecord) string {
	var newest *runlog.RunRecord
	for _, rec := range records {
		if rec == nil || rec.ModelFile == "" {
			continue
		}

		if newest == nil || rec.Timestamp.After(newest.Timestamp) {
			newest = rec
		}
	}

	if newest == nil {
		return ""
	}

	return newest.ModelFile
}

// buildComparisonContent builds the content for the comparison dialog.
func (a *App) buildComparisonContent(result *comparison.ComparisonResult) fyne.CanvasObject {
	// Build LRT summary section
	lrtSection := a.buildLRTSection(result)

	// Build parameter comparison table
	paramTable := a.buildComparisonTable(result)

	// Build metadata section
	metadataSection := a.buildMetadataSection(result)

	// Build legend section
	legendSection := a.buildComparisonLegend()

	// Combine all sections in a scrollable container
	content := container.NewVBox(
		lrtSection,
		widget.NewSeparator(),
		widget.NewLabel("Parameter Comparison"),
		paramTable,
		widget.NewSeparator(),
		metadataSection,
		widget.NewSeparator(),
		legendSection,
	)

	return container.NewScroll(content)
}

// buildLRTSection builds the LRT summary section.
func (a *App) buildLRTSection(result *comparison.ComparisonResult) fyne.CanvasObject {
	if len(result.Runs) < 2 || result.OFVRow == nil {
		return widget.NewLabel("OFV comparison not available")
	}

	// Get first and last run for LRT
	firstRun := result.Runs[0]
	lastRun := result.Runs[len(result.Runs)-1]

	// Calculate parameter count difference
	deltaParams := countParams(lastRun) - countParams(firstRun)

	// Build OFV display
	var ofvText string
	if result.OFVRow.Delta != nil {
		ofvText = fmt.Sprintf("ΔOFV: %s (%s)",
			comparison.FormatDelta(result.OFVRow.Delta),
			comparison.FormatDeltaPct(result.OFVRow.DeltaPct))
	} else {
		ofvText = "ΔOFV: N/A"
	}

	ofvLabel := widget.NewLabel(ofvText)
	ofvLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Perform LRT if we have valid OFV values
	var lrtText string

	switch {
	case firstRun.OFV != nil && lastRun.OFV != nil && deltaParams > 0:
		lrt := comparison.CalculateLRT(*firstRun.OFV, *lastRun.OFV, deltaParams, 0.05)
		lrtText = comparison.FormatLRTResult(lrt)
	case deltaParams == 0:
		lrtText = "Same parameter count - direct OFV comparison"
	default:
		lrtText = "LRT not applicable"
	}

	lrtLabel := widget.NewLabel(lrtText)

	// Run IDs row
	runIDsText := "Comparing: "
	for i, run := range result.Runs {
		if i > 0 {
			runIDsText += " → "
		}
		runIDsText += fmt.Sprintf("#%s", shortRunID(run.ID))
	}
	runIDsLabel := widget.NewLabel(runIDsText)
	runIDsLabel.TextStyle = fyne.TextStyle{Italic: true}

	return container.NewVBox(
		runIDsLabel,
		ofvLabel,
		lrtLabel,
	)
}

// countParams counts total estimated parameters in a run snapshot.
func countParams(snap *comparison.RunSnapshot) int {
	count := 0
	for _, t := range snap.Thetas {
		if !t.Fixed {
			count++
		}
	}
	for _, o := range snap.Omegas {
		if !o.Fixed {
			count++
		}
	}
	for _, s := range snap.Sigmas {
		if !s.Fixed {
			count++
		}
	}

	return count
}

// buildComparisonTable builds the parameter comparison table.
func (a *App) buildComparisonTable(result *comparison.ComparisonResult) fyne.CanvasObject {
	numRuns := len(result.Runs)
	// Columns: Parameter, [Run1, Run2, ...], Delta, %Δ
	numCols := 1 + numRuns + 2

	// Include OFV row + parameter rows
	allRows := []comparison.ParameterRow{}
	if result.OFVRow != nil {
		allRows = append(allRows, *result.OFVRow)
	}
	allRows = append(allRows, result.ParameterRows...)

	table := widget.NewTable(
		func() (int, int) {
			return len(allRows) + 1, numCols // +1 for header
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label, ok := obj.(*widget.Label)
			if !ok {
				return
			}

			// Header row
			if id.Row == 0 {
				switch {
				case id.Col == 0:
					label.SetText("Parameter")
					label.TextStyle = fyne.TextStyle{Bold: true}
				case id.Col <= numRuns:
					run := result.Runs[id.Col-1]
					label.SetText(fmt.Sprintf("#%s", shortRunID(run.ID)))
					label.TextStyle = fyne.TextStyle{Bold: true}
				case id.Col == numRuns+1:
					label.SetText("Delta")
					label.TextStyle = fyne.TextStyle{Bold: true}
				case id.Col == numRuns+2:
					label.SetText("%Δ")
					label.TextStyle = fyne.TextStyle{Bold: true}
				}
				label.Importance = widget.MediumImportance

				return
			}

			// Data rows
			rowIdx := id.Row - 1
			if rowIdx >= len(allRows) {
				return
			}

			row := allRows[rowIdx]
			label.TextStyle = fyne.TextStyle{}
			label.Importance = widget.MediumImportance // Reset importance to avoid stale highlighting

			switch {
			case id.Col == 0:
				// Display parameter with label if available (e.g., "THETA1 (CL)")
				displayName := comparison.FormatParameterWithLabel(row.Name, row.Label)
				label.SetText(displayName)
				// Bold for section headers (OFV, first of each type)
				if row.Type == "ofv" {
					label.TextStyle = fyne.TextStyle{Bold: true}
				}
			case id.Col <= numRuns:
				valIdx := id.Col - 1
				if valIdx < len(row.Values) {
					label.SetText(comparison.FormatValue(row.Values[valIdx]))
				} else {
					label.SetText(comparison.MissingValueDisplay)
				}
			case id.Col == numRuns+1:
				label.SetText(comparison.FormatDelta(row.Delta))
				a.applyHighlightStyle(label, row.Highlight)
			case id.Col == numRuns+2:
				label.SetText(comparison.FormatDeltaPct(row.DeltaPct))
				a.applyHighlightStyle(label, row.Highlight)
			}
		},
	)

	// Set column widths
	table.SetColumnWidth(0, 150) // Parameter name (with label)
	for i := 1; i <= numRuns; i++ {
		table.SetColumnWidth(i, 100) // Run values
	}
	table.SetColumnWidth(numRuns+1, 80) // Delta
	table.SetColumnWidth(numRuns+2, 80) // %Delta

	// Wrap in scroll container
	scroll := container.NewScroll(table)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	return scroll
}

// applyHighlightStyle applies color styling based on highlight level.
func (a *App) applyHighlightStyle(label *widget.Label, highlight comparison.HighlightLevel) {
	// For OFV and most params, negative delta (decrease) is neutral/good
	// Large changes get warning colors
	switch highlight {
	case comparison.HighlightWarning:
		label.Importance = widget.DangerImportance
	case comparison.HighlightMajor:
		label.Importance = widget.WarningImportance
	case comparison.HighlightMinor, comparison.HighlightNone:
		label.Importance = widget.MediumImportance
	}
	label.Refresh()
}

// buildMetadataSection builds the metadata comparison section.
func (a *App) buildMetadataSection(result *comparison.ComparisonResult) fyne.CanvasObject {
	if len(result.MetadataRows) == 0 {
		return widget.NewLabel("")
	}

	numRuns := len(result.Runs)

	// Build grid of metadata
	grid := container.NewGridWithColumns(numRuns + 1)

	// Header row
	grid.Add(widget.NewLabel(""))
	for _, run := range result.Runs {
		lbl := widget.NewLabel(fmt.Sprintf("#%s", shortRunID(run.ID)))
		lbl.TextStyle = fyne.TextStyle{Bold: true}
		grid.Add(lbl)
	}

	// Data rows
	for _, row := range result.MetadataRows {
		nameLbl := widget.NewLabel(row.Name)
		nameLbl.TextStyle = fyne.TextStyle{Bold: true}
		grid.Add(nameLbl)

		for _, val := range row.Values {
			valLbl := widget.NewLabel(val)
			// Color code checkmarks
			switch val {
			case "✓":
				valLbl.Importance = widget.SuccessImportance
			case "✗":
				valLbl.Importance = widget.DangerImportance
			}
			grid.Add(valLbl)
		}
	}

	return container.NewVBox(
		widget.NewLabel("Run Metadata"),
		grid,
	)
}

// buildComparisonLegend builds the legend explaining symbols and colors.
func (a *App) buildComparisonLegend() fyne.CanvasObject {
	header := widget.NewLabel("Legend")
	header.TextStyle = fyne.TextStyle{Bold: true}

	// Color legend items
	warningLbl := widget.NewLabel("Red text")
	warningLbl.Importance = widget.DangerImportance
	warningDesc := widget.NewLabel("= Change > 10% (large deviation)")

	successLbl := widget.NewLabel("✓")
	successLbl.Importance = widget.SuccessImportance
	successDesc := widget.NewLabel("= Success (minimized, covariance step passed)")

	failLbl := widget.NewLabel("✗")
	failLbl.Importance = widget.DangerImportance
	failDesc := widget.NewLabel("= Failure (did not minimize, covariance step failed)")

	// Symbol legend
	dashDesc := widget.NewLabel("—  = Value not available (parameter missing or run failed)")
	deltaDesc := widget.NewLabel("Delta = Change from first to last run in comparison")

	// Build legend grid
	legendGrid := container.NewGridWithColumns(2,
		container.NewHBox(warningLbl, warningDesc),
		container.NewHBox(successLbl, successDesc),
		container.NewHBox(failLbl, failDesc),
		dashDesc,
	)

	return container.NewVBox(
		header,
		legendGrid,
		deltaDesc,
	)
}

// showVisualizationDialog displays charts for selected runs.
func (a *App) showVisualizationDialog() {
	// Collect selected run IDs
	var selectedIDs []string
	for id := range a.selectedRunsForCompare {
		selectedIDs = append(selectedIDs, id)
	}

	if len(selectedIDs) < 2 {
		return
	}

	// Fetch run records from store
	var records []*runlog.RunRecord
	for _, id := range selectedIDs {
		rec, err := a.runLogStore.GetRun(id)
		if err != nil {
			a.sendError(fmt.Errorf("failed to get run %s: %w", id, err))

			return
		}
		records = append(records, rec)
	}

	// Build visualization dialog content
	content := a.buildVisualizationContent(records)

	// Create custom dialog
	vizDialog := dialog.NewCustom(
		fmt.Sprintf("Visualize Runs (%d)", len(selectedIDs)),
		"Close",
		content,
		a.window,
	)
	vizDialog.Resize(fyne.NewSize(900, 650))
	vizDialog.Show()
}

// buildVisualizationContent builds the content for the visualization dialog.
func (a *App) buildVisualizationContent(records []*runlog.RunRecord) fyne.CanvasObject {
	// Extract OFV data
	ofvData := visualization.ExtractOFVData(records)

	// Create tabs for different chart types
	tabs := container.NewAppTabs()

	// OFV Trend Chart Tab
	if len(ofvData) >= 2 {
		ofvChart := a.buildOFVChartContent(ofvData)
		tabs.Append(container.NewTabItem("OFV Trend", ofvChart))
	}

	// Parameter Charts Tab
	paramNames := visualization.GetAllParameterNames(records)
	if len(paramNames) > 0 {
		paramContent := a.buildParameterChartContent(records, paramNames)
		tabs.Append(container.NewTabItem("Parameters", paramContent))
	}

	// Convergence Chart Tab
	if len(ofvData) >= 2 {
		convChart := a.buildConvergenceChartContent(ofvData)
		tabs.Append(container.NewTabItem("Convergence", convChart))
	}

	if len(tabs.Items) == 0 {
		return widget.NewLabel("No visualization data available")
	}

	return tabs
}

// buildOFVChartContent builds the OFV trend chart tab content.
func (a *App) buildOFVChartContent(data []visualization.OFVDataPoint) fyne.CanvasObject {
	opts := visualization.DefaultChartOptions()
	opts.Title = "OFV Trend Across Runs"
	opts.Width = 800
	opts.Height = 350

	result, err := visualization.GenerateOFVTrendChart(data, opts)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Error generating chart: %v", err))
	}

	// Convert PNG bytes to Fyne image
	img := fyne.NewStaticResource("ofv_trend.png", result.Data)

	// "Open in Browser" button for interactive chart
	openBrowserBtn := widget.NewButton("Open Interactive in Browser", func() {
		interactiveOpts := visualization.DefaultInteractiveOptions()
		interactiveOpts.Title = "OFV Trend Across Runs"
		if err := visualization.GenerateAndOpenInteractiveOFV(data, interactiveOpts); err != nil {
			a.sendError(fmt.Errorf("failed to open interactive chart: %w", err))
		}
	})

	return container.NewBorder(
		nil,
		container.NewHBox(openBrowserBtn),
		nil, nil,
		container.NewScroll(widget.NewIcon(img)),
	)
}

// buildParameterChartContent builds the parameter comparison chart tab content.
func (a *App) buildParameterChartContent(records []*runlog.RunRecord, paramNames []string) fyne.CanvasObject {
	// Create a dropdown to select which parameter to visualize
	paramSelect := widget.NewSelect(paramNames, nil)
	if len(paramNames) > 0 {
		paramSelect.SetSelected(paramNames[0])
	}

	// Chart container that will be updated when selection changes
	chartContainer := container.NewStack()

	// Track current parameter data for browser export
	var currentParamData []visualization.ParameterDataPoint
	var currentParamName string

	updateChart := func(paramName string) {
		chartContainer.RemoveAll()

		data := visualization.ExtractParameterData(records, paramName)
		currentParamData = data
		currentParamName = paramName

		if len(data) < 2 {
			chartContainer.Add(widget.NewLabel("Not enough data points"))
			chartContainer.Refresh()

			return
		}

		opts := visualization.DefaultChartOptions()
		opts.Title = fmt.Sprintf("%s Across Runs", paramName)
		opts.Width = 800
		opts.Height = 350

		result, err := visualization.GenerateParameterBarChart(data, opts)
		if err != nil {
			chartContainer.Add(widget.NewLabel(fmt.Sprintf("Error: %v", err)))
			chartContainer.Refresh()

			return
		}

		img := fyne.NewStaticResource("param_chart.png", result.Data)
		chartContainer.Add(widget.NewIcon(img))
		chartContainer.Refresh()
	}

	paramSelect.OnChanged = updateChart

	// Initial chart
	if len(paramNames) > 0 {
		updateChart(paramNames[0])
	}

	// "Open in Browser" button for interactive chart
	openBrowserBtn := widget.NewButton("Open Interactive in Browser", func() {
		if len(currentParamData) < 2 {
			return
		}

		interactiveOpts := visualization.DefaultInteractiveOptions()
		interactiveOpts.Title = fmt.Sprintf("%s Across Runs", currentParamName)
		if err := visualization.GenerateAndOpenInteractiveParameter(currentParamData, interactiveOpts); err != nil {
			a.sendError(fmt.Errorf("failed to open interactive chart: %w", err))
		}
	})

	return container.NewBorder(
		container.NewHBox(widget.NewLabel("Parameter:"), paramSelect),
		container.NewHBox(openBrowserBtn),
		nil, nil,
		container.NewScroll(chartContainer),
	)
}

// buildConvergenceChartContent builds the convergence (OFV delta) chart tab content.
func (a *App) buildConvergenceChartContent(data []visualization.OFVDataPoint) fyne.CanvasObject {
	opts := visualization.DefaultChartOptions()
	opts.Title = "OFV Changes Between Runs"
	opts.Width = 800
	opts.Height = 350

	result, err := visualization.GenerateConvergenceChart(data, opts)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("Error generating chart: %v", err))
	}

	img := fyne.NewStaticResource("convergence.png", result.Data)

	return container.NewScroll(widget.NewIcon(img))
}

func (a *App) displayRunDetails(run *runlog.RunRecord) {
	// Store the selected run for the diagnostics button
	a.selectedRun = run

	// Enable/disable diagnostics button based on whether GOF data is available
	// Note: We only check existing data - we do NOT modify the run record here
	// to avoid invalidating signatures (critical for GxP compliance)
	if a.diagnosticsButton != nil {
		if run != nil && run.Summary != nil && run.Summary.TableDiagnostics != nil &&
			run.Summary.TableDiagnostics.GOFData != nil {

			a.diagnosticsButton.Enable()
		} else {
			a.diagnosticsButton.Disable()
		}
	}

	// Enable/disable inherit parameters button based on whether parameter estimates exist
	if a.inheritParametersButton != nil {
		if run != nil && run.Summary != nil && len(run.Summary.Parameters.Thetas) > 0 {
			a.inheritParametersButton.Enable()
		} else {
			a.inheritParametersButton.Disable()
		}
	}

	// Clear current displays immediately
	a.stdoutDisplay.SetText("")
	a.stderrDisplay.SetText("")

	// Clear parameters container
	a.parametersContainer.RemoveAll()
	a.parametersContainer.Add(widget.NewLabel("Loading..."))

	// Do ALL decompression work in background
	go func(runPtr *runlog.RunRecord) {
		// Decompress stdout/stderr
		stdout, err := runPtr.GetStdout()
		if err != nil {
			stdout = runPtr.Stdout // Fallback to legacy field
		}

		stderr, err := runPtr.GetStderr()
		if err != nil {
			stderr = runPtr.Stderr // Fallback to legacy field
		}

		// Build parameters view (tables)
		parametersView := buildParametersView(runPtr)

		// Build buttons container for embedded files (before UI update)
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

		// Add buttons for embedded output files (keyed by filename now).
		names := runlog.GetEmbeddedFileNames(runPtr)
		for _, name := range names {
			nameCopy := name // Capture for closure
			fileBtn := widget.NewButton(name, func() {
				a.showFileWindow(runPtr, nameCopy, nameCopy)
			})
			buttons = append(buttons, fileBtn)
		}

		// A PsN analysis (vpc/bootstrap/scm) can re-render its result from the
		// collected artifacts on disk — no re-running needed (#189). Detect it from
		// the recorded command and offer a button.
		if tool := psnToolFromCommand(runPtr.Command); tool != "" {
			modelDir := filepath.Dir(runPtr.ModelFile)
			resultsBtn := widget.NewButton(psnResultButtonLabel(tool), func() {
				showPSNResultsDialog(a.window, tool, modelDir)
			})
			resultsBtn.Importance = widget.HighImportance
			buttons = append(buttons, resultsBtn)
		}

		// A saga parent (e.g. a horizontal bootstrap) gets its own tab listing the
		// per-fit children and a download for the whole workspace. Fetch the
		// children here (cheap index read) and build the tab on the UI thread.
		var sagaChildren []*runlog.RunRecord
		isSaga := runPtr.Kind == runlog.KindSaga
		if isSaga && a.runLogStore != nil {
			sagaChildren, _ = a.runLogStore.ChildrenOf(runPtr.ID)
		}

		var buttonsContainer *fyne.Container
		// Create buttons container if we have any buttons
		if len(buttons) > 0 {
			// Add export button
			exportBtn := widget.NewButton("Export (.zip)", func() {
				a.exportRunFiles(runPtr)
			})
			exportBtn.Importance = widget.HighImportance

			buttonsLabel := widget.NewLabel("Embedded Files:")
			buttonsGrid := container.NewGridWithColumns(4, buttons...)
			buttonsContainer = container.NewVBox(
				buttonsLabel,
				buttonsGrid,
				widget.NewSeparator(),
				exportBtn,
			)
		}

		// Update UI on main thread
		fyne.Do(func() {
			a.stdoutDisplay.SetText(stdout)
			a.stderrDisplay.SetText(stderr)

			// Update parameters container
			a.parametersContainer.RemoveAll()
			a.parametersContainer.Add(parametersView)

			if buttonsContainer != nil {
				// Add to output tabs (replacing any existing "Files" tab)
				for i := len(a.outputTabs.Items) - 1; i >= 0; i-- {
					if a.outputTabs.Items[i].Text == "Files" {
						a.outputTabs.RemoveIndex(i)
					}
				}

				filesTab := container.NewTabItem("Files", buttonsContainer)
				a.outputTabs.Append(filesTab)
			}

			// Replace any prior "Saga" tab, then add one for this saga parent.
			for i := len(a.outputTabs.Items) - 1; i >= 0; i-- {
				if a.outputTabs.Items[i].Text == "Saga" {
					a.outputTabs.RemoveIndex(i)
				}
			}

			if isSaga {
				sagaTab := container.NewTabItem("Saga", a.buildSagaView(runPtr, sagaChildren))
				a.outputTabs.Append(sagaTab)
			}
		})
	}(run)
}

// buildSagaView renders the details panel for a saga parent (e.g. a horizontal
// bootstrap, #192): a summary, the per-fit children with their statuses, and a
// button to download the whole saga workspace as a zip.
func (a *App) buildSagaView(run *runlog.RunRecord, children []*runlog.RunRecord) fyne.CanvasObject {
	description, err := run.GetDescription()
	if err != nil || description == "" {
		description = run.Description
	}

	if description == "" {
		description = "Bootstrap saga."
	}

	summary := widget.NewLabel(description)
	summary.Wrapping = fyne.TextWrapWord

	header := widget.NewLabelWithStyle(
		fmt.Sprintf("Fits (%d)", len(children)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Virtualized list so a large fan-out (hundreds of fits) stays responsive.
	list := widget.NewList(
		func() int { return len(children) },
		func() fyne.CanvasObject {
			id := widget.NewLabel("")
			args := widget.NewLabel("")
			status := widget.NewLabel("")

			return container.NewBorder(nil, nil, id, status, args)
		},
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if i < 0 || i >= len(children) {
				return
			}

			child := children[i]
			border, ok := obj.(*fyne.Container)
			if !ok || len(border.Objects) < 3 {
				return
			}

			// container.NewBorder lays out as [center, left, right].
			args, _ := border.Objects[0].(*widget.Label)
			id, _ := border.Objects[1].(*widget.Label)
			status, _ := border.Objects[2].(*widget.Label)
			if args == nil || id == nil || status == nil {
				return
			}

			id.SetText(fmt.Sprintf("#%s", shortRunID(child.ID)))
			_, cmdArgs := a.splitCommand(child.Command)
			args.SetText(cmdArgs)

			status.SetText(strings.ToUpper(child.Status))
			switch child.Status {
			case "completed":
				status.Importance = widget.SuccessImportance
			case "failed":
				status.Importance = widget.DangerImportance
			default:
				status.Importance = widget.MediumImportance
			}
		},
	)

	downloadBtn := widget.NewButton("Download all files (.zip)", func() {
		a.exportSagaFiles(run)
	})
	downloadBtn.Importance = widget.HighImportance

	// The list needs a bounded height to scroll inside the tab.
	listScroll := container.NewVScroll(list)
	listScroll.SetMinSize(fyne.NewSize(0, 260))

	return container.NewBorder(
		container.NewVBox(summary, widget.NewSeparator(), header),
		container.NewVBox(widget.NewSeparator(), downloadBtn),
		nil, nil,
		listScroll,
	)
}

// exportSagaFiles zips a completed saga's on-disk workspace (resampled datasets,
// per-fit outputs, and aggregate result CSVs under the model directory's bs/
// tree) to a user-chosen folder.
func (a *App) exportSagaFiles(run *runlog.RunRecord) {
	modelDir := filepath.Dir(run.ModelFile)
	workspaceDir := filepath.Join(modelDir, execution.BootstrapWorkspaceDirName)

	if info, err := os.Stat(workspaceDir); err != nil || !info.IsDir() {
		dialog.ShowError(fmt.Errorf(
			"no saga workspace found on disk at %s — the results may have been moved or the saga predates workspace persistence",
			workspaceDir), a.window)

		return
	}

	modelName := strings.TrimSuffix(filepath.Base(run.ModelFile), filepath.Ext(run.ModelFile))
	defaultFileName := fmt.Sprintf("%s_bootstrap_%s.zip", modelName, shortRunID(run.ID))

	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil || dir == nil {
			return
		}

		zipPath := filepath.Join(dir.Path(), defaultFileName)

		go func() {
			file, err := os.Create(zipPath)
			if err != nil {
				a.sendError(fmt.Errorf("failed to create zip file: %w", err))

				return
			}
			defer file.Close()

			if err := zipDirectory(workspaceDir, execution.BootstrapWorkspaceDirName, file); err != nil {
				a.sendError(fmt.Errorf("failed to create saga zip archive: %w", err))

				return
			}

			a.showSuccessToast(fmt.Sprintf("Exported saga #%s files to %s", shortRunID(run.ID), zipPath))
		}()
	}, a.window)
}

// zipDirectory writes every file under srcDir into the zip writer, each entry
// prefixed with prefix (so the archive expands into a single named directory).
func zipDirectory(srcDir, prefix string, w io.Writer) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}

		// Zip entries always use forward slashes regardless of host OS.
		entryName := prefix + "/" + filepath.ToSlash(rel)

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		entry, err := zipWriter.Create(entryName)
		if err != nil {
			return fmt.Errorf("creating zip entry %s: %w", entryName, err)
		}

		if _, err := entry.Write(content); err != nil {
			return fmt.Errorf("writing %s to zip: %w", entryName, err)
		}

		return nil
	})
}

// buildParametersView creates a structured view with tables for model parameters.
func buildParametersView(run *runlog.RunRecord) fyne.CanvasObject {
	if run.Summary == nil {
		return container.NewVBox(
			widget.NewLabel("No parameter summary available."),
			widget.NewLabel(""),
			widget.NewLabel("This run may not have completed successfully,"),
			widget.NewLabel("or parameter extraction was not performed."),
		)
	}

	summary := run.Summary
	var sections []fyne.CanvasObject

	// Summary header section
	summaryGrid := container.NewGridWithColumns(4,
		widget.NewLabelWithStyle("OFV:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(formatOFV(summary.GoodnessOfFit.ObjectiveFunctionValue)),
		widget.NewLabelWithStyle("Method:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(summary.Estimation.Method),

		widget.NewLabelWithStyle("Subjects:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("%d", summary.Estimation.Subjects)),
		widget.NewLabelWithStyle("Observations:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("%d", summary.Estimation.Observations)),

		widget.NewLabelWithStyle("Minimized:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		createStatusLabel(summary.Estimation.Minimized),
		widget.NewLabelWithStyle("Covariance:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		createStatusLabel(summary.Diagnostics.CovarianceStepSuccess),
	)

	sections = append(sections,
		widget.NewRichTextFromMarkdown("## Model Summary"),
		summaryGrid,
		widget.NewSeparator(),
	)

	// THETA table
	if len(summary.Parameters.Thetas) > 0 {
		thetaTable := buildParameterTable(summary.Parameters.Thetas, false)
		sections = append(sections,
			widget.NewRichTextFromMarkdown("### THETA (Fixed Effects)"),
			thetaTable,
			widget.NewSeparator(),
		)
	}

	// OMEGA table
	if len(summary.Parameters.Omegas) > 0 {
		omegaTable := buildParameterTable(summary.Parameters.Omegas, true)
		sections = append(sections,
			widget.NewRichTextFromMarkdown("### OMEGA (Inter-Individual Variability)"),
			omegaTable,
			widget.NewSeparator(),
		)
	}

	// SIGMA table
	if len(summary.Parameters.Sigmas) > 0 {
		sigmaTable := buildParameterTable(summary.Parameters.Sigmas, false)
		sections = append(sections,
			widget.NewRichTextFromMarkdown("### SIGMA (Residual Variability)"),
			sigmaTable,
			widget.NewSeparator(),
		)
	}

	// Diagnostics section
	if len(summary.Diagnostics.Warnings) > 0 || len(summary.Diagnostics.Errors) > 0 {
		diagItems := []fyne.CanvasObject{
			widget.NewRichTextFromMarkdown("### Diagnostics"),
		}

		if len(summary.Diagnostics.Warnings) > 0 {
			warningLabel := widget.NewLabel(fmt.Sprintf("⚠ %d warning(s)", len(summary.Diagnostics.Warnings)))
			warningLabel.Importance = widget.WarningImportance
			diagItems = append(diagItems, warningLabel)
		}

		if len(summary.Diagnostics.Errors) > 0 {
			errorLabel := widget.NewLabel(fmt.Sprintf("✗ %d error(s)", len(summary.Diagnostics.Errors)))
			errorLabel.Importance = widget.DangerImportance
			diagItems = append(diagItems, errorLabel)
		}

		sections = append(sections, diagItems...)
	}

	return container.NewVBox(sections...)
}

// buildParameterTable creates a table widget for a slice of parameter estimates.
func buildParameterTable(params []model.ParameterEstimate, showCV bool) fyne.CanvasObject {
	// Determine column count
	colCount := 2 // Name, Estimate
	if showCV {
		colCount = 3 // Name, Estimate, %CV
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(params) + 1, colCount // +1 for header row
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("placeholder")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label, ok := obj.(*widget.Label)
			if !ok {
				return
			}

			if id.Row == 0 {
				// Header row
				label.TextStyle = fyne.TextStyle{Bold: true}
				switch id.Col {
				case 0:
					label.SetText("Parameter")
				case 1:
					label.SetText("Estimate")
				case 2:
					if showCV {
						label.SetText("%CV")
					}
				}

				return
			}

			// Data rows
			label.TextStyle = fyne.TextStyle{}
			paramIdx := id.Row - 1
			if paramIdx >= len(params) {
				label.SetText("")

				return
			}

			param := params[paramIdx]
			switch id.Col {
			case 0:
				label.SetText(param.Name)
			case 1:
				if param.Estimate != nil {
					label.SetText(fmt.Sprintf("%.4g", *param.Estimate))
				} else {
					label.SetText("-")
				}
			case 2:
				if showCV && param.Estimate != nil && *param.Estimate > 0 {
					// %CV = sqrt(variance) * 100 for OMEGA diagonal elements
					cv := 100.0 * math.Sqrt(*param.Estimate)
					label.SetText(fmt.Sprintf("%.1f%%", cv))
				} else if showCV {
					label.SetText("-")
				}
			}
		},
	)

	// Set column widths - these are minimums, table will expand to fill space
	table.SetColumnWidth(0, 150) // Parameter name
	table.SetColumnWidth(1, 150) // Estimate
	if showCV {
		table.SetColumnWidth(2, 100) // %CV
	}

	// Calculate height based on number of rows (header + data)
	rowHeight := float32(30)
	tableHeight := rowHeight * float32(len(params)+1)
	if tableHeight < 60 {
		tableHeight = 60 // Minimum height
	}
	if tableHeight > 250 {
		tableHeight = 250 // Max height before scroll
	}

	// Wrap in scroll container with controlled height, allowing width expansion
	scroll := container.NewVScroll(table)
	scroll.SetMinSize(fyne.NewSize(0, tableHeight))

	return scroll
}

// createStatusLabel creates a label with checkmark or X based on boolean status.
func createStatusLabel(status bool) *widget.Label {
	label := widget.NewLabel("")
	if status {
		label.SetText("✓ Yes")
		label.Importance = widget.SuccessImportance
	} else {
		label.SetText("✗ No")
		label.Importance = widget.DangerImportance
	}

	return label
}

// formatOFV formats the objective function value for display.
func formatOFV(ofv *float64) string {
	if ofv == nil {
		return "-"
	}

	return fmt.Sprintf("%.2f", *ofv)
}

// showFileWindow opens a separate window to display a file from the run record.
func (a *App) showFileWindow(run *runlog.RunRecord, title, fileType string) {
	// Create new window for file display
	window := a.fyneApp.NewWindow(fmt.Sprintf("%s - Run #%s", title, shortRunID(run.ID)))
	window.Resize(fyne.NewSize(800, 600))

	// Load content synchronously (fast for embedded files)
	var content string

	if fileType == "description" {
		content, _ = run.GetDescription()
		if content == "" {
			content = run.Description
		}
	} else {
		// It's an embedded file
		data, err := runlog.ExtractEmbeddedFile(run, fileType)
		if err != nil {
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
	window.Show()
}

// exportRunFiles exports all embedded files from a run record to a zip archive.
func (a *App) exportRunFiles(run *runlog.RunRecord) {
	// Get model name from model file path
	modelName := strings.TrimSuffix(filepath.Base(run.ModelFile), filepath.Ext(run.ModelFile))
	defaultFileName := fmt.Sprintf("%s_run_%s.zip", modelName, shortRunID(run.ID))

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
				a.showSuccessToast(fmt.Sprintf("Exported run #%s files to %s", shortRunID(run.ID), zipPath))
			}
		}()
	}, a.window)
}

// createRunFilesZip creates a zip archive containing all embedded files from a run.
func (a *App) createRunFilesZip(run *runlog.RunRecord, modelName string, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Add description if available
	hasDescription := run.DescriptionCompressed != "" || run.Description != ""
	if hasDescription {
		description, err := run.GetDescription()
		if err != nil {
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

	// Add all embedded files (keyed by their real filename).
	names := runlog.GetEmbeddedFileNames(run)
	for _, name := range names {
		content, err := runlog.ExtractEmbeddedFile(run, name)
		if err != nil {
			continue
		}

		fileWriter, err := zipWriter.Create(name)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", name, err)
		}

		if _, err := fileWriter.Write(content); err != nil {
			return fmt.Errorf("failed to write %s to zip: %w", name, err)
		}
	}

	return nil
}

func (a *App) showSettingsPanel() {
	if a.settingsOpen {
		return // Settings already open
	}

	a.settingsOpen = true
	settingsDialog := NewSettingsDialog(a)
	settingsDialog.Show()

	// Update settingsOpen when dialog closes (must be after Show() creates the window)
	settingsDialog.window.SetOnClosed(func() {
		a.settingsOpen = false
	})
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

	if a.runBtn != nil {
		if loaded {
			a.runBtn.Enable()
		} else {
			a.runBtn.Disable()
		}
	}

	if a.targetRadio != nil {
		if loaded {
			a.targetRadio.Enable()
		} else {
			a.targetRadio.Disable()
		}
	}

	if a.hermesConfigBtn != nil {
		// Show Hermes config button only in Hermes mode
		if a.config != nil && a.config.ExecutionMode == config.ExecutionModeHERMES {
			a.hermesConfigBtn.Show()
			if loaded {
				a.hermesConfigBtn.Enable()
			} else {
				a.hermesConfigBtn.Disable()
			}
		} else {
			a.hermesConfigBtn.Hide()
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

	// Update custom model editor
	if a.nonmemEditor != nil {
		// Direct access to editor (no longer wrapped in scroll container)
		if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
			if loaded {
				modelEditor.Enable()
				modelEditor.SetText(a.fileContent)
				// Request focus after loading content
				if a.window != nil && a.window.Canvas() != nil {
					a.window.Canvas().Focus(modelEditor)
				}
			} else {
				modelEditor.Disable()
				modelEditor.SetText("// Load a model file to begin editing...")
			}
		}
	}

	// Note: Fyne doesn't support disabling individual tabs
	// The Run History tab will remain available but could show a message
	// when accessed without a loaded model

	// Update run button styling based on license state
	if loaded {
		a.updateRunButtonState()
	}

	// Reflect the (un)loaded model in the command preview and retain summary.
	a.refreshCommandPreview()
	a.refreshRetainInfo()
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

	// Store the file information (currentFilePath is guarded because MCP HTTP
	// goroutines read it via resolveRunLogStore).
	a.modelMu.Lock()
	a.currentFilePath = filePath
	a.modelMu.Unlock()
	a.fileContent = string(content)
	a.fileHash = sha256.Sum256(content)

	// Detect model category for multi-modal support
	detector := category.NewDetector()
	detectedCategory, detectErr := detector.Detect(filePath)
	if detectErr != nil {
		log.Printf("Warning: failed to detect model category: %v", detectErr)
		a.currentModelCategory = category.CategoryUnknown
	} else {
		a.currentModelCategory = detectedCategory
		log.Printf("Detected model category: %s", detectedCategory)
	}

	// Set the appropriate lexer based on detected model category
	if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
		lexer := editor.LexerForCategory(a.currentModelCategory)
		modelEditor.SetLexer(lexer)
		log.Printf("Set editor lexer: %s", lexer.Name())
	}

	// Check if required modeling software license exists
	a.modelingLicenseFound, a.modelingLicenseMessage = a.checkModelingLicense(a.currentModelCategory)
	if !a.modelingLicenseFound {
		log.Printf("Modeling license check: %s", a.modelingLicenseMessage)
	}

	// Set up model-specific run history
	a.setupModelRunHistory(filePath)

	// Create the Run Details tab when model is loaded but don't switch to it
	// This ensures run history is available if the user wants to view it
	a.createRunDetailsTab()

	// Parse and update the $DATA variable (for NONMEM models)
	dataValue, hasData := a.parseDataVariableWithValidation(a.fileContent)
	if a.dataEntry != nil {
		a.dataEntry.SetText(dataValue)
	}

	// Show category-appropriate warnings
	if a.currentModelCategory == category.CategoryUnknown {
		a.showWarningToast("Unknown model type - syntax highlighting and execution may be limited.")
	} else if a.currentModelCategory == category.CategoryNONMEM && !hasData {
		a.showWarningToast("NONMEM model detected but no $DATA section found.")
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

	// Get the current content from the model editor (or legacy text editor as fallback)
	var content string
	if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
		content = modelEditor.GetText()
	} else {
		// Fallback to legacy editor if type assertion fails
		content = a.textEditor.Text
	}

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

	// Clear dirty state after successful save
	if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
		modelEditor.ClearDirtyState()
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

	// Update custom model editor
	if a.nonmemEditor != nil {
		// Direct access to editor (no longer wrapped in scroll container)
		if modelEditor, ok := a.nonmemEditor.(*editor.ModelEditor); ok {
			modelEditor.SetText(newContent)
		}
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

// onRunClicked dispatches a run based on the selected execution target.
func (a *App) onRunClicked() {
	// SCM cannot run without a config file.
	psnMode := a.config != nil && a.config.ExecutionMode == config.ExecutionModePSN
	if a.psnForm != nil && psnMode && a.psnForm.scmConfigMissing(a.selectedPSNFunction()) {
		dialog.ShowError(fmt.Errorf("SCM requires a config file (set it in the SCM parameters)"), a.window)

		return
	}

	switch a.targetRadio.Selected {
	case "Scheduler":
		a.runRemote = false
		a.showGridConfigurationModal()
	case "SSH":
		a.runRemote = true
		a.executeRun(false)
	default: // Here
		a.runRemote = false
		a.executeRun(false)
	}
}

// refreshTargetOptions gates the execution-target options by execution mode and
// remote configuration, and shows the Hermes config button only in HERMES mode.
func (a *App) refreshTargetOptions() {
	if a.targetRadio == nil {
		return
	}

	mode := ""
	remoteConfigured := false

	if a.config != nil {
		mode = a.config.ExecutionMode
		remoteConfigured = a.config.Remote.Host != ""
	}

	options := []string{"Here"}

	switch mode {
	case config.ExecutionModeNONMEM, config.ExecutionModePSN:
		options = append(options, "Scheduler")
		if remoteConfigured {
			options = append(options, "SSH")
		}
	}

	a.targetRadio.Options = options
	if !slices.Contains(options, a.targetRadio.Selected) {
		a.targetRadio.SetSelected("Here")
	}

	a.targetRadio.Refresh()

	if a.hermesConfigBtn != nil {
		if mode == config.ExecutionModeHERMES {
			a.hermesConfigBtn.Show()
		} else {
			a.hermesConfigBtn.Hide()
		}
	}

	// Engine/target changes (e.g. after a settings save) change the command and
	// the retained-files summary.
	a.refreshCommandPreview()
	a.refreshRetainInfo()
}

// currentModelRetain resolves the retain globs of the current model's output
// files of interest, and a short label for where they come from: the model's own
// .janus.config.json retain, else the global default, else the NONMEM defaults.
// Retain is a model-wide property, so this reads it via LoadModelRetain (no Hermes
// validation) and surfaces it for any loaded model, not just Hermes runs.
func (a *App) currentModelRetain() (globs []string, source string) {
	if a.currentFilePath != "" {
		if modelRetain := config.LoadModelRetain(a.currentFilePath); len(modelRetain) > 0 {
			return modelRetain, "this model"
		}
	}

	if a.config != nil && len(a.config.Hermes.Retain) > 0 {
		return a.config.Hermes.Retain, "global default"
	}

	return config.DefaultNONMEMRetain(), "NONMEM defaults"
}

// refreshRetainInfo updates the primary-window "Output files of interest"
// summary. Retain is a model-wide property (it drives run-log embedding for every
// run, not just Hermes), so this is shown for any loaded model.
func (a *App) refreshRetainInfo() {
	if a.retainInfoBox == nil || a.retainGlobsLabel == nil {
		return
	}

	if a.currentFilePath == "" {
		a.retainInfoBox.Hide()

		return
	}

	globs, source := a.currentModelRetain()
	a.retainGlobsLabel.SetText(fmt.Sprintf("%s\n(source: %s — collected after a run and embedded in the run log)", strings.Join(globs, ", "), source))
	a.retainInfoBox.Show()
}

// showEditRetainDialog opens a small editor for the current model's output files
// of interest (its retain globs). It writes the model-wide ModelConfig.Retain via
// saveModelRetain, which works for any model — Hermes or not.
func (a *App) showEditRetainDialog() {
	if a.currentFilePath == "" {
		dialog.ShowInformation("No Model Loaded", "Load a model first to edit its output files of interest.", a.window)

		return
	}

	seed, _ := a.currentModelRetain()
	editor := newStringListEditor("Add retain glob", "e.g. *.lst", seed...)

	content := container.NewVBox(
		widget.NewLabel("Glob patterns for this model's output files of interest.\nThese files are collected after a run and embedded in the run log."),
		editor.widget(),
	)

	d := dialog.NewCustomConfirm("Output Files of Interest", "Save", "Cancel", content, func(save bool) {
		if !save {
			return
		}

		if err := a.saveModelRetain(a.currentFilePath, editor.items()); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save output files: %w", err), a.window)

			return
		}

		a.refreshRetainInfo()
		a.showSuccessToast(fmt.Sprintf("Saved output files of interest for %s", filepath.Base(a.currentFilePath)))
	}, a.window)

	d.Resize(fyne.NewSize(520, 420))
	d.Show()
}

// saveModelRetain writes the model-wide retain list to the model's
// .janus.config.json, preserving all other settings. It uses LoadModelConfig (no
// Hermes validation) and starts a fresh config when none exists, so it works for
// non-Hermes models that only declare their output files.
func (a *App) saveModelRetain(modelPath string, retain []string) error {
	cfg, err := config.LoadModelConfig(modelPath)
	if err != nil || cfg == nil {
		cfg = &config.ModelConfig{}
	}

	cfg.Retain = retain

	return config.SaveModelConfig(modelPath, cfg)
}

// psnDefaultPreset is the run-panel label for the plain PsN execute tool.
const psnDefaultPreset = "execute (default)"

// refreshPSNPresets populates the PSN analysis-preset picker (shown whenever the
// PsN engine is selected, for any destination) from the configured presets.
func (a *App) refreshPSNPresets() {
	if a.psnPresetSelect == nil || a.psnPresetContainer == nil {
		return
	}

	if !a.engineIsPSN() {
		a.psnPresetContainer.Hide()

		if a.psnForm != nil {
			a.psnForm.show("")
		}

		return
	}

	// Standard PsN analyses (execute is the default), plus any custom presets.
	options := []string{psnDefaultPreset, "vpc", "bootstrap", "scm"}
	for _, p := range a.config.PSN.Presets {
		if p.Name != "" && !slices.Contains(options, p.Name) {
			options = append(options, p.Name)
		}
	}

	a.psnPresetSelect.Options = options
	if !slices.Contains(options, a.psnPresetSelect.Selected) {
		a.psnPresetSelect.SetSelected(psnDefaultPreset)
	}

	a.psnPresetSelect.Refresh()
	a.psnPresetContainer.Show()

	if a.psnForm != nil {
		a.psnForm.show(a.selectedPSNFunction())
	}
}

// engineIsPSN reports whether the PsN engine is selected. It reads the Engine
// axis (always populated at runtime by normalizeExecutionAxes), falling back to
// the legacy ExecutionMode for configs/tests that set only the flat field. The
// engine — not the derived ExecutionMode — is the right gate: Engine=PSN with the
// Hermes destination derives ExecutionMode=HERMES, but the run is still PsN (e.g.
// the bootstrap saga), so the analysis picker must stay visible.
func (a *App) engineIsPSN() bool {
	if a.config == nil {
		return false
	}

	if a.config.Engine != "" {
		return a.config.Engine == config.EnginePSN
	}

	return a.config.ExecutionMode == config.ExecutionModePSN
}

// selectedPSNFunction returns the chosen PsN analysis (built-in or preset name),
// or "" for the plain execute default.
func (a *App) selectedPSNFunction() string {
	if a.psnPresetSelect == nil {
		return ""
	}

	if s := a.psnPresetSelect.Selected; s != "" && s != psnDefaultPreset {
		return s
	}

	return ""
}

func (a *App) executeRun(isGrid bool) {
	if a.currentFilePath == "" {
		dialog.ShowError(fmt.Errorf("no model file loaded"), a.window)

		return
	}

	// Check if modeling software license is available
	if !a.modelingLicenseFound {
		dialog.ShowError(fmt.Errorf("cannot execute model:\n\n%s", a.modelingLicenseMessage), a.window)

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
	// Point the active NONMEM install at the selected version (no-op unless
	// multiple installations are configured).
	a.applySelectedInstallation()

	// Hermes mode doesn't support grid execution (container-only)
	if isGrid && a.config != nil && a.config.ExecutionMode == config.ExecutionModeHERMES {
		dialog.ShowError(fmt.Errorf("hermes execution mode does not support grid execution - it runs in local containers only"), a.window)

		return
	}

	// Validate NONMEM license at execution time (if required by execution mode)
	if a.config != nil && config.RequiresNONMEMLicense(a.config.ExecutionMode) {
		_, err := config.ValidateNONMEMLicenseForExecution(a.config)
		if err != nil {
			dialog.ShowError(fmt.Errorf("cannot execute model: %w", err), a.window)

			return
		}
	}

	// Check for Hermes config BEFORE creating run record
	if a.config != nil && a.config.ExecutionMode == config.ExecutionModeHERMES {
		// Check if .janus.config.json exists
		factory := execution.NewExecutorFactory(a.config)
		_, err := factory.CreateHermesExecutor(a.currentFilePath)

		// If config missing, show dialog and don't create run record yet
		if err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), "hermes execution requires .janus.config.json")) {
			a.showHermesConfigDialog(a.currentFilePath,
				// onSuccess: Retry execution after config created
				func() {
					a.executeRunWithDescription(isGrid, isParallel, cores, description)
				},
				// onCancel: Do nothing (no run record to clean up)
				nil,
			)

			return
		}

		// If other error, show it
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to create Hermes executor: %w", err), a.window)

			return
		}
	}

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

	return mcpservice.WritePnmFile(a.currentFilePath, cores)
}

// isNonmemModeName reports whether an execution mode uses a local NONMEM
// installation (and thus the per-run version picker).
func isNonmemModeName(mode string) bool {
	switch mode {
	case config.ExecutionModeNONMEM, config.ExecutionModeBBI, config.ExecutionModePSN:
		return true
	default:
		return false
	}
}

// refreshVersionSelect repopulates the per-run NONMEM version picker, showing it
// only when multiple installations are configured for a NONMEM-style mode.
func (a *App) refreshVersionSelect() {
	if a.versionSelect == nil || a.versionContainer == nil {
		return
	}

	if a.config == nil || len(a.config.Installations) == 0 || !isNonmemModeName(a.config.ExecutionMode) {
		a.versionContainer.Hide()

		return
	}

	a.versionSelect.Options = a.config.InstallationNames()
	a.versionSelect.SetSelected(a.config.DefaultNonmemInstallation().Name)
	a.versionContainer.Show()
}

// applySelectedInstallation points the active NONMEM path/binary at the
// installation chosen in the version picker, just before a run. It is a no-op
// unless multiple installations are configured and one is selected.
func (a *App) applySelectedInstallation() {
	if a.versionSelect == nil || a.config == nil || a.versionSelect.Selected == "" {
		return
	}

	if inst, ok := a.config.NonmemInstallation(a.versionSelect.Selected); ok {
		a.config.NonmemPath = inst.Path
		a.config.NonmemBinary = inst.Binary
	}
}

func (a *App) showToast(title, message string, duration time.Duration) {
	// Skip toast if no window (e.g., in tests)
	if a.window == nil {
		return
	}

	// All UI operations must be on the main thread
	fyne.Do(func() {
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
		// Use a separate goroutine to avoid blocking, but protect against race conditions
		go func() {
			time.Sleep(duration)
			// Check if popup is still valid before hiding (UI update must be on main thread)
			// This prevents panic if the popup was already hidden or the window closed
			fyne.Do(func() {
				//nolint:staticcheck // SA9003: intentionally empty - silently ignore panic from hiding already-hidden popup
				defer func() {
					if r := recover(); r != nil {
					}
				}()
				popup.Hide()
			})
		}()
	})
}

func (a *App) showSuccessToast(message string) {
	a.showToast("SUCCESS", message, 500*time.Millisecond)
}

func (a *App) showInfoToast(title, message string) {
	a.showToast(strings.ToUpper(title), message, 4*time.Second)
}

func (a *App) showWarningToast(message string) {
	a.showToast("WARNING", message, 5*time.Second)
}

// buildActualExecutionCommand builds the actual command string that will be executed
// using the same logic as the executor to ensure run log accuracy.
// refreshCommandPreview updates the inline command preview to mirror what the
// current run-panel selections will run and record. It is a no-op until the
// preview widget exists, and shows a placeholder until a model is loaded.
func (a *App) refreshCommandPreview() {
	if a.commandPreview == nil {
		return
	}

	if a.currentFilePath == "" {
		a.commandPreview.SetText("(load a model to preview the command)")

		return
	}

	// Reflect the per-run NONMEM installation choice so the previewed binary
	// path matches what will actually run (idempotent; no-op without versions).
	a.applySelectedInstallation()

	isGrid := a.targetRadio != nil && a.targetRadio.Selected == "Scheduler"
	isParallel := a.syncRadio != nil && a.syncRadio.Selected == "Parallel"

	cores := 1
	if isParallel && a.coresEntry != nil {
		if n, err := strconv.Atoi(strings.TrimSpace(a.coresEntry.Text)); err == nil && n > 0 {
			cores = n
		}
	}

	var nonmemOptions *string
	if a.nonmemOptionsEntry != nil {
		if opts := strings.TrimSpace(a.nonmemOptionsEntry.Text); opts != "" {
			nonmemOptions = &opts
		}
	}

	a.commandPreview.SetText(a.buildActualExecutionCommand(isGrid, isParallel, cores, nonmemOptions))
}

func (a *App) buildActualExecutionCommand(isGrid, isParallel bool, cores int, nonmemOptions *string) string {
	// Handle different execution modes
	if a.config == nil {
		return fmt.Sprintf("nonmem %s", filepath.Base(a.currentFilePath))
	}

	// Any PsN-engine run with a selected analysis records the PsN command the user
	// invoked (e.g. "vpc acop.mod -samples=200"), not a NONMEM command — even when
	// the destination is Hermes (which derives ExecutionMode=HERMES) or it's the
	// bootstrap saga. Reuse the canonical PsN command builder so the run-log row
	// mirrors the real dispatch (and the result-reopen button can detect the tool).
	if a.engineIsPSN() && a.selectedPSNFunction() != "" {
		return a.buildPSNCommandString(isParallel, cores, isGrid, nil)
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
	modelPath := a.currentFilePath
	if isGrid {
		if absModelPath, err := filepath.Abs(a.currentFilePath); err == nil {
			modelPath = absModelPath
		}
	}

	// Reflect the selected PsN analysis (vpc / bootstrap / scm / preset) and its
	// typed parameters, mirroring the real RunFunction dispatch. The function's
	// args precede the free-form additional options.
	fn := a.selectedPSNFunction()

	psnArgs := additionalOptions
	if fn != "" && a.psnForm != nil {
		psnArgs = append(a.psnForm.args(fn), additionalOptions...)
	}

	if psnExec, ok := execution.NewPSNExecutor(a.config).(*execution.PSNExecutor); ok {
		if binary, args, err := psnExec.BuildFunctionCommand(fn, modelPath, isParallel, cores, isGrid, psnArgs); err == nil {
			return strings.Join(append([]string{binary}, args...), " ")
		}
	}

	// Fallback: plain execute (e.g. config unavailable).
	return strings.Join(append([]string{"execute", modelPath}, additionalOptions...), " ")
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

func (a *App) createRunRecord(isGrid, isParallel bool, cores int, description *string, nonmemOptions *string) *runlog.RunRecord {
	// Build the actual command that will be executed using the same logic as the executor
	command := a.buildActualExecutionCommand(isGrid, isParallel, cores, nonmemOptions)

	runRecord := &runlog.RunRecord{
		// ID and Timestamp will be set by RunLogStore.AddRun if not provided
		ModelFile:  a.currentFilePath,
		Command:    command,
		ExitCode:   -1, // Not completed yet
		IsParallel: isParallel,
		Cores:      cores,
		IsGrid:     isGrid,
		Status:     "running",
	}

	// Mark a bootstrap saga as the saga parent up front (before signing) so its
	// live "running" record renders the saga progress card — Kind drives that — and
	// so Kind is covered by the signature from the start rather than only being set
	// once the saga completes.
	if psnFn := a.selectedPSNFunction(); execution.IsBootstrapSagaRun(a.localRunConfig(), psnFn) {
		runRecord.Kind = runlog.KindSaga
	}

	// Only set description if provided (use compressed storage)
	if description != nil && *description != "" {
		if err := runRecord.SetDescription(*description); err != nil {
			runRecord.Description = *description // Fallback to uncompressed
		}
	}

	// Only set NONMEM options if provided and execution mode is NONMEM
	if nonmemOptions != nil && *nonmemOptions != "" && a.config != nil && a.config.ExecutionMode == "NONMEM" {
		runRecord.NonmemOptions = nonmemOptions
	}

	// Add to store (generates UUID, signs if configured, writes atomically)
	if a.runLogStore != nil {
		if err := a.runLogStore.AddRun(runRecord); err != nil {
			a.sendError(fmt.Errorf("failed to save run record: %w", err))
		}
	}

	// Refresh cached runs for table display
	a.refreshCachedRuns()

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

	// UI updates must be on the main thread
	fyne.Do(func() {
		// Create and add Run Details tab without selecting it
		a.runDetailsTab = container.NewTabItem("Run Details", a.buildRunDetailsTab())
		a.modelSubTabs.Append(a.runDetailsTab)

		// Refresh the run history table
		a.runHistoryTable.Refresh()
	})
}

func (a *App) updateRunRecord(runID string, exitCode int, stdout, stderr string, container *runlog.ContainerProvenance) {
	if a.runLogStore == nil {
		return
	}

	// The retain globs (model's output files of interest) select what gets
	// embedded in the run log. currentModelRetain resolves per-model → global →
	// NONMEM default for the loaded model.
	retain, _ := a.currentModelRetain()
	if err := mcpservice.ApplyRunResult(a.errorCtx, a.runLogStore, runID, exitCode, stdout, stderr, container, retain); err != nil {
		a.sendError(err)
	}

	// Refresh cached runs
	a.refreshCachedRuns()

	// Refresh UI if Run Details tab is visible (must be on main thread)
	if a.runHistoryTable != nil {
		fyne.Do(func() {
			a.runHistoryTable.Refresh()
		})
	}
}

func (a *App) setupModelRunHistory(modelFilePath string) {
	// Clear any previous run selections when switching models
	a.clearRunSelections()

	modelDir := filepath.Dir(modelFilePath)
	modelName := strings.TrimSuffix(filepath.Base(modelFilePath), filepath.Ext(modelFilePath))

	// Create RunLogStore for this model (guarded; MCP HTTP goroutines read
	// runLogStore via resolveRunLogStore).
	a.modelMu.Lock()
	a.runLogStore = runlog.NewRunLogStore(modelDir, modelName)
	a.modelMu.Unlock()

	// Configure signer if available
	if a.signer != nil && a.licenseClaims != nil {
		a.runLogStore.SetSigner(a.signer, a.licenseClaims.UserEmail)
	}

	// Configure the verification trust anchor (nil degrades to Unverifiable).
	a.runLogStore.SetTrustStore(a.trustStore)

	// Load existing run logs (creates directory structure if needed)
	if err := a.runLogStore.Load(); err != nil {
		a.sendError(fmt.Errorf("failed to load run history: %w", err))
	}

	// Refresh cached runs for table display
	a.refreshCachedRuns()

	// If run history table exists, refresh it to show loaded data (must be on main thread)
	if a.runHistoryTable != nil {
		fyne.Do(func() {
			a.runHistoryTable.Refresh()
		})
	}
}

// refreshCachedRuns updates the local cache of runs for table display. Saga
// child fits (e.g. the per-resample bootstrap runs, #192) are collapsed out of
// the top-level history: they carry a ParentID and would otherwise flood the
// table with one row per fit. The parent saga record stands in for them and the
// children are reachable from its details view.
func (a *App) refreshCachedRuns() {
	if a.runLogStore == nil {
		a.cachedRuns = []runlog.RunRecord{}

		return
	}

	all := a.runLogStore.GetAllRuns()
	top := all[:0:0]

	for _, run := range all {
		if run.ParentID != "" {
			continue
		}

		top = append(top, run)
	}

	a.cachedRuns = top
}

func (a *App) buildActiveRunsTable() *fyne.Container {
	runningLocalRuns := a.getRunningLocalRuns()
	if len(runningLocalRuns) == 0 {
		return nil // No active runs, don't show the section
	}

	// A bootstrap saga parent (#192) gets a rich live-progress card; every other
	// active run stays in the per-run table.
	var sagaRuns, normalRuns []runlog.RunRecord
	for _, run := range runningLocalRuns {
		if run.Kind == runlog.KindSaga {
			sagaRuns = append(sagaRuns, run)
		} else {
			normalRuns = append(normalRuns, run)
		}
	}

	// The "Active Local Executions" header must remain the first child:
	// updateActiveRunsDisplay locates this section for removal by that exact label.
	header := container.NewHBox(
		widget.NewLabelWithStyle("Active Local Executions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)
	content := []fyne.CanvasObject{header}

	for _, sr := range sagaRuns {
		content = append(content, a.buildSagaProgressCard(sr))
	}

	if len(normalRuns) > 0 {
		content = append(content, a.buildNormalActiveRunsTable(normalRuns))
	}

	content = append(content, widget.NewSeparator(), widget.NewLabel("")) // trailing spacing

	return container.NewVBox(content...)
}

// buildNormalActiveRunsTable builds the per-run table for non-saga active runs.
func (a *App) buildNormalActiveRunsTable(runs []runlog.RunRecord) fyne.CanvasObject {
	a.activeRunsTable = widget.NewTable(
		func() (int, int) {
			return len(runs), 6 // rows, columns: Run, Model, Type, Elapsed, Live Output, Cancel
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel(""))
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			if id.Row >= len(runs) {
				return
			}

			run := runs[id.Row]
			container, ok := obj.(*fyne.Container)
			if !ok {
				return
			}
			container.Objects = nil // Clear existing objects

			elapsed := time.Since(run.Timestamp)

			switch id.Col {
			case 0: // Run ID
				container.Add(widget.NewLabel(fmt.Sprintf("#%s", shortRunID(run.ID))))
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

	// Set column widths - sized to fit content without overlap
	a.activeRunsTable.SetColumnWidth(0, 70)  // Run ID (#XXXXXX)
	a.activeRunsTable.SetColumnWidth(1, 150) // Model name
	a.activeRunsTable.SetColumnWidth(2, 130) // Execution type (Parallel (8 cores))
	a.activeRunsTable.SetColumnWidth(3, 60)  // Elapsed time (XXm XXs)
	a.activeRunsTable.SetColumnWidth(4, 110) // Live Output button
	a.activeRunsTable.SetColumnWidth(5, 75)  // Cancel button

	return a.activeRunsTable
}

// sagaStageText is the human-readable stage line for a saga progress card.
func sagaStageText(p execution.SagaProgress) string {
	switch p.Stage {
	case execution.SagaStageSetup:
		return "Stage: Resampling…"
	case execution.SagaStageFits:
		return "Stage: Fitting"
	case execution.SagaStageAggregate:
		return "Stage: Aggregating…"
	default:
		return "Stage: Starting…"
	}
}

// sagaCardMaxPods caps how many live pods the progress card lists before
// summarizing the remainder, so a large fan-out can't grow the card unbounded.
const sagaCardMaxPods = 10

// buildSagaProgressCard renders the live-progress card for a running bootstrap
// saga (#192/#197): stage, a k/N progress bar during the fits, the running tally,
// and the pods Janus is currently driving. The card is rebuilt from the latest
// snapshot on each ticker tick, so per-pod elapsed and counts stay live.
func (a *App) buildSagaProgressCard(run runlog.RunRecord) fyne.CanvasObject {
	a.sagaMu.Lock()
	p := a.sagaProgress[run.ID]
	a.sagaMu.Unlock()

	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Active Execution — Bootstrap #%s", shortRunID(run.ID)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	cancelBtn := widget.NewButton("Cancel", func() { a.cancelRun(run.ID) })
	cancelBtn.Importance = widget.DangerImportance

	stage := widget.NewLabel(sagaStageText(p))

	// Determinate bar with k/N (%) during the fits; an indeterminate bar while the
	// single setup/aggregate pod works (no count to show).
	var progress fyne.CanvasObject
	if p.Stage == execution.SagaStageFits && p.Total > 0 {
		bar := widget.NewProgressBar()
		bar.Max = float64(p.Total)
		bar.SetValue(float64(p.Done))
		bar.TextFormatter = func() string {
			pct := 0.0
			if p.Total > 0 {
				pct = float64(p.Done) / float64(p.Total) * 100
			}

			return fmt.Sprintf("%d/%d  (%.0f%%)", p.Done, p.Total, pct)
		}
		progress = bar
	} else {
		progress = widget.NewProgressBarInfinite()
	}

	counts := widget.NewLabel(fmt.Sprintf("✓ %d succeeded    ✗ %d failed", p.Succeeded, p.Failed))

	podsHeader := widget.NewLabelWithStyle(
		fmt.Sprintf("Pods running (%d / P=%d):", len(p.Live), a.bootstrapParallelism()),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	podRows := make([]fyne.CanvasObject, 0, len(p.Live))
	for i, pod := range p.Live {
		if i >= sagaCardMaxPods {
			podRows = append(podRows, widget.NewLabel(fmt.Sprintf("  … +%d more", len(p.Live)-sagaCardMaxPods)))

			break
		}

		podRows = append(podRows, widget.NewLabel(
			fmt.Sprintf("  %s    %s", pod.Name, formatDuration(time.Since(pod.Since)))))
	}

	if len(podRows) == 0 {
		podRows = append(podRows, widget.NewLabel("  (none)"))
	}

	pods := container.NewVScroll(container.NewVBox(podRows...))
	pods.SetMinSize(fyne.NewSize(0, 160))

	body := container.NewVBox(
		container.NewBorder(nil, nil, title, cancelBtn),
		stage,
		progress,
		counts,
		widget.NewSeparator(),
		podsHeader,
		pods,
	)

	return widget.NewCard("", "", body)
}

// bootstrapParallelism returns the configured fit-pod watermark (P) for display,
// or the default when unset.
func (a *App) bootstrapParallelism() int {
	if a.config != nil {
		return a.config.Hermes.Kubernetes.BootstrapParallelismOrDefault()
	}

	return config.DefaultBootstrapParallelism
}

func (a *App) getRunningLocalRuns() []runlog.RunRecord {
	var runningRuns []runlog.RunRecord

	for _, run := range a.cachedRuns {
		if run.Status == "running" && !run.IsGrid {
			runningRuns = append(runningRuns, run)
		}
	}

	return runningRuns
}

func (a *App) updateActiveRunsDisplay() {
	if a.activeRunsArea == nil {
		return
	}

	// All UI updates must be on the main thread. Repopulate the persistent bottom
	// slot rather than mutating the main Border (which can't place extra children).
	fyne.Do(func() {
		a.activeRunsArea.Objects = nil

		if section := a.buildActiveRunsTable(); section != nil {
			a.activeRunsArea.Add(section)
		}

		a.activeRunsArea.Refresh()
	})
}

func (a *App) showLiveOutput(runID string) {
	// Check if window already exists
	if window, exists := a.liveOutputWindows[runID]; exists {
		window.RequestFocus()

		return
	}

	// Check if streaming is available
	streaming, exists := a.activeStreams[runID]
	if !exists {
		a.sendError(fmt.Errorf("no live output available for run #%s", shortRunID(runID)))

		return
	}

	// Create new window for live output
	window := a.fyneApp.NewWindow(fmt.Sprintf("Live Output - Run #%s", shortRunID(runID)))
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

func (a *App) cancelRun(runID string) {
	if cancelFunc, exists := a.activeRuns[runID]; exists {
		cancelFunc() // Cancel the context
		delete(a.activeRuns, runID)

		// Update run status to failed with cancellation message
		a.updateRunRecord(runID, -1, "", "Run cancelled by user", nil)

		a.showInfoToast("Run Cancelled", fmt.Sprintf("Run #%s has been cancelled", shortRunID(runID)))

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

// localRunConfig returns the config for a non-grid run (Here or SSH). A "Here"
// run clears Remote so execution stays local; an SSH run keeps Remote so the
// executors run over SSH. It returns a shallow clone so a.config is never
// mutated and the implicit "remote when host set" only applies to the SSH target.
func (a *App) localRunConfig() *config.Config {
	if a.config == nil {
		return nil
	}

	runCfg := *a.config
	if !a.runRemote {
		runCfg.Remote = config.RemoteConfig{}
	}

	return &runCfg
}

func (a *App) executeLocalRun(runRecord *runlog.RunRecord) {
	modeText := "synchronous"
	if runRecord.IsParallel {
		modeText = fmt.Sprintf("parallel (%d cores)", runRecord.Cores)
	}

	a.showInfoToast("Run Started",
		fmt.Sprintf("Starting local %s NONMEM run...\nModel: %s",
			modeText, filepath.Base(a.currentFilePath)))

	// Create executor factory and get the appropriate executor. Use the per-run
	// config so a "Here" run stays local even when a remote host is configured.
	factory := execution.NewExecutorFactory(a.localRunConfig())
	if concreteFactory, ok := factory.(*execution.DefaultExecutorFactory); ok {
		concreteFactory.SetRunLogEnabled(a.HasFeature("runlog"))
	}

	// Horizontal PsN bootstrap saga (#192): a host-orchestrated Kubernetes fan-out,
	// distinct from the single-pod executors. Detect it before building a normal
	// executor and run it on its own path.
	if psnFn := a.selectedPSNFunction(); execution.IsBootstrapSagaRun(a.localRunConfig(), psnFn) {
		samples := 0
		if a.psnForm != nil {
			samples = parseSamplesArg(a.psnForm.args(psnFn))
		}

		a.runBootstrapSaga(factory, runRecord, samples)

		return
	}

	var executor execution.Executor
	var err error

	// Create appropriate executor (Hermes config already validated at this point)
	if a.config.ExecutionMode == config.ExecutionModeHERMES {
		executor, err = factory.CreateHermesExecutor(a.currentFilePath)
	} else {
		executor, err = factory.CreateExecutor(a.config.ExecutionMode)
	}

	if err != nil {
		a.sendError(fmt.Errorf("failed to create executor: %w", err))
		a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Executor creation failed: %s", err.Error()), nil)

		return
	}

	// Hermes + PsN engine + a selected analysis: run the PsN tool (vpc/scm/…) in
	// the container instead of NONMEM. The container image must provide PsN.
	if hx, ok := executor.(*execution.HermesExecutor); ok {
		if fn := a.selectedPSNFunction(); fn != "" && a.engineIsPSN() {
			var fnArgs []string
			if a.psnForm != nil {
				fnArgs = a.psnForm.args(fn)
			}

			hx.SetPSNFunction(fn, fnArgs)
		}
	}

	// Show the active runs table immediately
	a.updateActiveRunsDisplay()

	// Capture the PsN analysis + model dir now, so a successful run can render
	// its result (the picker may change after the run starts).
	psnResultFn := a.selectedPSNFunction()
	psnResultDir := filepath.Dir(a.currentFilePath)

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

		// Note: GUI streaming support via SetOutputWriters could be added here in the future
		// For now, GUI execution uses buffered output (non-streaming)

		// Parse additional NONMEM options
		var additionalOptions []string
		if runRecord.NonmemOptions != nil {
			additionalOptions = parseNonmemOptions(*runRecord.NonmemOptions)
		}

		// In PSN mode with a preset selected, run the preset (vpc/bootstrap/scm/…);
		// otherwise the plain execute. The remote/local choice is already baked
		// into the per-run config.
		var result *execution.ExecutionResult
		var err error
		if psnExec, ok := executor.(*execution.PSNExecutor); ok && a.selectedPSNFunction() != "" {
			fn := a.selectedPSNFunction()

			fnArgs := additionalOptions
			if a.psnForm != nil {
				fnArgs = append(a.psnForm.args(fn), additionalOptions...)
			}

			result, err = psnExec.RunFunction(ctx, fn, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, false, fnArgs)
		} else {
			result, err = executor.Execute(ctx, a.currentFilePath, runRecord.IsParallel, runRecord.Cores, false, additionalOptions)
		}
		if err != nil {
			a.sendError(fmt.Errorf("execution failed: %w", err))
			a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Execution failed: %s", err.Error()), nil)

			return
		}

		// Convert []byte to string for storage/display
		stdout := string(result.Stdout)
		stderr := string(result.Stderr)

		// Update run record with results (includes container provenance for Hermes executions)
		a.updateRunRecord(runRecord.ID, result.ExitCode, stdout, stderr, result.Container)

		// Show completion notification
		if result.ExitCode == 0 {
			a.showSuccessToast(fmt.Sprintf("NONMEM run #%s completed successfully", shortRunID(runRecord.ID)))

			// Render the PsN analysis result (bootstrap CIs / scm summary / vpc).
			if slices.Contains([]string{"bootstrap", "vpc", "scm"}, psnResultFn) {
				fyne.Do(func() { showPSNResultsDialog(a.window, psnResultFn, psnResultDir) })
			}
		} else {
			a.sendError(fmt.Errorf("NONMEM run #%s failed with exit code %d", shortRunID(runRecord.ID), result.ExitCode))
		}
	}()
}

// runBootstrapSaga executes the horizontal PsN bootstrap saga on Kubernetes in
// the background, then records it as a parent run with one child per fit
// (#192/#195/#196) and renders the confidence intervals on success.
func (a *App) runBootstrapSaga(factory execution.ExecutorFactory, runRecord *runlog.RunRecord, samples int) {
	a.updateActiveRunsDisplay()

	store := a.runLogStore
	modelPath := a.currentFilePath
	resultDir := filepath.Dir(modelPath)

	go func() {
		// Bootstrap fan-out can take a long time; give it a generous ceiling.
		ctx, cancel := context.WithTimeout(a.errorCtx, 6*time.Hour)
		defer func() {
			cancel()
			delete(a.activeRuns, runRecord.ID)
			a.updateActiveRunsDisplay()
		}()

		a.activeRuns[runRecord.ID] = cancel

		concrete, ok := factory.(*execution.DefaultExecutorFactory)
		if !ok {
			a.failSaga(store, runRecord, fmt.Errorf("executor factory does not support the bootstrap saga"))

			return
		}

		if samples <= 0 {
			a.failSaga(store, runRecord, fmt.Errorf("bootstrap requires a positive number of samples"))

			return
		}

		saga, err := concrete.CreateBootstrapSaga(modelPath)
		if err != nil {
			a.failSaga(store, runRecord, fmt.Errorf("failed to create bootstrap saga: %w", err))

			return
		}

		// Publish live progress (stage, k/N, running pods) for the active-runs card.
		// The 2s ticker redraws the card from the latest snapshot; the callback only
		// stores it (fires concurrently from fit goroutines).
		saga.SetProgressFunc(func(p execution.SagaProgress) {
			a.sagaMu.Lock()
			a.sagaProgress[runRecord.ID] = p
			a.sagaMu.Unlock()
		})

		res, err := saga.Run(ctx, modelPath, samples)
		if err != nil {
			a.failSaga(store, runRecord, err)

			return
		}

		a.recordSagaSuccess(store, runRecord, res)
		a.showSuccessToast(fmt.Sprintf("Bootstrap #%s: %d/%d fits succeeded",
			shortRunID(runRecord.ID), res.Succeeded, res.Samples))
		fyne.Do(func() { showPSNResultsDialog(a.window, "bootstrap", resultDir) })
	}()
}

// clearSagaProgress drops a finished saga's live-progress snapshot so the card
// stops being rendered once the run leaves the active set.
func (a *App) clearSagaProgress(runID string) {
	a.sagaMu.Lock()
	delete(a.sagaProgress, runID)
	a.sagaMu.Unlock()
}

// failSaga marks the saga's parent run failed and surfaces the error. A
// cancellation (the user hit Cancel, which cancels the saga context) is recorded
// as a cancellation rather than an error — the pods are still swept by the saga's
// deferred label cleanup either way.
func (a *App) failSaga(store *runlog.RunLogStore, runRecord *runlog.RunRecord, err error) {
	cancelled := errors.Is(err, context.Canceled)

	if cancelled {
		a.sendError(fmt.Errorf("bootstrap saga cancelled: %w", err))
	} else {
		a.sendError(fmt.Errorf("bootstrap saga failed: %w", err))
	}

	a.clearSagaProgress(runRecord.ID)

	if store == nil {
		return
	}

	description := fmt.Sprintf("Bootstrap saga failed: %s", err.Error())
	if cancelled {
		description = "Bootstrap saga cancelled by user; requested pods torn down."
	}

	runRecord.Kind = runlog.KindSaga
	runRecord.ExitCode = -1
	runRecord.Status = "failed"
	_ = runRecord.SetDescription(description)

	if uerr := store.UpdateRun(runRecord); uerr != nil {
		log.Printf("Warning: failed to record failed bootstrap saga: %v", uerr)
	}

	a.refreshCachedRuns()
}

// recordSagaSuccess updates the parent run with the saga summary and records one
// child run per fit, linked via RunLogStore.AddSaga.
func (a *App) recordSagaSuccess(store *runlog.RunLogStore, runRecord *runlog.RunRecord, res *execution.BootstrapSagaResult) {
	a.clearSagaProgress(runRecord.ID)

	if store == nil {
		return
	}

	runRecord.ExitCode = 0
	runRecord.Status = "completed"
	_ = runRecord.SetDescription(fmt.Sprintf(
		"Bootstrap saga: %d samples, %d succeeded, %d failed. Results: %s",
		res.Samples, res.Succeeded, res.Failed, strings.Join(res.OutputFiles, ", ")))

	children := make([]*runlog.RunRecord, 0, res.Samples)
	for i := 1; i <= res.Samples; i++ {
		status := "completed"
		exit := 0

		if _, failed := res.FitErrors[i]; failed {
			status = "failed"
			exit = -1
		}

		children = append(children, &runlog.RunRecord{
			ModelFile: runRecord.ModelFile,
			Command:   fmt.Sprintf("nonmem bs_pr1_%d.mod bs_pr1_%d.lst", i, i),
			ExitCode:  exit,
			Status:    status,
		})
	}

	if err := store.AddSaga(runRecord, children); err != nil {
		log.Printf("Warning: failed to record bootstrap saga to run log: %v", err)
	}

	a.refreshCachedRuns()
}

// parseSamplesArg extracts the integer N from a "-samples=N" argument, returning
// 0 when none is present.
func parseSamplesArg(args []string) int {
	for _, arg := range args {
		if v, ok := strings.CutPrefix(arg, "-samples="); ok {
			n, _ := strconv.Atoi(strings.TrimSpace(v))

			return n
		}
	}

	return 0
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

func (a *App) createGridRunRecord(gridSettings *GridSettings, description string) *runlog.RunRecord {
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

	runRecord := &runlog.RunRecord{
		// ID and Timestamp will be set by RunLogStore.AddRun if not provided
		ModelFile:  a.currentFilePath,
		Command:    command,
		ExitCode:   -1, // Not completed yet
		IsParallel: gridSettings.NONMEM.Parallel,
		Cores:      gridSettings.NONMEM.Threads,
		IsGrid:     true,
		Status:     "running",
	}

	// Add description if provided (use compressed storage)
	if description != "" {
		if err := runRecord.SetDescription(description); err != nil {
			runRecord.Description = description // Fallback to uncompressed
		}
	}

	// Add NONMEM options if provided
	if len(gridSettings.NONMEM.AdditionalOptions) > 0 {
		optionsStr := strings.Join(gridSettings.NONMEM.AdditionalOptions, " ")
		runRecord.NonmemOptions = &optionsStr
	}

	// Add to store (generates UUID, signs if configured, writes atomically)
	if a.runLogStore != nil {
		if err := a.runLogStore.AddRun(runRecord); err != nil {
			a.sendError(fmt.Errorf("failed to save run record: %w", err))
		}
	}

	// Refresh cached runs for table display
	a.refreshCachedRuns()

	return runRecord
}

func (a *App) executeGridRun(runRecord *runlog.RunRecord) {
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
		concreteFactory.SetRunLogEnabled(a.HasFeature("runlog"))
	}

	var executor execution.Executor
	var err error

	// Create appropriate executor (Hermes config already validated at this point)
	if a.config.ExecutionMode == config.ExecutionModeHERMES {
		executor, err = factory.CreateHermesExecutor(a.currentFilePath)
	} else {
		executor, err = factory.CreateExecutor(a.config.ExecutionMode)
	}

	if err != nil {
		a.sendError(fmt.Errorf("failed to create executor: %w", err))
		a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Executor creation failed: %s", err.Error()), nil)

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
			a.updateRunRecord(runRecord.ID, -1, "", fmt.Sprintf("Grid execution failed: %s", err.Error()), nil)

			return
		}

		// Convert []byte to string for storage/display
		stdout := string(result.Stdout)
		stderr := string(result.Stderr)

		// Update run record with results (includes container provenance for Hermes executions)
		a.updateRunRecord(runRecord.ID, result.ExitCode, stdout, stderr, result.Container)

		// Show completion notification
		if result.ExitCode == 0 {
			a.showSuccessToast(fmt.Sprintf("Grid job #%s completed successfully", shortRunID(runRecord.ID)))
		} else {
			a.sendError(fmt.Errorf("grid job #%s failed with exit code %d", shortRunID(runRecord.ID), result.ExitCode))
		}
	}()
}
