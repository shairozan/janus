package runlog

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"

	"github.com/pharmalytica/janus/internal/signing"
)

// RunLogStore manages directory-based run log storage with UUID-indexed files.
// This structure eliminates Git merge conflicts by storing each run as a separate file.
type RunLogStore struct {
	baseDir   string // Model directory (contains .janus/)
	modelFile string // Model filename for context

	index *RunIndex             // In-memory index for fast queries
	cache map[string]*RunRecord // Loaded records cache

	// Signing configuration
	signer      *signing.Signer
	signerEmail string
	fallbackKey string // Fallback public key for verification

	mu sync.RWMutex // Protects index and cache
}

// RunIndex is a lightweight index for fast run queries.
// This file is gitignored and auto-regenerates from individual run files.
type RunIndex struct {
	ModelFile   string          `json:"model_file"`
	LastUpdated time.Time       `json:"last_updated"`
	TotalRuns   int             `json:"total_runs"`
	Entries     []RunIndexEntry `json:"index"`
}

// RunIndexEntry contains minimal metadata for UI display and sorting.
type RunIndexEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Signed    bool      `json:"signed"`

	// Saga linkage (omitempty), mirrored from the record so the UI can build the
	// parent/child tree from the index without loading every file.
	Kind     string `json:"kind,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

// NewRunLogStore creates a new run log store for the given model directory.
func NewRunLogStore(baseDir, modelFile string) *RunLogStore {
	return &RunLogStore{
		baseDir:   baseDir,
		modelFile: modelFile,
		cache:     make(map[string]*RunRecord),
	}
}

// SetSigner configures the signer for new run records.
func (s *RunLogStore) SetSigner(signer *signing.Signer, email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signer = signer
	s.signerEmail = email
}

// SetFallbackKey sets the fallback public key for signature verification.
func (s *RunLogStore) SetFallbackKey(publicKeyPEM string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fallbackKey = publicKeyPEM
}

// runlogDir returns the path to the run log directory.
func (s *RunLogStore) runlogDir() string {
	return filepath.Join(s.baseDir, ".janus", "runlog")
}

// indexPath returns the path to the index file.
func (s *RunLogStore) indexPath() string {
	return filepath.Join(s.runlogDir(), "index.json")
}

// ensureDirectories creates the run log directory structure if needed.
func (s *RunLogStore) ensureDirectories() error {
	return os.MkdirAll(s.runlogDir(), 0755)
}

// Load initializes the store by loading or rebuilding the index.
func (s *RunLogStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDirectories(); err != nil {
		return fmt.Errorf("creating runlog directory: %w", err)
	}

	index, err := s.loadIndex()
	if err != nil {
		// Index corrupted or missing - rebuild from files
		index, err = s.rebuildIndex()
		if err != nil {
			return fmt.Errorf("rebuilding index: %w", err)
		}

		// Persist rebuilt index
		if writeErr := s.writeIndexLocked(); writeErr != nil {
			// Log but don't fail - index is optional
			log.Printf("Warning: failed to persist rebuilt index: %v", writeErr)
		}
	}

	s.index = index

	return nil
}

// loadIndex reads the index from disk.
func (s *RunLogStore) loadIndex() (*RunIndex, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return nil, err
	}

	var index RunIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	return &index, nil
}

// rebuildIndex scans all run files and rebuilds the index.
func (s *RunLogStore) rebuildIndex() (*RunIndex, error) {
	pattern := filepath.Join(s.runlogDir(), "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("scanning run files: %w", err)
	}

	entries := make([]RunIndexEntry, 0, len(files))

	for _, file := range files {
		// Skip index file
		if filepath.Base(file) == "index.json" {
			continue
		}

		entry, err := s.readIndexEntryFromFile(file)
		if err != nil {
			// Skip corrupted files but log warning
			log.Printf("Warning: skipping corrupted run file %s: %v", file, err)

			continue
		}

		entries = append(entries, *entry)
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	return &RunIndex{
		ModelFile:   s.modelFile,
		LastUpdated: time.Now(),
		TotalRuns:   len(entries),
		Entries:     entries,
	}, nil
}

// readIndexEntryFromFile reads minimal metadata from a run file.
func (s *RunLogStore) readIndexEntryFromFile(path string) (*RunIndexEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return &RunIndexEntry{
		ID:        record.ID,
		Timestamp: record.Timestamp,
		Status:    record.Status,
		Signed:    record.Signature != "",
		Kind:      record.Kind,
		ParentID:  record.ParentID,
	}, nil
}

// writeIndexLocked writes the index to disk. Caller must hold the lock.
func (s *RunLogStore) writeIndexLocked() error {
	return writeIndexAtomic(s.indexPath(), s.index)
}

// writeIndexAtomic writes idx to path via a temp file + rename, so a concurrent
// reader (possibly in another process) never observes a torn index.
func writeIndexAtomic(path string, idx *RunIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling index: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing temp index: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)

		return fmt.Errorf("renaming index: %w", err)
	}

	return nil
}

// lockPath returns the path of the cross-process advisory lock file guarding the
// index read-modify-write. It is separate from index.json so the atomic rename
// (which replaces the index inode) never disturbs the lock.
func (s *RunLogStore) lockPath() string {
	return s.indexPath() + ".lock"
}

// upsertIndexForRecord performs a cross-process-safe read-modify-write of the
// index for a single record. Under an OS file lock it re-reads the on-disk index
// (rebuilding from run files if it is missing or corrupt), upserts this record's
// entry, sorts newest-first, and writes atomically. Re-reading under the lock is
// what makes GUI + daemon coexistence safe: neither process trusts its own
// possibly-stale in-memory copy when mutating. The caller holds s.mu.
func (s *RunLogStore) upsertIndexForRecord(record *RunRecord) error {
	fileLock := flock.New(s.lockPath())
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("acquiring index lock: %w", err)
	}
	defer func() { _ = fileLock.Unlock() }()

	// Re-read the authoritative on-disk index (other processes may have written
	// to it since we last loaded), falling back to a rebuild from run files.
	index, err := s.loadIndex()
	if err != nil {
		index, err = s.rebuildIndex()
		if err != nil {
			return fmt.Errorf("rebuilding index under lock: %w", err)
		}
	}

	entry := RunIndexEntry{
		ID:        record.ID,
		Timestamp: record.Timestamp,
		Status:    record.Status,
		Signed:    record.Signature != "",
		Kind:      record.Kind,
		ParentID:  record.ParentID,
	}

	// Upsert: drop any existing entry for this ID, then append the fresh one.
	merged := index.Entries[:0:0]
	for _, e := range index.Entries {
		if e.ID != entry.ID {
			merged = append(merged, e)
		}
	}
	merged = append(merged, entry)

	// Maintain newest-first ordering regardless of insertion order.
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.After(merged[j].Timestamp)
	})

	index.Entries = merged
	index.TotalRuns = len(merged)
	index.LastUpdated = time.Now()
	if index.ModelFile == "" {
		index.ModelFile = s.modelFile
	}

	if err := writeIndexAtomic(s.indexPath(), index); err != nil {
		return err
	}

	// Adopt the merged index as our in-memory view.
	s.index = index

	return nil
}

// AddRun adds a new run record to the store.
// Generates UUID if not set, signs if signer configured, and writes atomically.
func (s *RunLogStore) AddRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate UUID if not set
	if record.ID == "" {
		record.ID = uuid.Must(uuid.NewV7()).String()
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Sign if signer configured
	if s.signer != nil {
		if err := SignRecordWithInfo(record, s.signer, s.signerEmail); err != nil {
			return fmt.Errorf("signing record: %w", err)
		}
	}

	// Atomic write
	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	// Update index under a cross-process lock so a coexisting GUI/daemon cannot
	// clobber each other's entries.
	return s.upsertIndexForRecord(record)
}

// AddSaga records a saga parent and its child records as one logical unit (e.g. a
// horizontal bootstrap: the saga summary + one record per fit). The parent is
// marked KindSaga and written first so its generated ID is available; each child
// is marked KindFit with ParentID set to the parent's ID. Each record is written
// atomically and signed independently if a signer is configured, so every fit and
// the saga remain individually verifiable.
func (s *RunLogStore) AddSaga(parent *RunRecord, children []*RunRecord) error {
	if parent == nil {
		return fmt.Errorf("saga parent record is nil")
	}

	parent.Kind = KindSaga

	if err := s.AddRun(parent); err != nil {
		return fmt.Errorf("recording saga parent: %w", err)
	}

	for i, child := range children {
		if child == nil {
			return fmt.Errorf("saga child %d is nil", i)
		}

		child.Kind = KindFit
		child.ParentID = parent.ID

		if err := s.AddRun(child); err != nil {
			return fmt.Errorf("recording saga child %d: %w", i, err)
		}
	}

	return nil
}

// ChildrenOf returns the fit records belonging to the saga with the given parent
// ID, using the index to find them. The result is empty (not an error) when the
// parent has no children.
func (s *RunLogStore) ChildrenOf(parentID string) ([]*RunRecord, error) {
	s.mu.RLock()

	var ids []string

	if s.index != nil {
		for _, e := range s.index.Entries {
			if e.ParentID == parentID {
				ids = append(ids, e.ID)
			}
		}
	}

	s.mu.RUnlock()

	children := make([]*RunRecord, 0, len(ids))

	for _, id := range ids {
		record, err := s.GetRun(id)
		if err != nil {
			return nil, fmt.Errorf("loading saga child %s: %w", id, err)
		}

		children = append(children, record)
	}

	return children, nil
}

// writeRunFileLocked writes a run record atomically. Caller must hold the lock.
func (s *RunLogStore) writeRunFileLocked(record *RunRecord) error {
	if err := s.ensureDirectories(); err != nil {
		return fmt.Errorf("creating runlog directory: %w", err)
	}

	filename := filepath.Join(s.runlogDir(), record.ID+".json")
	tmpFile := filename + ".tmp"

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	// Write to temp file first
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, filename); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpFile)

		return fmt.Errorf("renaming to final path: %w", err)
	}

	// Cache the record
	s.cache[record.ID] = record

	return nil
}

// GetRuns returns paginated run records sorted by timestamp (newest first).
func (s *RunLogStore) GetRuns(limit, offset int) ([]*RunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil || len(s.index.Entries) == 0 {
		return []*RunRecord{}, nil
	}

	// Calculate bounds
	start := offset
	if start >= len(s.index.Entries) {
		return []*RunRecord{}, nil
	}

	end := start + limit
	if end > len(s.index.Entries) {
		end = len(s.index.Entries)
	}

	entries := s.index.Entries[start:end]
	runs := make([]*RunRecord, 0, len(entries))

	for _, entry := range entries {
		run, err := s.loadRunLocked(entry.ID)
		if err != nil {
			// Skip corrupted entries
			log.Printf("Warning: failed to load run %s: %v", entry.ID, err)

			continue
		}

		runs = append(runs, run)
	}

	return runs, nil
}

// GetRun retrieves a single run by ID.
func (s *RunLogStore) GetRun(id string) (*RunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadRunLocked(id)
}

// loadRunLocked loads a run record with caching. Caller must hold at least read lock.
func (s *RunLogStore) loadRunLocked(id string) (*RunRecord, error) {
	// Check cache first
	if cached, ok := s.cache[id]; ok {
		return cached, nil
	}

	// Read from disk
	filename := filepath.Join(s.runlogDir(), id+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading run file: %w", err)
	}

	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling record: %w", err)
	}

	// Verify signature if present
	if record.Signature != "" {
		status := VerifyRecordStatus(&record, s.fallbackKey)
		// Store verification status (not persisted, computed on load)
		_ = status // Can be used by caller via VerifyRecordStatus
	}

	return &record, nil
}

// GetLatestRun returns the most recent run record.
func (s *RunLogStore) GetLatestRun() (*RunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil || len(s.index.Entries) == 0 {
		return nil, fmt.Errorf("no runs available")
	}

	return s.loadRunLocked(s.index.Entries[0].ID)
}

// Count returns the total number of runs.
func (s *RunLogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil {
		return 0
	}

	return s.index.TotalRuns
}

// GetAllRuns returns all run records as a slice (for backward compatibility).
// Note: This loads all records into memory - use GetRuns for pagination.
func (s *RunLogStore) GetAllRuns() []RunRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil || len(s.index.Entries) == 0 {
		return []RunRecord{}
	}

	runs := make([]RunRecord, 0, len(s.index.Entries))

	for _, entry := range s.index.Entries {
		run, err := s.loadRunLocked(entry.ID)
		if err != nil {
			continue
		}

		runs = append(runs, *run)
	}

	return runs
}

// UpdateRun updates an existing run record in place.
// This is used to update status, output, etc. after execution completes.
func (s *RunLogStore) UpdateRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sign if signer configured and record isn't already signed
	if s.signer != nil && record.Signature == "" {
		if err := SignRecordWithInfo(record, s.signer, s.signerEmail); err != nil {
			return fmt.Errorf("signing record: %w", err)
		}
	}

	// Write updated record
	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	// Re-read and upsert the index under the cross-process lock; this both updates
	// this record's status/signed flags and preserves entries another process may
	// have added since we loaded.
	return s.upsertIndexForRecord(record)
}
