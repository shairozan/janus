//go:build gui
// +build gui

package gui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/runlog"
)

func createTestApp() *App {
	ctx := context.Background()

	testConfig := &config.Config{
		Input: config.Input{
			Organization:  "Test Organization",
			NonmemPath:    "/opt/NONMEM",
			ExecutionMode: config.ExecutionModeNONMEM,
		},
	}

	appInstance := NewApp(ctx)
	appInstance.SetConfiguration(testConfig)
	return appInstance
}

func TestAppMainInterface(t *testing.T) {
	test.NewApp() // Initialize test app

	t.Run("build_main_interface", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		mainInterface := app.buildMainInterface()
		require.NotNil(t, mainInterface)

		// Main interface is now a border container, not directly AppTabs
		// Just verify it's a valid container
		assert.NotNil(t, mainInterface, "Main interface should be a valid container")
	})
}

func TestModelRunTab(t *testing.T) {
	test.NewApp()

	t.Run("build_model_run_tab", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		runTab := app.buildModelRunTab()
		require.NotNil(t, runTab)

		// Check that it's a container
		if container, ok := runTab.(*container.Split); ok {
			assert.NotNil(t, container.Leading)
			assert.NotNil(t, container.Trailing)
		} else if container, ok := runTab.(fyne.CanvasObject); ok {
			// Alternative layout
			assert.NotNil(t, container)
		} else {
			t.Errorf("Expected Split or Border container, got %T", runTab)
		}
	})
}

func TestModelContent(t *testing.T) {
	test.NewApp()

	t.Run("build_model_content", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		modelContent := app.buildModelContent()
		require.NotNil(t, modelContent)

		// Model content should contain form elements
		// In a real UI test environment, we would check the container structure
		// For now, just verify the object is not nil
		assert.NotNil(t, modelContent)
	})
}

func TestTextEditor(t *testing.T) {
	test.NewApp()

	t.Run("build_text_editor", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		editor := app.buildTextEditor()
		require.NotNil(t, editor)

		// Should be a container with text editor
		// In a real UI test environment, we would check the specific container type
		// For now, just verify the object is not nil
		assert.NotNil(t, editor)
	})
}

func TestRunDetailsTab(t *testing.T) {
	test.NewApp()

	t.Run("build_run_details_tab", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Create some mock run history using cached runs
		app.cachedRuns = []runlog.RunRecord{
			{
				ID:        "test-uuid-001",
				Timestamp: time.Now(),
				ModelFile: "test_model.ctl",
				Command:   "nmfe test_model.ctl test_model.lst",
				ExitCode:  0,
				Status:    "completed",
			},
		}

		runDetails := app.buildRunDetailsTab()
		require.NotNil(t, runDetails)

		// Should contain a table or list widget
		// In a real UI test environment, we would check the container structure
		// For now, just verify the object is not nil
		assert.NotNil(t, runDetails)
	})
}

func TestActiveRunsTable(t *testing.T) {
	test.NewApp()

	t.Run("build_active_runs_table", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Add some mock active runs using cached runs
		app.cachedRuns = []runlog.RunRecord{
			{
				ID:        "test-uuid-001",
				Timestamp: time.Now(),
				ModelFile: "running_model.ctl",
				Status:    "running",
			},
			{
				ID:        "test-uuid-002",
				Timestamp: time.Now().Add(-5 * time.Minute),
				ModelFile: "completed_model.ctl",
				Status:    "completed",
			},
		}

		activeTable := app.buildActiveRunsTable()
		require.NotNil(t, activeTable)

		// Should be a container with table
		assert.Greater(t, len(activeTable.Objects), 0)

		// Look for a table widget in the container
		foundTable := false
		for _, obj := range activeTable.Objects {
			if _, ok := obj.(*widget.Table); ok {
				foundTable = true
				break
			}
		}
		assert.True(t, foundTable, "Active runs should contain a table widget")
	})
}

func TestAppConfiguration(t *testing.T) {
	test.NewApp()

	t.Run("set_configuration", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		newConfig := &config.Config{
			Input: config.Input{
				Organization:  "Updated Organization",
				NonmemPath:    "/opt/NONMEM76",
				ExecutionMode: config.ExecutionModeBBI,
			},
		}

		app.SetConfiguration(newConfig)
		assert.Equal(t, newConfig, app.config)
		assert.Equal(t, "Updated Organization", app.config.Organization)
		assert.Equal(t, config.ExecutionModeBBI, app.config.ExecutionMode)
	})
}

func TestAppCleanup(t *testing.T) {
	test.NewApp()

	t.Run("cleanup_resources", func(t *testing.T) {
		app := createTestApp()

		// Add some mock resources to cleanup
		app.activeRuns = map[string]context.CancelFunc{
			"test-uuid-001": func() {}, // Mock cancel function
		}

		// Cleanup should not panic
		assert.NotPanics(t, func() {
			app.Cleanup()
		})

		// Active runs should be cleared
		assert.Empty(t, app.activeRuns)
	})
}

func TestRunRecordCreation(t *testing.T) {
	test.NewApp()

	t.Run("create_run_record", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// A run log store is created when a model is loaded; the store owns UUID
		// and timestamp assignment via AddRun. Back it with a temp dir so this test
		// exercises that real path instead of leaving the store nil.
		app.runLogStore = runlog.NewRunLogStore(t.TempDir(), "model.mod")

		// Create a run record
		isGrid := false
		isParallel := true
		cores := 4
		description := "Test run"
		nonmemOptions := "-maxeval=9999"

		record := app.createRunRecord(isGrid, isParallel, cores, &description, &nonmemOptions)

		require.NotNil(t, record)
		assert.NotEmpty(t, record.ID, "Record ID should be a non-empty UUID string")
		assert.Equal(t, isGrid, record.IsGrid)
		assert.Equal(t, isParallel, record.IsParallel)
		assert.Equal(t, cores, record.Cores)
		// Description is stored compressed, use GetDescription() to retrieve
		retrievedDesc, err := record.GetDescription()
		require.NoError(t, err)
		assert.Equal(t, description, retrievedDesc)
		assert.Equal(t, nonmemOptions, *record.NonmemOptions)
		assert.Equal(t, "running", record.Status)
	})
}

func TestErrorHandling(t *testing.T) {
	test.NewApp()

	t.Run("send_error", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Start error handling goroutine
		go app.handleErrors()

		// Send an error
		testError := assert.AnError
		app.sendError(testError)

		// Give some time for error handling
		time.Sleep(100 * time.Millisecond)

		// Error handling should not panic
		// In a real test, we might capture log output or check UI state
	})
}

func TestModelFileLoading(t *testing.T) {
	test.NewApp()

	t.Run("load_model_file_ui_not_ready", func(t *testing.T) {
		ctx := context.Background()
		fyneApp := app.New()

		// Create app without fully initializing UI
		appInstance := &App{
			fyneApp:     fyneApp,
			window:      fyneApp.NewWindow("Test"),
			cachedRuns:  []runlog.RunRecord{},
			activeRuns:  make(map[string]context.CancelFunc),
			errorCh:     make(chan error, 100),
			errorCtx:    ctx,
			errorCancel: func() {},
			// Leave modelEntry and textEditor nil to simulate UI not ready
		}

		testFilePath := "/tmp/test_model.ctl"
		err := appInstance.LoadModelFile(testFilePath)
		assert.NoError(t, err)

		// Should store path for later loading
		assert.Equal(t, testFilePath, appInstance.pendingModelPath)
	})
}

func TestNonmemCommandGeneration(t *testing.T) {
	test.NewApp()

	t.Run("build_nonmem_command_string", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		app.currentFilePath = "/tmp/test_model.ctl"

		tests := []struct {
			name              string
			isParallel        bool
			cores             int
			isGrid            bool
			additionalOptions []string
			expectedContains  []string
		}{
			{
				name:             "synchronous_execution",
				isParallel:       false,
				cores:            1,
				isGrid:           false,
				expectedContains: []string{"nmfe", "test_model.ctl"},
			},
			{
				name:             "parallel_execution",
				isParallel:       true,
				cores:            4,
				isGrid:           false,
				expectedContains: []string{"nmfe", "test_model.ctl", "-parafile"},
			},
			{
				name:              "with_additional_options",
				isParallel:        false,
				cores:             1,
				isGrid:            false,
				additionalOptions: []string{"-maxeval=9999", "-files=100"},
				expectedContains:  []string{"nmfe", "-maxeval=9999", "-files=100"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				command := app.buildNONMEMCommandString(tt.isParallel, tt.cores, tt.isGrid, tt.additionalOptions)
				assert.NotEmpty(t, command)

				for _, expected := range tt.expectedContains {
					assert.Contains(t, command, expected)
				}
			})
		}
	})
}

// TestSettingsBuildFormNoPanic guards against the axis-select handlers firing
// during buildForm's SetSelected calls (before later widgets exist). A Hermes
// destination is the worst case: the command preview reads hermesImageEntry,
// which is constructed after the orchestrator select.
func TestSettingsBuildFormNoPanic(t *testing.T) {
	test.NewApp()

	for _, dest := range []string{config.DestinationHere, config.DestinationScheduler, config.DestinationHermes} {
		app := createTestApp()
		app.config.Engine = config.EngineNONMEM
		app.config.Destination = dest
		app.config.Orchestrator = config.OrchestratorDocker

		dialog := NewSettingsDialog(app)

		// buildForm must not panic for any destination.
		assert.NotNil(t, dialog.buildForm(), "buildForm(%s) should return content", dest)
	}
}

func TestRefreshCommandPreview(t *testing.T) {
	test.NewApp()

	app := createTestApp()
	app.commandPreview = widget.NewLabel("")
	app.targetRadio = widget.NewRadioGroup([]string{"Here", "Scheduler"}, nil)
	app.targetRadio.SetSelected("Here")
	app.syncRadio = widget.NewRadioGroup([]string{"Synchronous", "Parallel"}, nil)
	app.syncRadio.SetSelected("Synchronous")
	app.coresEntry = widget.NewEntry()
	app.coresEntry.SetText("4")
	app.nonmemOptionsEntry = widget.NewEntry()

	// No model loaded → placeholder, not a command.
	app.currentFilePath = ""
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "load a model")

	// Model loaded → preview reflects the binary and the model.
	app.currentFilePath = "/tmp/run1.ctl"
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "nmfe")
	assert.Contains(t, app.commandPreview.Text, "run1.ctl")

	// Additional options flow into the preview.
	app.nonmemOptionsEntry.SetText("-maxeval=9999")
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "-maxeval=9999")

	// Parallel mode adds the parafile to the preview.
	app.syncRadio.SetSelected("Parallel")
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "-parafile")
}

func TestRefreshCommandPreviewPSNFunction(t *testing.T) {
	test.NewApp()

	app := createTestApp()
	app.config.ExecutionMode = config.ExecutionModePSN
	app.currentFilePath = "/tmp/run1.mod"

	app.commandPreview = widget.NewLabel("")
	app.targetRadio = widget.NewRadioGroup([]string{"Here", "Scheduler"}, nil)
	app.targetRadio.SetSelected("Here")
	app.syncRadio = widget.NewRadioGroup([]string{"Synchronous", "Parallel"}, nil)
	app.syncRadio.SetSelected("Synchronous")
	app.coresEntry = widget.NewEntry()
	app.coresEntry.SetText("4")
	app.nonmemOptionsEntry = widget.NewEntry()
	app.psnForm = newPSNFunctionForm()
	app.psnPresetSelect = widget.NewSelect([]string{psnDefaultPreset, "vpc", "bootstrap", "scm"}, nil)

	// Default analysis → plain execute.
	app.psnPresetSelect.SetSelected(psnDefaultPreset)
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "execute")
	assert.Contains(t, app.commandPreview.Text, "run1.mod")

	// vpc with a typed sample count → the preview shows the vpc tool and the arg.
	app.psnPresetSelect.SetSelected("vpc")
	app.psnForm.vpcSamples.SetText("500")
	app.refreshCommandPreview()
	assert.Contains(t, app.commandPreview.Text, "vpc")
	assert.Contains(t, app.commandPreview.Text, "-samples=500")
	assert.NotContains(t, app.commandPreview.Text, "execute /tmp")
}
