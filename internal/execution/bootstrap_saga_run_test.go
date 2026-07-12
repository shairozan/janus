package execution

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/pharmalytica/janus/internal/config"
)

// fakeStageRunner records every stage it is asked to run and returns scripted
// outcomes, so the saga orchestration can be tested without a cluster.
type fakeStageRunner struct {
	mu         sync.Mutex
	calls      []stagePod
	cleanups   []string // sagaIDs passed to cleanup, in order
	setupFiles map[string][]byte
	failFits   map[int]bool
	failStage  string // when set, runStage returns an error for this stage label
}

func (f *fakeStageRunner) cleanup(_ context.Context, sagaID string) error {
	f.mu.Lock()
	f.cleanups = append(f.cleanups, sagaID)
	f.mu.Unlock()

	return nil
}

func (f *fakeStageRunner) runStage(_ context.Context, p stagePod) (*stageOutcome, error) {
	f.mu.Lock()
	f.calls = append(f.calls, p)
	f.mu.Unlock()

	if f.failStage != "" && p.stageLabel == f.failStage {
		return nil, fmt.Errorf("stage %s forced failure", p.stageLabel)
	}

	switch p.stageLabel {
	case "setup":
		return &stageOutcome{files: f.setupFiles}, nil
	case "fit":
		i := fitIndexFromPrefix(p.namePrefix)
		if f.failFits[i] {
			return nil, fmt.Errorf("fit %d boom", i)
		}

		return &stageOutcome{files: map[string][]byte{
			fitListName(i): []byte("lst"),
			fitExtName(i):  []byte("ext"),
		}}, nil
	case "aggregate":
		return &stageOutcome{outputPaths: []string{"bs/bootstrap_results.csv"}}, nil
	default:
		return nil, fmt.Errorf("unexpected stage %q", p.stageLabel)
	}
}

func (f *fakeStageRunner) byLabel(label string) []stagePod {
	var out []stagePod

	for _, c := range f.calls {
		if c.stageLabel == label {
			out = append(out, c)
		}
	}

	return out
}

func fitIndexFromPrefix(prefix string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(prefix, "bs-fit-"))

	return n
}

func setupFilesFor(indices ...int) map[string][]byte {
	m := map[string][]byte{"bs/meta.yaml": []byte("meta")}

	for _, i := range indices {
		m[m1Path(resampleModelName(i))] = []byte("$PROB")
		m[m1Path(resampleDataName(i))] = []byte("ID,DV")
	}

	return m
}

func newTestSaga(runner stageRunner) *BootstrapSaga {
	return &BootstrapSaga{
		runner:      runner,
		psnImage:    "psn:test",
		execImage:   "nonmem:test",
		commandPath: "/opt/nm/nmfe",
		cpuCores:    2,
		memory:      "4Gi",
		parallelism: 2,
		collectFiles: func(string) (map[string][]byte, error) {
			return map[string][]byte{"acop.mod": []byte("$PROB"), "acop.csv": []byte("ID")}, nil
		},
		getLicense: func() ([]byte, error) { return []byte("LIC"), nil },
	}
}

func TestSweepOrphanedHermesPodsGatedOnNamespace(t *testing.T) {
	// No namespace configured → a no-op that never touches the cluster.
	if err := SweepOrphanedHermesPods(context.Background(), &config.Config{}); err != nil {
		t.Errorf("empty namespace should be a no-op, got %v", err)
	}

	if err := SweepOrphanedHermesPods(context.Background(), nil); err != nil {
		t.Errorf("nil config should be a no-op, got %v", err)
	}
}

func TestBootstrapSagaSweepsPodsOnSuccess(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2)}
	saga := newTestSaga(runner)

	if _, err := saga.Run(context.Background(), "/models/acop.mod", 2); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// A pre-run sweep (clear orphans) and a deferred sweep (safety net), both
	// targeting this model's stable saga id.
	want := shortModelHash("/models/acop.mod")
	if len(runner.cleanups) < 2 {
		t.Fatalf("expected at least 2 sweeps (pre-run + deferred), got %d", len(runner.cleanups))
	}

	for _, id := range runner.cleanups {
		if id != want {
			t.Errorf("sweep sagaID = %q, want %q", id, want)
		}
	}
}

func TestBootstrapSagaSweepsPodsOnStageFailure(t *testing.T) {
	// A failing stage stands in for a cancellation: Run returns early with an
	// error, and the deferred sweep must still tear everything down.
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2), failStage: "fit"}
	saga := newTestSaga(runner)

	if _, err := saga.Run(context.Background(), "/models/acop.mod", 2); err == nil {
		t.Fatal("expected Run to fail when every fit errors")
	}

	// Pre-run sweep + deferred sweep still ran despite the failure.
	if len(runner.cleanups) < 2 {
		t.Errorf("expected the deferred sweep to run on the error path, got %d sweeps", len(runner.cleanups))
	}
}

func TestBootstrapSagaProgress(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2, 3), failFits: map[int]bool{2: true}}
	saga := newTestSaga(runner)

	var mu sync.Mutex
	var snaps []SagaProgress
	saga.SetProgressFunc(func(p SagaProgress) {
		mu.Lock()
		snaps = append(snaps, p)
		mu.Unlock()
	})

	if _, err := saga.Run(context.Background(), "/models/acop.mod", 3); err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(snaps) == 0 {
		t.Fatal("no progress snapshots emitted")
	}

	// The fits stage should have been announced with the full sample total.
	sawFitsTotal := false
	maxDone := 0
	for _, s := range snaps {
		if s.Stage == SagaStageFits && s.Total == 3 {
			sawFitsTotal = true
		}

		if s.Done > maxDone {
			maxDone = s.Done
		}
	}

	if !sawFitsTotal {
		t.Error("expected a fits-stage snapshot with Total=3")
	}

	if maxDone != 3 {
		t.Errorf("expected all 3 fits to report done, got max Done=%d", maxDone)
	}

	// The final snapshot is the aggregate pod finishing: all pods torn down, and
	// the provisional tally reflects the one failed fit.
	last := snaps[len(snaps)-1]
	if last.Stage != SagaStageAggregate {
		t.Errorf("final stage = %q, want %q", last.Stage, SagaStageAggregate)
	}

	if len(last.Live) != 0 {
		t.Errorf("expected no live pods at the end, got %d", len(last.Live))
	}

	if last.Succeeded != 2 || last.Failed != 1 {
		t.Errorf("final tally: succeeded=%d failed=%d, want 2/1", last.Succeeded, last.Failed)
	}
}

func TestBootstrapSagaHappyPath(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2, 3)}
	saga := newTestSaga(runner)

	res, err := saga.Run(context.Background(), "/models/acop.mod", 3)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Samples != 3 || res.Succeeded != 3 || res.Failed != 0 {
		t.Errorf("result counts: samples=%d ok=%d fail=%d", res.Samples, res.Succeeded, res.Failed)
	}

	// One setup, three fits, one aggregate.
	if got := len(runner.byLabel("setup")); got != 1 {
		t.Errorf("setup calls = %d", got)
	}

	if got := len(runner.byLabel("fit")); got != 3 {
		t.Errorf("fit calls = %d", got)
	}

	if got := len(runner.byLabel("aggregate")); got != 1 {
		t.Errorf("aggregate calls = %d", got)
	}

	// Setup/aggregate use the PsN image; fits use the execution image.
	if runner.byLabel("setup")[0].image != "psn:test" || runner.byLabel("aggregate")[0].image != "psn:test" {
		t.Error("setup/aggregate should use the PsN image")
	}

	for _, fit := range runner.byLabel("fit") {
		if fit.image != "nonmem:test" {
			t.Errorf("fit should use execution image, got %q", fit.image)
		}

		if string(fit.stage.license) != "LIC" {
			t.Error("fit stage should carry the license")
		}
	}

	// Order: setup is first, aggregate is last.
	if runner.calls[0].stageLabel != "setup" || runner.calls[len(runner.calls)-1].stageLabel != "aggregate" {
		t.Errorf("stage order wrong: %v", labelsOf(runner.calls))
	}

	// Aggregate workspace merged the fit outputs under bs/m1.
	agg := runner.byLabel("aggregate")[0]
	for _, i := range []int{1, 2, 3} {
		if _, ok := agg.stage.files[m1Path(fitListName(i))]; !ok {
			t.Errorf("aggregate workspace missing %s", m1Path(fitListName(i)))
		}
	}
}

func TestBootstrapSagaNoResamples(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: map[string][]byte{"bs/meta.yaml": []byte("x")}}
	saga := newTestSaga(runner)

	_, err := saga.Run(context.Background(), "/models/acop.mod", 5)
	if err == nil {
		t.Fatal("expected error when setup produced no resamples")
	}

	if len(runner.byLabel("fit")) != 0 || len(runner.byLabel("aggregate")) != 0 {
		t.Error("no fit/aggregate stages should run when setup is empty")
	}
}

func TestBootstrapSagaFitFailureIsolation(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2, 3), failFits: map[int]bool{2: true}}
	saga := newTestSaga(runner)

	res, err := saga.Run(context.Background(), "/models/acop.mod", 3)
	if err != nil {
		t.Fatalf("a single fit failure should not fail the saga: %v", err)
	}

	if res.Succeeded != 2 || res.Failed != 1 {
		t.Errorf("counts: ok=%d fail=%d", res.Succeeded, res.Failed)
	}

	if _, ok := res.FitErrors[2]; !ok {
		t.Error("expected FitErrors[2]")
	}

	// Aggregate still ran, and excludes the failed sample's outputs.
	agg := runner.byLabel("aggregate")[0]
	if _, ok := agg.stage.files[m1Path(fitListName(2))]; ok {
		t.Error("failed fit should not be in the aggregate workspace")
	}

	if _, ok := agg.stage.files[m1Path(fitListName(1))]; !ok {
		t.Error("successful fit should be in the aggregate workspace")
	}
}

func TestBootstrapSagaAllFitsFail(t *testing.T) {
	runner := &fakeStageRunner{setupFiles: setupFilesFor(1, 2), failFits: map[int]bool{1: true, 2: true}}
	saga := newTestSaga(runner)

	_, err := saga.Run(context.Background(), "/models/acop.mod", 2)
	if err == nil {
		t.Fatal("expected error when all fits fail")
	}

	if len(runner.byLabel("aggregate")) != 0 {
		t.Error("aggregate should not run when no fits succeeded")
	}
}

func TestBootstrapSagaRejectsNonPositiveSamples(t *testing.T) {
	saga := newTestSaga(&fakeStageRunner{})
	if _, err := saga.Run(context.Background(), "/models/acop.mod", 0); err == nil {
		t.Fatal("expected error for samples=0")
	}
}

func axesCfg(engine, dest, orch string) *config.Config {
	c := &config.Config{}
	c.Engine = engine
	c.Destination = dest
	c.Orchestrator = orch

	return c
}

func TestIsBootstrapSagaRun(t *testing.T) {
	saga := axesCfg(config.EnginePSN, config.DestinationHermes, config.OrchestratorKubernetes)

	tests := []struct {
		name string
		cfg  *config.Config
		fn   string
		want bool
	}{
		{name: "all axes match", cfg: saga, fn: "bootstrap", want: true},
		{name: "wrong function", cfg: saga, fn: "vpc", want: false},
		{name: "wrong engine", cfg: axesCfg(config.EngineNONMEM, config.DestinationHermes, config.OrchestratorKubernetes), fn: "bootstrap", want: false},
		{name: "wrong destination", cfg: axesCfg(config.EnginePSN, config.DestinationHere, config.OrchestratorKubernetes), fn: "bootstrap", want: false},
		{name: "wrong orchestrator", cfg: axesCfg(config.EnginePSN, config.DestinationHermes, config.OrchestratorDocker), fn: "bootstrap", want: false},
		{name: "nil config", cfg: nil, fn: "bootstrap", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBootstrapSagaRun(tt.cfg, tt.fn); got != tt.want {
				t.Errorf("IsBootstrapSagaRun = %v, want %v", got, tt.want)
			}
		})
	}
}

func labelsOf(calls []stagePod) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.stageLabel
	}

	return out
}
