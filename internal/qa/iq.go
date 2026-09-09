package qa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shairozan/janus/internal/config"
)

// categoryInstallation tags IQ checks for report grouping.
const categoryInstallation = "installation"

// RunIQ performs the Installation Qualification: a lightweight self-check that
// Janus itself is installed and operable. It deliberately does NOT probe NONMEM,
// the license, Hermes, or docker — those belong to OQ phase 1.
//
// cfg is the configuration the caller already loaded; a nil cfg signals a load
// failure. runDir is the freshly created qa/<uuid> directory — its grandparent
// is treated as the Janus config directory, so this package never has to call
// os.UserHomeDir itself.
//
// IQ records every outcome as a Check and never returns an error: a failed
// self-check is data, not a transport error.
func RunIQ(cfg *config.Config, runDir string, opts Options) *IQResult {
	result := &IQResult{
		Kind:      KindIQ,
		StartedAt: opts.now(),
	}

	// config_loads — caller hands in a loaded config, or nil on load failure.
	if cfg == nil {
		result.Checks = append(result.Checks, Check{
			Name:     "config_loads",
			Status:   StatusFail,
			Reason:   "configuration failed to load",
			Category: categoryInstallation,
		})
	} else {
		result.Checks = append(result.Checks, Check{
			Name:     "config_loads",
			Status:   StatusPass,
			Category: categoryInstallation,
		})
		result.JanusVersion = cfg.Version
		result.User = cfg.User
	}

	// janus_build_info — version populated by build ldflags.
	buildInfo := Check{Name: "janus_build_info", Category: categoryInstallation}
	if cfg != nil && cfg.Version != "" {
		buildInfo.Status = StatusPass
		buildInfo.Reason = fmt.Sprintf("version %s", cfg.Version)
	} else {
		buildInfo.Status = StatusFail
		buildInfo.Reason = "build version not set (ldflags missing)"
	}

	result.Checks = append(result.Checks, buildInfo)

	// config_dir_writable — the config dir is the grandparent of the run dir
	// (qa/<uuid>), keeping this package free of environment lookups.
	configDir := filepath.Dir(filepath.Dir(runDir))
	result.Checks = append(result.Checks, writableCheck("config_dir_writable", configDir))

	// qa_dir_writable — the run dir itself.
	result.Checks = append(result.Checks, writableCheck("qa_dir_writable", runDir))

	// executor_component — the bundled executor is locatable/runnable, via the
	// caller-injected probe (Janus delegates execution to it).
	result.Checks = append(result.Checks, executorCheck(opts))

	result.Status = overallStatus(result.Checks)
	result.CompletedAt = opts.now()

	return result
}

// writableCheck verifies dir exists, is a directory, and accepts a temp file.
func writableCheck(name, dir string) Check {
	c := Check{Name: name, Category: categoryInstallation}

	info, err := os.Stat(dir)
	switch {
	case err != nil:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s: %v", dir, err)
	case !info.IsDir():
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s is not a directory", dir)
	case probeWritable(dir) != nil:
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("%s is not writable", dir)
	default:
		c.Status = StatusPass
		c.Reason = dir
	}

	return c
}

// executorCheck runs the injected executor probe. A nil probe is a skip (the
// caller did not wire one), not a failure.
func executorCheck(opts Options) Check {
	c := Check{Name: "executor_component", Category: categoryInstallation}

	if opts.ExecutorProbe == nil {
		c.Status = StatusSkip
		c.Reason = "no executor probe provided"

		return c
	}

	version, err := opts.ExecutorProbe(context.Background())
	if err != nil {
		c.Status = StatusFail
		c.Reason = fmt.Sprintf("executor not runnable: %v", err)

		return c
	}

	c.Status = StatusPass
	c.Reason = version

	return c
}

// probeWritable confirms dir accepts a file by creating and removing a temp file.
func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".qa-write-probe-*")
	if err != nil {
		return err
	}

	name := f.Name()
	if cerr := f.Close(); cerr != nil {
		os.Remove(name)

		return cerr
	}

	return os.Remove(name)
}
