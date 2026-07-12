package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pharmalytica/janus/internal/config"
)

// SetupWizard handles first-time configuration setup.
type SetupWizard struct {
	app        fyne.App
	window     fyne.Window
	onComplete func(*config.Input)
}

// NewSetupWizard creates a new setup wizard.
func NewSetupWizard(app fyne.App, onComplete func(*config.Input)) *SetupWizard {
	return &SetupWizard{
		app:        app,
		window:     app.NewWindow("Janus Setup"),
		onComplete: onComplete,
	}
}

// Show displays the setup wizard and ensures it's in front.
func (sw *SetupWizard) Show() {
	sw.window.Resize(fyne.NewSize(500, 400))
	sw.window.CenterOnScreen()
	sw.window.SetFixedSize(true)

	content := sw.buildWizardContent()
	sw.window.SetContent(content)

	// Show the wizard window and bring it to front
	sw.window.Show()
	sw.window.RequestFocus()
}

func (sw *SetupWizard) buildWizardContent() fyne.CanvasObject {
	// Welcome message
	welcome := widget.NewRichTextFromMarkdown(`# Welcome to Janus!

This is your first time running Janus. Let's configure some basic settings to get you started.`)

	// Form fields
	orgEntry := widget.NewEntry()
	orgEntry.SetPlaceHolder("e.g., Acme Pharmaceuticals")
	orgEntry.SetText("BigPharma LLC") // Reasonable default

	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder("e.g., ~/models")
	dirEntry.SetText("~/models") // Reasonable default

	nonmemEntry := widget.NewEntry()
	nonmemEntry.SetPlaceHolder("e.g., /opt/NONMEM/nm76/run")
	nonmemEntry.SetText("/opt/NONMEM/nm76/run") // Common default

	binaryEntry := widget.NewEntry()
	binaryEntry.SetPlaceHolder("e.g., nmfe76")
	binaryEntry.SetText("nmfe76") // Common default

	schedulerSelect := widget.NewSelect([]string{"SLURM", "SGE", "TORQUE"}, nil)
	schedulerSelect.SetSelected("SLURM") // Default selection

	executionModeSelect := widget.NewSelect(config.GetValidExecutionModes(), nil)
	executionModeSelect.SetSelected(config.ExecutionModeNONMEM) // Default selection

	// SLURM-specific configuration fields
	slurmModeSelect := widget.NewSelect(config.GetValidSLURMModes(), nil)
	slurmModeSelect.SetSelected(config.SLURMModeCLI) // Default to CLI mode

	slurmSocketEntry := widget.NewEntry()
	slurmSocketEntry.SetPlaceHolder("e.g., /var/spool/slurm/restd/rest")
	slurmSocketEntry.SetText("/var/run/slurm/slurmrestd.sock") // Common default

	slurmAPIVersionEntry := widget.NewEntry()
	slurmAPIVersionEntry.SetPlaceHolder("e.g., v0.0.40")
	slurmAPIVersionEntry.SetText("v0.0.40") // Recent default

	slurmTimeoutEntry := widget.NewEntry()
	slurmTimeoutEntry.SetPlaceHolder("e.g., 30s")
	slurmTimeoutEntry.SetText("30s") // Reasonable default

	// Create SLURM configuration container that shows/hides based on scheduler selection
	slurmContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("SLURM Configuration:"),
		widget.NewLabel("SLURM Mode:"),
		slurmModeSelect,
	)

	// REST-specific fields container
	restContainer := container.NewVBox(
		widget.NewLabel("REST Socket Path:"),
		slurmSocketEntry,
		widget.NewLabel("API Version:"),
		slurmAPIVersionEntry,
		widget.NewLabel("Timeout:"),
		slurmTimeoutEntry,
	)

	// Function to update SLURM fields visibility
	updateSlurmFields := func() {
		if schedulerSelect.Selected == "SLURM" {
			slurmContainer.Show()
			if slurmModeSelect.Selected == config.SLURMModeREST {
				if len(slurmContainer.Objects) < 5 { // Only add if not already present
					slurmContainer.Add(restContainer)
				}
			} else {
				if len(slurmContainer.Objects) > 4 { // Remove REST fields if present
					slurmContainer.Objects = slurmContainer.Objects[:4]
					slurmContainer.Refresh()
				}
			}
		} else {
			slurmContainer.Hide()
		}
	}

	// Set up event handlers
	schedulerSelect.OnChanged = func(_ string) {
		updateSlurmFields()
	}
	slurmModeSelect.OnChanged = func(_ string) {
		updateSlurmFields()
	}

	// Initial state
	updateSlurmFields()

	// Form layout
	form := container.NewVBox(
		widget.NewLabel("Organization:"),
		orgEntry,
		widget.NewSeparator(),

		widget.NewLabel("Default Models Directory:"),
		dirEntry,
		widget.NewSeparator(),

		widget.NewLabel("NONMEM Installation Path:"),
		nonmemEntry,
		widget.NewSeparator(),

		widget.NewLabel("NONMEM Binary Name:"),
		binaryEntry,
		widget.NewSeparator(),

		widget.NewLabel("Default Scheduler:"),
		schedulerSelect,
		slurmContainer, // SLURM-specific configuration

		widget.NewSeparator(),
		widget.NewLabel("Execution Mode:"),
		executionModeSelect,
	)

	// Buttons
	cancelBtn := widget.NewButton("Cancel", func() {
		sw.app.Quit()
	})

	saveBtn := widget.NewButton("Save & Continue", func() {
		sw.saveConfiguration(
			orgEntry.Text, dirEntry.Text, nonmemEntry.Text, binaryEntry.Text,
			schedulerSelect.Selected, executionModeSelect.Selected,
			slurmModeSelect.Selected, slurmSocketEntry.Text, slurmAPIVersionEntry.Text, slurmTimeoutEntry.Text,
		)
	})
	saveBtn.Importance = widget.HighImportance

	buttons := container.NewHBox(
		cancelBtn,
		widget.NewLabel(""), // Spacer
		saveBtn,
	)

	// Main layout with scrollable content
	return container.NewBorder(nil, buttons, nil, nil,
		container.NewScroll(container.NewVBox(welcome, widget.NewSeparator(), form)))
}

func (sw *SetupWizard) saveConfiguration(org, dir, nonmemPath, nonmemBinary, scheduler, executionMode, slurmMode, slurmSocket, slurmAPIVersion, slurmTimeout string) {
	// Validate basic inputs
	if org == "" || dir == "" || nonmemPath == "" || nonmemBinary == "" || scheduler == "" || executionMode == "" {
		dialog.ShowError(fmt.Errorf("all fields are required"), sw.window)

		return
	}

	// Validate SLURM-specific inputs if SLURM is selected
	if scheduler == "SLURM" {
		if slurmMode == "" {
			dialog.ShowError(fmt.Errorf("SLURM mode is required when SLURM is selected"), sw.window)

			return
		}
		if slurmMode == config.SLURMModeREST {
			if slurmSocket == "" || slurmAPIVersion == "" {
				dialog.ShowError(fmt.Errorf("REST socket path and API version are required for SLURM REST mode"), sw.window)

				return
			}
		}
	}

	// Create SLURM configuration
	slurmConfig := config.SLURMConfig{
		Mode:    slurmMode,
		Timeout: slurmTimeout,
	}

	// Add REST-specific configuration if REST mode is selected
	if slurmMode == config.SLURMModeREST {
		slurmConfig.REST = config.SLURMRESTConfig{
			SocketPath: slurmSocket,
			APIVersion: slurmAPIVersion,
			Timeout:    slurmTimeout, // Use same timeout for REST API calls
		}
	}

	// Create Input struct with user-provided values
	input := &config.Input{
		Organization:     org,
		DefaultDirectory: dir,
		NonmemPath:       nonmemPath,
		NonmemBinary:     nonmemBinary,
		Scheduler:        scheduler,
		ExecutionMode:    executionMode,
		SLURM:            slurmConfig, // Include SLURM configuration
		ProjectsEnable:   true,
		Validation: config.ValidationControl{
			IQ: "~/.config/janus/validation/iq-report.json",
			OQ: "~/.config/janus/validation/oq-report.json",
		},
	}

	// Create config directory
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		dialog.ShowError(fmt.Errorf("failed to create config directory: %w", err), sw.window)

		return
	}

	// Write config file
	if err := writeConfigFile(configPath, input); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save configuration: %w", err), sw.window)

		return
	}

	// Signal completion FIRST - this will trigger main window display
	if sw.onComplete != nil {
		sw.onComplete(input)
	}

	// Close wizard window properly after main window is shown
	sw.window.Close()
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.yml"
	}

	return filepath.Join(home, ".config", "janus", "config.yml")
}

func writeConfigFile(configPath string, input *config.Input) error {
	// Build SLURM configuration section
	slurmSection := ""
	if input.Scheduler == "SLURM" && input.SLURM.Mode != "" {
		slurmSection = fmt.Sprintf(`
# SLURM Configuration
slurm:
  mode: "%s"
  timeout: "%s"`, input.SLURM.Mode, input.SLURM.Timeout)

		// Add REST-specific configuration if REST mode
		if input.SLURM.Mode == config.SLURMModeREST {
			slurmSection += fmt.Sprintf(`
  rest:
    socket_path: "%s"
    api_version: "%s"
    timeout: "%s"`,
				input.SLURM.REST.SocketPath,
				input.SLURM.REST.APIVersion,
				input.SLURM.REST.Timeout)
		}
	} else {
		slurmSection = `
# SLURM Configuration (disabled - using default CLI mode)
# slurm:
#   mode: "CLI"
#   timeout: "30s"
#   rest:
#     socket_path: "/var/run/slurm/slurmrestd.sock"
#     api_version: "v0.0.40"
#     timeout: "30s"`
	}

	configContent := fmt.Sprintf(`# Janus Configuration File
# Created through setup wizard

# User Configuration
# Note: Username is automatically detected from the OS user
organization: "%s"
default-directory: "%s"
nonmem-path: "%s"
nonmem-binary: "%s"

# Scheduler Configuration
scheduler: "%s"%s

# Execution Configuration
execution-mode: "%s"

# Feature Flags
projects: %t

# Validation Configuration for CFR 21 Part 11 compliance
validation:
  iq: "%s"
  oq: "%s"

# Additional Settings (for future use)
# audit:
#   backend: "filesystem"
#   path: "~/.config/janus/audit"
#
# projects:
#   default-template: "standard"
#   auto-backup: true
`,
		input.Organization,
		input.DefaultDirectory,
		input.NonmemPath,
		input.NonmemBinary,
		input.Scheduler,
		slurmSection,
		input.ExecutionMode,
		input.ProjectsEnable,
		input.Validation.IQ,
		input.Validation.OQ,
	)

	return os.WriteFile(configPath, []byte(configContent), 0600)
}
