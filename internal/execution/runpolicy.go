package execution

import (
	"log"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/runpolicy"
)

// policyFromConfig builds a runpolicy.Policy from configuration. A nil config
// yields the zero (no-op) policy; otherwise it delegates to config.RunPolicy so
// the in-process executors and the headless executor binary share one mapping.
func policyFromConfig(cfg *config.Config) runpolicy.Policy {
	if cfg == nil {
		return runpolicy.Policy{}
	}

	return cfg.RunPolicy()
}

// applyBeforeRun runs the pre-run policy (sequential archiving). Failures are
// logged but never block execution — housekeeping must not fail a run.
func applyBeforeRun(cfg *config.Config, modelPath string) {
	if _, err := policyFromConfig(cfg).BeforeRun(modelPath); err != nil {
		log.Printf("Warning: run-output policy (before run) failed for %s: %v", modelPath, err)
	}
}

// applyAfterRun runs the post-run policy (backup, cleanup). Failures are logged
// but never affect the run's result.
func applyAfterRun(cfg *config.Config, modelPath string) {
	if err := policyFromConfig(cfg).AfterRun(modelPath); err != nil {
		log.Printf("Warning: run-output policy (after run) failed for %s: %v", modelPath, err)
	}
}
