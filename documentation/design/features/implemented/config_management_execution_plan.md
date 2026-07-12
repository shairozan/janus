# Configuration Management - Execution Plan

## Overview

This document provides a detailed execution plan for implementing editable configuration management in the Janus GUI settings window. The implementation will be done in three phases following the phased approach defined in the feature specification.

**Feature Specification**: [config_management.md](config_management.md)

## Pre-Implementation Analysis

### Current State Assessment

**Existing Implementation**: [internal/gui/app.go:952-1025](../../../../internal/gui/app.go#L952-L1025)

**Current Behavior**:
- Settings window shows read-only labels for all configuration values
- No editing capability
- No save/cancel buttons
- Uses `widget.NewLabel()` for all fields
- Window is non-modal
- Configuration sourced from `a.config` (already loaded from Viper)

**Configuration System**:
- Uses Viper for config file management
- Config structure: [internal/config/config.go](../../../../internal/config/config.go)
- YAML format: `~/.config/janus/config.yaml` (Linux) or `%APPDATA%\janus\config.yaml` (Windows)
- Viper already handles config file location via `viper.ConfigFileUsed()`

### Files to Modify

1. **internal/gui/app.go** (Primary changes):
   - Replace `showSettingsPanel()` function (lines 952-1025)
   - Add validation helper functions
   - Add config save/reload logic

2. **internal/config/config.go** (Minor additions):
   - Add validation helper functions (if not already present)
   - Export any needed constants for UI dropdown options

3. **New file: internal/gui/settings.go** (Recommended):
   - Extract settings window logic into separate file
   - Keep app.go focused on main window

### Dependencies

**Existing**:
- Fyne GUI framework (already in use)
- Viper config library (already in use)
- Cobra command framework (already in use)

**No New Dependencies Required**

## Phase 1: Basic Editable Form (MVP)

**Goal**: Replace read-only labels with editable widgets, implement save/cancel functionality

**Estimated Effort**: 4-6 hours

**Validation Testing**: 1-2 hours

### Implementation Tasks

#### Task 1.1: Create Settings Dialog Structure
**File**: `internal/gui/settings.go` (NEW)

**Implementation**:
```go
package gui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/internal/config"
)

type SettingsDialog struct {
	window fyne.Window
	app    *App

	// Form widgets
	organizationEntry    *widget.Entry
	defaultDirEntry      *widget.Entry
	executionModeSelect  *widget.Select
	schedulerSelect      *widget.Select
	nonmemPathEntry      *widget.Entry
	nonmemBinaryEntry    *widget.Entry
	dockerSocketEntry    *widget.Entry
	startupTimeoutEntry  *widget.Entry
	autoCleanupCheck     *widget.Check
	validationIQEntry    *widget.Entry
	validationOQEntry    *widget.Entry

	// Conditional sections
	nonmemSection *fyne.Container
	hermesSection *fyne.Container

	// Original values for change detection
	originalValues map[string]string
}

func NewSettingsDialog(app *App) *SettingsDialog {
	return &SettingsDialog{
		app:            app,
		originalValues: make(map[string]string),
	}
}

func (s *SettingsDialog) Show() {
	// Create modal window
	s.window = s.app.fyneApp.NewWindow("Janus Settings")
	s.window.Resize(fyne.NewSize(600, 500))
	s.window.SetFixedSize(true)

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

	generalSection := container.NewVBox(
		widget.NewLabelWithStyle("General Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Organization:"),
		s.organizationEntry,
		widget.NewLabel("Default Directory:"),
		s.defaultDirEntry,
		widget.NewSeparator(),
	)

	// Execution configuration
	executionModes := []string{
		config.ExecutionModeNONMEM,
		config.ExecutionModeBBI,
		config.ExecutionModePSN,
		config.ExecutionModeHERMES,
	}

	s.executionModeSelect = widget.NewSelect(executionModes, s.onExecutionModeChanged)
	s.executionModeSelect.SetSelected(s.app.config.ExecutionMode)
	s.originalValues["execution-mode"] = s.app.config.ExecutionMode

	schedulers := []string{"LOCAL", "SLURM", "SGE", "TORQUE", "PBS"}
	s.schedulerSelect = widget.NewSelect(schedulers, nil)
	s.schedulerSelect.SetSelected(s.app.config.Scheduler)
	s.originalValues["scheduler"] = s.app.config.Scheduler

	executionSection := container.NewVBox(
		widget.NewLabelWithStyle("Execution Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Execution Mode:"),
		s.executionModeSelect,
		widget.NewLabel("Scheduler:"),
		s.schedulerSelect,
		widget.NewSeparator(),
	)

	// NONMEM-specific configuration
	s.nonmemPathEntry = widget.NewEntry()
	s.nonmemPathEntry.SetText(s.app.config.NonmemPath)
	s.originalValues["nonmem-path"] = s.app.config.NonmemPath

	s.nonmemBinaryEntry = widget.NewEntry()
	s.nonmemBinaryEntry.SetText(s.app.config.NonmemBinary)
	s.originalValues["nonmem-binary"] = s.app.config.NonmemBinary

	s.nonmemSection = container.NewVBox(
		widget.NewLabelWithStyle("NONMEM Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("NONMEM Path:"),
		s.nonmemPathEntry,
		widget.NewLabel("NONMEM Binary:"),
		s.nonmemBinaryEntry,
		widget.NewSeparator(),
	)

	// Hermes-specific configuration
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

	s.hermesSection = container.NewVBox(
		widget.NewLabelWithStyle("Hermes Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Docker Socket (optional):"),
		s.dockerSocketEntry,
		widget.NewLabel("Startup Timeout:"),
		s.startupTimeoutEntry,
		s.autoCleanupCheck,
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

	// Read-only footer
	footerSection := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Version: %s", s.app.config.Version)),
		widget.NewLabel(fmt.Sprintf("User: %s", s.app.config.User)),
	)

	// Set initial visibility based on execution mode
	s.updateSectionVisibility(s.app.config.ExecutionMode)

	// Action buttons
	cancelBtn := widget.NewButton("Cancel", s.onCancel)
	saveBtn := widget.NewButton("Save Changes", s.onSave)
	saveBtn.Importance = widget.HighImportance

	buttonBar := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, saveBtn))

	// Combine all sections with scroll
	formContent := container.NewVBox(
		infoSection,
		generalSection,
		executionSection,
		s.nonmemSection,
		s.hermesSection,
		validationSection,
		footerSection,
	)

	scrollContent := container.NewVScroll(formContent)
	scrollContent.SetMinSize(fyne.NewSize(580, 400))

	return container.NewBorder(nil, buttonBar, nil, nil, scrollContent)
}

func (s *SettingsDialog) onExecutionModeChanged(mode string) {
	s.updateSectionVisibility(mode)
	s.window.Content().Refresh()
}

func (s *SettingsDialog) updateSectionVisibility(mode string) {
	switch mode {
	case config.ExecutionModeNONMEM, config.ExecutionModeBBI, config.ExecutionModePSN:
		s.nonmemSection.Show()
		s.hermesSection.Hide()
	case config.ExecutionModeHERMES:
		s.nonmemSection.Hide()
		s.hermesSection.Show()
	}
}

func (s *SettingsDialog) hasChanges() bool {
	if s.organizationEntry.Text != s.originalValues["organization"] {
		return true
	}
	if s.defaultDirEntry.Text != s.originalValues["default-directory"] {
		return true
	}
	if s.executionModeSelect.Selected != s.originalValues["execution-mode"] {
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

	return false
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

func (s *SettingsDialog) onSave() {
	// Validate inputs
	if err := s.validate(); err != nil {
		dialog.ShowError(err, s.window)
		return
	}

	// Update Viper configuration
	viper.Set("organization", s.organizationEntry.Text)
	viper.Set("default-directory", s.defaultDirEntry.Text)
	viper.Set("execution-mode", s.executionModeSelect.Selected)
	viper.Set("scheduler", s.schedulerSelect.Selected)
	viper.Set("nonmem-path", s.nonmemPathEntry.Text)
	viper.Set("nonmem-binary", s.nonmemBinaryEntry.Text)
	viper.Set("hermes.container.docker_socket", s.dockerSocketEntry.Text)
	viper.Set("hermes.container.startup_timeout", s.startupTimeoutEntry.Text)
	viper.Set("hermes.container.cleanup", s.autoCleanupCheck.Checked)
	viper.Set("validation.iq", s.validationIQEntry.Text)
	viper.Set("validation.oq", s.validationOQEntry.Text)

	// Write config file
	if err := viper.WriteConfig(); err != nil {
		// If WriteConfig fails (no config file exists), try SafeWriteConfig
		if err := viper.SafeWriteConfig(); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save configuration: %w", err), s.window)
			return
		}
	}

	// Reload application config
	input, err := config.UnmarshalInputFromViper()
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to reload configuration: %w (config was saved, please restart Janus)", err), s.window)
		return
	}

	newConfig, err := config.NewConfig(input)
	if err != nil {
		dialog.ShowError(fmt.Errorf("configuration validation failed: %w (config was saved, please restart Janus)", err), s.window)
		return
	}

	// Update app config
	s.app.config = newConfig

	// Show success and close
	dialog.ShowInformation("Settings Saved", "Configuration has been saved successfully.", s.window)
	s.window.Close()
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

	// Execution mode must be selected
	if s.executionModeSelect.Selected == "" {
		return fmt.Errorf("execution mode must be selected")
	}

	// Scheduler must be selected
	if s.schedulerSelect.Selected == "" {
		return fmt.Errorf("scheduler must be selected")
	}

	// NONMEM-specific validation
	mode := s.executionModeSelect.Selected
	if mode == config.ExecutionModeNONMEM || mode == config.ExecutionModeBBI || mode == config.ExecutionModePSN {
		if s.nonmemPathEntry.Text == "" {
			return fmt.Errorf("NONMEM path is required for %s execution mode", mode)
		}
		if s.nonmemBinaryEntry.Text == "" {
			return fmt.Errorf("NONMEM binary is required for %s execution mode", mode)
		}
	}

	// Hermes-specific validation
	if mode == config.ExecutionModeHERMES {
		// Docker socket is optional, but if provided, validate format
		if s.dockerSocketEntry.Text != "" {
			socket := s.dockerSocketEntry.Text
			if socket != "npipe:////" && socket != "unix://" && socket != "tcp://" {
				// More lenient: just check it's not completely invalid
				// Full validation happens in config.ValidateHermesConfig
			}
		}

		// Startup timeout must be valid duration
		if s.startupTimeoutEntry.Text != "" {
			if _, err := time.ParseDuration(s.startupTimeoutEntry.Text); err != nil {
				return fmt.Errorf("startup timeout must be a valid duration (e.g., '30s', '1m'): %w", err)
			}
		}
	}

	// Validation paths - just check they're not empty for now
	if s.validationIQEntry.Text == "" {
		return fmt.Errorf("validation IQ report path cannot be empty")
	}
	if s.validationOQEntry.Text == "" {
		return fmt.Errorf("validation OQ report path cannot be empty")
	}

	return nil
}
```

#### Task 1.2: Integrate Settings Dialog into App
**File**: `internal/gui/app.go`

**Changes**:
1. Replace `showSettingsPanel()` function:
```go
func (a *App) showSettingsPanel() {
	if a.settingsOpen {
		return // Settings already open
	}

	a.settingsOpen = true
	settingsDialog := NewSettingsDialog(a)
	settingsDialog.Show()

	// Update settingsOpen when dialog closes
	settingsDialog.window.SetOnClosed(func() {
		a.settingsOpen = false
	})
}
```

2. No other changes needed in app.go for Phase 1

#### Task 1.3: Manual Testing
**Test Plan**:

```markdown
## Phase 1 Manual Testing Checklist

### Basic Functionality
- [ ] Open settings window via menu
- [ ] Verify all fields are populated from current config
- [ ] Edit organization field → changes reflected
- [ ] Edit default directory → changes reflected
- [ ] Change execution mode dropdown → verify selection changes
- [ ] Change scheduler dropdown → verify selection changes
- [ ] Click Cancel without changes → window closes immediately
- [ ] Make changes → Click Cancel → confirmation dialog appears
- [ ] Confirm cancel → changes discarded, window closes
- [ ] Decline cancel → return to form with changes intact

### Save Functionality
- [ ] Make valid changes → Click Save → success dialog appears
- [ ] Verify config file updated (check ~/.config/janus/config.yaml)
- [ ] Reopen settings → verify changes persisted
- [ ] Restart Janus → verify changes still present

### Validation
- [ ] Clear organization → Click Save → error dialog
- [ ] Clear default directory → Click Save → error dialog
- [ ] Select NONMEM mode, clear NONMEM path → Click Save → error dialog
- [ ] Select NONMEM mode, clear binary → Click Save → error dialog
- [ ] Select HERMES mode, invalid timeout format → Click Save → error dialog
- [ ] Enter valid values → Click Save → success

### Execution Mode Switching
- [ ] Select NONMEM → verify NONMEM section shows, Hermes section hides
- [ ] Select BBI → verify NONMEM section shows, Hermes section hides
- [ ] Select PSN → verify NONMEM section shows, Hermes section hides
- [ ] Select HERMES → verify Hermes section shows, NONMEM section hides
- [ ] Switch modes multiple times → verify sections toggle correctly

### Edge Cases
- [ ] Open settings when no config file exists → displays defaults
- [ ] Save when config file is read-only → clear error message
- [ ] Config file path display shows actual path used
- [ ] Version and User fields are read-only (labels, not entries)
```

#### Task 1.4: Unit Tests
**File**: `internal/gui/settings_test.go` (NEW)

```go
//go:build unit
// +build unit

package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/pharmalytica/janus/internal/config"
)

func TestSettingsDialog_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*SettingsDialog)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText("/home/user/models")
				s.executionModeSelect.SetSelected(config.ExecutionModeHERMES)
				s.schedulerSelect.SetSelected("LOCAL")
				s.validationIQEntry.SetText("/path/to/iq.json")
				s.validationOQEntry.SetText("/path/to/oq.json")
			},
			expectError: false,
		},
		{
			name: "empty organization",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("")
				s.defaultDirEntry.SetText("/home/user/models")
				s.executionModeSelect.SetSelected(config.ExecutionModeHERMES)
				s.schedulerSelect.SetSelected("LOCAL")
				s.validationIQEntry.SetText("/path/to/iq.json")
				s.validationOQEntry.SetText("/path/to/oq.json")
			},
			expectError: true,
			errorMsg:    "organization cannot be empty",
		},
		{
			name: "NONMEM mode missing path",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText("/home/user/models")
				s.executionModeSelect.SetSelected(config.ExecutionModeNONMEM)
				s.schedulerSelect.SetSelected("LOCAL")
				s.nonmemPathEntry.SetText("")
				s.nonmemBinaryEntry.SetText("nmfe76")
				s.validationIQEntry.SetText("/path/to/iq.json")
				s.validationOQEntry.SetText("/path/to/oq.json")
			},
			expectError: true,
			errorMsg:    "NONMEM path is required",
		},
		{
			name: "invalid Hermes timeout",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText("/home/user/models")
				s.executionModeSelect.SetSelected(config.ExecutionModeHERMES)
				s.schedulerSelect.SetSelected("LOCAL")
				s.startupTimeoutEntry.SetText("invalid")
				s.validationIQEntry.SetText("/path/to/iq.json")
				s.validationOQEntry.SetText("/path/to/oq.json")
			},
			expectError: true,
			errorMsg:    "startup timeout must be a valid duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal SettingsDialog for testing
			s := &SettingsDialog{}
			s.organizationEntry = widget.NewEntry()
			s.defaultDirEntry = widget.NewEntry()
			s.executionModeSelect = widget.NewSelect(nil, nil)
			s.schedulerSelect = widget.NewSelect(nil, nil)
			s.nonmemPathEntry = widget.NewEntry()
			s.nonmemBinaryEntry = widget.NewEntry()
			s.startupTimeoutEntry = widget.NewEntry()
			s.validationIQEntry = widget.NewEntry()
			s.validationOQEntry = widget.NewEntry()

			tt.setup(s)

			err := s.validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSettingsDialog_HasChanges(t *testing.T) {
	s := &SettingsDialog{
		originalValues: make(map[string]string),
	}

	s.organizationEntry = widget.NewEntry()
	s.organizationEntry.SetText("Original Org")
	s.originalValues["organization"] = "Original Org"

	// No changes initially
	assert.False(t, s.hasChanges())

	// Make a change
	s.organizationEntry.SetText("Modified Org")
	assert.True(t, s.hasChanges())
}
```

### Deliverables

**Phase 1 Complete When**:
- [ ] `internal/gui/settings.go` created with full SettingsDialog implementation
- [ ] `internal/gui/app.go` updated to use new SettingsDialog
- [ ] `internal/gui/settings_test.go` created with unit tests
- [ ] All unit tests pass: `go test -tags=unit ./internal/gui/...`
- [ ] Manual testing checklist 100% complete
- [ ] No regressions in existing functionality

**Expected Test Coverage**: >80% for settings.go

---

## Phase 2: Enhanced UX

**Goal**: Add file/folder pickers, improve validation, enhance user feedback

**Estimated Effort**: 3-4 hours

**Validation Testing**: 1 hour

### Implementation Tasks

#### Task 2.1: Add File/Folder Picker Dialogs

**File**: `internal/gui/settings.go`

**Changes**:
1. Replace plain entry widgets with entry + button combos for path fields:

```go
// In buildForm(), replace default directory entry with:
s.defaultDirEntry = widget.NewEntry()
s.defaultDirEntry.SetText(s.app.config.DefaultDirectory)

defaultDirBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		s.defaultDirEntry.SetText(uri.Path())
	}, s.window)
})

defaultDirRow := container.NewBorder(nil, nil, nil, defaultDirBtn, s.defaultDirEntry)

// Add to generalSection instead of just the entry
generalSection := container.NewVBox(
	widget.NewLabelWithStyle("General Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	widget.NewLabel("Organization:"),
	s.organizationEntry,
	widget.NewLabel("Default Directory:"),
	defaultDirRow,  // <-- Use the border container
	widget.NewSeparator(),
)

// Similarly for NONMEM path
nonmemPathBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		s.nonmemPathEntry.SetText(uri.Path())
	}, s.window)
})
nonmemPathRow := container.NewBorder(nil, nil, nil, nonmemPathBtn, s.nonmemPathEntry)

// For validation report paths (file picker, not folder picker)
validationIQBtn := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
	dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
		if err != nil || uri == nil {
			return
		}
		defer uri.Close()
		s.validationIQEntry.SetText(uri.URI().Path())
	}, s.window)
})
validationIQRow := container.NewBorder(nil, nil, nil, validationIQBtn, s.validationIQEntry)

// Similar for OQ
```

#### Task 2.2: Enhanced Validation with Path Checking

**File**: `internal/gui/settings.go`

**Update `validate()` function**:
```go
func (s *SettingsDialog) validate() error {
	// ... existing validation ...

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

	// Enhanced NONMEM path validation (warn only)
	mode := s.executionModeSelect.Selected
	if mode == config.ExecutionModeNONMEM || mode == config.ExecutionModeBBI || mode == config.ExecutionModePSN {
		nonmemPath := s.nonmemPathEntry.Text
		if nonmemPath != "" {
			if _, err := os.Stat(nonmemPath); os.IsNotExist(err) {
				// Show warning but don't block save
				dialog.ShowInformation(
					"NONMEM Path Warning",
					fmt.Sprintf("NONMEM path does not exist: %s\nThis may be okay if it's on an unmounted drive.", nonmemPath),
					s.window,
				)
			}
		}
	}

	// Enhanced validation path checking
	iqPath := s.validationIQEntry.Text
	if iqPath != "" {
		parentDir := filepath.Dir(iqPath)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("validation IQ report parent directory does not exist: %s", parentDir)
		}
	}

	oqPath := s.validationOQEntry.Text
	if oqPath != "" {
		parentDir := filepath.Dir(oqPath)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			return fmt.Errorf("validation OQ report parent directory does not exist: %s", parentDir)
		}
	}

	return nil
}
```

#### Task 2.3: Improved Success Feedback

**File**: `internal/gui/settings.go`

**Update `onSave()` function**:
```go
func (s *SettingsDialog) onSave() {
	// ... existing validation and save logic ...

	// Enhanced success message
	changedFields := []string{}
	if s.organizationEntry.Text != s.originalValues["organization"] {
		changedFields = append(changedFields, "Organization")
	}
	if s.executionModeSelect.Selected != s.originalValues["execution-mode"] {
		changedFields = append(changedFields, "Execution Mode")
	}
	// ... check other fields ...

	successMsg := "Configuration has been saved successfully."
	if len(changedFields) > 0 {
		successMsg += fmt.Sprintf("\n\nUpdated fields: %s", strings.Join(changedFields, ", "))
	}

	dialog.ShowInformation("Settings Saved", successMsg, s.window)
	s.window.Close()
}
```

### Deliverables

**Phase 2 Complete When**:
- [ ] Folder picker buttons added for directory fields
- [ ] File picker buttons added for validation report paths
- [ ] Enhanced validation with path existence checking
- [ ] Improved success messages showing what changed
- [ ] Manual testing checklist for Phase 2 complete
- [ ] All existing tests still pass

---

## Phase 3: Advanced Features (Optional/Future)

**Goal**: Config backup, rollback, real-time validation, tooltips

**Estimated Effort**: 4-6 hours

**Status**: Deferred to post-MVP

**Features**:
1. Config file backup before save (`.bak` file)
2. Rollback capability if reload fails
3. Real-time validation feedback (not just on save)
4. Help text/tooltips for each field
5. "Reset to defaults" button

*Implementation details TBD when prioritized*

---

## Testing Strategy

### Unit Tests

**Files**:
- `internal/gui/settings_test.go` (Phase 1)
- Additional test cases in Phase 2

**Coverage Target**: >80%

**Run Command**:
```bash
go test -tags=unit -v ./internal/gui/...
```

**Test Cases**:
- [ ] Validation logic for all fields
- [ ] Change detection logic
- [ ] Section visibility toggling
- [ ] Viper config update logic (mocked)

### Integration Tests

**File**: `internal/gui/settings_integration_test.go` (NEW)

```go
//go:build integration
// +build integration

package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestSettingsSaveLoad_Integration(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initialize viper with temp config
	viper.Reset()
	viper.SetConfigFile(configPath)
	viper.Set("organization", "Original Org")
	viper.Set("default-directory", "/original/path")
	viper.Set("execution-mode", "NONMEM")
	require.NoError(t, viper.WriteConfig())

	// Load config
	require.NoError(t, viper.ReadInConfig())
	input, err := config.UnmarshalInputFromViper()
	require.NoError(t, err)

	cfg, err := config.NewConfig(input)
	require.NoError(t, err)

	// Simulate settings change
	viper.Set("organization", "Modified Org")
	viper.Set("execution-mode", "HERMES")
	require.NoError(t, viper.WriteConfig())

	// Reload config
	viper.Reset()
	viper.SetConfigFile(configPath)
	require.NoError(t, viper.ReadInConfig())
	input, err = config.UnmarshalInputFromViper()
	require.NoError(t, err)

	newCfg, err := config.NewConfig(input)
	require.NoError(t, err)

	// Verify changes persisted
	assert.Equal(t, "Modified Org", newCfg.Organization)
	assert.Equal(t, "HERMES", newCfg.ExecutionMode)
}

func TestConfigFilePermissions_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create read-only config file
	viper.Reset()
	viper.SetConfigFile(configPath)
	viper.Set("organization", "Test")
	require.NoError(t, viper.WriteConfig())

	// Make read-only
	require.NoError(t, os.Chmod(configPath, 0444))

	// Attempt to write should fail
	viper.Set("organization", "Modified")
	err := viper.WriteConfig()
	assert.Error(t, err)

	// Cleanup
	os.Chmod(configPath, 0644)
}
```

**Run Command**:
```bash
go test -tags=integration -v ./internal/gui/...
```

### GUI Tests

**File**: `internal/gui/settings_gui_test.go` (NEW)

```go
//go:build gui
// +build gui

package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/config"
)

func TestSettingsDialog_UIInteraction(t *testing.T) {
	// Create test app
	app := test.NewApp()
	defer app.Quit()

	// Create minimal App instance
	janusApp := &App{
		fyneApp: app,
		config: &config.Config{
			Input: config.Input{
				Organization:     "Test Org",
				DefaultDirectory: "/home/user/models",
				ExecutionMode:    config.ExecutionModeHERMES,
				Scheduler:        "LOCAL",
			},
			Version: "test",
			User:    "testuser",
		},
	}

	// Create settings dialog
	settings := NewSettingsDialog(janusApp)
	settings.Show()

	// Test organization entry
	settings.organizationEntry.SetText("Modified Org")
	assert.Equal(t, "Modified Org", settings.organizationEntry.Text)

	// Test has changes detection
	assert.True(t, settings.hasChanges())

	// Test execution mode change
	settings.executionModeSelect.SetSelected(config.ExecutionModeNONMEM)
	assert.Equal(t, config.ExecutionModeNONMEM, settings.executionModeSelect.Selected)

	// Verify NONMEM section visible, Hermes hidden
	// (Visual verification in manual tests)
}
```

**Run Command**:
```bash
go test -tags=gui -v ./internal/gui/...
```

### Manual Testing

**Document**: `docs/testing/config-management-manual-tests.md` (NEW)

Create comprehensive manual testing document with screenshots and expected results for each test case.

---

## CI/CD Integration

### GitHub Actions Changes

**File**: `.github/workflows/test.yml`

**Changes Required**: ✅ **NONE** for Phase 1 & 2

**Reason**:
- New GUI code uses existing Fyne testing infrastructure
- Unit tests run with existing `go test -tags=unit` command
- Integration tests run with existing `go test -tags=integration` command
- GUI tests are manual (Fyne GUI tests don't run in headless CI easily)

**Existing CI Flow**:
```yaml
- name: Unit Tests
  run: go test -tags=unit -v ./...

- name: Integration Tests
  run: go test -tags=integration -v ./...
```

**This already covers the new test files**, no changes needed.

### Docker Build Changes

**File**: `Dockerfile` / Docker build commands

**Changes Required**: ✅ **NONE**

**Reason**: Settings dialog is part of GUI binary, no separate build artifacts needed.

---

## Validation & Acceptance Criteria

### Phase 1 Acceptance Criteria

**Functional**:
- [ ] FR-001: Users can edit all mutable configuration values
- [ ] FR-002: Changes persist to config file
- [ ] FR-003: Config file path displayed (read-only)
- [ ] FR-004: Users can cancel changes without persisting
- [ ] FR-005: Invalid values rejected with clear error messages
- [ ] FR-006: Successfully saved config reloads without restart
- [ ] FR-007: Execution mode changes show/hide relevant sections

**Non-Functional**:
- [ ] NFR-001: Settings window is modal
- [ ] NFR-002: Config save is atomic (Viper handles this)
- [ ] NFR-003: Validation before file write
- [ ] NFR-004: Immediate feedback for validation errors

**Testing**:
- [ ] All unit tests pass
- [ ] Manual testing checklist 100% complete
- [ ] No regressions in existing functionality
- [ ] Code review approved

### Phase 2 Acceptance Criteria

**Enhanced UX**:
- [ ] Folder picker dialogs for directory fields
- [ ] File picker dialogs for validation report paths
- [ ] Enhanced validation with path existence checking
- [ ] Success message shows what changed
- [ ] Confirmation dialog on cancel with unsaved changes

**Testing**:
- [ ] All Phase 1 tests still pass
- [ ] Enhanced validation tests added
- [ ] File picker dialogs manually tested
- [ ] Code review approved

---

## Risk Assessment

### Technical Risks

**Risk 1: Viper Config Write Fails**
- **Probability**: Low
- **Impact**: High (users can't save settings)
- **Mitigation**:
  - Comprehensive error handling with clear messages
  - Guide users to check file permissions
  - Log errors for debugging

**Risk 2: Config Reload Breaks Application State**
- **Probability**: Medium
- **Impact**: High (app becomes unstable)
- **Mitigation**:
  - Validate config before reload
  - Warn user to restart if reload fails
  - Keep backup of working config

**Risk 3: Conditional Section Visibility Bugs**
- **Probability**: Medium
- **Impact**: Low (cosmetic issue)
- **Mitigation**:
  - Extensive manual testing of mode switching
  - Clear logic in `updateSectionVisibility()`

**Risk 4: File Path Validation Too Strict**
- **Probability**: Medium
- **Impact**: Medium (blocks valid use cases)
- **Mitigation**:
  - Warn but don't block for non-critical paths
  - Allow paths on unmounted drives

### User Experience Risks

**Risk 1: Users Lose Unsaved Changes**
- **Probability**: Medium
- **Impact**: Medium (frustration)
- **Mitigation**:
  - Confirmation dialog on cancel with changes
  - Clear indication of unsaved changes

**Risk 2: Validation Errors Unclear**
- **Probability**: Low
- **Impact**: Medium (user confusion)
- **Mitigation**:
  - Specific error messages for each field
  - Context-aware validation messages

---

## Timeline & Milestones

### Phase 1: Basic Editable Form (MVP)
- **Duration**: 1-2 days
- **Milestone**: Users can edit and save configuration

**Day 1**:
- Morning: Implement SettingsDialog structure and UI (Tasks 1.1, 1.2)
- Afternoon: Implement validation and save logic (Task 1.1 continued)

**Day 2**:
- Morning: Unit tests (Task 1.4)
- Afternoon: Manual testing (Task 1.3)
- Evening: Bug fixes and polish

### Phase 2: Enhanced UX
- **Duration**: 1 day
- **Milestone**: Improved user experience with pickers and better validation

**Day 3**:
- Morning: File/folder picker dialogs (Task 2.1)
- Afternoon: Enhanced validation and feedback (Tasks 2.2, 2.3)
- Evening: Manual testing and bug fixes

### Total Estimated Timeline: 2-3 days

---

## Success Metrics

**Quantitative**:
- Unit test coverage >80%
- Zero regressions in existing tests
- Manual testing checklist 100% complete
- All acceptance criteria met

**Qualitative**:
- User feedback: "Settings are easy to understand and modify"
- No confusion about which fields are editable
- Clear error messages guide users to fix issues
- Config changes take effect without app restart

---

## Post-Implementation Tasks

1. **Documentation**:
   - [ ] Update user documentation with settings screenshots
   - [ ] Document all configuration options
   - [ ] Add troubleshooting guide for common config issues

2. **User Guide**:
   - [ ] Create video tutorial for settings configuration
   - [ ] Document execution mode differences
   - [ ] Explain validation report paths

3. **Monitoring**:
   - [ ] Track config save success/failure rates (future analytics)
   - [ ] Monitor user feedback for usability issues

4. **Future Enhancements** (Phase 3):
   - [ ] Config backup/rollback
   - [ ] Real-time validation
   - [ ] Tooltips for each field
   - [ ] "Reset to defaults" button

---

## References

- **Feature Specification**: [config_management.md](config_management.md)
- **Fyne Documentation**: https://developer.fyne.io/
- **Viper Documentation**: https://github.com/spf13/viper
- **Current Settings Code**: [internal/gui/app.go:952-1025](../../../../internal/gui/app.go#L952-L1025)
- **Config Structure**: [internal/config/config.go](../../../../internal/config/config.go)
