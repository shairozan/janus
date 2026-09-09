//go:build validation

package qarun

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/qa"
	"github.com/shairozan/janus/internal/summary"
)

// TestOQ_EndToEnd_MockNONMEM runs the full OQ functional loop — materialize the
// embedded ACOP model, execute it through the REAL local NONMEM executor (the
// factory the GUI/CLI use), parse the output, and gate on the OFV — using the
// testdata mock NONMEM script as the binary. It is gated behind the `validation`
// build tag so default `go test` stays fast and hermetic; CI exercises it via
// `mage docker:validation` (go test -tags=validation).
func TestOQ_EndToEnd_MockNONMEM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mock NONMEM is a bash script; not supported on Windows")
	}

	mockDir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "mock-nonmem"))
	if err != nil {
		t.Fatalf("resolving mock dir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(mockDir, "mock-nonmem-fast.sh")); err != nil {
		t.Fatalf("mock NONMEM script not found: %v", err)
	}

	// A license file so phase-1 nonmem_license passes.
	licPath := filepath.Join(t.TempDir(), "nonmem.lic")
	if err := os.WriteFile(licPath, []byte("MOCK LICENSE"), 0o600); err != nil {
		t.Fatalf("writing license: %v", err)
	}

	cfg := &config.Config{Version: "e2e", User: "ci"}
	cfg.ExecutionMode = config.ExecutionModeNONMEM
	cfg.NonmemPath = mockDir
	cfg.NonmemBinary = "mock-nonmem-fast.sh"
	cfg.NONMEM.License.Path = licPath

	store := qa.NewStore(filepath.Join(t.TempDir(), "qa"))

	_, runDir, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	provider := BuildExecutorProvider(cfg)

	res, err := qa.RunOQ(context.Background(), cfg, provider, summary.NewNONMEMSummarizer(), runDir, qa.Options{})
	if err != nil {
		t.Fatalf("RunOQ: %v", err)
	}

	if res.Status != qa.StatusPass {
		t.Fatalf("OQ status = %q, want pass; checks=%+v", res.Status, res.Checks)
	}

	if res.Phase != qa.PhaseFunctional {
		t.Fatalf("phase = %q, want functional", res.Phase)
	}

	if res.ObservedOFV == nil {
		t.Fatalf("ObservedOFV is nil; expected a parsed OFV near %v", qa.ReferenceOFV)
	}

	// Persist the report and confirm it landed with artifacts.
	if err := store.Save(runDir, qa.KindOQ, res); err != nil {
		t.Fatalf("saving OQ report: %v", err)
	}

	if _, err := os.Stat(filepath.Join(runDir, "oq.json")); err != nil {
		t.Fatalf("oq.json not written: %v", err)
	}

	for _, name := range []string{"acop.mod", "acop.csv", "acop.lst"} {
		if _, err := os.Stat(filepath.Join(runDir, name)); err != nil {
			t.Fatalf("expected artifact %s in run dir: %v", name, err)
		}
	}
}
