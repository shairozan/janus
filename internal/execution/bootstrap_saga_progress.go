package execution

import (
	"sort"
	"time"
)

// Saga stage identifiers for live progress (SagaProgress.Stage).
const (
	SagaStageSetup     = "setup"
	SagaStageFits      = "fits"
	SagaStageAggregate = "aggregate"
)

// Pod-name prefixes for each saga stage. Kept as constants so the live-progress
// pod names always match the names kubeStageRunner provisions (both derive the
// final name via sagaPodName(prefix, sagaID)).
const (
	bsSetupPodPrefix     = "bs-setup"
	bsAggregatePodPrefix = "bs-aggregate"
	bsFitPodPrefixFmt    = "bs-fit-%d"
)

// SagaPod is a pod the saga currently has provisioned and is driving — live from
// provision until teardown.
type SagaPod struct {
	Name  string
	Fit   int       // 1-based fit index; 0 for the setup/aggregate pods
	Since time.Time // when the saga started driving it
}

// SagaProgress is a point-in-time snapshot of a running bootstrap saga, for live
// display. Live is sorted oldest-first. Succeeded/Failed are provisional during
// the run (a fit that returns without producing a list file is reconciled to a
// failure at aggregation); the final counts come from BootstrapSagaResult.
type SagaProgress struct {
	Stage     string
	Total     int // total fits (0 until the fits stage begins)
	Done      int // fits finished (succeeded + failed)
	Succeeded int
	Failed    int
	Live      []SagaPod
}

// SetProgressFunc registers a callback invoked with a fresh snapshot whenever the
// saga's progress changes (stage transition, pod start/stop, fit completion). The
// callback may be invoked concurrently from fit goroutines, so it must be safe to
// call from any goroutine and must not block. Passing nil disables reporting.
func (s *BootstrapSaga) SetProgressFunc(fn func(SagaProgress)) {
	s.progressMu.Lock()
	s.progressFn = fn
	s.progressMu.Unlock()
}

// setStage records the current stage (and, for the fits stage, the total) and
// emits a snapshot.
func (s *BootstrapSaga) setStage(stage string, total int) {
	s.progressMu.Lock()
	s.prog.Stage = stage
	if total > 0 {
		s.prog.Total = total
	}
	snap, fn := s.snapshotLocked()
	s.progressMu.Unlock()

	if fn != nil {
		fn(snap)
	}
}

// podStarted marks a pod live and emits a snapshot.
func (s *BootstrapSaga) podStarted(name string, fit int, since time.Time) {
	s.progressMu.Lock()
	if s.livePods == nil {
		s.livePods = make(map[string]SagaPod)
	}
	s.livePods[name] = SagaPod{Name: name, Fit: fit, Since: since}
	snap, fn := s.snapshotLocked()
	s.progressMu.Unlock()

	if fn != nil {
		fn(snap)
	}
}

// podFinished removes a live pod. For a fit pod it advances the done counter and
// the provisional succeeded/failed tally. It then emits a snapshot.
func (s *BootstrapSaga) podFinished(name string, isFit, ok bool) {
	s.progressMu.Lock()
	delete(s.livePods, name)
	if isFit {
		s.prog.Done++
		if ok {
			s.prog.Succeeded++
		} else {
			s.prog.Failed++
		}
	}
	snap, fn := s.snapshotLocked()
	s.progressMu.Unlock()

	if fn != nil {
		fn(snap)
	}
}

// snapshotLocked builds an independent SagaProgress copy (Live materialized and
// sorted oldest-first) and returns it alongside the current callback. The caller
// must hold progressMu and invoke the callback after releasing it.
func (s *BootstrapSaga) snapshotLocked() (SagaProgress, func(SagaProgress)) {
	snap := s.prog
	snap.Live = make([]SagaPod, 0, len(s.livePods))
	for _, p := range s.livePods {
		snap.Live = append(snap.Live, p)
	}

	sort.Slice(snap.Live, func(i, j int) bool {
		return snap.Live[i].Since.Before(snap.Live[j].Since)
	})

	return snap, s.progressFn
}
