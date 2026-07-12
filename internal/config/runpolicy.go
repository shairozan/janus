package config

import "github.com/pharmalytica/janus/internal/runpolicy"

// RunPolicy builds the run-output policy from configuration, honoring the legacy
// projects_config.auto-backup flag as a fallback for auto-backup. This is the
// single source of truth used by the in-process executors and the headless
// executor binary alike.
func (c Config) RunPolicy() runpolicy.Policy {
	return runpolicy.Policy{
		Overwrite:    c.Runs.OverwritePolicy,
		AutoBackup:   c.Runs.AutoBackup || c.ProjectsConf.AutoBackup,
		CleanupGlobs: c.Runs.CleanupGlobs,
	}
}
