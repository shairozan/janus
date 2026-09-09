package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/scheduler"
)

func profileEditor() *recordListEditor {
	return newRecordListEditor("Add", false,
		recordColumn{}, recordColumn{}, recordColumn{}, recordColumn{})
}

func TestSchedulerProfilesMergePreservesUneditedFields(t *testing.T) {
	orig := []scheduler.Profile{{
		Name:          "slurm",
		Submit:        scheduler.CommandSpec{Command: "sbatch", Args: []string{"--parsable"}},
		Status:        scheduler.CommandSpec{Command: "squeue"},
		Cancel:        scheduler.CommandSpec{Command: "scancel"},
		StateMap:      map[string]string{"R": "running"},
		StatusPattern: `job_state=(?P<state>\w+)`,
	}}

	s := &SettingsDialog{schedulerProfiles: orig}
	s.schedulerProfilesEditor = profileEditor()
	// Edit only the submit command.
	s.schedulerProfilesEditor.addRow(recordValue{Fields: []string{"slurm", "sbatch --hold", "squeue", "scancel"}})

	got := s.schedulerProfilesMerged()
	require.Len(t, got, 1)
	assert.Equal(t, "sbatch --hold", got[0].Submit.Command)
	// Fields the GUI doesn't edit survive the round-trip.
	assert.Equal(t, []string{"--parsable"}, got[0].Submit.Args)
	assert.Equal(t, map[string]string{"R": "running"}, got[0].StateMap)
	assert.Equal(t, `job_state=(?P<state>\w+)`, got[0].StatusPattern)
}

func TestSchedulerProfilesMergeAddsAndDrops(t *testing.T) {
	// One retained original; the editor keeps a new one and drops the original.
	s := &SettingsDialog{schedulerProfiles: []scheduler.Profile{{Name: "old"}}}
	s.schedulerProfilesEditor = profileEditor()
	s.schedulerProfilesEditor.addRow(recordValue{Fields: []string{"custom", "qsub", "qstat", "qdel"}})

	got := s.schedulerProfilesMerged()
	require.Len(t, got, 1)
	assert.Equal(t, "custom", got[0].Name)
	assert.Equal(t, "qsub", got[0].Submit.Command)
}

func TestTypedReaders(t *testing.T) {
	s := &SettingsDialog{}

	s.remoteMountsEditor = newRecordListEditor("Add", false, recordColumn{}, recordColumn{})
	s.remoteMountsEditor.addRow(recordValue{Fields: []string{"C:\\m", "/m"}})
	assert.Equal(t, []config.RemoteMount{{Local: "C:\\m", Remote: "/m"}}, s.remoteMounts())

	s.psnPresetsEditor = newRecordListEditor("Add", false, recordColumn{}, recordColumn{}, recordColumn{})
	s.psnPresetsEditor.addRow(recordValue{Fields: []string{"vpc", "vpc", "-samples=200 -auto"}})
	presets := s.psnPresets()
	require.Len(t, presets, 1)
	assert.Equal(t, "vpc", presets[0].Name)
	assert.Equal(t, []string{"-samples=200", "-auto"}, presets[0].Args)

	s.hooksEditor = newRecordListEditor("Add", false, recordColumn{}, recordColumn{}, recordColumn{})
	s.hooksEditor.addRow(recordValue{Fields: []string{"gof", "post", "gof.R"}})
	assert.Equal(t, []config.IntegrationHook{{Name: "gof", When: "post", Script: "gof.R"}}, s.integrationHooks())

	s.toolsEditor = newRecordListEditor("Add", false, recordColumn{}, recordColumn{})
	s.toolsEditor.addRow(recordValue{Fields: []string{"Stan", "/opt/stan"}})
	assert.Equal(t, []config.IntegrationTool{{Name: "Stan", Path: "/opt/stan"}}, s.integrationTools())
}

func TestSchedulerOptions(t *testing.T) {
	// nil config → built-ins only.
	assert.Equal(t, []string{"LOCAL", "SLURM", "SGE", "TORQUE", "PBS"}, schedulerOptions(nil))

	// Custom profiles append; blanks skip; built-in names dedup case-insensitively.
	cfg := &config.Config{Input: config.Input{Schedulers: []scheduler.Profile{
		{Name: "bigmem-slurm"}, {Name: "slurm"}, {Name: ""}, {Name: "gpu-sge"},
	}}}
	assert.Equal(t,
		[]string{"LOCAL", "SLURM", "SGE", "TORQUE", "PBS", "bigmem-slurm", "gpu-sge"},
		schedulerOptions(cfg))
}

func TestParsePortOrZero(t *testing.T) {
	assert.Equal(t, 2222, parsePortOrZero("2222"))
	assert.Equal(t, 0, parsePortOrZero(""))
	assert.Equal(t, 0, parsePortOrZero("notaport"))
	assert.Equal(t, 22, parsePortOrZero("  22 "))
}
