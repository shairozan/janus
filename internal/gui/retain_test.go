//go:build gui
// +build gui

package gui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func TestSaveModelRetain_NonHermesModel(t *testing.T) {
	test.NewApp()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB\n"), 0600))

	a := &App{}

	// No .janus.config.json yet: saving retain must create one, with no Hermes
	// section required (retain is a model-wide property).
	require.NoError(t, a.saveModelRetain(modelPath, []string{"*.lst", "patab*"}))

	cfg, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"*.lst", "patab*"}, cfg.Retain)
	assert.Nil(t, cfg.Hermes, "a non-Hermes model must not gain a hermes section")

	// LoadModelRetain surfaces it (no Hermes validation).
	assert.Equal(t, []string{"*.lst", "patab*"}, config.LoadModelRetain(modelPath))
}

func TestSaveModelRetain_PreservesHermes(t *testing.T) {
	test.NewApp()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB\n"), 0600))

	// Seed a config that already has a hermes section.
	require.NoError(t, config.SaveModelConfig(modelPath, &config.ModelConfig{
		Hermes: &config.HermesExecutionConfig{
			Image:     "test/image:v1",
			Resources: config.ResourceConfig{CPUCores: 4, Memory: "8Gi"},
		},
	}))

	a := &App{}
	require.NoError(t, a.saveModelRetain(modelPath, []string{"sdtab*"}))

	cfg, err := config.LoadModelConfig(modelPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"sdtab*"}, cfg.Retain)
	require.NotNil(t, cfg.Hermes, "the hermes section must be preserved")
	assert.Equal(t, "test/image:v1", cfg.Hermes.Image)
}

func TestCurrentModelRetain_NonHermesModel(t *testing.T) {
	test.NewApp()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB\n"), 0600))
	require.NoError(t, config.SaveModelConfig(modelPath, &config.ModelConfig{
		Retain: []string{"*.custom"},
	}))

	a := &App{currentFilePath: modelPath, config: &config.Config{}}

	globs, source := a.currentModelRetain()
	assert.Equal(t, []string{"*.custom"}, globs)
	assert.Equal(t, "this model", source)
}

func TestRefreshRetainInfo_ShownForNonHermesModel(t *testing.T) {
	test.NewApp()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.mod")
	require.NoError(t, os.WriteFile(modelPath, []byte("$PROB\n"), 0600))

	app := createTestApp()
	defer app.Cleanup()

	// Build the run tab so the retain summary box is constructed.
	require.NotNil(t, app.buildModelRunTab())

	// A non-Hermes execution mode: retain is model-wide, so the summary must
	// still be shown once a model is loaded.
	app.currentFilePath = modelPath

	app.refreshRetainInfo()

	require.NotNil(t, app.retainInfoBox)
	assert.True(t, app.retainInfoBox.Visible(), "retain summary should be shown for any loaded model")
}
