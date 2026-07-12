package qa

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/google/uuid"
)

// Store persists qualification reports under a per-user QA base directory
// (typically ~/.config/janus/qa). Each run gets its own UUIDv7-keyed
// subdirectory holding the report JSON plus any functional-run artifacts —
// a per-run, time-sortable audit trail. It mirrors internal/runlog/store.go's
// atomic-write + UUIDv7 conventions.
//
// The base directory is handed in by the caller; Store never derives it from
// the environment (orthogonality rule).
type Store struct {
	baseDir string
}

// NewStore returns a Store rooted at baseDir.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// NewRunDir allocates a fresh UUIDv7-keyed run directory under the base dir and
// returns its id and absolute path.
func (s *Store) NewRunDir() (id, dir string, err error) {
	id = uuid.Must(uuid.NewV7()).String()
	dir = filepath.Join(s.baseDir, id)

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating qa run directory: %w", err)
	}

	return id, dir, nil
}

// Save atomically writes v as <dir>/<kind>.json (0600). It writes to a .tmp
// sibling first and renames into place so a reader never sees a partial file.
func (s *Store) Save(dir string, kind Kind, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s report: %w", kind, err)
	}

	final := filepath.Join(dir, string(kind)+".json")
	tmp := final + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing temp report: %w", err)
	}

	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)

		return fmt.Errorf("renaming report into place: %w", err)
	}

	return nil
}

// Latest returns the path and raw bytes of the most recent report of the given
// kind. Run directories are named with UUIDv7, whose string form sorts
// chronologically, so the newest report is found by a reverse directory sort —
// no index file needed. It returns os.ErrNotExist when no report of that kind
// has ever been written (the "never run" sentinel).
func (s *Store) Latest(kind Kind) (path string, raw []byte, err error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, os.ErrNotExist
		}

		return "", nil, fmt.Errorf("reading qa base directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	// Newest first: UUIDv7 lexical order == chronological order.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	for _, name := range names {
		candidate := filepath.Join(s.baseDir, name, string(kind)+".json")

		data, readErr := os.ReadFile(candidate)
		if readErr == nil {
			return candidate, data, nil
		}

		if !errors.Is(readErr, os.ErrNotExist) {
			return "", nil, fmt.Errorf("reading report %s: %w", candidate, readErr)
		}
	}

	return "", nil, os.ErrNotExist
}
