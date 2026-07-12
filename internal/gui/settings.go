package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/pirana"
	"github.com/pharmalytica/janus/internal/qa"
	"github.com/pharmalytica/janus/internal/qarun"
	"github.com/pharmalytica/janus/internal/scheduler"
	"github.com/pharmalytica/janus/internal/summary"
)

// SettingsDialog represents the configuration settings editor dialog.
type SettingsDialog struct {
	window fyne.Window
	app    *App

	// Form widgets
	organizationEntry    *widget.Entry
	defaultDirEntry      *widget.Entry
	runPrefixEntry       *widget.Entry
	altDataDirEntry      *widget.Entry
	closeConsoleCheck    *widget.Check
	engineSelect         *widget.Select
	destinationSelect    *widget.Select
	orchestratorSelect   *widget.Select
	orchestratorNote     *widget.Label
	kubeconfigEntry      *widget.Entry
	kubeContextEntry     *widget.Entry
	kubeNamespaceEntry   *widget.Entry
	kubePullSecretEntry  *widget.Entry
	kubeParallelismEntry *widget.Entry // bootstrap saga fit-pod watermark (P)
	kubeFitTimeoutEntry  *widget.Entry // bootstrap saga per-fit deadline (Go duration)
	kubernetesSection    *fyne.Container
	commandPreview       *widget.Label
	schedulerSelect      *widget.Select
	nonmemPathEntry      *widget.Entry
	nonmemBinaryEntry    *widget.Entry
	installEditor        *installationsEditor

	// Live-validation controls
	saveBtn             *widget.Button
	validationLabel     *widget.Label
	profileSelect       *widget.Select
	dockerSocketEntry   *widget.Entry
	startupTimeoutEntry *widget.Entry
	autoCleanupCheck    *widget.Check
	validationIQEntry   *widget.Entry
	validationOQEntry   *widget.Entry

	// Runtime qualification (IQ/OQ)
	runIQBtn      *widget.Button
	runOQBtn      *widget.Button
	downloadIQBtn *widget.Button
	downloadOQBtn *widget.Button
	lastIQLabel   *widget.Label
	lastOQLabel   *widget.Label

	// MCP server section
	mcpEnabledCheck      *widget.Check
	mcpHostEntry         *widget.Entry
	mcpPortEntry         *widget.Entry
	mcpAllowExecuteCheck *widget.Check
	mcpTokenEntry        *widget.Entry
	mcpStatusLabel       *widget.Label
	mcpStartStopBtn      *widget.Button

	// Execution — run-output policy
	overwritePolicySelect *widget.Select
	autoBackupCheck       *widget.Check
	cleanupGlobsEditor    *stringListEditor

	// NONMEM license + Hermes advanced
	nonmemLicenseEntry  *widget.Entry
	hermesImageEntry    *widget.Entry
	hermesPsnImageEntry *widget.Entry // global default PsN orchestration image
	hermesCPUEntry      *widget.Entry
	hermesMemEntry      *widget.Entry
	hermesTimeoutEntry  *widget.Entry
	hermesPortEntry     *widget.Entry
	hermesRetainEditor  *stringListEditor

	// Tools & Integrations
	psnPathEntry           *widget.Entry
	psnConfEntry           *widget.Entry
	psnPreEntry            *widget.Entry
	psnPostEntry           *widget.Entry
	psnPresetsEditor       *recordListEditor
	integrationsRPathEntry *widget.Entry
	hooksEditor            *recordListEditor
	toolsEditor            *recordListEditor

	// Remote & Schedulers
	remoteHostEntry    *widget.Entry
	remoteUserEntry    *widget.Entry
	remotePortEntry    *widget.Entry
	remoteKeyEntry     *widget.Entry
	remoteMountsEditor *recordListEditor

	slurmModeSelect       *widget.Select
	slurmHostEntry        *widget.Entry
	slurmPortEntry        *widget.Entry
	slurmTimeoutEntry     *widget.Entry
	slurmRestSocketEntry  *widget.Entry
	slurmRestVersionEntry *widget.Entry
	slurmRestTimeoutEntry *widget.Entry
	slurmRestTokenEntry   *widget.Entry

	schedulerProfiles       []scheduler.Profile
	schedulerProfilesEditor *recordListEditor

	// Compliance
	signingKeyEntry    *widget.Entry
	runLogBackendEntry *widget.Entry
	runLogPathEntry    *widget.Entry

	// Conditional sections
	nonmemSection    *fyne.Container
	hermesSection    *fyne.Container
	schedulerSection *fyne.Container

	// Original values for change detection
	originalValues map[string]string
}

// NewSettingsDialog creates a new settings dialog for the given app.
func NewSettingsDialog(app *App) *SettingsDialog {
	return &SettingsDialog{
		app:            app,
		originalValues: make(map[string]string),
	}
}

// Show displays the settings dialog as a modal window.
func (s *SettingsDialog) Show() {
	// Create modal window. Resizable, since the tabbed layout benefits from extra
	// height on larger displays.
	s.window = s.app.fyneApp.NewWindow("Janus Settings")
	s.window.Resize(fyne.NewSize(760, 620))

	// Build form
	content := s.buildForm()

	s.window.SetContent(content)
	s.window.Show()
}

func (s *SettingsDialog) buildForm() *fyne.Container {
	// Read-only info section
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = "(no config file loaded)"
	}

	infoSection := container.NewVBox(
		widget.NewLabelWithStyle("Configuration File", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel(configPath),
		widget.NewSeparator(),
	)

	// General settings
	s.organizationEntry = widget.NewEntry()
	s.organizationEntry.SetText(s.app.config.Organization)
	s.originalValues["organization"] = s.app.config.Organization

	s.defaultDirEntry = widget.NewEntry()
	s.defaultDirEntry.SetText(s.app.config.DefaultDirectory)
	s.originalValues["default-directory"] = s.app.config.DefaultDirectory

	s.runPrefixEntry = widget.NewEntry()
	s.runPrefixEntry.SetPlaceHolder("e.g., run_")
	s.runPrefixEntry.SetText(s.app.config.RunPrefix)
	s.originalValues["run-prefix"] = s.app.config.RunPrefix

	s.altDataDirEntry = widget.NewEntry()
	s.altDataDirEntry.SetPlaceHolder("Alternate directory to search for data files")
	s.altDataDirEntry.SetText(s.app.config.AltDataDirectory)
	s.originalValues["alt-data-directory"] = s.app.config.AltDataDirectory

	s.closeConsoleCheck = widget.NewCheck("Close run console after a run finishes", nil)
	s.closeConsoleCheck.SetChecked(s.app.config.CloseConsoleAfterRun)
	s.originalValues["close-console-after-run"] = strconv.FormatBool(s.app.config.CloseConsoleAfterRun)

	generalSection := container.NewVBox(
		widget.NewLabelWithStyle("General Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Organization *:"),
		s.organizationEntry,
		widget.NewLabel("Default Directory *:"),
		s.defaultDirEntry,
		widget.NewLabel("Run/Model Prefix:"),
		s.runPrefixEntry,
		widget.NewLabel("Alternative Data Directory:"),
		s.altDataDirEntry,
		s.closeConsoleCheck,
		widget.NewSeparator(),
	)

	// Execution configuration — two orthogonal axes: the Engine (the command
	// structure) and the Destination (where it runs). The legacy ExecutionMode
	// is derived from these on save.
	// OnChanged is wired at the end of buildForm — see wireExecutionAxisHandlers.
	// Wiring it here would fire during the SetSelected calls below, before later
	// widgets (e.g. hermesImageEntry) the preview reads even exist.
	s.engineSelect = widget.NewSelect(config.GetValidEngines(), nil)
	s.engineSelect.SetSelected(s.app.config.Engine)
	s.originalValues["engine"] = s.app.config.Engine

	s.destinationSelect = widget.NewSelect(config.GetValidDestinations(), nil)
	s.destinationSelect.SetSelected(s.app.config.Destination)
	s.originalValues["destination"] = s.app.config.Destination

	schedulers := schedulerOptions(s.app.config)
	s.schedulerSelect = widget.NewSelect(schedulers, nil)
	s.schedulerSelect.SetSelected(s.app.config.Scheduler)
	s.originalValues["scheduler"] = s.app.config.Scheduler

	// Read-only command preview, assembled from the selected axes and the
	// engine's configured fields. Updated live as those inputs change.
	s.commandPreview = widget.NewLabel("")
	s.commandPreview.Wrapping = fyne.TextWrapWord
	s.commandPreview.TextStyle = fyne.TextStyle{Monospace: true}

	executionSection := container.NewVBox(
		widget.NewLabelWithStyle("Execution Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Engine (command structure):"),
		s.engineSelect,
		widget.NewLabel("Destination (where it runs):"),
		s.destinationSelect,
		widget.NewLabel("Command preview:"),
		s.commandPreview,
	)

	// Scheduler section (hidden for HERMES mode)
	s.schedulerSection = container.NewVBox(
		widget.NewLabel("Scheduler:"),
		s.schedulerSelect,
		widget.NewButton("🔍 Test polling regex…", func() { showRegexTester(s.window) }),
	)
	// The scheduler section now lives in the "Remote & Schedulers" tab.
	executionSection.Add(widget.NewSeparator())

	// NONMEM-specific configuration
	s.nonmemPathEntry = widget.NewEntry()
	s.nonmemPathEntry.SetText(s.app.config.NonmemPath)
	s.originalValues["nonmem-path"] = s.app.config.NonmemPath

	s.nonmemBinaryEntry = widget.NewEntry()
	s.nonmemBinaryEntry.SetText(s.app.config.NonmemBinary)
	s.originalValues["nonmem-binary"] = s.app.config.NonmemBinary

	s.installEditor = newInstallationsEditor(s.app.config.Installations)
	s.originalValues["installations"] = serializeInstalls(s.app.config.Installations)

	s.nonmemLicenseEntry = widget.NewEntry()
	s.nonmemLicenseEntry.SetPlaceHolder("Path to nonmem.lic (optional)")
	s.nonmemLicenseEntry.SetText(s.app.config.NONMEM.License.Path)
	s.originalValues["nonmem-license"] = s.app.config.NONMEM.License.Path

	s.nonmemSection = container.NewVBox(
		widget.NewLabelWithStyle("NONMEM Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("NONMEM Path *:"),
		s.nonmemPathEntry,
		widget.NewLabel("NONMEM Binary *:"),
		s.nonmemBinaryEntry,
		widget.NewLabel("License path:"),
		s.nonmemLicenseEntry,
		widget.NewLabel("Installations (optional — overrides the single path/binary; mark one default):"),
		s.installEditor.widget(),
		widget.NewSeparator(),
	)

	// Hermes-specific configuration
	//
	// Orchestrator picks where the Hermes pod runs: Docker (local) or Kubernetes.
	// The Kubernetes fields below are revealed only when Kubernetes is selected.
	// OnChanged wired later (see wireExecutionAxisHandlers), as for the other axes.
	s.orchestratorSelect = widget.NewSelect(config.GetValidOrchestrators(), nil)
	orchestrator := s.app.config.Orchestrator
	if orchestrator == "" {
		orchestrator = config.OrchestratorDocker
	}
	s.orchestratorSelect.SetSelected(orchestrator)
	s.originalValues["orchestrator"] = orchestrator

	s.orchestratorNote = widget.NewLabel("")
	s.orchestratorNote.Wrapping = fyne.TextWrapWord

	// Kubernetes settings: where Janus provisions the Hermes pod. Image and
	// resources still come from the per-model .janus.config.json.
	kube := s.app.config.Hermes.Kubernetes

	s.kubeconfigEntry = widget.NewEntry()
	s.kubeconfigEntry.SetPlaceHolder("path to kubeconfig (empty = ~/.kube/config)")
	s.kubeconfigEntry.SetText(kube.Kubeconfig)
	s.originalValues["hermes-kube-config"] = kube.Kubeconfig

	s.kubeContextEntry = widget.NewEntry()
	s.kubeContextEntry.SetPlaceHolder("context (empty = current-context)")
	s.kubeContextEntry.SetText(kube.Context)
	s.originalValues["hermes-kube-context"] = kube.Context

	s.kubeNamespaceEntry = widget.NewEntry()
	s.kubeNamespaceEntry.SetPlaceHolder("namespace (required)")
	s.kubeNamespaceEntry.SetText(kube.Namespace)
	s.originalValues["hermes-kube-namespace"] = kube.Namespace

	s.kubePullSecretEntry = widget.NewEntry()
	s.kubePullSecretEntry.SetPlaceHolder("image pull secret (for private registries, optional)")
	s.kubePullSecretEntry.SetText(kube.ImagePullSecret)
	s.originalValues["hermes-kube-pull-secret"] = kube.ImagePullSecret

	// Bootstrap-saga (#192) knobs: the FIFO worker-pool watermark P (how many fit
	// pods run at once — the cost/speed dial) and an optional per-fit deadline.
	s.kubeParallelismEntry = widget.NewEntry()
	s.kubeParallelismEntry.SetPlaceHolder(fmt.Sprintf("fit pods in flight (default %d)", config.DefaultBootstrapParallelism))
	if kube.BootstrapParallelism > 0 {
		s.kubeParallelismEntry.SetText(strconv.Itoa(kube.BootstrapParallelism))
	}
	s.originalValues["hermes-kube-parallelism"] = s.kubeParallelismEntry.Text

	s.kubeFitTimeoutEntry = widget.NewEntry()
	s.kubeFitTimeoutEntry.SetPlaceHolder("per-fit deadline, e.g. 30m (empty = none)")
	s.kubeFitTimeoutEntry.SetText(kube.FitTimeout)
	s.originalValues["hermes-kube-fit-timeout"] = kube.FitTimeout

	s.kubernetesSection = container.NewVBox(
		widget.NewLabel("Kubeconfig:"),
		s.kubeconfigEntry,
		widget.NewLabel("Context:"),
		s.kubeContextEntry,
		widget.NewLabel("Namespace *:"),
		s.kubeNamespaceEntry,
		widget.NewLabel("Image pull secret:"),
		s.kubePullSecretEntry,
		widget.NewLabel("Bootstrap fit parallelism:"),
		s.kubeParallelismEntry,
		widget.NewLabel("Bootstrap fit timeout:"),
		s.kubeFitTimeoutEntry,
	)

	s.dockerSocketEntry = widget.NewEntry()
	s.dockerSocketEntry.SetPlaceHolder("Leave empty for default")
	s.dockerSocketEntry.SetText(s.app.config.Hermes.Container.DockerSocket)
	s.originalValues["hermes-docker-socket"] = s.app.config.Hermes.Container.DockerSocket

	s.startupTimeoutEntry = widget.NewEntry()
	s.startupTimeoutEntry.SetPlaceHolder("30s")
	s.startupTimeoutEntry.SetText(s.app.config.Hermes.Container.StartupTimeout)
	s.originalValues["hermes-startup-timeout"] = s.app.config.Hermes.Container.StartupTimeout

	s.autoCleanupCheck = widget.NewCheck("Auto-cleanup containers after execution", nil)
	s.autoCleanupCheck.SetChecked(s.app.config.Hermes.Container.Cleanup)

	s.hermesImageEntry = widget.NewEntry()
	s.hermesImageEntry.SetPlaceHolder("default container image (per-model overrides win)")
	s.hermesImageEntry.SetText(s.app.config.Hermes.Image)
	s.originalValues["hermes-image"] = s.app.config.Hermes.Image

	// Default PsN orchestration image for the bootstrap saga's SETUP/AGGREGATE
	// pods (no NONMEM license needed). A per-model .janus.config.json psn_image
	// overrides it; the fits run on the per-model execution image above.
	s.hermesPsnImageEntry = widget.NewEntry()
	s.hermesPsnImageEntry.SetPlaceHolder("PsN image for bootstrap (e.g. ghcr.io/pharmalytica/janus-psn:5.7.1)")
	s.hermesPsnImageEntry.SetText(s.app.config.Hermes.PsNImage)
	s.originalValues["hermes-psn-image"] = s.app.config.Hermes.PsNImage

	s.hermesSection = container.NewVBox(
		widget.NewLabelWithStyle("Hermes Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Orchestrator:"),
		s.orchestratorSelect,
		s.orchestratorNote,
		s.kubernetesSection,
		widget.NewLabel("Default image:"),
		s.hermesImageEntry,
		widget.NewLabel("PsN image (bootstrap):"),
		s.hermesPsnImageEntry,
		widget.NewLabel("Docker Socket (optional):"),
		s.dockerSocketEntry,
		widget.NewLabel("Startup Timeout:"),
		s.startupTimeoutEntry,
		s.autoCleanupCheck,
		widget.NewSeparator(),
		s.buildHermesRetainSection(),
		s.buildHermesAdvancedSection(),
		widget.NewSeparator(),
	)

	// Validation configuration
	s.validationIQEntry = widget.NewEntry()
	s.validationIQEntry.SetText(s.app.config.Validation.IQ)
	s.originalValues["validation-iq"] = s.app.config.Validation.IQ

	s.validationOQEntry = widget.NewEntry()
	s.validationOQEntry.SetText(s.app.config.Validation.OQ)
	s.originalValues["validation-oq"] = s.app.config.Validation.OQ

	validationSection := container.NewVBox(
		widget.NewLabelWithStyle("Validation (CFR 21 Part 11)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("IQ Report Path:"),
		s.validationIQEntry,
		widget.NewLabel("OQ Report Path:"),
		s.validationOQEntry,
		widget.NewSeparator(),
	)

	// Runtime qualification — run IQ/OQ on demand, show the last result, and let
	// the user download the latest report without hunting on the filesystem.
	s.runIQBtn = widget.NewButton("Run IQ", s.runIQ)
	s.runOQBtn = widget.NewButton("Run OQ", s.runOQ)
	s.downloadIQBtn = widget.NewButtonWithIcon("Download IQ", theme.DownloadIcon(), func() { s.downloadReport(qa.KindIQ) })
	s.downloadOQBtn = widget.NewButtonWithIcon("Download OQ", theme.DownloadIcon(), func() { s.downloadReport(qa.KindOQ) })
	s.lastIQLabel = widget.NewLabel("Last IQ: " + s.latestLabelText(qa.KindIQ))
	s.lastOQLabel = widget.NewLabel("Last OQ: " + s.latestLabelText(qa.KindOQ))
	s.refreshDownloadButtons() // disabled until a report of that kind exists

	qualificationSection := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Runtime Qualification (IQ/OQ)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Run a qualification on this machine, then download the latest JSON report."),
		container.NewHBox(s.runIQBtn, s.downloadIQBtn, s.lastIQLabel),
		container.NewHBox(s.runOQBtn, s.downloadOQBtn, s.lastOQLabel),
		widget.NewSeparator(),
	)

	// MCP server section
	mcpSection := s.buildMCPSection()

	// Read-only footer
	footerSection := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Version: %s", s.app.config.Version)),
		widget.NewLabel(fmt.Sprintf("User: %s", s.app.config.User)),
	)

	// Set initial visibility, command preview, and orchestrator note from the
	// selected axes.
	s.updateExecutionVisibility()
	s.updateCommandPreview()
	s.updateOrchestratorNote()

	// Wire the axis-change handlers only now that every widget the handlers read
	// (including the Hermes-section fields) has been constructed. Wiring earlier
	// would fire them during the SetSelected calls above and dereference nil.
	s.wireExecutionAxisHandlers()

	// Action buttons
	importBtn := widget.NewButton("Set from Pirana", s.onImportFromPirana)
	cancelBtn := widget.NewButton("Cancel", s.onCancel)
	s.saveBtn = widget.NewButton("Save Changes", s.onSave)
	s.saveBtn.Importance = widget.HighImportance

	// Live validation status, shown above the buttons; Save is disabled while
	// the form is invalid.
	s.validationLabel = widget.NewLabel("")
	s.validationLabel.Wrapping = fyne.TextWrapWord

	// "Set from Pirana" sits on the left; Cancel/Save stay on the right. It only
	// prefills the form — the user still reviews and clicks Save Changes.
	buttonBar := container.NewBorder(
		s.validationLabel, nil, importBtn,
		container.NewHBox(cancelBtn, s.saveBtn),
	)

	s.wireLiveValidation()
	s.refreshValidation()

	// Combine all sections with scroll
	// Config profiles: snapshot / switch / delete named settings sets.
	s.profileSelect = widget.NewSelect(nil, nil)
	s.refreshProfileList()
	profileButtons := container.NewHBox(
		widget.NewButton("Switch", s.onSwitchProfile),
		widget.NewButton("Save as…", s.onSaveAsProfile),
		widget.NewButton("Delete", s.onDeleteProfile),
	)
	profilesSection := container.NewVBox(
		widget.NewLabelWithStyle("Config Profiles", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Profile:"), profileButtons, s.profileSelect),
		widget.NewSeparator(),
	)

	// Group the sections into tabs. The persistent buttonBar (validation + Save)
	// stays pinned at the bottom across all tabs, so the global live-validation
	// model is unchanged.
	schedulerTab := container.NewVBox(
		widget.NewLabelWithStyle("Scheduler", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		s.schedulerSection,
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("General", settingsTabScroll(infoSection, profilesSection, generalSection, footerSection)),
		container.NewTabItem("Execution", settingsTabScroll(executionSection, s.nonmemSection, s.hermesSection, s.buildRunPolicySection())),
		container.NewTabItem("Remote & Schedulers", settingsTabScroll(schedulerTab, s.buildRemoteExtras())),
		container.NewTabItem("Tools & Integrations", settingsTabScroll(s.buildToolsTab())),
		container.NewTabItem("Compliance & Agents", settingsTabScroll(validationSection, qualificationSection, s.buildComplianceExtras(), mcpSection)),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	return container.NewBorder(nil, buttonBar, nil, nil, tabs)
}

// settingsTabScroll wraps a tab's sections in a scrollable, min-sized body.
func settingsTabScroll(sections ...fyne.CanvasObject) fyne.CanvasObject {
	body := container.NewVScroll(container.NewVBox(sections...))
	body.SetMinSize(fyne.NewSize(720, 460))

	return body
}

// buildMCPSection builds the MCP (Model Context Protocol) settings section: the
// enable/host/port/allow-execute fields, a read-only token with Regenerate, a
// "Copy connect command" button, and a runtime Start/Stop control.
func (s *SettingsDialog) buildMCPSection() *fyne.Container {
	mcpCfg := s.app.config.MCP

	s.mcpEnabledCheck = widget.NewCheck("Enable MCP server (lets local agents query the run log)", nil)
	s.mcpEnabledCheck.SetChecked(mcpCfg.Enabled)

	s.mcpHostEntry = widget.NewEntry()
	s.mcpHostEntry.SetPlaceHolder(config.DefaultMCPHost)
	if mcpCfg.Host != "" {
		s.mcpHostEntry.SetText(mcpCfg.Host)
	}

	s.mcpPortEntry = widget.NewEntry()
	s.mcpPortEntry.SetPlaceHolder(strconv.Itoa(config.DefaultMCPPort))
	if mcpCfg.Port != 0 {
		s.mcpPortEntry.SetText(strconv.Itoa(mcpCfg.Port))
	}

	s.mcpAllowExecuteCheck = widget.NewCheck("Allow execute_run (lets agents launch runs)", nil)
	s.mcpAllowExecuteCheck.SetChecked(mcpCfg.AllowExecute)

	s.mcpTokenEntry = widget.NewEntry()
	s.mcpTokenEntry.SetText(mcpCfg.AuthToken)
	s.mcpTokenEntry.SetPlaceHolder("(generated on first start)")
	s.mcpTokenEntry.Disable() // read-only; managed via Regenerate

	regenerateBtn := widget.NewButton("Regenerate", s.onMCPRegenerate)
	copyBtn := widget.NewButtonWithIcon("Copy connect command", theme.ContentCopyIcon(), s.onMCPCopyConnect)

	s.mcpStatusLabel = widget.NewLabel("")
	s.mcpStartStopBtn = widget.NewButton("Start", s.onMCPStartStop)
	s.refreshMCPStatus()

	return container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("MCP Server (Agent Access)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Expose the run log to local agents (e.g. Claude Code) over loopback HTTP."),
		s.mcpEnabledCheck,
		widget.NewLabel("Host:"),
		s.mcpHostEntry,
		widget.NewLabel("Port:"),
		s.mcpPortEntry,
		s.mcpAllowExecuteCheck,
		widget.NewLabel("Auth token:"),
		s.mcpTokenEntry,
		container.NewHBox(regenerateBtn, copyBtn),
		container.NewHBox(s.mcpStartStopBtn, s.mcpStatusLabel),
		widget.NewSeparator(),
	)
}

// mcpPort returns the port from the form, defaulting to config.DefaultMCPPort
// when the field is empty. A non-numeric value is an error.
func (s *SettingsDialog) mcpPort() (int, error) {
	text := strings.TrimSpace(s.mcpPortEntry.Text)
	if text == "" {
		return config.DefaultMCPPort, nil
	}

	port, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("MCP port must be a number: %q", text)
	}

	return port, nil
}

// refreshMCPStatus updates the Start/Stop button label and the status text to
// reflect whether the server is currently running.
func (s *SettingsDialog) refreshMCPStatus() {
	if s.mcpStartStopBtn == nil || s.mcpStatusLabel == nil {
		return
	}

	if s.app.MCPServerRunning() {
		s.mcpStartStopBtn.SetText("Stop")
		s.mcpStatusLabel.SetText("Status: running")
	} else {
		s.mcpStartStopBtn.SetText("Start")
		s.mcpStatusLabel.SetText("Status: stopped")
	}
}

// applyMCPFormToConfig copies the MCP form fields into the live app config and
// validates them, so a runtime Start uses the current form values.
func (s *SettingsDialog) applyMCPFormToConfig() error {
	port := config.DefaultMCPPort
	if text := strings.TrimSpace(s.mcpPortEntry.Text); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil {
			return fmt.Errorf("MCP port must be a number: %q", text)
		}

		port = parsed
	}

	s.app.config.MCP.Enabled = s.mcpEnabledCheck.Checked
	s.app.config.MCP.Host = strings.TrimSpace(s.mcpHostEntry.Text)
	s.app.config.MCP.Port = port
	s.app.config.MCP.AllowExecute = s.mcpAllowExecuteCheck.Checked

	return config.ValidateMCPConfig(&s.app.config.MCP)
}

// onMCPStartStop starts or stops the embedded server at runtime.
func (s *SettingsDialog) onMCPStartStop() {
	if s.app.MCPServerRunning() {
		if err := s.app.StopMCPServer(); err != nil {
			dialog.ShowError(fmt.Errorf("failed to stop MCP server: %w", err), s.window)
		}

		s.refreshMCPStatus()

		return
	}

	// Starting: reflect the current form into config first.
	if err := s.applyMCPFormToConfig(); err != nil {
		dialog.ShowError(err, s.window)

		return
	}

	if !s.app.config.MCP.Enabled {
		dialog.ShowInformation("MCP disabled", "Enable the MCP server before starting it.", s.window)

		return
	}

	if err := s.app.StartMCPServer(); err != nil {
		dialog.ShowError(fmt.Errorf("failed to start MCP server: %w", err), s.window)

		return
	}

	// A token may have been generated on start; reflect it in the field.
	s.mcpHostEntry.SetText(s.app.config.MCP.Host)
	s.mcpPortEntry.SetText(strconv.Itoa(s.app.config.MCP.Port))
	s.mcpTokenEntry.SetText(s.app.config.MCP.AuthToken)
	s.refreshMCPStatus()
}

// onMCPRegenerate generates and persists a fresh bearer token. If the server is
// running it is restarted so the new token takes effect.
func (s *SettingsDialog) onMCPRegenerate() {
	token, err := s.app.RegenerateMCPToken()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to regenerate token: %w", err), s.window)

		return
	}

	s.mcpTokenEntry.SetText(token)

	if s.app.MCPServerRunning() {
		if stopErr := s.app.StopMCPServer(); stopErr != nil {
			dialog.ShowError(fmt.Errorf("failed to restart MCP server: %w", stopErr), s.window)

			return
		}

		if startErr := s.app.StartMCPServer(); startErr != nil {
			dialog.ShowError(fmt.Errorf("failed to restart MCP server: %w", startErr), s.window)
		}
	}

	s.refreshMCPStatus()
	dialog.ShowInformation("Token regenerated", "Re-run the connect command to re-pair your agent.", s.window)
}

// onMCPCopyConnect copies the full `claude mcp add ...` command to the clipboard.
func (s *SettingsDialog) onMCPCopyConnect() {
	token := strings.TrimSpace(s.mcpTokenEntry.Text)
	if token == "" {
		dialog.ShowInformation("No token yet",
			"Start the MCP server once to generate a token, then copy the command.", s.window)

		return
	}

	host := strings.TrimSpace(s.mcpHostEntry.Text)
	if host == "" {
		host = config.DefaultMCPHost
	}

	port := strings.TrimSpace(s.mcpPortEntry.Text)
	if port == "" {
		port = strconv.Itoa(config.DefaultMCPPort)
	}

	command := fmt.Sprintf(
		"claude mcp add --transport http janus http://%s:%s/mcp --header \"Authorization: Bearer %s\"",
		host, port, token)

	s.app.GetFyneApp().Clipboard().SetContent(command)
	dialog.ShowInformation("Copied", "Connect command copied to clipboard:\n\n"+command, s.window)
}

// wireExecutionAxisHandlers attaches the change handlers for the Engine,
// Destination, and Orchestrator selects. It must be called only after the whole
// form is built, since onExecutionAxisChanged reads widgets from every section.
func (s *SettingsDialog) wireExecutionAxisHandlers() {
	handler := func(string) { s.onExecutionAxisChanged() }
	s.engineSelect.OnChanged = handler
	s.destinationSelect.OnChanged = handler
	s.orchestratorSelect.OnChanged = handler
}

// onExecutionAxisChanged reacts to a change in the Engine, Destination, or
// Orchestrator selection: it re-derives section visibility, the command
// preview, the Kubernetes note, and validity.
func (s *SettingsDialog) onExecutionAxisChanged() {
	s.updateExecutionVisibility()
	s.updateCommandPreview()
	s.updateOrchestratorNote()
	s.refreshValidation()

	// Only refresh if window content exists (may not during initial form building)
	if s.window != nil && s.window.Content() != nil {
		s.window.Content().Refresh()
	}
}

// wireLiveValidation attaches change handlers to the inputs that affect
// validity, so the form re-validates as the user edits.
func (s *SettingsDialog) wireLiveValidation() {
	refresh := func(string) { s.refreshValidation() }

	// These entries also feed the command preview, so re-render it on change.
	previewRefresh := func(string) {
		s.refreshValidation()
		s.updateCommandPreview()
	}

	for _, e := range []*widget.Entry{
		s.organizationEntry, s.defaultDirEntry,
		s.startupTimeoutEntry, s.mcpHostEntry, s.mcpPortEntry,
		s.kubeNamespaceEntry,
	} {
		if e != nil {
			e.OnChanged = refresh
		}
	}

	for _, e := range []*widget.Entry{
		s.nonmemPathEntry, s.nonmemBinaryEntry, s.hermesImageEntry,
	} {
		if e != nil {
			e.OnChanged = previewRefresh
		}
	}

	if s.schedulerSelect != nil {
		s.schedulerSelect.OnChanged = func(string) {
			s.refreshValidation()
			s.updateCommandPreview()
		}
	}

	if s.mcpEnabledCheck != nil {
		s.mcpEnabledCheck.OnChanged = func(bool) { s.refreshValidation() }
	}
}

// refreshValidation re-runs validation, showing the first error inline and
// disabling Save while the form is invalid.
func (s *SettingsDialog) refreshValidation() {
	if s.saveBtn == nil || s.validationLabel == nil {
		return
	}

	if err := s.validate(); err != nil {
		s.validationLabel.SetText("⚠ " + err.Error())
		s.validationLabel.Importance = widget.DangerImportance
		s.validationLabel.Refresh()
		s.saveBtn.Disable()

		return
	}

	s.validationLabel.SetText("")
	s.validationLabel.Importance = widget.MediumImportance
	s.validationLabel.Refresh()
	s.saveBtn.Enable()
}

// updateExecutionVisibility shows the configuration sections relevant to the
// selected Destination: the engine runs on the host for Here/Scheduler (NONMEM
// paths), inside a container for Hermes, and the scheduler picker is only
// relevant for the Scheduler destination.
func (s *SettingsDialog) updateExecutionVisibility() {
	// Sections may not be created yet during initial form building
	if s.nonmemSection == nil || s.hermesSection == nil || s.schedulerSection == nil {
		return
	}

	switch s.destinationSelect.Selected {
	case config.DestinationHere:
		s.nonmemSection.Show()
		s.hermesSection.Hide()
		s.schedulerSection.Hide()
	case config.DestinationScheduler:
		s.nonmemSection.Show()
		s.hermesSection.Hide()
		s.schedulerSection.Show()
	case config.DestinationHermes:
		s.nonmemSection.Hide() // engine runs inside the Hermes container
		s.hermesSection.Show()
		s.schedulerSection.Hide()
	}
}

// updateCommandPreview re-renders the read-only command preview from the current
// axis selections and engine fields.
func (s *SettingsDialog) updateCommandPreview() {
	if s.commandPreview == nil {
		return
	}

	s.commandPreview.SetText(s.executionCommandPreview())
}

// executionCommandPreview assembles a representative command string for the
// selected engine and destination, mirroring the GUI's run-time command
// builders. It is illustrative (a sample model name) rather than tied to a
// loaded model.
func (s *SettingsDialog) executionCommandPreview() string {
	const model = "run1.mod"

	if s.destinationSelect.Selected == config.DestinationHermes {
		image := strings.TrimSpace(s.hermesImageEntry.Text)
		if image == "" {
			image = "<default image>"
		}

		return fmt.Sprintf("hermes execute --image %s %s   [orchestrator: %s]",
			image, model, s.orchestratorSelect.Selected)
	}

	var base string

	switch s.engineSelect.Selected {
	case config.EnginePSN:
		base = "execute " + model
	case config.EngineBBI:
		base = "bbi nonmem run local " + model
	default: // NONMEM
		binary := strings.TrimSpace(s.nonmemBinaryEntry.Text)
		if binary == "" {
			binary = "nmfe75"
		}

		if path := strings.TrimSpace(s.nonmemPathEntry.Text); path != "" {
			binary = filepath.Join(path, binary)
		}

		base = fmt.Sprintf("%s %s", binary, model)
	}

	if s.destinationSelect.Selected == config.DestinationScheduler {
		sched := s.schedulerSelect.Selected
		if sched == "" {
			sched = "scheduler"
		}

		return base + "   [submitted via " + sched + "]"
	}

	return base + "   [local]"
}

// updateOrchestratorNote reveals the Kubernetes settings (and a short hint) when
// the Kubernetes orchestrator is selected, and hides them otherwise.
func (s *SettingsDialog) updateOrchestratorNote() {
	if s.orchestratorNote == nil || s.kubernetesSection == nil {
		return
	}

	kubernetes := s.orchestratorSelect.Selected == config.OrchestratorKubernetes

	if kubernetes {
		s.orchestratorNote.SetText("Janus provisions the Hermes pod in the namespace below, port-forwards to it, then runs and tears it down.")
		s.orchestratorNote.Importance = widget.MediumImportance
		s.kubernetesSection.Show()
	} else {
		s.orchestratorNote.SetText("")
		s.orchestratorNote.Importance = widget.MediumImportance
		s.kubernetesSection.Hide()
	}

	s.orchestratorNote.Refresh()
}

// qaStore returns a qualification report store rooted at the per-user QA dir.
func (s *SettingsDialog) qaStore() *qa.Store {
	return qa.NewStore(qarun.DefaultQABaseDir())
}

// latestLabelText renders the "<timestamp> — STATUS" suffix for a last-run
// label, or "never run" when no report of that kind exists.
func (s *SettingsDialog) latestLabelText(kind qa.Kind) string {
	_, raw, err := s.qaStore().Latest(kind)
	if errors.Is(err, os.ErrNotExist) {
		return "never run"
	}

	if err != nil {
		return "unknown"
	}

	var report struct {
		Status      string    `json:"status"`
		CompletedAt time.Time `json:"completed_at"`
	}

	if json.Unmarshal(raw, &report) != nil {
		return "unknown"
	}

	return fmt.Sprintf("%s — %s", report.CompletedAt.Local().Format("2006-01-02 15:04"), strings.ToUpper(report.Status))
}

// refreshDownloadButtons enables each download button only when a report of that
// kind exists on disk.
func (s *SettingsDialog) refreshDownloadButtons() {
	s.setDownloadEnabled(s.downloadIQBtn, qa.KindIQ)
	s.setDownloadEnabled(s.downloadOQBtn, qa.KindOQ)
}

func (s *SettingsDialog) setDownloadEnabled(btn *widget.Button, kind qa.Kind) {
	if btn == nil {
		return
	}

	if _, _, err := s.qaStore().Latest(kind); err == nil {
		btn.Enable()
	} else {
		btn.Disable()
	}
}

// downloadReport saves the latest report of the given kind to a user-chosen
// location via a file-save dialog, so results don't have to be hunted for under
// ~/.config/janus/qa/.
func (s *SettingsDialog) downloadReport(kind qa.Kind) {
	_, raw, err := s.qaStore().Latest(kind)
	if errors.Is(err, os.ErrNotExist) {
		s.app.showToast("No report", "Run "+strings.ToUpper(string(kind))+" first.", 3*time.Second)

		return
	}

	if err != nil {
		s.app.sendError(fmt.Errorf("loading %s report: %w", strings.ToUpper(string(kind)), err))

		return
	}

	save := dialog.NewFileSave(func(w fyne.URIWriteCloser, derr error) {
		if derr != nil {
			s.app.sendError(derr)

			return
		}

		if w == nil { // user cancelled
			return
		}

		defer w.Close()

		if _, werr := w.Write(raw); werr != nil {
			s.app.sendError(fmt.Errorf("writing %s report: %w", strings.ToUpper(string(kind)), werr))

			return
		}

		s.app.showSuccessToast(strings.ToUpper(string(kind)) + " report saved")
	}, s.window)

	save.SetFileName(fmt.Sprintf("janus-%s-report.json", kind))
	save.Show()
}

// runIQ runs the Installation Qualification in the background and refreshes the
// last-run label on completion.
func (s *SettingsDialog) runIQ() {
	s.runIQBtn.Disable()
	s.app.showToast("IQ", "Running installation qualification…", 3*time.Second)

	go func() {
		store := s.qaStore()

		_, runDir, err := store.NewRunDir()
		if err != nil {
			s.finishQA(s.runIQBtn, s.lastIQLabel, qa.KindIQ, err)

			return
		}

		res := qa.RunIQ(s.app.config, runDir, qa.Options{ExecutorProbe: qarun.NewExecutorProbe()})
		s.finishQA(s.runIQBtn, s.lastIQLabel, qa.KindIQ, store.Save(runDir, qa.KindIQ, res))
	}()
}

// runOQ runs the Operational Qualification in the background and refreshes the
// last-run label on completion. The executor is built via the same factory used
// for real runs.
func (s *SettingsDialog) runOQ() {
	s.runOQBtn.Disable()
	s.app.showToast("OQ", "Running operational qualification…", 3*time.Second)

	go func() {
		store := s.qaStore()

		_, runDir, err := store.NewRunDir()
		if err != nil {
			s.finishQA(s.runOQBtn, s.lastOQLabel, qa.KindOQ, err)

			return
		}

		ctx, cancel := context.WithTimeout(s.app.errorCtx, 30*time.Minute)
		defer cancel()

		provider := qarun.BuildExecutorProvider(s.app.config)

		res, runErr := qa.RunOQ(ctx, s.app.config, provider, summary.NewNONMEMSummarizer(), runDir,
			qa.Options{ExecutorProbe: qarun.NewExecutorProbe()})

		if res != nil {
			if saveErr := store.Save(runDir, qa.KindOQ, res); saveErr != nil && runErr == nil {
				runErr = saveErr
			}
		}

		s.finishQA(s.runOQBtn, s.lastOQLabel, qa.KindOQ, runErr)
	}()
}

// finishQA re-enables the button, refreshes the last-run label on the UI thread,
// and reports success or error. Safe to call from a background goroutine.
func (s *SettingsDialog) finishQA(btn *widget.Button, label *widget.Label, kind qa.Kind, err error) {
	labelText := "Last " + strings.ToUpper(string(kind)) + ": " + s.latestLabelText(kind)

	fyne.Do(func() {
		btn.Enable()
		label.SetText(labelText)
		s.refreshDownloadButtons() // a fresh report may now be downloadable
	})

	if err != nil {
		s.app.sendError(fmt.Errorf("%s qualification: %w", strings.ToUpper(string(kind)), err))

		return
	}

	s.app.showSuccessToast(strings.ToUpper(string(kind)) + " qualification complete")
}

func (s *SettingsDialog) hasChanges() bool {
	if s.organizationEntry.Text != s.originalValues["organization"] {
		return true
	}
	if s.defaultDirEntry.Text != s.originalValues["default-directory"] {
		return true
	}
	if s.runPrefixEntry.Text != s.originalValues["run-prefix"] {
		return true
	}
	if s.altDataDirEntry.Text != s.originalValues["alt-data-directory"] {
		return true
	}
	if s.closeConsoleCheck.Checked != (s.originalValues["close-console-after-run"] == "true") {
		return true
	}
	if s.engineSelect.Selected != s.originalValues["engine"] {
		return true
	}
	if s.destinationSelect.Selected != s.originalValues["destination"] {
		return true
	}
	if s.orchestratorSelect.Selected != s.originalValues["orchestrator"] {
		return true
	}
	if s.schedulerSelect.Selected != s.originalValues["scheduler"] {
		return true
	}
	if s.nonmemPathEntry.Text != s.originalValues["nonmem-path"] {
		return true
	}
	if s.nonmemBinaryEntry.Text != s.originalValues["nonmem-binary"] {
		return true
	}
	if serializeInstalls(s.installEditor.installations()) != s.originalValues["installations"] {
		return true
	}
	if s.dockerSocketEntry.Text != s.originalValues["hermes-docker-socket"] {
		return true
	}
	if s.startupTimeoutEntry.Text != s.originalValues["hermes-startup-timeout"] {
		return true
	}
	if s.validationIQEntry.Text != s.originalValues["validation-iq"] {
		return true
	}
	if s.validationOQEntry.Text != s.originalValues["validation-oq"] {
		return true
	}

	return s.hasAdvancedChanges()
}

func (s *SettingsDialog) onCancel() {
	if s.hasChanges() {
		// Show confirmation dialog
		dialog.ShowConfirm(
			"Unsaved Changes",
			"You have unsaved changes. Are you sure you want to cancel?",
			func(confirmed bool) {
				if confirmed {
					s.window.Close()
				}
			},
			s.window,
		)
	} else {
		s.window.Close()
	}
}

// onImportFromPirana detects an existing Pirana configuration and prefills the
// form fields with whatever maps onto Janus settings. It does not save; the user
// reviews the populated form and clicks Save Changes to persist via the normal
// onSave path.
func (s *SettingsDialog) onImportFromPirana() {
	importFromPirana(s.window, func(p *pirana.Settings) {
		if p.NonmemPath != "" {
			s.nonmemPathEntry.SetText(p.NonmemPath)
		}
		if p.NonmemBinary != "" {
			s.nonmemBinaryEntry.SetText(p.NonmemBinary)
		}
		if p.DefaultDir != "" {
			s.defaultDirEntry.SetText(p.DefaultDir)
		}
		if p.Scheduler != "" {
			s.schedulerSelect.SetSelected(p.Scheduler)
		}
		if p.Researcher != "" && s.organizationEntry.Text == "" {
			s.organizationEntry.SetText(p.Researcher)
		}
		if p.RunPrefix != "" {
			s.runPrefixEntry.SetText(p.RunPrefix)
		}
		if p.AltDataDir != "" {
			s.altDataDirEntry.SetText(p.AltDataDir)
		}
		if p.CloseConsole != nil {
			s.closeConsoleCheck.SetChecked(*p.CloseConsole)
		}
	})
}

func (s *SettingsDialog) onSave() {
	if err := s.validate(); err != nil {
		dialog.ShowError(err, s.window)

		return
	}

	if err := s.persistForm(); err != nil {
		dialog.ShowError(err, s.window)

		return
	}

	if err := s.reloadAppConfig(); err != nil {
		dialog.ShowError(fmt.Errorf("%w (config was saved, please restart Janus)", err), s.window)

		return
	}

	dialog.ShowInformation("Settings Saved", "Configuration has been saved successfully.", s.window)
	s.window.Close()
}

// persistForm writes the current form values to the active config file.
func (s *SettingsDialog) persistForm() error {
	// Update Viper configuration
	viper.Set("organization", s.organizationEntry.Text)
	viper.Set("default-directory", s.defaultDirEntry.Text)
	viper.Set("run-prefix", s.runPrefixEntry.Text)
	viper.Set("alt-data-directory", s.altDataDirEntry.Text)
	viper.Set("close-console-after-run", s.closeConsoleCheck.Checked)

	// Persist the orthogonal axes, plus the derived legacy ExecutionMode the
	// executor factory still dispatches on (kept in sync by normalizeExecutionAxes
	// on reload). Only the Scheduler destination carries a real scheduler.
	engine := s.engineSelect.Selected
	destination := s.destinationSelect.Selected
	viper.Set("engine", engine)
	viper.Set("destination", destination)
	viper.Set("orchestrator", s.orchestratorSelect.Selected)
	viper.Set("execution-mode", config.DeriveExecutionMode(engine, destination))

	if destination == config.DestinationScheduler {
		viper.Set("scheduler", s.schedulerSelect.Selected)
	} else {
		viper.Set("scheduler", "LOCAL")
	}

	// Installations override the single nonmem-path/binary when present: persist
	// the list and keep the legacy fields in sync with the default install so the
	// rest of the app (which reads nonmem-path) stays correct.
	installs := s.installEditor.installations()
	viper.Set("installations", installs)

	if len(installs) > 0 {
		def := config.Input{Installations: installs}.DefaultNonmemInstallation()
		viper.Set("nonmem-path", def.Path)
		viper.Set("nonmem-binary", def.Binary)
	} else {
		viper.Set("nonmem-path", s.nonmemPathEntry.Text)
		viper.Set("nonmem-binary", s.nonmemBinaryEntry.Text)
	}
	viper.Set("hermes.container.docker_socket", s.dockerSocketEntry.Text)
	viper.Set("hermes.container.startup_timeout", s.startupTimeoutEntry.Text)
	viper.Set("hermes.container.cleanup", s.autoCleanupCheck.Checked)
	viper.Set("validation.iq", s.validationIQEntry.Text)
	viper.Set("validation.oq", s.validationOQEntry.Text)

	// MCP server settings
	viper.Set("mcp.enabled", s.mcpEnabledCheck.Checked)
	viper.Set("mcp.host", strings.TrimSpace(s.mcpHostEntry.Text))
	if port, err := s.mcpPort(); err == nil {
		viper.Set("mcp.port", port)
	}
	viper.Set("mcp.allow_execute", s.mcpAllowExecuteCheck.Checked)
	viper.Set("mcp.auth_token", s.mcpTokenEntry.Text)

	// Newly-surfaced settings (run policy, integrations, remote, etc.).
	s.persistAdvanced()

	// Write config file
	if err := viper.WriteConfig(); err != nil {
		// If WriteConfig fails (no config file exists), try SafeWriteConfig
		if err := viper.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
	}

	return nil
}

// reloadAppConfig re-reads viper into the application config and refreshes
// dependent UI.
func (s *SettingsDialog) reloadAppConfig() error {
	input, err := config.UnmarshalInputFromViper()
	if err != nil {
		return fmt.Errorf("failed to reload configuration: %w", err)
	}

	newConfig, err := config.NewConfig(input)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	s.app.config = newConfig
	s.app.refreshVersionSelect()
	s.app.refreshTargetOptions()
	s.app.refreshPSNPresets()

	return nil
}

// configPath returns the active config file path.
func (s *SettingsDialog) configPath() string {
	if p := viper.ConfigFileUsed(); p != "" {
		return p
	}

	return config.DefaultConfigPath()
}

// refreshProfileList repopulates the profile picker from disk.
func (s *SettingsDialog) refreshProfileList() {
	if s.profileSelect == nil {
		return
	}

	names, _ := config.ListProfiles(s.configPath()) // empty on error; nothing to show

	s.profileSelect.Options = names
	s.profileSelect.Refresh()
}

// onSaveAsProfile snapshots the current (validated, persisted) settings as a
// named profile.
func (s *SettingsDialog) onSaveAsProfile() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("profile name")

	dialog.ShowForm("Save profile", "Save", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Name", nameEntry)},
		func(ok bool) {
			if !ok {
				return
			}

			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				return
			}

			if err := s.validate(); err != nil {
				dialog.ShowError(err, s.window)

				return
			}

			if err := s.persistForm(); err != nil {
				dialog.ShowError(err, s.window)

				return
			}

			if err := config.SaveProfile(s.configPath(), name); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save profile: %w", err), s.window)

				return
			}

			s.refreshProfileList()
			s.profileSelect.SetSelected(name)
			dialog.ShowInformation("Profile saved", "Saved profile: "+name, s.window)
		}, s.window)
}

// onSwitchProfile makes the selected profile the active config and reloads.
func (s *SettingsDialog) onSwitchProfile() {
	name := s.profileSelect.Selected
	if name == "" {
		return
	}

	dialog.ShowConfirm("Switch profile",
		fmt.Sprintf("Switch to profile %q? Unsaved changes in this dialog will be lost.", name),
		func(ok bool) {
			if !ok {
				return
			}

			if err := config.SwitchProfile(s.configPath(), name); err != nil {
				dialog.ShowError(err, s.window)

				return
			}

			viper.SetConfigFile(s.configPath())
			if err := viper.ReadInConfig(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to read profile config: %w", err), s.window)

				return
			}

			if err := s.reloadAppConfig(); err != nil {
				dialog.ShowError(err, s.window)

				return
			}

			// Rebuild the form so it reflects the newly active profile.
			s.window.SetContent(s.buildForm())
			dialog.ShowInformation("Profile active", "Switched to profile: "+name, s.window)
		}, s.window)
}

// onDeleteProfile removes the selected profile.
func (s *SettingsDialog) onDeleteProfile() {
	name := s.profileSelect.Selected
	if name == "" {
		return
	}

	dialog.ShowConfirm("Delete profile", fmt.Sprintf("Delete profile %q?", name), func(ok bool) {
		if !ok {
			return
		}

		if err := config.DeleteProfile(s.configPath(), name); err != nil {
			dialog.ShowError(err, s.window)

			return
		}

		s.refreshProfileList()
		s.profileSelect.ClearSelected()
	}, s.window)
}

func (s *SettingsDialog) validate() error {
	// FR-005: Invalid configuration values shall be rejected with clear error messages

	// Organization must not be empty
	if s.organizationEntry.Text == "" {
		return fmt.Errorf("organization cannot be empty")
	}

	// Default directory must not be empty
	if s.defaultDirEntry.Text == "" {
		return fmt.Errorf("default directory cannot be empty")
	}

	// Engine and Destination must both be selected.
	if s.engineSelect.Selected == "" {
		return fmt.Errorf("engine must be selected")
	}

	if s.destinationSelect.Selected == "" {
		return fmt.Errorf("destination must be selected")
	}

	engine := s.engineSelect.Selected
	destination := s.destinationSelect.Selected

	// The Scheduler destination requires a scheduler selection.
	if destination == config.DestinationScheduler && s.schedulerSelect.Selected == "" {
		return fmt.Errorf("scheduler must be selected for the Scheduler destination")
	}

	// Host-run engines (Here/Scheduler) need a NONMEM path and binary; for Hermes
	// the engine runs inside the container.
	if destination != config.DestinationHermes {
		if s.nonmemPathEntry.Text == "" {
			return fmt.Errorf("NONMEM path is required for the %s engine", engine)
		}
		if s.nonmemBinaryEntry.Text == "" {
			return fmt.Errorf("NONMEM binary is required for the %s engine", engine)
		}
	}

	// Hermes-specific validation.
	if destination == config.DestinationHermes {
		// The Kubernetes orchestrator needs a target namespace.
		kubernetes := s.orchestratorSelect.Selected == config.OrchestratorKubernetes
		if kubernetes && strings.TrimSpace(s.kubeNamespaceEntry.Text) == "" {
			return fmt.Errorf("kubernetes orchestrator requires a namespace")
		}

		// Startup timeout must be valid duration if provided.
		if s.startupTimeoutEntry.Text != "" {
			if _, err := time.ParseDuration(s.startupTimeoutEntry.Text); err != nil {
				return fmt.Errorf("startup timeout must be a valid duration (e.g., '30s', '1m'): %w", err)
			}
		}
	}

	// MCP validation: when enabled, host must be loopback and port in range. We
	// reuse the config validator against a copy of the form values.
	if s.mcpEnabledCheck != nil && s.mcpEnabledCheck.Checked {
		port, err := s.mcpPort()
		if err != nil {
			return err
		}

		probe := config.MCPConfig{
			Enabled: true,
			Host:    strings.TrimSpace(s.mcpHostEntry.Text),
			Port:    port,
		}
		if err := config.ValidateMCPConfig(&probe); err != nil {
			return err
		}
	}

	// Validation paths are optional - they will be populated by bootstrap/verification commands
	// No validation needed here

	// Enhanced validation for default directory
	defaultDir := s.defaultDirEntry.Text
	if defaultDir != "" {
		// Expand ~ to home directory
		if strings.HasPrefix(defaultDir, "~") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				defaultDir = filepath.Join(homeDir, defaultDir[1:])
			}
		}

		// Check if parent directory exists (don't require target to exist yet)
		parentDir := filepath.Dir(defaultDir)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("parent directory does not exist: %s", parentDir)
		}
	}

	return s.validateAdvanced()
}
