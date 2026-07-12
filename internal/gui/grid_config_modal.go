package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// GridSettings represents the grid scheduler configuration settings.
type GridSettings struct {
	Version   string    `json:"version"`
	Scheduler string    `json:"scheduler"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Resources struct {
		Nodes        int    `json:"nodes"`
		CPUsPerTask  int    `json:"cpus_per_task"`
		MemoryGB     int    `json:"memory_gb,omitempty"`
		TimeLimit    string `json:"time_limit,omitempty"`
		Partition    string `json:"partition,omitempty"`
	} `json:"resources"`
	NONMEM struct {
		Parallel          bool     `json:"parallel"`
		Threads           int      `json:"threads"`
		AdditionalOptions []string `json:"additional_options,omitempty"`
	} `json:"nonmem"`
	Job struct {
		CustomName string `json:"custom_name,omitempty"`
	} `json:"job"`
}

// GridConfigResult represents the result of the grid configuration modal.
type GridConfigResult struct {
	Settings    *GridSettings
	SaveAsTemplate bool
	Cancelled   bool
}

// ShowGridConfigModal displays the grid scheduler configuration modal.
func (a *App) ShowGridConfigModal(callback func(*GridConfigResult)) {
	if a.config == nil {
		callback(&GridConfigResult{Cancelled: true})

		return
	}

	// Load existing settings if available
	existingSettings := a.loadGridSettings()

	// Create modal window
	modal := a.fyneApp.NewWindow("Grid Scheduler Configuration")
	modal.Resize(fyne.NewSize(600, 700))
	modal.CenterOnScreen()

	// Get scheduler type from config
	schedulerType := a.config.Scheduler
	if schedulerType == "" {
		schedulerType = "SLURM"
	}

	// Header with scheduler info
	headerIcon := "🖥️" // Generic grid icon
	switch schedulerType {
	case "SLURM":
		headerIcon = "🔗"
	case "SGE":
		headerIcon = "☸️"
	case "TORQUE":
		headerIcon = "⚙️"
	}

	// Update header to show if settings are loaded
	headerText := fmt.Sprintf("%s %s Grid Configuration", headerIcon, schedulerType)
	if existingSettings != nil {
		headerText += " (Settings Loaded)"
	}

	headerLabel := widget.NewLabelWithStyle(
		headerText,
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Resource Configuration Section
	resourceCard := widget.NewCard("Compute Resources", "", a.buildResourcesSection(existingSettings))

	// Time & Queue Management Section
	queueCard := widget.NewCard("Time & Queue Management", "", a.buildQueueSection(existingSettings, schedulerType))

	// NONMEM-Specific Options Section
	nonmemCard := widget.NewCard("NONMEM Options", "", a.buildNonmemSection(existingSettings))

	// Job Configuration Section
	jobCard := widget.NewCard("Job Configuration", "", a.buildJobSection(existingSettings))

	// Save settings checkbox - positioned near submit button
	saveSettingsCheck := widget.NewCheck("Save settings for future executions", nil)
	if existingSettings != nil {
		saveSettingsCheck.SetChecked(true) // Default to checked if settings already exist
	}

	// Add help text showing where settings are saved
	modelName := "model"
	if a.currentFilePath != "" {
		modelName = strings.TrimSuffix(filepath.Base(a.currentFilePath), filepath.Ext(a.currentFilePath))
	}
	settingsHelpText := widget.NewLabelWithStyle(
		fmt.Sprintf("Settings will be saved as .%s.settings.grid.json", modelName),
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	// Action buttons
	submitBtn := widget.NewButton("Submit Job", func() {
		result := a.collectGridSettings(schedulerType, saveSettingsCheck.Checked)
		modal.Close()
		callback(result)
	})
	submitBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		modal.Close()
		callback(&GridConfigResult{Cancelled: true})
	})

	// Button area with save checkbox and help text
	buttonBox := container.NewVBox(
		saveSettingsCheck,
		settingsHelpText,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewLabel(""), // Spacer
			cancelBtn,
			submitBtn,
		),
	)

	// Create scrollable content area that expands to fill available space
	scrollContent := container.NewVBox(
		resourceCard,
		queueCard,
		nonmemCard,
		jobCard,
		widget.NewLabel(""), // Spacer to push content up
	)

	scrollArea := container.NewScroll(scrollContent)
	scrollArea.SetMinSize(fyne.NewSize(580, 500)) // Ensure minimum size

	// Main layout using border layout for better space utilization
	content := container.NewBorder(
		container.NewVBox(headerLabel, widget.NewSeparator()), // Top: header
		buttonBox, // Bottom: save checkbox and buttons
		nil, // Left
		nil, // Right
		scrollArea, // Center: takes all remaining space
	)

	modal.SetContent(content)
	modal.Show()
}

// Store UI components for later collection.
var modalComponents = make(map[string]fyne.CanvasObject)

func (a *App) buildResourcesSection(settings *GridSettings) fyne.CanvasObject {
	// Nodes
	nodesEntry := widget.NewEntry()
	nodesEntry.SetText("1") // Default
	if settings != nil && settings.Resources.Nodes > 0 {
		nodesEntry.SetText(strconv.Itoa(settings.Resources.Nodes))
	}
	modalComponents["nodes"] = nodesEntry

	// CPUs per Task
	cpusEntry := widget.NewEntry()
	cpusEntry.SetText("") // Empty for cluster default
	if settings != nil && settings.Resources.CPUsPerTask > 0 {
		cpusEntry.SetText(strconv.Itoa(settings.Resources.CPUsPerTask))
	}
	modalComponents["cpus"] = cpusEntry

	// Memory (GB)
	memoryEntry := widget.NewEntry()
	memoryEntry.SetText("") // Empty for cluster default
	if settings != nil && settings.Resources.MemoryGB > 0 {
		memoryEntry.SetText(strconv.Itoa(settings.Resources.MemoryGB))
	}
	modalComponents["memory"] = memoryEntry

	// Quick preset buttons for memory
	presetButtons := container.NewHBox(
		widget.NewButton("4 GB", func() { memoryEntry.SetText("4") }),
		widget.NewButton("8 GB", func() { memoryEntry.SetText("8") }),
		widget.NewButton("16 GB", func() { memoryEntry.SetText("16") }),
		widget.NewButton("32 GB", func() { memoryEntry.SetText("32") }),
	)

	// Set entry field widths for better appearance
	nodesEntry.Resize(fyne.NewSize(200, nodesEntry.Size().Height))
	cpusEntry.Resize(fyne.NewSize(200, cpusEntry.Size().Height))
	memoryEntry.Resize(fyne.NewSize(200, memoryEntry.Size().Height))

	// Create form with consistent spacing and alignment
	form := container.NewVBox(
		// Nodes row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Nodes:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, nodesEntry),
		widget.NewSeparator(),

		// CPUs row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("CPUs per Task:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, cpusEntry),
		widget.NewSeparator(),

		// Memory row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Memory (GB):", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, memoryEntry),

		// Memory presets
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Quick presets:", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
			nil, presetButtons),
		widget.NewSeparator(),

		widget.NewRichTextFromMarkdown("*Leave empty fields to use cluster defaults*"),
	)

	return form
}

func (a *App) buildQueueSection(settings *GridSettings, schedulerType string) fyne.CanvasObject {
	// Time Limit
	timeLimitEntry := widget.NewEntry()
	timeLimitEntry.SetPlaceHolder("HH:MM:SS")
	if settings != nil && settings.Resources.TimeLimit != "" {
		timeLimitEntry.SetText(settings.Resources.TimeLimit)
	}
	modalComponents["timeLimit"] = timeLimitEntry

	// Partition/Queue - terminology varies by scheduler
	queueLabel := "Partition:"
	switch schedulerType {
	case "SGE", "TORQUE":
		queueLabel = "Queue:"
	}

	partitionEntry := widget.NewEntry()
	partitionEntry.SetPlaceHolder("Leave empty for default")
	if settings != nil && settings.Resources.Partition != "" {
		partitionEntry.SetText(settings.Resources.Partition)
	}
	modalComponents["partition"] = partitionEntry

	// Set entry field widths
	timeLimitEntry.Resize(fyne.NewSize(200, timeLimitEntry.Size().Height))
	partitionEntry.Resize(fyne.NewSize(200, partitionEntry.Size().Height))

	return container.NewVBox(
		// Time Limit row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Time Limit:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, timeLimitEntry),
		widget.NewSeparator(),

		// Partition/Queue row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle(queueLabel, fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, partitionEntry),
		widget.NewSeparator(),

		widget.NewRichTextFromMarkdown("*Leave empty to use cluster defaults*"),
	)
}

func (a *App) buildNonmemSection(settings *GridSettings) fyne.CanvasObject {
	// Parallel Execution checkbox
	parallelCheck := widget.NewCheck("Enable parallel execution", func(checked bool) {
		if threadsEntry, ok := modalComponents["threads"].(*widget.Entry); ok {
			if checked {
				threadsEntry.Enable()
			} else {
				threadsEntry.Disable()
				threadsEntry.SetText("")
			}
		}
	})
	if settings != nil {
		parallelCheck.SetChecked(settings.NONMEM.Parallel)
	}
	modalComponents["parallel"] = parallelCheck

	// Threads for parallel
	threadsEntry := widget.NewEntry()
	threadsEntry.SetPlaceHolder("Number of threads")
	if settings != nil && settings.NONMEM.Threads > 0 {
		threadsEntry.SetText(strconv.Itoa(settings.NONMEM.Threads))
	}
	if settings == nil || !settings.NONMEM.Parallel {
		threadsEntry.Disable()
	}
	modalComponents["threads"] = threadsEntry

	// Additional NONMEM options
	optionsEntry := widget.NewMultiLineEntry()
	optionsEntry.SetPlaceHolder("Additional NONMEM options (e.g., -maxeval=9999)")
	if settings != nil && len(settings.NONMEM.AdditionalOptions) > 0 {
		optionsEntry.SetText(strings.Join(settings.NONMEM.AdditionalOptions, " "))
	}
	modalComponents["options"] = optionsEntry

	// Set entry field widths
	threadsEntry.Resize(fyne.NewSize(200, threadsEntry.Size().Height))
	optionsEntry.Resize(fyne.NewSize(500, 100)) // Larger for multiline

	return container.NewVBox(
		parallelCheck,
		widget.NewSeparator(),

		// Threads row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Threads:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, threadsEntry),
		widget.NewSeparator(),

		// Additional Options
		widget.NewLabelWithStyle("Additional Options:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
		optionsEntry,
	)
}

func (a *App) buildJobSection(settings *GridSettings) fyne.CanvasObject {
	// Job Name
	jobNameEntry := widget.NewEntry()
	jobNameEntry.SetPlaceHolder("Auto-generated if empty")
	if settings != nil && settings.Job.CustomName != "" {
		jobNameEntry.SetText(settings.Job.CustomName)
	}
	modalComponents["jobName"] = jobNameEntry

	// Set entry field width
	jobNameEntry.Resize(fyne.NewSize(300, jobNameEntry.Size().Height))

	return container.NewVBox(
		// Job Name row
		container.NewBorder(nil, nil,
			widget.NewLabelWithStyle("Job Name:", fyne.TextAlignLeading, fyne.TextStyle{Bold: false}),
			nil, jobNameEntry),
		widget.NewSeparator(),

		widget.NewRichTextFromMarkdown("*Leave empty for auto-generated name*"),
	)
}

func (a *App) collectGridSettings(schedulerType string, saveTemplate bool) *GridConfigResult {
	settings := &GridSettings{
		Version:   "1.0",
		Scheduler: schedulerType,
		CreatedBy: "user", // TODO: Get from config
		CreatedAt: time.Now(),
	}

	// Collect nodes
	if nodesEntry, ok := modalComponents["nodes"].(*widget.Entry); ok {
		if nodes, err := strconv.Atoi(strings.TrimSpace(nodesEntry.Text)); err == nil && nodes > 0 {
			settings.Resources.Nodes = nodes
		} else {
			settings.Resources.Nodes = 1 // Default
		}
	}

	// Collect CPUs
	if cpusEntry, ok := modalComponents["cpus"].(*widget.Entry); ok {
		if cpus, err := strconv.Atoi(strings.TrimSpace(cpusEntry.Text)); err == nil && cpus > 0 {
			settings.Resources.CPUsPerTask = cpus
		}
	}

	// Collect memory
	if memoryEntry, ok := modalComponents["memory"].(*widget.Entry); ok {
		if memory, err := strconv.Atoi(strings.TrimSpace(memoryEntry.Text)); err == nil && memory > 0 {
			settings.Resources.MemoryGB = memory
		}
	}

	// Collect time limit
	if timeLimitEntry, ok := modalComponents["timeLimit"].(*widget.Entry); ok {
		settings.Resources.TimeLimit = strings.TrimSpace(timeLimitEntry.Text)
	}

	// Collect partition
	if partitionEntry, ok := modalComponents["partition"].(*widget.Entry); ok {
		settings.Resources.Partition = strings.TrimSpace(partitionEntry.Text)
	}

	// Collect parallel settings
	if parallelCheck, ok := modalComponents["parallel"].(*widget.Check); ok {
		settings.NONMEM.Parallel = parallelCheck.Checked
		if parallelCheck.Checked {
			if threadsEntry, ok := modalComponents["threads"].(*widget.Entry); ok {
				if threads, err := strconv.Atoi(strings.TrimSpace(threadsEntry.Text)); err == nil && threads > 0 {
					settings.NONMEM.Threads = threads
				}
			}
		}
	}

	// Collect additional options
	if optionsEntry, ok := modalComponents["options"].(*widget.Entry); ok {
		optionsText := strings.TrimSpace(optionsEntry.Text)
		if optionsText != "" {
			settings.NONMEM.AdditionalOptions = strings.Fields(optionsText)
		}
	}

	// Collect job name
	if jobNameEntry, ok := modalComponents["jobName"].(*widget.Entry); ok {
		settings.Job.CustomName = strings.TrimSpace(jobNameEntry.Text)
	}

	// Clear components map for next use
	modalComponents = make(map[string]fyne.CanvasObject)

	return &GridConfigResult{
		Settings:       settings,
		SaveAsTemplate: saveTemplate,
		Cancelled:      false,
	}
}

// loadGridSettings loads existing grid settings for the current model.
func (a *App) loadGridSettings() *GridSettings {
	if a.currentFilePath == "" {
		return nil
	}

	settingsPath := a.getGridSettingsPath()
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}

	var settings GridSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	return &settings
}

// saveGridSettings saves grid settings to the model-specific file.
func (a *App) saveGridSettings(settings *GridSettings) error {
	if a.currentFilePath == "" {
		return fmt.Errorf("no model file loaded")
	}

	settingsPath := a.getGridSettingsPath()

	// Set user info from config if available
	if a.config != nil {
		settings.CreatedBy = a.config.User
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal grid settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to save grid settings: %w", err)
	}

	return nil
}

// getGridSettingsPath returns the path for the grid settings file.
func (a *App) getGridSettingsPath() string {
	modelDir := filepath.Dir(a.currentFilePath)
	modelName := strings.TrimSuffix(filepath.Base(a.currentFilePath), filepath.Ext(a.currentFilePath))

	return filepath.Join(modelDir, fmt.Sprintf(".%s.settings.grid.json", modelName))
}