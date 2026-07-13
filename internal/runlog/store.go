package runlog

import (
	"encoding/json"
	"errors"
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
	trust       TrustStore // Trust anchor for signature verification (nil if not configured)

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

// SetTrustStore configures the trust anchor used to verify record signatures. A
// nil trust store (the default) makes every signed record report Unverifiable —
// never Valid — until a trust anchor is configured.
func (s *RunLogStore) SetTrustStore(trust TrustStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trust = trust
}

// runlogDir returns the path to the run log directory.
func (s *RunLogStore) runlogDir() string {
	return filepath.Join(s.baseDir, ".janus", "runlog")
}

// indexPath returns the path to the index file.
func (s *RunLogStore) indexPath() string {
	return filepath.Join(s.runlogDir(), "index.json")
}

// headPath returns the path to the signed chain head checkpoint.
func (s *RunLogStore) headPath() string {
	return filepath.Join(s.runlogDir(), "head.json")
}

// isReservedRunlogFile reports whether name is one of the runlog directory's
// metadata files rather than a run record — index.json (the rebuildable query
// cache) or head.json (the signed chain-tip checkpoint). rebuildIndex globs
// every *.json file in the directory, so any metadata file added here in the
// future must be listed to avoid becoming a phantom index entry.
func isReservedRunlogFile(name string) bool {
	switch name {
	case "index.json", "head.json":
		return true
	default:
		return false
	}
}

// IsTerminal reports whether a status means the run is over and the record is now
// an audit fact rather than mutable state.
func IsTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// nextChainLocked returns the sequence and prev-hash the next sealed record must
// take. The caller holds s.mu AND the cross-process file lock. A missing head is
// genesis: sequence 1, no predecessor.
func (s *RunLogStore) nextChainLocked() (int, string, error) {
	head, err := ReadHead(s.headPath())
	if err != nil {
		return 0, "", fmt.Errorf("reading chain head: %w", err)
	}

	if head == nil {
		return 1, "", nil
	}

	return head.Sequence + 1, head.TipHash, nil
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
		// Skip reserved metadata files (index.json, head.json) — they are not runs.
		if isReservedRunlogFile(filepath.Base(file)) {
			continue
		}

		entry, err := s.readIndexEntryFromFile(file)
		if err != nil {
			// Skip corrupted files but log warning
			log.Printf("Warning: skipping corrupted run file %s: %v", file, err)

			continue
		}

		// A record with no ID is not a run — guard against any future metadata
		// file (or a truly corrupt/empty run file) silently unmarshaling into a
		// zero-value RunRecord and entering the index as a phantom entry.
		if entry.ID == "" {
			log.Printf("Warning: skipping run file %s with empty ID", file)

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

// AddRun adds a new run record. A run that is still executing is not yet an audit
// fact, so it is written as an unsigned DRAFT. It is signed and chained only when
// it reaches a terminal status — see Seal.
func (s *RunLogStore) AddRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.Must(uuid.NewV7()).String()
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// A record that arrives already finished (the common case for a fast run, and
	// for saga children) is sealed immediately. sealLocked writes the file itself.
	if IsTerminal(record.Status) && s.signer != nil {
		if err := s.sealLocked(record); err != nil {
			return err
		}

		return s.upsertIndexForRecord(record)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

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

// Seal makes a record an audit fact: it assigns the record's chain position,
// signs it, writes it, and advances the head — in that order, once, for good.
//
// Order matters. Sequence and PrevHash are assigned BEFORE signing so the
// signature covers them; an unsigned sequence number is worthless. And signing is
// the LAST mutation, which is what makes it impossible to sign a record and then
// change it — the defect this replaces.
func (s *RunLogStore) Seal(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sealLocked(record); err != nil {
		return err
	}

	// sealLocked no-ops when no signer is configured — the record stays an
	// unsigned draft on disk, exactly as AddRun/UpdateRun last wrote it. Upserting
	// the index here would only be safe if the caller had not already mutated the
	// record in memory (as the no-signer path explicitly permits, since there is
	// no immutability to protect yet); a caller that flips Status after building
	// the record but before calling Seal would otherwise desync the index from
	// disk. Only a real seal changes what belongs in the index.
	if !record.Sealed {
		return nil
	}

	// A sealed record that is not in the index is invisible to GetRuns/GetAllRuns
	// and therefore to the GUI: rebuildIndex only runs when the index is missing or
	// corrupt, never merely incomplete. AddRun and UpdateRun both upsert after
	// sealing; so must this.
	return s.upsertIndexForRecord(record)
}

// restorePreSealLocked undoes, in one place, every mutation a failed seal made —
// the in-memory struct, the cache entry, and the file on disk — so the record is
// left exactly as it was found and the seal can simply be retried.
//
// recordFileWritten says whether writeRunFileLocked already replaced the file; only
// then does the byte snapshot need to be put back. priorBytes/priorExisted describe
// what the file held before the seal: a legitimate in-flight DRAFT on the common
// production path (AddRun writes "running", UpdateRun later seals to "completed"),
// which must be restored exactly, not deleted — or nothing at all, for a record that
// arrived already terminal, where removing the file is the correct rollback.
//
// The caller holds s.mu and the cross-process file lock.
func (s *RunLogStore) restorePreSealLocked(
	record *RunRecord,
	filename string,
	priorBytes []byte,
	priorExisted bool,
	recordFileWritten bool,
) error {
	record.Sealed = false
	record.Sequence = 0
	record.PrevHash = ""
	clearSignatureFields(record)

	// s.cache may hold this very pointer (writeRunFileLocked does
	// s.cache[record.ID] = record), so a stale sealed struct could otherwise
	// survive here and be served by GetRun. Drop the entry: the next read comes
	// from disk, which is the state we are restoring to.
	delete(s.cache, record.ID)

	if !recordFileWritten {
		// The file on disk was never touched — it still holds the pre-seal bytes.
		return nil
	}

	if priorExisted {
		if err := writeFileAtomic(filename, priorBytes); err != nil {
			return fmt.Errorf("restoring pre-seal record file: %w", err)
		}

		return nil
	}

	if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing partially sealed record file: %w", err)
	}

	return nil
}

// sealLocked is Seal's body. The caller holds s.mu.
//
// The error return is named because every failure AFTER the record is mutated must
// run through the single rollback installed below — a defer, so that no future
// fallible step can be added between the mutation and the head write without being
// covered by it.
func (s *RunLogStore) sealLocked(record *RunRecord) (err error) {
	if record.Sealed {
		return fmt.Errorf("record %s is already sealed; corrections are amendments, not rewrites", record.ID)
	}

	if s.signer == nil {
		// Nothing to seal with. The record stays a draft; this is not an error,
		// it is simply an unsigned deployment.
		return nil
	}

	// The chain head is shared across processes, so read-modify-write it under the
	// same advisory lock that guards the index.
	fileLock := flock.New(s.lockPath())
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("acquiring chain lock: %w", err)
	}
	defer func() { _ = fileLock.Unlock() }()

	// The in-memory record.Sealed flag is only this process's view. Another
	// process (or an earlier call on this same store against a stale caller
	// struct) may have already sealed this ID on disk; that is the only
	// authoritative answer, so read it directly, bypassing s.cache.
	if onDisk, err := s.loadRunFromDiskLocked(record.ID); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("checking on-disk seal state for record %s: %w", record.ID, err)
		}
		// No file on disk yet for this ID — nothing to protect against.
	} else if onDisk.Sealed {
		return fmt.Errorf(
			"record %s is already sealed on disk; corrections are amendments, not rewrites",
			record.ID,
		)
	}

	sequence, prevHash, err := s.nextChainLocked()
	if err != nil {
		return err
	}

	// Capture whatever is on disk for this ID right now — an in-flight DRAFT in
	// the common case, nothing at all for a record that arrives already
	// terminal — BEFORE the record is mutated and writeRunFileLocked overwrites
	// the file with the sealed version. If anything below fails, this is what
	// lets the rollback restore the file to exactly what it was instead of
	// destroying it.
	filename := filepath.Join(s.runlogDir(), record.ID+".json")

	priorBytes, readErr := os.ReadFile(filename)

	priorExisted := true
	if readErr != nil {
		if !errors.Is(readErr, os.ErrNotExist) {
			return fmt.Errorf("reading existing record %s before seal: %w", record.ID, readErr)
		}

		priorExisted = false
	}

	record.Sequence = sequence
	record.PrevHash = prevHash
	record.Sealed = true

	// From here the caller's record (and possibly s.cache, which may hold this very
	// pointer) claims a chain position it has not earned, and every remaining step
	// is fallible. ONE rollback covers all of them: signing, the record write, the
	// hash, the head signature and the head write. Without it a failure would leave
	// a sealed, sequenced record in memory that exists nowhere on disk — or worse, a
	// sealed record file above a head that still points at the previous sequence, so
	// the next seal would hand this same sequence number to a DIFFERENT record and
	// break the chain permanently. The in-memory guard at the top of this function
	// would also refuse every retry ("already sealed") for the life of the process.
	recordFileWritten := false

	defer func() {
		if err == nil {
			return
		}

		restoreErr := s.restorePreSealLocked(record, filename, priorBytes, priorExisted, recordFileWritten)
		if restoreErr != nil {
			err = fmt.Errorf(
				"%w; additionally, restoring record %s to its pre-seal state failed: %v"+
					" — the run log chain may now be inconsistent and requires manual verification",
				err, record.ID, restoreErr,
			)

			return
		}

		err = fmt.Errorf("%w; record %s was rolled back to its pre-seal state", err, record.ID)
	}()

	// Sign LAST — after every other field is final.
	if err := SignRecordWithInfo(record, s.signer, s.signerEmail); err != nil {
		return fmt.Errorf("signing record: %w", err)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return fmt.Errorf("writing sealed record %s: %w", record.ID, err)
	}

	recordFileWritten = true

	tipHash, err := RecordHash(record)
	if err != nil {
		return err
	}

	head := &Head{Sequence: sequence, TipHash: tipHash}
	if err := SignHead(head, s.signer); err != nil {
		return fmt.Errorf("signing chain head: %w", err)
	}

	if err := WriteHead(s.headPath(), head); err != nil {
		return fmt.Errorf("writing chain head: %w", err)
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

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	if err := writeFileAtomic(filename, data); err != nil {
		return err
	}

	// Cache the record
	s.cache[record.ID] = record

	return nil
}

// writeFileAtomic writes data to path via a temp file + rename, so a
// concurrent reader (possibly in another process) never observes a torn file.
func writeFileAtomic(path string, data []byte) error {
	tmpFile := path + ".tmp"

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpFile)

		return fmt.Errorf("renaming to final path: %w", err)
	}

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
		status := VerifyRecordStatus(&record, s.trust)
		// Store verification status (not persisted, computed on load)
		_ = status // Can be used by caller via VerifyRecordStatus
	}

	return &record, nil
}

// loadRunFromDiskLocked reads and unmarshals a run record directly from its file,
// bypassing s.cache entirely. Unlike loadRunLocked, this is the authoritative
// answer for questions like "is this record sealed?" — the in-memory cache is only
// this process's possibly-stale view, but two RunLogStore instances (e.g. the GUI
// and the daemon) can be pointed at the same directory, and only the file on disk
// is shared truth between them. Caller must hold at least read lock.
func (s *RunLogStore) loadRunFromDiskLocked(id string) (*RunRecord, error) {
	filename := filepath.Join(s.runlogDir(), id+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading run file: %w", err)
	}

	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling record: %w", err)
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

// SealedRecords returns every sealed record, sorted ascending by chain
// sequence, plus the IDs of any records whose files exist but could not be
// loaded. Drafts are excluded: they hold no chain position and are not audit
// facts.
//
// The unreadable list matters because "missing" and "unreadable" are
// different failures for whoever has to explain the report to an auditor: a
// genuinely absent file is what CREATES the sequence gap the chain check
// exists to detect, but a corrupt file sitting right there on disk is not a
// deletion, and must not be reported as one.
func (s *RunLogStore) SealedRecords() ([]*RunRecord, []string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil {
		return nil, nil, nil
	}

	sealed := make([]*RunRecord, 0, len(s.index.Entries))

	var unreadable []string

	for _, entry := range s.index.Entries {
		// loadRunFromDiskLocked, not loadRunLocked: a deleted record's file is
		// exactly the fact this method must surface, and loadRunLocked would
		// happily paper over the deletion with this process's own stale
		// s.cache entry (AddRun/Seal populate it on write and nothing ever
		// invalidates it). Only the file on disk is authoritative here.
		record, err := s.loadRunFromDiskLocked(entry.ID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// A missing file is exactly the thing we are here to detect.
				// Do not swallow it — the chain check will surface it as a gap.
				continue
			}

			// The file exists but failed to load (corrupt JSON, a permission
			// error, etc). Do not fold this into the gap it would otherwise
			// masquerade as — surface it distinctly so the report can say
			// "corrupt", not "missing".
			unreadable = append(unreadable, entry.ID)

			continue
		}

		if record.Sealed {
			sealed = append(sealed, record)
		}
	}

	sort.Slice(sealed, func(i, j int) bool {
		return sealed[i].Sequence < sealed[j].Sequence
	})

	return sealed, unreadable, nil
}

// VerifyIntegrity verifies the run log as a set — the check that did not exist
// before: every prior verification path examined one record at a time and so could
// not, even in principle, notice one that had been removed.
func (s *RunLogStore) VerifyIntegrity(trust TrustStore) (ChainReport, error) {
	sealed, unreadable, err := s.SealedRecords()
	if err != nil {
		return ChainReport{}, err
	}

	head, err := ReadHead(s.headPath())
	if err != nil {
		return ChainReport{}, fmt.Errorf("reading chain head: %w", err)
	}

	report := VerifyChain(sealed, head, trust)
	report.Unreadable = unreadable

	if len(unreadable) > 0 {
		report.OK = false
	}

	return report, nil
}

// UpdateRun updates a DRAFT record — a run that is still in flight. When the
// update carries the run to a terminal status, the record is sealed: chained,
// signed, and made immutable.
//
// A sealed record cannot be updated. That is the point: it is an audit entry, and
// audit entries are appended to, never rewritten. This is what closes the defect
// where a signed record was mutated and saved with its now-stale signature,
// causing Janus to report its own completion path as tampering.
func (s *RunLogStore) UpdateRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Trust the record on disk, not the caller's in-memory copy — the caller may
	// be holding a stale struct, and "am I sealed?" must be answered by the file,
	// not by s.cache. loadRunLocked would happily return this process's own
	// possibly-stale cached copy on a cache hit; loadRunFromDiskLocked always
	// reads the file, which is the only state shared across store instances
	// (e.g. the GUI and the daemon pointed at the same run log directory).
	existing, err := s.loadRunFromDiskLocked(record.ID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking on-disk seal state for record %s: %w", record.ID, err)
	}

	if err == nil && existing.Sealed {
		return fmt.Errorf(
			"run %s is sealed and cannot be modified; append an amendment record instead",
			record.ID,
		)
	}

	if IsTerminal(record.Status) && s.signer != nil {
		if err := s.sealLocked(record); err != nil {
			return err
		}

		return s.upsertIndexForRecord(record)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	return s.upsertIndexForRecord(record)
}
