package qa

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/config"
)

// newRunDir builds a configDir/qa/<uuid> layout under a temp dir and returns the
// run dir, matching what the Store would produce.
func newRunDir(t *testing.T) string {
	t.Helper()

	runDir := filepath.Join(t.TempDir(), "qa", "run-0001")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	return runDir
}

func findCheck(t *testing.T, checks []Check, name string) Check {
	t.Helper()

	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}

	t.Fatalf("check %q not found in %+v", name, checks)

	return Check{}
}

func fixedClock() func() time.Time {
	ts := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	return func() time.Time { return ts }
}

func TestRunIQ_AllPass(t *testing.T) {
	runDir := newRunDir(t)
	cfg := &config.Config{Version: "1.2.3", User: "tester"}

	opts := Options{
		Now: fixedClock(),
		ExecutorProbe: func(_ context.Context) (string, error) {
			return "9.9.9", nil
		},
	}

	res := RunIQ(cfg, runDir, opts)

	if res.Status != StatusPass {
		t.Fatalf("overall status = %q, want pass; checks=%+v", res.Status, res.Checks)
	}

	if res.JanusVersion != "1.2.3" || res.User != "tester" {
		t.Fatalf("provenance not propagated: %+v", res)
	}

	if !res.StartedAt.Equal(res.CompletedAt) {
		t.Fatalf("expected fixed clock to give equal timestamps, got %v / %v", res.StartedAt, res.CompletedAt)
	}

	if got := findCheck(t, res.Checks, "executor_component"); got.Status != StatusPass {
		t.Fatalf("executor_component = %q, want pass", got.Status)
	}
}

func TestRunIQ_MissingBuildInfoFails(t *testing.T) {
	runDir := newRunDir(t)
	cfg := &config.Config{} // no Version

	res := RunIQ(cfg, runDir, Options{Now: fixedClock()})

	if got := findCheck(t, res.Checks, "janus_build_info"); got.Status != StatusFail {
		t.Fatalf("janus_build_info = %q, want fail", got.Status)
	}

	if res.Status != StatusFail {
		t.Fatalf("overall status = %q, want fail", res.Status)
	}
}

func TestRunIQ_NilConfigFailsConfigLoads(t *testing.T) {
	runDir := newRunDir(t)

	res := RunIQ(nil, runDir, Options{Now: fixedClock()})

	if got := findCheck(t, res.Checks, "config_loads"); got.Status != StatusFail {
		t.Fatalf("config_loads = %q, want fail", got.Status)
	}
}

func TestRunIQ_ExecutorProbeNilSkips(t *testing.T) {
	runDir := newRunDir(t)
	cfg := &config.Config{Version: "1.0.0"}

	res := RunIQ(cfg, runDir, Options{Now: fixedClock()})

	got := findCheck(t, res.Checks, "executor_component")
	if got.Status != StatusSkip {
		t.Fatalf("executor_component = %q, want skip", got.Status)
	}

	// A skipped executor check must not fail the overall run.
	if res.Status != StatusPass {
		t.Fatalf("overall status = %q, want pass (skip is not fail)", res.Status)
	}
}

func TestRunIQ_ExecutorProbeErrorFails(t *testing.T) {
	runDir := newRunDir(t)
	cfg := &config.Config{Version: "1.0.0"}

	opts := Options{
		Now: fixedClock(),
		ExecutorProbe: func(_ context.Context) (string, error) {
			return "", errors.New("exec format error")
		},
	}

	res := RunIQ(cfg, runDir, opts)

	if got := findCheck(t, res.Checks, "executor_component"); got.Status != StatusFail {
		t.Fatalf("executor_component = %q, want fail", got.Status)
	}

	if res.Status != StatusFail {
		t.Fatalf("overall status = %q, want fail", res.Status)
	}
}

func TestRunIQ_NonWritableQADirFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits unreliable on Windows")
	}

	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}

	runDir := newRunDir(t)
	cfg := &config.Config{Version: "1.0.0"}

	// Make the run dir read-only so the temp-file probe fails.
	if err := os.Chmod(runDir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o755) })

	res := RunIQ(cfg, runDir, Options{Now: fixedClock()})

	if got := findCheck(t, res.Checks, "qa_dir_writable"); got.Status != StatusFail {
		t.Fatalf("qa_dir_writable = %q, want fail", got.Status)
	}
}
