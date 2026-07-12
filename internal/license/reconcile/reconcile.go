// Package reconcile provides the scaffolding for the management portal's
// multi-system consistency strategy (a pragmatic saga, not distributed
// transactions). The pattern, applied by the signup/Cognito/billing flows:
//
//  1. Intent-first: write the local row (status=pending) in a DB transaction,
//     THEN call the external system (Stripe/Cognito), THEN flip the status.
//  2. Idempotency: pass a deterministic Key (below) to every external call so a
//     retry after a crash does not double-create.
//  3. Reconciler: a periodic Runner sweeps rows stuck in a pending state past a
//     threshold, queries the real external state, and advances or flags them —
//     preferring forward-completion over rollback when the orphan is harmless.
//
// The Runner is safe for the single-replica license-server (no leader election).
package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strings"
	"sync"
	"time"
)

// Job reconciles records stuck in a pending/provisioning state for one subsystem
// (e.g. Cognito IdP provisioning, Stripe subscription creation).
type Job interface {
	// Name identifies the job in logs.
	Name() string
	// Reconcile performs one reconciliation pass. It must be idempotent.
	Reconcile(ctx context.Context) error
}

// Runner periodically runs registered reconcile jobs.
type Runner struct {
	mu       sync.Mutex
	jobs     []Job
	interval time.Duration
	logger   *log.Logger
}

// NewRunner creates a Runner that runs its jobs every interval.
func NewRunner(interval time.Duration, logger *log.Logger) *Runner {
	return &Runner{interval: interval, logger: logger}
}

// Register adds a job to the runner.
func (r *Runner) Register(j Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, j)
}

// RunOnce runs every registered job once. A job error is logged and does not
// stop the other jobs (each subsystem reconciles independently).
func (r *Runner) RunOnce(ctx context.Context) {
	r.mu.Lock()
	jobs := make([]Job, len(r.jobs))
	copy(jobs, r.jobs)
	r.mu.Unlock()

	for _, j := range jobs {
		if err := j.Reconcile(ctx); err != nil {
			r.logger.Printf("reconcile job %q failed: %v", j.Name(), err)
		}
	}
}

// Start runs all jobs every interval in a background goroutine until ctx is done.
func (r *Runner) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunOnce(ctx)
			}
		}
	}()
}

// Key builds a deterministic idempotency key from a record's identity, suitable
// for a Stripe idempotency key or a Cognito client token, so a retry reuses the
// same key instead of double-creating. The result is a fixed-length hex digest.
func Key(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, ":")))

	return hex.EncodeToString(sum[:])
}
