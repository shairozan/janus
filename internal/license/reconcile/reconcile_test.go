package reconcile

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"testing"
	"time"
)

type countingJob struct {
	name  string
	runs  atomic.Int64
	err   error
	ranCh chan struct{}
}

func (j *countingJob) Name() string { return j.name }

func (j *countingJob) Reconcile(_ context.Context) error {
	j.runs.Add(1)

	if j.ranCh != nil {
		select {
		case j.ranCh <- struct{}{}:
		default:
		}
	}

	return j.err
}

func TestRunOnce_RunsAllJobsAndContinuesOnError(t *testing.T) {
	a := &countingJob{name: "a", err: errors.New("boom")}
	b := &countingJob{name: "b"}

	r := NewRunner(time.Minute, log.Default())
	r.Register(a)
	r.Register(b)

	r.RunOnce(context.Background())

	if a.runs.Load() != 1 {
		t.Fatalf("job a runs = %d, want 1", a.runs.Load())
	}

	if b.runs.Load() != 1 {
		t.Fatalf("job b runs = %d, want 1 (an error in a must not stop b)", b.runs.Load())
	}
}

func TestStart_RunsUntilContextCanceled(t *testing.T) {
	job := &countingJob{name: "j", ranCh: make(chan struct{}, 1)}
	r := NewRunner(5*time.Millisecond, log.Default())
	r.Register(job)

	ctx, cancel := context.WithCancel(context.Background())
	r.Start(ctx)

	select {
	case <-job.ranCh:
		// ran at least once
	case <-time.After(2 * time.Second):
		t.Fatal("job did not run within timeout")
	}

	cancel()
}

func TestKey_DeterministicAndDistinct(t *testing.T) {
	k1 := Key("proposal", "42", "create-subscription")
	k2 := Key("proposal", "42", "create-subscription")
	k3 := Key("proposal", "43", "create-subscription")

	if k1 != k2 {
		t.Fatal("Key must be deterministic for identical inputs")
	}

	if k1 == k3 {
		t.Fatal("Key must differ for different inputs")
	}

	if len(k1) != 64 {
		t.Fatalf("Key length = %d, want 64 (sha256 hex)", len(k1))
	}
}
