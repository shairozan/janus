//go:build gui
// +build gui

package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGridConfigModal(t *testing.T) {
	test.NewApp()

	t.Run("show_grid_config_modal", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Show the modal with a callback
		app.ShowGridConfigModal(func(result *GridConfigResult) {
			// Test callback - just verify it's called
		})

		// Verify that modal would be created (in a headless test environment,
		// we can't actually verify the modal is displayed, but we can verify
		// the code doesn't panic)
		assert.NotPanics(t, func() {
			app.ShowGridConfigModal(func(result *GridConfigResult) {
				// Test callback
			})
		})
	})

	t.Run("show_grid_config_modal_no_config", func(t *testing.T) {
		app := createTestApp()
		app.config = nil // Remove config
		defer app.Cleanup()

		callbackResult := (*GridConfigResult)(nil)
		callbackCalled := false

		app.ShowGridConfigModal(func(result *GridConfigResult) {
			callbackResult = result
			callbackCalled = true
		})

		// Should call callback immediately with cancelled result
		assert.True(t, callbackCalled)
		require.NotNil(t, callbackResult)
		assert.True(t, callbackResult.Cancelled)
	})
}

func TestGridConfigDataCollection(t *testing.T) {
	test.NewApp()

	t.Run("collect_grid_settings", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Clear any existing modal components
		modalComponents = make(map[string]fyne.CanvasObject)

		// Simulate form inputs by adding mock widgets to modalComponents
		modalComponents["nodes"] = widget.NewEntry()
		modalComponents["nodes"].(*widget.Entry).SetText("2")

		modalComponents["cpus"] = widget.NewEntry()
		modalComponents["cpus"].(*widget.Entry).SetText("8")

		modalComponents["memory"] = widget.NewEntry()
		modalComponents["memory"].(*widget.Entry).SetText("16")

		modalComponents["timeLimit"] = widget.NewEntry()
		modalComponents["timeLimit"].(*widget.Entry).SetText("02:00:00")

		modalComponents["partition"] = widget.NewEntry()
		modalComponents["partition"].(*widget.Entry).SetText("cpu")

		modalComponents["parallel"] = widget.NewCheck("Parallel", nil)
		modalComponents["parallel"].(*widget.Check).SetChecked(true)

		modalComponents["threads"] = widget.NewEntry()
		modalComponents["threads"].(*widget.Entry).SetText("4")

		modalComponents["options"] = widget.NewEntry()
		modalComponents["options"].(*widget.Entry).SetText("-maxeval=9999 -files=100")

		modalComponents["jobName"] = widget.NewEntry()
		modalComponents["jobName"].(*widget.Entry).SetText("test-job")

		// Test data collection
		result := app.collectGridSettings("SLURM", true)

		require.NotNil(t, result)
		assert.False(t, result.Cancelled)
		assert.True(t, result.SaveAsTemplate)

		settings := result.Settings
		require.NotNil(t, settings)

		// Verify collected data
		assert.Equal(t, "SLURM", settings.Scheduler)
		assert.Equal(t, "1.0", settings.Version)
		assert.Equal(t, 2, settings.Resources.Nodes)
		assert.Equal(t, 8, settings.Resources.CPUsPerTask)
		assert.Equal(t, 16, settings.Resources.MemoryGB)
		assert.Equal(t, "02:00:00", settings.Resources.TimeLimit)
		assert.Equal(t, "cpu", settings.Resources.Partition)
		assert.True(t, settings.NONMEM.Parallel)
		assert.Equal(t, 4, settings.NONMEM.Threads)
		assert.Equal(t, []string{"-maxeval=9999", "-files=100"}, settings.NONMEM.AdditionalOptions)
		assert.Equal(t, "test-job", settings.Job.CustomName)
	})

	t.Run("collect_grid_settings_invalid_inputs", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Clear and set invalid inputs
		modalComponents = make(map[string]fyne.CanvasObject)

		// Invalid nodes input
		modalComponents["nodes"] = widget.NewEntry()
		modalComponents["nodes"].(*widget.Entry).SetText("invalid")

		// Invalid CPU input
		modalComponents["cpus"] = widget.NewEntry()
		modalComponents["cpus"].(*widget.Entry).SetText("-1")

		// Invalid memory input
		modalComponents["memory"] = widget.NewEntry()
		modalComponents["memory"].(*widget.Entry).SetText("not-a-number")

		result := app.collectGridSettings("SLURM", false)

		require.NotNil(t, result)
		settings := result.Settings

		// Should use defaults for invalid inputs
		assert.Equal(t, 1, settings.Resources.Nodes) // Default
		assert.Equal(t, 0, settings.Resources.CPUsPerTask) // Invalid input results in 0
		assert.Equal(t, 0, settings.Resources.MemoryGB) // Invalid input results in 0
	})

	t.Run("collect_grid_settings_empty_inputs", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Clear all inputs
		modalComponents = make(map[string]fyne.CanvasObject)

		result := app.collectGridSettings("SLURM", false)

		require.NotNil(t, result)
		settings := result.Settings

		// Should handle empty inputs gracefully
		assert.Equal(t, "SLURM", settings.Scheduler)
		assert.Equal(t, "1.0", settings.Version)
		assert.Equal(t, "user", settings.CreatedBy) // TODO value
	})
}

func TestGridConfigResourcesSections(t *testing.T) {
	test.NewApp()

	t.Run("build_resources_section", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Create test settings
		settings := &GridSettings{}
		settings.Resources.Nodes = 2
		settings.Resources.CPUsPerTask = 8
		settings.Resources.MemoryGB = 16
		settings.Resources.TimeLimit = "02:00:00"
		settings.Resources.Partition = "gpu"

		section := app.buildResourcesSection(settings)
		require.NotNil(t, section)

		// This tests that the function doesn't panic and returns a valid widget
		// In a full UI test environment, we could verify the form fields are created
	})

	t.Run("build_nonmem_section", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		settings := &GridSettings{}
		settings.NONMEM.Parallel = true
		settings.NONMEM.Threads = 4
		settings.NONMEM.AdditionalOptions = []string{"-maxeval=9999"}

		section := app.buildNonmemSection(settings)
		require.NotNil(t, section)

		// Verify section creation doesn't panic
	})

	t.Run("build_job_section", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		settings := &GridSettings{}
		settings.Job.CustomName = "custom-job-name"

		section := app.buildJobSection(settings)
		require.NotNil(t, section)

		// Verify section creation doesn't panic
	})
}

func TestGridSettingsFileOperations(t *testing.T) {
	test.NewApp()

	t.Run("load_grid_settings_no_file", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Set a non-existent file path
		app.currentFilePath = "/tmp/nonexistent_model.ctl"

		settings := app.loadGridSettings()

		// Should return default settings when file doesn't exist
		assert.Nil(t, settings)
	})

	t.Run("get_grid_settings_path", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		app.currentFilePath = "/tmp/test_model.ctl"

		path := app.getGridSettingsPath()
		assert.Contains(t, path, "/tmp")
		assert.Contains(t, path, ".test_model.settings.grid.json")
		assert.Contains(t, path, ".test_model")
	})

	t.Run("get_grid_settings_path_empty", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		app.currentFilePath = ""

		// Should not panic with empty path
		assert.NotPanics(t, func() {
			path := app.getGridSettingsPath()
			assert.NotEmpty(t, path) // Should return some default path
		})
	})
}

func TestGridConfigStructs(t *testing.T) {
	t.Run("grid_config_result", func(t *testing.T) {
		result := &GridConfigResult{
			Settings:       &GridSettings{},
			SaveAsTemplate: true,
			Cancelled:      false,
		}

		assert.NotNil(t, result.Settings)
		assert.True(t, result.SaveAsTemplate)
		assert.False(t, result.Cancelled)
	})

	t.Run("grid_settings_structure", func(t *testing.T) {
		settings := &GridSettings{
			Version:   "1.0",
			Scheduler: "SLURM",
			CreatedBy: "test-user",
		}
		settings.Resources.Nodes = 4
		settings.Resources.CPUsPerTask = 16
		settings.Resources.MemoryGB = 32
		settings.Resources.TimeLimit = "04:00:00"
		settings.Resources.Partition = "bigmem"
		settings.NONMEM.Parallel = true
		settings.NONMEM.Threads = 8
		settings.NONMEM.AdditionalOptions = []string{"-maxeval=9999", "-files=100"}
		settings.Job.CustomName = "analysis-job"

		// Verify structure integrity
		assert.Equal(t, "1.0", settings.Version)
		assert.Equal(t, "SLURM", settings.Scheduler)
		assert.Equal(t, "test-user", settings.CreatedBy)
		assert.Equal(t, 4, settings.Resources.Nodes)
		assert.Equal(t, 16, settings.Resources.CPUsPerTask)
		assert.Equal(t, 32, settings.Resources.MemoryGB)
		assert.Equal(t, "04:00:00", settings.Resources.TimeLimit)
		assert.Equal(t, "bigmem", settings.Resources.Partition)
		assert.True(t, settings.NONMEM.Parallel)
		assert.Equal(t, 8, settings.NONMEM.Threads)
		assert.Len(t, settings.NONMEM.AdditionalOptions, 2)
		assert.Equal(t, "analysis-job", settings.Job.CustomName)
	})
}