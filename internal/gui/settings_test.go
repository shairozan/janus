//go:build unit
// +build unit

package gui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/config"
)

func TestSettingsDialog_Validate(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	validDefaultDir := filepath.Join(tmpDir, "models")

	tests := []struct {
		name        string
		setup       func(*SettingsDialog)
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid Hermes configuration",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
				s.startupTimeoutEntry.SetText("30s")
			},
			expectError: false,
		},
		{
			name: "valid local NONMEM configuration",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHere)
				s.nonmemPathEntry.SetText("/opt/NONMEM")
				s.nonmemBinaryEntry.SetText("nmfe76")
			},
			expectError: false,
		},
		{
			name: "empty organization",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
			},
			expectError: true,
			errorMsg:    "organization cannot be empty",
		},
		{
			name: "empty default directory",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText("")
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
			},
			expectError: true,
			errorMsg:    "default directory cannot be empty",
		},
		{
			name: "host engine missing NONMEM path",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHere)
				s.nonmemPathEntry.SetText("")
				s.nonmemBinaryEntry.SetText("nmfe76")
			},
			expectError: true,
			errorMsg:    "NONMEM path is required",
		},
		{
			name: "host engine missing NONMEM binary",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHere)
				s.nonmemPathEntry.SetText("/opt/NONMEM")
				s.nonmemBinaryEntry.SetText("")
			},
			expectError: true,
			errorMsg:    "NONMEM binary is required",
		},
		{
			name: "BBI engine requires NONMEM path",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineBBI)
				s.destinationSelect.SetSelected(config.DestinationHere)
				s.nonmemPathEntry.SetText("")
				s.nonmemBinaryEntry.SetText("nmfe76")
			},
			expectError: true,
			errorMsg:    "NONMEM path is required",
		},
		{
			name: "PSN engine requires NONMEM binary",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EnginePSN)
				s.destinationSelect.SetSelected(config.DestinationHere)
				s.nonmemPathEntry.SetText("/opt/NONMEM")
				s.nonmemBinaryEntry.SetText("")
			},
			expectError: true,
			errorMsg:    "NONMEM binary is required",
		},
		{
			name: "invalid Hermes timeout format",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
				s.startupTimeoutEntry.SetText("invalid")
			},
			expectError: true,
			errorMsg:    "startup timeout must be a valid duration",
		},
		{
			name: "valid Hermes timeout formats",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
				s.startupTimeoutEntry.SetText("1m30s")
			},
			expectError: false,
		},
		{
			name: "Kubernetes orchestrator requires a namespace",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorKubernetes)
				s.kubeNamespaceEntry.SetText("")
			},
			expectError: true,
			errorMsg:    "kubernetes orchestrator requires a namespace",
		},
		{
			name: "Kubernetes orchestrator with a namespace is valid",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorKubernetes)
				s.kubeNamespaceEntry.SetText("janus")
			},
			expectError: false,
		},
		{
			name: "no engine selected",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected("")
				s.destinationSelect.SetSelected(config.DestinationHere)
			},
			expectError: true,
			errorMsg:    "engine must be selected",
		},
		{
			name: "no destination selected",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected("")
			},
			expectError: true,
			errorMsg:    "destination must be selected",
		},
		{
			name: "scheduler destination requires a scheduler",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationScheduler)
				s.schedulerSelect.SetSelected("")
				s.nonmemPathEntry.SetText("/opt/NONMEM")
				s.nonmemBinaryEntry.SetText("nmfe76")
			},
			expectError: true,
			errorMsg:    "scheduler must be selected",
		},
		{
			name: "Hermes destination does not require a scheduler",
			setup: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Test Org")
				s.defaultDirEntry.SetText(validDefaultDir)
				s.engineSelect.SetSelected(config.EngineNONMEM)
				s.destinationSelect.SetSelected(config.DestinationHermes)
				s.orchestratorSelect.SetSelected(config.OrchestratorDocker)
				s.schedulerSelect.SetSelected("") // Empty scheduler is OK for Hermes
				s.startupTimeoutEntry.SetText("30s")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create minimal SettingsDialog for testing
			s := &SettingsDialog{}
			s.organizationEntry = widget.NewEntry()
			s.defaultDirEntry = widget.NewEntry()

			// Initialize Select widgets with options to allow SetSelected to work
			s.engineSelect = widget.NewSelect(config.GetValidEngines(), nil)
			s.destinationSelect = widget.NewSelect(config.GetValidDestinations(), nil)
			s.orchestratorSelect = widget.NewSelect(config.GetValidOrchestrators(), nil)
			s.kubeNamespaceEntry = widget.NewEntry()

			schedulers := []string{"LOCAL", "SLURM", "SGE", "TORQUE", "PBS"}
			s.schedulerSelect = widget.NewSelect(schedulers, nil)

			s.nonmemPathEntry = widget.NewEntry()
			s.nonmemBinaryEntry = widget.NewEntry()
			s.installEditor = newInstallationsEditor(nil)
			s.dockerSocketEntry = widget.NewEntry()
			s.startupTimeoutEntry = widget.NewEntry()
			s.autoCleanupCheck = widget.NewCheck("", nil)
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
	tests := []struct {
		name           string
		originalValues map[string]string
		currentValues  func(*SettingsDialog)
		expectChanges  bool
	}{
		{
			name: "no changes",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Original Org")
				s.defaultDirEntry.SetText("/original/path")
			},
			expectChanges: false,
		},
		{
			name: "organization changed",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Modified Org")
				s.defaultDirEntry.SetText("/original/path")
			},
			expectChanges: true,
		},
		{
			name: "default directory changed",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Original Org")
				s.defaultDirEntry.SetText("/modified/path")
			},
			expectChanges: true,
		},
		{
			name: "engine changed",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
				"engine":            config.EngineNONMEM,
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Original Org")
				s.defaultDirEntry.SetText("/original/path")
				s.engineSelect.SetSelected(config.EnginePSN)
			},
			expectChanges: true,
		},
		{
			name: "destination changed",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
				"destination":       config.DestinationHere,
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Original Org")
				s.defaultDirEntry.SetText("/original/path")
				s.destinationSelect.SetSelected(config.DestinationHermes)
			},
			expectChanges: true,
		},
		{
			name: "scheduler changed",
			originalValues: map[string]string{
				"organization":      "Original Org",
				"default-directory": "/original/path",
				"scheduler":         "LOCAL",
			},
			currentValues: func(s *SettingsDialog) {
				s.organizationEntry.SetText("Original Org")
				s.defaultDirEntry.SetText("/original/path")
				s.schedulerSelect.SetSelected("SLURM")
			},
			expectChanges: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SettingsDialog{
				originalValues: tt.originalValues,
			}

			s.organizationEntry = widget.NewEntry()
			s.defaultDirEntry = widget.NewEntry()
			s.runPrefixEntry = widget.NewEntry()
			s.altDataDirEntry = widget.NewEntry()
			s.closeConsoleCheck = widget.NewCheck("", nil)
			s.engineSelect = widget.NewSelect(config.GetValidEngines(), nil)
			s.destinationSelect = widget.NewSelect(config.GetValidDestinations(), nil)
			s.orchestratorSelect = widget.NewSelect(config.GetValidOrchestrators(), nil)
			s.schedulerSelect = widget.NewSelect(nil, nil)
			s.nonmemPathEntry = widget.NewEntry()
			s.nonmemBinaryEntry = widget.NewEntry()
			s.installEditor = newInstallationsEditor(nil)
			s.dockerSocketEntry = widget.NewEntry()
			s.startupTimeoutEntry = widget.NewEntry()
			s.validationIQEntry = widget.NewEntry()
			s.validationOQEntry = widget.NewEntry()

			tt.currentValues(s)

			result := s.hasChanges()
			assert.Equal(t, tt.expectChanges, result)
		})
	}
}

func TestSettingsDialog_UpdateExecutionVisibility(t *testing.T) {
	tests := []struct {
		name                   string
		destination            string
		expectNONMEMVisible    bool
		expectHermesVisible    bool
		expectSchedulerVisible bool
	}{
		{
			name:                   "Here destination shows NONMEM section, no scheduler",
			destination:            config.DestinationHere,
			expectNONMEMVisible:    true,
			expectHermesVisible:    false,
			expectSchedulerVisible: false,
		},
		{
			name:                   "Scheduler destination shows NONMEM section and scheduler",
			destination:            config.DestinationScheduler,
			expectNONMEMVisible:    true,
			expectHermesVisible:    false,
			expectSchedulerVisible: true,
		},
		{
			name:                   "Hermes destination shows Hermes section only",
			destination:            config.DestinationHermes,
			expectNONMEMVisible:    false,
			expectHermesVisible:    true,
			expectSchedulerVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SettingsDialog{}
			s.destinationSelect = widget.NewSelect(config.GetValidDestinations(), nil)
			s.destinationSelect.SetSelected(tt.destination)

			// Create dummy containers to test visibility
			s.nonmemSection = container.NewVBox()
			s.hermesSection = container.NewVBox()
			s.schedulerSection = container.NewVBox()

			// Set initial visibility to opposite of expected
			if tt.expectNONMEMVisible {
				s.nonmemSection.Hide()
			} else {
				s.nonmemSection.Show()
			}

			if tt.expectHermesVisible {
				s.hermesSection.Hide()
			} else {
				s.hermesSection.Show()
			}

			if tt.expectSchedulerVisible {
				s.schedulerSection.Hide()
			} else {
				s.schedulerSection.Show()
			}

			// Update visibility based on destination
			s.updateExecutionVisibility()

			// Check visibility matches expectations
			assert.Equal(t, tt.expectNONMEMVisible, s.nonmemSection.Visible())
			assert.Equal(t, tt.expectHermesVisible, s.hermesSection.Visible())
			assert.Equal(t, tt.expectSchedulerVisible, s.schedulerSection.Visible())
		})
	}
}
