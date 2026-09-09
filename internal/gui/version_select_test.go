package gui

import (
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"

	"github.com/shairozan/janus/internal/config"
)

func TestRefreshPSNPresets(t *testing.T) {
	a := &App{}
	a.psnPresetSelect = widget.NewSelect(nil, nil)
	a.psnPresetContainer = container.NewVBox()

	// Non-PSN mode → hidden.
	a.config = &config.Config{Input: config.Input{ExecutionMode: config.ExecutionModeNONMEM}}
	a.refreshPSNPresets()
	assert.True(t, a.psnPresetContainer.Hidden)

	// PSN mode → default + built-in analyses + any custom presets (deduped).
	a.config = &config.Config{Input: config.Input{
		ExecutionMode: config.ExecutionModePSN,
		PSN:           config.PSNConfig{Presets: []config.PSNPreset{{Name: "my-vpc"}, {Name: "vpc"}}},
	}}
	a.refreshPSNPresets()
	assert.False(t, a.psnPresetContainer.Hidden)
	assert.Equal(t, []string{psnDefaultPreset, "vpc", "bootstrap", "scm", "my-vpc"}, a.psnPresetSelect.Options)
	assert.Equal(t, psnDefaultPreset, a.psnPresetSelect.Selected)

	// selectedPSNFunction: "" for the default, the name otherwise.
	assert.Equal(t, "", a.selectedPSNFunction())
	a.psnPresetSelect.SetSelected("bootstrap")
	assert.Equal(t, "bootstrap", a.selectedPSNFunction())

	// PsN engine with the Hermes destination derives ExecutionMode=HERMES, but the
	// run is still PsN (the bootstrap saga) → the picker must stay visible so the
	// user can select bootstrap.
	a.config = &config.Config{Input: config.Input{
		Engine:        config.EnginePSN,
		Destination:   config.DestinationHermes,
		Orchestrator:  config.OrchestratorKubernetes,
		ExecutionMode: config.ExecutionModeHERMES,
	}}
	a.refreshPSNPresets()
	assert.False(t, a.psnPresetContainer.Hidden)
	assert.Contains(t, a.psnPresetSelect.Options, "bootstrap")

	// NONMEM engine over Hermes → not PsN, picker hidden.
	a.config = &config.Config{Input: config.Input{
		Engine:        config.EngineNONMEM,
		Destination:   config.DestinationHermes,
		ExecutionMode: config.ExecutionModeHERMES,
	}}
	a.refreshPSNPresets()
	assert.True(t, a.psnPresetContainer.Hidden)
}

func TestIsNonmemModeName(t *testing.T) {
	assert.True(t, isNonmemModeName(config.ExecutionModeNONMEM))
	assert.True(t, isNonmemModeName(config.ExecutionModeBBI))
	assert.True(t, isNonmemModeName(config.ExecutionModePSN))
	assert.False(t, isNonmemModeName(config.ExecutionModeHERMES))
	assert.False(t, isNonmemModeName(""))
}

func TestApplySelectedInstallation(t *testing.T) {
	a := &App{config: &config.Config{Input: config.Input{
		Installations: []config.NonmemInstall{
			{Name: "nm74", Path: "/opt/nm74", Binary: "nmfe74"},
			{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
		},
	}}}
	a.versionSelect = widget.NewSelect([]string{"nm74", "nm75"}, nil)

	a.versionSelect.SetSelected("nm74")
	a.applySelectedInstallation()
	assert.Equal(t, "/opt/nm74", a.config.NonmemPath)
	assert.Equal(t, "nmfe74", a.config.NonmemBinary)

	a.versionSelect.SetSelected("nm75")
	a.applySelectedInstallation()
	assert.Equal(t, "/opt/nm75", a.config.NonmemPath)
	assert.Equal(t, "nmfe75", a.config.NonmemBinary)
}

func TestRefreshTargetOptions(t *testing.T) {
	a := &App{}
	a.targetRadio = widget.NewRadioGroup([]string{"Here"}, nil)

	// NONMEM, no remote → Here + Scheduler.
	a.config = &config.Config{Input: config.Input{ExecutionMode: config.ExecutionModeNONMEM}}
	a.refreshTargetOptions()
	assert.Equal(t, []string{"Here", "Scheduler"}, a.targetRadio.Options)

	// NONMEM + remote host → adds SSH.
	a.config.Remote.Host = "cluster.hpc"
	a.refreshTargetOptions()
	assert.Equal(t, []string{"Here", "Scheduler", "SSH"}, a.targetRadio.Options)

	// PSN + remote → all three.
	a.config = &config.Config{Input: config.Input{ExecutionMode: config.ExecutionModePSN, Remote: config.RemoteConfig{Host: "h"}}}
	a.refreshTargetOptions()
	assert.Equal(t, []string{"Here", "Scheduler", "SSH"}, a.targetRadio.Options)

	// HERMES → Here only; a previously-selected SSH falls back to Here.
	a.targetRadio.SetSelected("SSH")
	a.config = &config.Config{Input: config.Input{ExecutionMode: config.ExecutionModeHERMES}}
	a.refreshTargetOptions()
	assert.Equal(t, []string{"Here"}, a.targetRadio.Options)
	assert.Equal(t, "Here", a.targetRadio.Selected)

	// BBI → Here only (grid is a stub, no remote).
	a.config = &config.Config{Input: config.Input{ExecutionMode: config.ExecutionModeBBI}}
	a.refreshTargetOptions()
	assert.Equal(t, []string{"Here"}, a.targetRadio.Options)
}

func TestApplySelectedInstallationNoOps(t *testing.T) {
	// No version select → no panic, no change.
	a := &App{config: &config.Config{Input: config.Input{NonmemPath: "/keep"}}}
	a.applySelectedInstallation()
	assert.Equal(t, "/keep", a.config.NonmemPath)

	// Nothing selected → no change.
	a.versionSelect = widget.NewSelect([]string{"x"}, nil)
	a.applySelectedInstallation()
	assert.Equal(t, "/keep", a.config.NonmemPath)
}
