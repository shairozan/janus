package qa

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_NewRunDir_UniqueAndCreated(t *testing.T) {
	base := t.TempDir()
	store := NewStore(base)

	id1, dir1, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	id2, dir2, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	if id1 == id2 || dir1 == dir2 {
		t.Fatalf("expected unique run dirs, got %q and %q", dir1, dir2)
	}

	for _, dir := range []string{dir1, dir2} {
		info, statErr := os.Stat(dir)
		if statErr != nil {
			t.Fatalf("run dir not created: %v", statErr)
		}

		if !info.IsDir() {
			t.Fatalf("run dir %q is not a directory", dir)
		}
	}
}

func TestStore_Save_AtomicValidJSON(t *testing.T) {
	base := t.TempDir()
	store := NewStore(base)

	_, dir, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	want := &IQResult{Kind: KindIQ, Status: StatusPass, JanusVersion: "1.2.3"}
	if err := store.Save(dir, KindIQ, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	final := filepath.Join(dir, "iq.json")

	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("reading saved report: %v", err)
	}

	var got IQResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("saved report is not valid JSON: %v", err)
	}

	if got.JanusVersion != want.JanusVersion || got.Status != want.Status {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}

	// No .tmp file should be left behind.
	if _, err := os.Stat(final + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no leftover .tmp file, stat err = %v", err)
	}
}

func TestStore_Latest_OrderingAndKindFilter(t *testing.T) {
	base := t.TempDir()
	store := NewStore(base)

	// First run: IQ only.
	_, dir1, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	if err := store.Save(dir1, KindIQ, &IQResult{Kind: KindIQ, Status: StatusFail}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Second (newer) run: both IQ and OQ.
	_, dir2, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	if err := store.Save(dir2, KindIQ, &IQResult{Kind: KindIQ, Status: StatusPass}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Save(dir2, KindOQ, &OQResult{Kind: KindOQ, Status: StatusPass}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Latest IQ should come from the newer run dir.
	path, raw, err := store.Latest(KindIQ)
	if err != nil {
		t.Fatalf("Latest(iq): %v", err)
	}

	if filepath.Dir(path) != dir2 {
		t.Fatalf("Latest(iq) returned %q, want a file under %q", path, dir2)
	}

	var latestIQ IQResult
	if err := json.Unmarshal(raw, &latestIQ); err != nil {
		t.Fatalf("unmarshal latest iq: %v", err)
	}

	if latestIQ.Status != StatusPass {
		t.Fatalf("Latest(iq) status = %q, want pass (newest run)", latestIQ.Status)
	}

	// Latest OQ exists only in the newer run dir.
	oqPath, _, err := store.Latest(KindOQ)
	if err != nil {
		t.Fatalf("Latest(oq): %v", err)
	}

	if filepath.Dir(oqPath) != dir2 {
		t.Fatalf("Latest(oq) returned %q, want a file under %q", oqPath, dir2)
	}
}

func TestStore_Latest_NeverRunSentinel(t *testing.T) {
	// Base dir does not exist yet.
	store := NewStore(filepath.Join(t.TempDir(), "qa-missing"))

	if _, _, err := store.Latest(KindIQ); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for never-run, got %v", err)
	}

	// Base dir exists but holds no report of the requested kind.
	base := t.TempDir()
	store = NewStore(base)

	_, dir, err := store.NewRunDir()
	if err != nil {
		t.Fatalf("NewRunDir: %v", err)
	}

	if err := store.Save(dir, KindIQ, &IQResult{Kind: KindIQ, Status: StatusPass}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, _, err := store.Latest(KindOQ); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist when no OQ report exists, got %v", err)
	}
}
