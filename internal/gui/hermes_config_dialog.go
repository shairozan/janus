package gui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fynecontainer "fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/container"
)

// imageSelector holds the components for the container image selection UI.
type imageSelector struct {
	selectEntry      *widget.SelectEntry
	refreshButton    *widget.Button
	container        *fyne.Container
	images           []container.DiscoveredImage
	commandPathEntry *widget.Entry // Reference to command path entry for auto-fill
}

// newImageSelector creates a new image selector with dropdown and refresh functionality.
func (a *App) newImageSelector(defaultValue string) *imageSelector {
	selector := &imageSelector{}

	// Create the SelectEntry (combo box with text input)
	selector.selectEntry = widget.NewSelectEntry([]string{})
	selector.selectEntry.SetPlaceHolder("e.g., ghcr.io/metrumresearchgroup/nonmem:7.5.1")
	selector.selectEntry.SetText(defaultValue)

	// Create refresh button
	selector.refreshButton = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		selector.refresh(a)
	})
	selector.refreshButton.Importance = widget.LowImportance

	// Combine into horizontal container
	selector.container = fynecontainer.NewBorder(
		nil, nil, nil, selector.refreshButton,
		selector.selectEntry,
	)

	// Initial discovery
	go selector.refresh(a)

	return selector
}

// refresh queries Docker for compatible images and updates the dropdown.
func (s *imageSelector) refresh(a *App) {
	// Get Docker socket path from config if available
	var socketPath string
	if a.config != nil && a.config.Hermes.Container.DockerSocket != "" {
		socketPath = a.config.Hermes.Container.DockerSocket
	}

	// Create discoverer
	discoverer, err := container.NewDockerDiscoverer(socketPath)
	if err != nil {
		return
	}
	defer discoverer.Close()

	// Discover images
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	images, err := discoverer.Discover(ctx, container.DiscoveryOptions{
		Platform: container.PlatformNONMEM,
	})
	if err != nil {
		return
	}

	s.images = images

	// Build options list with display names
	options := make([]string, 0, len(images))
	for _, img := range images {
		options = append(options, img.PrimaryTag())
	}

	// Update the SelectEntry options on the UI thread
	fyne.Do(func() {
		s.selectEntry.SetOptions(options)
	})
}

// Text returns the current text value.
func (s *imageSelector) Text() string {
	return s.selectEntry.Text
}

// SetText sets the text value.
func (s *imageSelector) SetText(text string) {
	s.selectEntry.SetText(text)
}

// Widget returns the container widget for use in forms.
func (s *imageSelector) Widget() fyne.CanvasObject {
	return s.container
}

// SetCommandPathEntry sets the command path entry for auto-fill functionality.
func (s *imageSelector) SetCommandPathEntry(entry *widget.Entry) {
	s.commandPathEntry = entry

	// Set up OnChanged to auto-fill command path when image is selected
	s.selectEntry.OnChanged = func(value string) {
		if s.commandPathEntry == nil {
			return
		}

		// Find the selected image in our discovered images
		for _, img := range s.images {
			if img.PrimaryTag() == value {
				// Found a match - set or clear the command path
				s.commandPathEntry.SetText(img.ContainerCommand)

				return
			}
		}

		// Not a discovered image (manual entry) - don't modify command path
	}
}

// showHermesConfigDialog displays a dialog to create .janus.config.json for a model.
// This is shown when the user tries to execute with Hermes mode but the config file
// doesn't exist yet.
//
// Parameters:
//   - modelPath: Path to the model file (config will be created in same directory)
//   - onSuccess: Callback when config is successfully created
//   - onCancel: Callback when user cancels
func (a *App) showHermesConfigDialog(modelPath string, onSuccess func(), onCancel func()) {
	// Default values
	imageSelector := a.newImageSelector("ghcr.io/metrumresearchgroup/nonmem:7.5.1")

	cpuEntry := widget.NewEntry()
	cpuEntry.SetPlaceHolder("e.g., 4")
	cpuEntry.Text = "4" // Default 4 cores

	memoryEntry := widget.NewEntry()
	memoryEntry.SetPlaceHolder("e.g., 8Gi, 4096Mi, 2G")
	memoryEntry.Text = "8Gi" // Default 8Gi

	commandPathEntry := widget.NewEntry()
	commandPathEntry.SetPlaceHolder("e.g., /opt/NONMEM/nm75/run/nmfe75")
	// No default - will be auto-filled from container label or must be entered manually

	// Wire up auto-fill for command path
	imageSelector.SetCommandPathEntry(commandPathEntry)

	// Per-model retain globs (#94), seeded from the effective default (global, else
	// NONMEM best-practice). Editable and authoritative for this model once saved.
	retainEditor := newStringListEditor("Add retain glob", "e.g. *.lst", a.effectiveRetainSeed(nil)...)

	// Create form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{
				Text:     "Container Image",
				Widget:   imageSelector.Widget(),
				HintText: "Select from discovered images or enter custom path",
			},
			{
				Text:     "Command Path",
				Widget:   commandPathEntry,
				HintText: "Path to binary inside container (auto-filled from image label)",
			},
			{
				Text:     "CPU Cores",
				Widget:   cpuEntry,
				HintText: "Number of cores to allocate (1-16)",
			},
			{
				Text:     "Memory",
				Widget:   memoryEntry,
				HintText: "Memory allocation (e.g., 8Gi, 4096Mi)",
			},
			{
				Text:     "Retain Globs",
				Widget:   retainEditor.widget(),
				HintText: "Output files collected after a run (this model wins over the global default)",
			},
		},
	}

	// Info text at the top
	infoLabel := widget.NewLabel(
		fmt.Sprintf("Config: %s", filepath.Join(filepath.Dir(modelPath), ".janus.config.json")),
	)

	content := fynecontainer.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		form,
	)

	// Create custom dialog with Save/Cancel buttons
	d := dialog.NewCustom("Configure Hermes Execution", "Cancel", content, a.window)

	// Create Save button
	saveButton := widget.NewButton("Create Config", func() {
		// Validate inputs
		image := imageSelector.Text()
		if image == "" {
			dialog.ShowError(fmt.Errorf("container image is required"), a.window)

			return
		}

		cpuText := cpuEntry.Text
		if cpuText == "" {
			dialog.ShowError(fmt.Errorf("CPU cores is required"), a.window)

			return
		}

		cpuCores, err := strconv.Atoi(cpuText)
		if err != nil || cpuCores <= 0 {
			dialog.ShowError(fmt.Errorf("CPU cores must be a positive integer, got: %s", cpuText), a.window)

			return
		}

		memory := memoryEntry.Text
		if memory == "" {
			dialog.ShowError(fmt.Errorf("memory is required"), a.window)

			return
		}

		commandPath := commandPathEntry.Text
		if commandPath == "" {
			dialog.ShowError(fmt.Errorf("command path is required"), a.window)

			return
		}

		// Create config structure
		hermesConfig := config.HermesExecutionConfig{
			Image:                image,
			ContainerCommandPath: commandPath,
			Resources: config.ResourceConfig{
				CPUCores: cpuCores,
				Memory:   memory,
			},
		}

		// Validate the config
		if err := hermesConfig.Validate(); err != nil {
			dialog.ShowError(fmt.Errorf("invalid configuration: %w", err), a.window)

			return
		}

		// Save to file
		if err := a.saveHermesConfig(modelPath, &hermesConfig, retainEditor.items()); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), a.window)

			return
		}

		// Success!
		d.Hide()
		a.refreshRetainInfo() // new per-model retain now applies
		a.showSuccessToast(fmt.Sprintf("Created .janus.config.json for %s", filepath.Base(modelPath)))
		onSuccess()
	})
	saveButton.Importance = widget.HighImportance

	// Add buttons to dialog
	d.SetButtons([]fyne.CanvasObject{
		saveButton,
		widget.NewButton("Cancel", func() {
			d.Hide()
			if onCancel != nil {
				onCancel()
			}
		}),
	})

	// Show dialog (taller to fit the retain-globs list editor).
	d.Resize(fyne.NewSize(600, 520))
	d.Show()
}

// saveHermesConfig writes the HermesExecutionConfig to .janus.config.json in the
// model's directory, along with the model-wide retain list (a top-level property,
// not part of the hermes section). This saves in the new nested format with the
// "hermes" key, preserving all other existing settings.
func (a *App) saveHermesConfig(modelPath string, cfg *config.HermesExecutionConfig, retain []string) error {
	// Try to load existing config to preserve all settings (not just specific fields)
	existingCfg, _ := config.LoadModelConfig(modelPath)

	var modelCfg *config.ModelConfig
	if existingCfg != nil {
		// Modify the existing config to preserve all current and future fields
		existingCfg.Hermes = cfg
		modelCfg = existingCfg
	} else {
		// No existing config - create a new one
		modelCfg = &config.ModelConfig{
			Hermes: cfg,
		}
	}

	// Retain is model-wide, stored at the top level of ModelConfig.
	modelCfg.Retain = retain

	return config.SaveModelConfig(modelPath, modelCfg)
}

// effectiveRetainSeed resolves the retain patterns to pre-fill a per-model config
// editor: the model's own retain when set, else the global default, else the
// NONMEM best-practice defaults. Mirrors HermesExecutor.resolveRetainPatterns'
// inheritance for the non-PsN case (#94).
func (a *App) effectiveRetainSeed(existing []string) []string {
	if len(existing) > 0 {
		return existing
	}

	if a.config != nil && len(a.config.Hermes.Retain) > 0 {
		return a.config.Hermes.Retain
	}

	return config.DefaultNONMEMRetain()
}

// showEditHermesConfigDialog displays a dialog to view and edit existing .janus.config.json.
// This is shown when the user clicks the "Hermes Config" button in the toolbar.
func (a *App) showEditHermesConfigDialog() {
	// Check if a model is loaded
	if a.currentFilePath == "" {
		dialog.ShowInformation("No Model Loaded", "Please load a model file first before configuring Hermes execution.", a.window)

		return
	}

	// Try to load existing config
	configPath := config.ConfigPath(a.currentFilePath)
	existingConfig, err := config.LoadHermesModelConfig(a.currentFilePath)

	// Determine default values
	var imageDefault, commandPathDefault, cpuDefault, memoryDefault, psnImageDefault string
	if err == nil && existingConfig != nil {
		imageDefault = existingConfig.Image
		commandPathDefault = existingConfig.ContainerCommandPath
		cpuDefault = fmt.Sprintf("%d", existingConfig.Resources.CPUCores)
		memoryDefault = existingConfig.Resources.Memory
		psnImageDefault = existingConfig.PsNImage
	} else {
		imageDefault = "ghcr.io/metrumresearchgroup/nonmem:7.5.1"
		commandPathDefault = "" // No default - must be provided by label or entered manually
		cpuDefault = "4"
		memoryDefault = "8Gi"
	}

	// Create entry fields with existing values or defaults
	imageSelector := a.newImageSelector(imageDefault)

	commandPathEntry := widget.NewEntry()
	commandPathEntry.SetPlaceHolder("e.g., /opt/NONMEM/nm75/run/nmfe75")
	commandPathEntry.Text = commandPathDefault

	// Wire up auto-fill for command path
	imageSelector.SetCommandPathEntry(commandPathEntry)

	cpuEntry := widget.NewEntry()
	cpuEntry.SetPlaceHolder("e.g., 4")
	cpuEntry.Text = cpuDefault

	memoryEntry := widget.NewEntry()
	memoryEntry.SetPlaceHolder("e.g., 8Gi, 4096Mi, 2G")
	memoryEntry.Text = memoryDefault

	// PsN orchestration image override for the bootstrap saga (#192). Optional —
	// falls back to the global hermes.psn_image. The fields above stay the NONMEM
	// execution image used for the per-fit pods.
	psnImageEntry := widget.NewEntry()
	psnImageEntry.SetPlaceHolder("optional; overrides global hermes.psn_image")
	psnImageEntry.Text = psnImageDefault

	// Per-model retain globs (#94). Authoritative for this model when set; seeded
	// from the existing config, else the effective default (global, else NONMEM).
	// Retain is a model-wide property, so read it via LoadModelRetain rather than
	// from the hermes section.
	existingRetain := config.LoadModelRetain(a.currentFilePath)
	retainEditor := newStringListEditor("Add retain glob", "e.g. *.lst", a.effectiveRetainSeed(existingRetain)...)

	// Create form
	form := &widget.Form{
		Items: []*widget.FormItem{
			{
				Text:     "Container Image",
				Widget:   imageSelector.Widget(),
				HintText: "Select from discovered images or enter custom path",
			},
			{
				Text:     "Command Path",
				Widget:   commandPathEntry,
				HintText: "Path to binary inside container (auto-filled from image label)",
			},
			{
				Text:     "CPU Cores",
				Widget:   cpuEntry,
				HintText: "Number of cores to allocate (1-16)",
			},
			{
				Text:     "Memory",
				Widget:   memoryEntry,
				HintText: "Memory allocation (e.g., 8Gi, 4096Mi)",
			},
			{
				Text:     "PsN Image",
				Widget:   psnImageEntry,
				HintText: "Bootstrap saga setup/aggregate image (no NONMEM license needed)",
			},
			{
				Text:     "Retain Globs",
				Widget:   retainEditor.widget(),
				HintText: "Output files collected after a run (this model wins over the global default)",
			},
		},
	}

	// Info text at the top
	infoLabel := widget.NewLabel(fmt.Sprintf("Config: %s", configPath))

	content := fynecontainer.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		form,
	)

	// Create custom dialog
	d := dialog.NewCustom("Hermes Configuration", "Cancel", content, a.window)

	// Create Save button
	saveButton := widget.NewButton("Save Config", func() {
		// Validate inputs
		image := imageSelector.Text()
		if image == "" {
			dialog.ShowError(fmt.Errorf("container image is required"), a.window)

			return
		}

		cpuText := cpuEntry.Text
		if cpuText == "" {
			dialog.ShowError(fmt.Errorf("CPU cores is required"), a.window)

			return
		}

		cpuCores, err := strconv.Atoi(cpuText)
		if err != nil || cpuCores <= 0 {
			dialog.ShowError(fmt.Errorf("CPU cores must be a positive integer, got: %s", cpuText), a.window)

			return
		}

		memory := memoryEntry.Text
		if memory == "" {
			dialog.ShowError(fmt.Errorf("memory is required"), a.window)

			return
		}

		commandPath := commandPathEntry.Text
		if commandPath == "" {
			dialog.ShowError(fmt.Errorf("command path is required"), a.window)

			return
		}

		// Create config structure
		hermesConfig := config.HermesExecutionConfig{
			Image:                image,
			ContainerCommandPath: commandPath,
			PsNImage:             strings.TrimSpace(psnImageEntry.Text),
			Resources: config.ResourceConfig{
				CPUCores: cpuCores,
				Memory:   memory,
			},
		}

		// Validate the config
		if err := hermesConfig.Validate(); err != nil {
			dialog.ShowError(fmt.Errorf("invalid configuration: %w", err), a.window)

			return
		}

		// Save to file
		if err := a.saveHermesConfig(a.currentFilePath, &hermesConfig, retainEditor.items()); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save config: %w", err), a.window)

			return
		}

		// Success!
		d.Hide()
		a.refreshRetainInfo() // per-model retain may have changed
		a.showSuccessToast(fmt.Sprintf("Saved .janus.config.json for %s", filepath.Base(a.currentFilePath)))
	})
	saveButton.Importance = widget.HighImportance

	// Add buttons to dialog
	d.SetButtons([]fyne.CanvasObject{
		saveButton,
		widget.NewButton("Cancel", func() {
			d.Hide()
		}),
	})

	// Show dialog (taller to fit the retain-globs list editor).
	d.Resize(fyne.NewSize(600, 520))
	d.Show()
}
