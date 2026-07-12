package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunPolicyFromConfig(t *testing.T) {
	c := Config{Input: Input{Runs: RunsConfig{
		OverwritePolicy: "sequential",
		CleanupGlobs:    []string{"FDATA", "FCON"},
	}}}

	p := c.RunPolicy()
	assert.Equal(t, "sequential", p.Overwrite)
	assert.Equal(t, []string{"FDATA", "FCON"}, p.CleanupGlobs)
	assert.False(t, p.AutoBackup)
}

func TestRunPolicyHonorsLegacyAutoBackup(t *testing.T) {
	// The legacy projects_config.auto-backup flag enables backup.
	c := Config{Input: Input{ProjectsConf: ProjectsConfig{AutoBackup: true}}}
	assert.True(t, c.RunPolicy().AutoBackup)

	// runs.auto_backup also enables it.
	c2 := Config{Input: Input{Runs: RunsConfig{AutoBackup: true}}}
	assert.True(t, c2.RunPolicy().AutoBackup)
}
