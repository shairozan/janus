package qa

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution"
	"github.com/shairozan/janus/internal/model"
	"github.com/shairozan/janus/internal/summary"
)

func mockLstPath() string {
	return filepath.Join("..", "..", "testdata", "mock-nonmem", "acop.lst")
}

func floatPtr(v float64) *float64 { return &v }

// provideFake wraps a fakeExecutor as an ExecutorProvider.
func provideFake(fe *fakeExecutor) ExecutorProvider {
	return func(_ string) (execution.Executor, error) { return fe, nil }
}

// fakeExecutor implements execution.Executor. When lstSource is set it copies
// that file to <model>.lst in the run dir (mimicking a real NONMEM run), then
// returns exitCode/err.
type fakeExecutor struct {
	calls     int
	exitCode  int
	err       error
	lstSource string
}

func (f *fakeExecutor) Execute(_ context.Context, modelPath string, _ bool, _ int, _ bool, _ []string) (*execution.ExecutionResult, error) {
	f.calls++

	if f.lstSource != "" {
		data, err := os.ReadFile(f.lstSource)
		if err != nil {
			return nil, err
		}

		dst := strings.TrimSuffix(modelPath, filepath.Ext(modelPath)) + ".lst"
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return nil, err
		}
	}

	if f.err != nil {
		return nil, f.err
	}

	return &execution.ExecutionResult{ExitCode: f.exitCode, Stdout: []byte("ok"), Stderr: []byte{}}, nil
}

// fakeSummarizer implements qa.Summarizer with a chosen OFV / minimization flag.
type fakeSummarizer struct {
	ofv       *float64
	minimized bool
	err       error
	calls     int
}

func (f *fakeSummarizer) SummarizeModel(_ context.Context, _ string, _ summary.SummaryOptions) (*model.ModelSummary, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	return &model.ModelSummary{
		GoodnessOfFit: model.GoodnessOfFitSummary{ObjectiveFunctionValue: f.ofv},
		Estimation:    model.EstimationSummary{Minimized: f.minimized},
	}, nil
}

func newOQ() *OQResult {
	return &OQResult{
		Kind:         KindOQ,
		ReferenceOFV: ReferenceOFV,
		AbsTolerance: AbsTolerance,
		RelTolerance: RelTolerance,
	}
}

func TestRunFunctional_RealSummarizer_AllGatesPass(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{lstSource: mockLstPath()}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), summary.NewNONMEMSummarizer(), runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := overallStatus(result.Checks); got != StatusPass {
		t.Fatalf("overall = %q, want pass; checks=%+v", got, result.Checks)
	}

	if result.ObservedOFV == nil || !withinTolerance(*result.ObservedOFV, ReferenceOFV) {
		t.Fatalf("ObservedOFV = %v, want within tolerance of %v", result.ObservedOFV, ReferenceOFV)
	}

	for _, name := range []string{"nonmem_execution", "ofv_present", "ofv_within_tolerance", "minimization_successful"} {
		if got := findCheck(t, result.Checks, name); got.Status != StatusPass {
			t.Fatalf("gate %s = %q, want pass", name, got.Status)
		}
	}

	// Artifacts recorded for the audit trail.
	for _, want := range []string{acopModelName, acopDataName, "acop.lst", "stdout.txt"} {
		if !containsArtifact(result.Artifacts, want) {
			t.Fatalf("artifact %q missing from %v", want, result.Artifacts)
		}
	}
}

func TestRunFunctional_NonZeroExit_FailsAndSkipsSummarize(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{exitCode: 1, lstSource: mockLstPath()}
	sum := &fakeSummarizer{ofv: floatPtr(ReferenceOFV), minimized: true}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := findCheck(t, result.Checks, "nonmem_execution"); got.Status != StatusFail {
		t.Fatalf("nonmem_execution = %q, want fail", got.Status)
	}

	if sum.calls != 0 {
		t.Fatalf("summarizer called %d times, want 0 (gate A failed)", sum.calls)
	}

	if result.NonmemExitCode == nil || *result.NonmemExitCode != 1 {
		t.Fatalf("NonmemExitCode = %v, want 1", result.NonmemExitCode)
	}
}

func TestRunFunctional_MissingLst_Fails(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{exitCode: 0} // no lstSource -> no .lst produced
	sum := &fakeSummarizer{ofv: floatPtr(ReferenceOFV), minimized: true}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := findCheck(t, result.Checks, "nonmem_execution"); got.Status != StatusFail {
		t.Fatalf("nonmem_execution = %q, want fail", got.Status)
	}

	if sum.calls != 0 {
		t.Fatalf("summarizer called %d times, want 0", sum.calls)
	}
}

func TestRunFunctional_ExecuteError_Fails(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{err: errors.New("boom")}
	sum := &fakeSummarizer{}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	got := findCheck(t, result.Checks, "nonmem_execution")
	if got.Status != StatusFail || !strings.Contains(got.Reason, "execution error") {
		t.Fatalf("nonmem_execution = %+v, want fail with execution error", got)
	}
}

func TestRunFunctional_OFVOutsideBand_Fails(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{lstSource: mockLstPath()}
	sum := &fakeSummarizer{ofv: floatPtr(9999.0), minimized: true}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := findCheck(t, result.Checks, "ofv_present"); got.Status != StatusPass {
		t.Fatalf("ofv_present = %q, want pass", got.Status)
	}

	if got := findCheck(t, result.Checks, "ofv_within_tolerance"); got.Status != StatusFail {
		t.Fatalf("ofv_within_tolerance = %q, want fail", got.Status)
	}
}

func TestRunFunctional_NilOFV_Fails(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{lstSource: mockLstPath()}
	sum := &fakeSummarizer{ofv: nil, minimized: true}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := findCheck(t, result.Checks, "ofv_present"); got.Status != StatusFail {
		t.Fatalf("ofv_present = %q, want fail", got.Status)
	}

	// Tolerance gate should not be evaluated when OFV is absent.
	for _, c := range result.Checks {
		if c.Name == "ofv_within_tolerance" {
			t.Fatalf("ofv_within_tolerance should not be present when OFV is nil")
		}
	}
}

func TestRunFunctional_MinimizationFailed_Fails(t *testing.T) {
	runDir := newRunDir(t)
	fe := &fakeExecutor{lstSource: mockLstPath()}
	sum := &fakeSummarizer{ofv: floatPtr(ReferenceOFV), minimized: false}

	result := newOQ()
	if err := runFunctional(context.Background(), provideFake(fe), sum, runDir, result); err != nil {
		t.Fatalf("runFunctional: %v", err)
	}

	if got := findCheck(t, result.Checks, "minimization_successful"); got.Status != StatusFail {
		t.Fatalf("minimization_successful = %q, want fail", got.Status)
	}

	if overallStatus(result.Checks) != StatusFail {
		t.Fatalf("overall = pass, want fail (minimization gate)")
	}
}

func TestRunOQ_Phase1Fail_SkipsExecutor(t *testing.T) {
	runDir := newRunDir(t)

	cfg := &config.Config{}
	cfg.ExecutionMode = config.ExecutionModeNONMEM // local mode, but no binary configured

	fe := &fakeExecutor{}

	res, err := RunOQ(context.Background(), cfg, provideFake(fe), summary.NewNONMEMSummarizer(), runDir, Options{Now: fixedClock()})
	if err != nil {
		t.Fatalf("RunOQ: %v", err)
	}

	if res.Status != StatusFail {
		t.Fatalf("status = %q, want fail", res.Status)
	}

	if res.Phase != PhasePreflight {
		t.Fatalf("phase = %q, want preflight", res.Phase)
	}

	if fe.calls != 0 {
		t.Fatalf("executor called %d times on phase-1 failure, want 0", fe.calls)
	}

	if got := findCheck(t, res.Checks, "nonmem_binary"); got.Status != StatusFail {
		t.Fatalf("nonmem_binary = %q, want fail", got.Status)
	}
}

func TestRunOQ_EndToEnd_Pass(t *testing.T) {
	base := t.TempDir()

	nmDir := filepath.Join(base, "nm")
	if err := os.MkdirAll(nmDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(nmDir, "nmfe"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	licPath := filepath.Join(base, "nonmem.lic")
	if err := os.WriteFile(licPath, []byte("LICENSE"), 0o600); err != nil {
		t.Fatalf("write license: %v", err)
	}

	cfg := &config.Config{Version: "1.0.0", User: "tester"}
	cfg.ExecutionMode = config.ExecutionModeNONMEM
	cfg.NonmemPath = nmDir
	cfg.NonmemBinary = "nmfe"
	cfg.NONMEM.License.Path = licPath

	runDir := filepath.Join(base, "qa", "run-0001")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	fe := &fakeExecutor{lstSource: mockLstPath()}

	res, err := RunOQ(context.Background(), cfg, provideFake(fe), summary.NewNONMEMSummarizer(), runDir, Options{Now: fixedClock()})
	if err != nil {
		t.Fatalf("RunOQ: %v", err)
	}

	if res.Status != StatusPass {
		t.Fatalf("status = %q, want pass; checks=%+v", res.Status, res.Checks)
	}

	if res.Phase != PhaseFunctional {
		t.Fatalf("phase = %q, want functional", res.Phase)
	}

	if fe.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", fe.calls)
	}

	if res.JanusVersion != "1.0.0" || res.User != "tester" {
		t.Fatalf("provenance not propagated: %+v", res)
	}
}

func containsArtifact(artifacts []string, want string) bool {
	for _, a := range artifacts {
		if a == want {
			return true
		}
	}

	return false
}
