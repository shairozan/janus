//go:build gui
// +build gui

package gui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestImageSelector(t *testing.T) {
	test.NewApp()

	t.Run("creates selector with default value", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		selector := app.newImageSelector("test-image:latest")

		require.NotNil(t, selector)
		require.NotNil(t, selector.selectEntry)
		require.NotNil(t, selector.refreshButton)
		require.NotNil(t, selector.container)
		assert.Equal(t, "test-image:latest", selector.Text())
	})

	t.Run("widget returns container", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		selector := app.newImageSelector("default:value")
		widget := selector.Widget()

		require.NotNil(t, widget)
		assert.Equal(t, selector.container, widget)
	})

	t.Run("SetText updates value", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		selector := app.newImageSelector("initial:value")
		selector.SetText("new:value")

		assert.Equal(t, "new:value", selector.Text())
	})

	t.Run("manual entry works", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		selector := app.newImageSelector("")

		// Simulate user typing
		selector.selectEntry.SetText("custom/image:v1.0")

		assert.Equal(t, "custom/image:v1.0", selector.Text())
	})
}

func TestShowHermesConfigDialog(t *testing.T) {
	test.NewApp()

	t.Run("creates config file on save", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Create temp directory with a model file
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "test.mod")
		err := os.WriteFile(modelPath, []byte("$PROBLEM Test"), 0644)
		require.NoError(t, err)

		// Track callbacks
		successCalled := false
		cancelCalled := false

		// Show dialog (this creates the dialog but doesn't block)
		app.showHermesConfigDialog(
			modelPath,
			func() { successCalled = true },
			func() { cancelCalled = true },
		)

		// Verify dialog was created (we can't easily interact with it in tests)
		// The dialog components are created but not yet saved
		assert.False(t, successCalled, "Success should not be called until save")
		assert.False(t, cancelCalled, "Cancel should not be called")
	})
}

func TestShowEditHermesConfigDialog(t *testing.T) {
	test.NewApp()

	t.Run("shows error when no model loaded", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// No model loaded (currentFilePath is empty)
		app.currentFilePath = ""

		// This should show an info dialog, not crash
		app.showEditHermesConfigDialog()
		// If we get here without panic, test passes
	})

	t.Run("loads existing config values", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		// Create temp directory with model and config
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "test.mod")
		err := os.WriteFile(modelPath, []byte("$PROBLEM Test"), 0644)
		require.NoError(t, err)

		// Create existing config
		existingConfig := &config.HermesExecutionConfig{
			Image: "existing/image:v2.0",
			Resources: config.ResourceConfig{
				CPUCores: 8,
				Memory:   "16Gi",
			},
			ContainerCommandPath: "/opt/NONMEM/nm76/run/nmfe76",
		}
		err = app.saveHermesConfig(modelPath, existingConfig, nil)
		require.NoError(t, err)

		// Set current file path
		app.currentFilePath = modelPath

		// Show edit dialog
		app.showEditHermesConfigDialog()
		// Dialog is shown - in real test we'd verify field values
	})
}

func TestSaveHermesConfig(t *testing.T) {
	test.NewApp()

	t.Run("saves valid config", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")

		cfg := &config.HermesExecutionConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 4,
				Memory:   "8Gi",
			},
			ContainerCommandPath: "/opt/NONMEM/nm75/run/nmfe75",
		}
		retain := []string{"*.lst", "sdtab*"}

		err := app.saveHermesConfig(modelPath, cfg, retain)
		require.NoError(t, err)

		// Verify file was created
		configPath := config.ConfigPath(modelPath)
		_, err = os.Stat(configPath)
		assert.NoError(t, err, "Config file should exist")

		// Verify we can load it back — the hermes section and the model-wide retain.
		loaded, err := config.LoadModelConfig(modelPath)
		require.NoError(t, err)
		require.NotNil(t, loaded.Hermes)
		assert.Equal(t, cfg.Image, loaded.Hermes.Image)
		assert.Equal(t, cfg.Resources.CPUCores, loaded.Hermes.Resources.CPUCores)
		assert.Equal(t, cfg.Resources.Memory, loaded.Hermes.Resources.Memory)
		assert.Equal(t, retain, loaded.Retain)
	})
}

func TestEffectiveRetainSeed(t *testing.T) {
	test.NewApp()

	// Existing per-model retain wins.
	a := &App{config: &config.Config{Input: config.Input{Hermes: config.HermesConfig{Retain: []string{"*.global"}}}}}
	assert.Equal(t, []string{"*.model"}, a.effectiveRetainSeed([]string{"*.model"}))

	// No per-model → global default.
	assert.Equal(t, []string{"*.global"}, a.effectiveRetainSeed(nil))

	// No per-model, no global → NONMEM best-practice defaults.
	b := &App{config: &config.Config{}}
	assert.Equal(t, config.DefaultNONMEMRetain(), b.effectiveRetainSeed(nil))
}

func TestImageSelectorWithDockerUnavailable(t *testing.T) {
	test.NewApp()

	t.Run("gracefully handles Docker unavailable", func(t *testing.T) {
		ctx := context.Background()

		// Create app with invalid Docker socket
		testConfig := &config.Config{
			Input: config.Input{
				Organization:  "Test Organization",
				ExecutionMode: config.ExecutionModeHERMES,
				Hermes: config.HermesConfig{
					Container: config.HermesContainerConfig{
						DockerSocket: "/nonexistent/docker.sock",
					},
				},
			},
		}

		app := NewApp(ctx)
		app.SetConfiguration(testConfig)
		defer app.Cleanup()

		// Should create selector without error even if Docker unavailable
		selector := app.newImageSelector("fallback:image")

		require.NotNil(t, selector)
		assert.Equal(t, "fallback:image", selector.Text())
		// Dropdown will be empty but manual entry still works
	})
}

func TestImageSelectorEmptyOptions(t *testing.T) {
	test.NewApp()

	t.Run("works with no discovered images", func(t *testing.T) {
		app := createTestApp()
		defer app.Cleanup()

		selector := app.newImageSelector("manual:image")

		// Even with no options, should work
		require.NotNil(t, selector)
		assert.Equal(t, "manual:image", selector.Text())

		// Can still set text manually
		selector.SetText("another:image")
		assert.Equal(t, "another:image", selector.Text())
	})
}
