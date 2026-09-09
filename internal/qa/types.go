// Package qa holds runtime IQ/OQ qualification logic for Janus.
//
// It is the dependency-injected core shared by every caller (GUI, executor,
// cobra): callers load configuration, build an execution.Executor via the same
// factory used for real runs, and hand both down. This package never reads
// viper, never calls os.UserHomeDir, and never constructs an executor — it
// respects the repo's orthogonality rule (build deps at the top layer, hand
// them down).
package qa

import (
	"context"
	"time"

	"github.com/shairozan/janus/internal/model"
	"github.com/shairozan/janus/internal/summary"
)

// Status is the outcome of a single check or an overall qualification result.
type Status string

const (
	// StatusPass indicates the check (or overall run) succeeded.
	StatusPass Status = "pass"
	// StatusFail indicates the check (or overall run) failed.
	StatusFail Status = "fail"
	// StatusSkip indicates the check was not run (e.g. not applicable to mode).
	StatusSkip Status = "skip"
)

// Kind identifies a qualification report kind. It doubles as the on-disk
// filename stem (iq.json / oq.json).
type Kind string

const (
	// KindIQ is the Installation Qualification report kind.
	KindIQ Kind = "iq"
	// KindOQ is the Operational Qualification report kind.
	KindOQ Kind = "oq"
)

// OQ phases recorded on an OQResult.
const (
	// PhasePreflight is recorded when OQ fails during phase-1 config validity.
	PhasePreflight = "preflight"
	// PhaseFunctional is recorded once OQ reaches the phase-2 ACOP run.
	PhaseFunctional = "functional"
)

// Check is a single qualification check and its outcome. The shape mirrors the
// build-tagged validation suite's JSON so reports are consistent across both.
type Check struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Reason   string `json:"reason,omitempty"`
	Category string `json:"category,omitempty"`
}

// Options carries cross-cutting, injectable knobs for a qualification run.
// Every field is optional; the zero value is usable.
type Options struct {
	// Now supplies the current time. If nil, time.Now is used. Tests inject a
	// fixed clock for deterministic timestamps.
	Now func() time.Time

	// ExecutorProbe reports the bundled executor's version string (typically by
	// running `executor --executor-version`). It is injected by the caller so
	// internal/qa never has to locate or construct the executor itself. If nil,
	// the IQ executor_component check is skipped rather than failed.
	ExecutorProbe func(ctx context.Context) (version string, err error)
}

// now returns the configured clock, defaulting to time.Now.
func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}

	return time.Now()
}

// Summarizer parses a completed model run into a ModelSummary. The real
// implementation is *summary.NONMEMSummarizer; tests supply a fake. Defining it
// here lets OQ depend on a narrow interface rather than the concrete type.
type Summarizer interface {
	SummarizeModel(ctx context.Context, modelPath string, options summary.SummaryOptions) (*model.ModelSummary, error)
}

// IQResult is the Installation Qualification report — a Janus tool self-check.
type IQResult struct {
	Kind         Kind      `json:"kind"`
	Status       Status    `json:"status"`
	Checks       []Check   `json:"checks"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	RunID        string    `json:"run_id,omitempty"`
	JanusVersion string    `json:"janus_version,omitempty"`
	User         string    `json:"user,omitempty"`
}

// OQResult is the Operational Qualification report — phase-1 config validity
// plus, when that passes, a phase-2 functional ACOP run with an OFV gate.
type OQResult struct {
	Kind        Kind      `json:"kind"`
	Status      Status    `json:"status"`
	Phase       string    `json:"phase,omitempty"`
	Checks      []Check   `json:"checks"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`

	// Functional (phase-2) details. Pointers are nil when phase 2 did not run.
	NonmemExitCode *int     `json:"nonmem_exit_code,omitempty"`
	ObservedOFV    *float64 `json:"observed_ofv,omitempty"`
	ReferenceOFV   float64  `json:"reference_ofv"`
	AbsTolerance   float64  `json:"abs_tolerance"`
	RelTolerance   float64  `json:"rel_tolerance"`
	Artifacts      []string `json:"artifacts,omitempty"`

	RunID        string `json:"run_id,omitempty"`
	JanusVersion string `json:"janus_version,omitempty"`
	User         string `json:"user,omitempty"`
}

// overallStatus folds a set of checks into a single outcome: fail if any check
// failed, otherwise pass. Skips do not fail the run.
func overallStatus(checks []Check) Status {
	for _, c := range checks {
		if c.Status == StatusFail {
			return StatusFail
		}
	}

	return StatusPass
}
