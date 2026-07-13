//go:build unit
// +build unit

package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

// newSignedStore returns a store with a signer configured, plus the public key PEM.
func newSignedStore(t *testing.T) (*RunLogStore, string) {
	t.Helper()

	dir := t.TempDir()
	privateKey, publicKey := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	store := NewRunLogStore(dir, "model.mod")
	require.NoError(t, store.Load())
	store.SetSigner(signer, "johnny@example.com")

	return store, encodePublicKeyPEM(t, publicKey)
}

func TestDraftIsNotSigned(t *testing.T) {
	store, _ := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))

	require.False(t, record.Sealed, "a running record is a draft")
	require.Empty(t, record.Signature, "drafts must not be signed — signing is the LAST thing that happens")
	require.Zero(t, record.Sequence, "drafts hold no chain position")
}

func TestSealAssignsChainPositionAndSigns(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	first := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(first))

	require.True(t, first.Sealed)
	require.Equal(t, 1, first.Sequence, "the genesis record is sequence 1")
	require.Empty(t, first.PrevHash, "genesis has no predecessor")
	require.NotEmpty(t, first.Signature)
	require.NoError(t, VerifyRecord(first, publicKeyPEM))

	second := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(second))

	require.Equal(t, 2, second.Sequence)

	firstHash, err := RecordHash(first)
	require.NoError(t, err)
	require.Equal(t, firstHash, second.PrevHash, "record 2 must chain to record 1")
}

func TestSealAdvancesTheHead(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	head, err := ReadHead(store.headPath())
	require.NoError(t, err)
	require.NotNil(t, head, "sealing must write a head")
	require.Equal(t, 1, head.Sequence)

	recordHash, err := RecordHash(record)
	require.NoError(t, err)
	require.Equal(t, recordHash, head.TipHash)
	require.NoError(t, VerifyHead(head, publicKeyPEM), "the head must be signed")
}

func TestUpdateRunOnSealedRecordIsRefused(t *testing.T) {
	store, _ := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))
	require.True(t, record.Sealed)

	record.Status = "failed"
	err := store.UpdateRun(record)

	require.Error(t, err, "a sealed record is immutable — corrections are amendments, not rewrites")
	require.Contains(t, err.Error(), "sealed")
}

func TestDraftUpdatedToTerminalGetsSealed(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	// This is the production path: record the run when it starts...
	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))
	require.Empty(t, record.Signature)

	// ...then record the outcome when it finishes.
	record.Status = "completed"
	record.ExitCode = 0
	require.NoError(t, store.UpdateRun(record))

	require.True(t, record.Sealed)
	require.Equal(t, 1, record.Sequence)
	require.NotEmpty(t, record.Signature)

	reloaded, err := store.GetRun(record.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", reloaded.Status)
	require.NoError(t, VerifyRecord(reloaded, publicKeyPEM),
		"the completed run must verify — this is the bug the whole task exists to fix")
}

// TestUpdateRunSealedCheckIsDiskAuthoritative reproduces the real GUI + daemon
// topology: two RunLogStore instances (internal/gui/app.go and
// internal/mcpservice/ each construct their own store) pointed at the same
// run log directory.
//
// storeA writes a draft. storeB — with no idea storeA exists — loads that same
// draft and seals it (sequence 1). storeA then flips its OWN stale in-memory
// copy to "completed" and calls UpdateRun. Because UpdateRun's "am I sealed?"
// guard must answer from the file, not from storeA's own cache (which still
// thinks the record is an unsealed draft, since storeA never saw storeB's
// write), this MUST be refused. Before the fix, storeA's cache hit made the
// guard blind to storeB's seal, and storeA would happily re-seal the same ID
// at sequence 2, silently overwriting what storeB sealed and dangling any
// record chained to the original sequence-1 hash.
func TestUpdateRunSealedCheckIsDiskAuthoritative(t *testing.T) {
	dir := t.TempDir()

	privateKey, _ := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	storeA := NewRunLogStore(dir, "model.mod")
	require.NoError(t, storeA.Load())
	storeA.SetSigner(signer, "johnny@example.com")

	storeB := NewRunLogStore(dir, "model.mod")
	require.NoError(t, storeB.Load())
	storeB.SetSigner(signer, "johnny@example.com")

	// storeA creates a running draft.
	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, storeA.AddRun(record))
	require.False(t, record.Sealed)

	// storeB independently loads the SAME record and seals it to "completed"
	// (sequence 1). storeA is never told.
	viaB, err := storeB.GetRun(record.ID)
	require.NoError(t, err)
	viaB.Status = "completed"
	require.NoError(t, storeB.UpdateRun(viaB))
	require.True(t, viaB.Sealed)
	require.Equal(t, 1, viaB.Sequence)

	// storeA, unaware, flips its OWN stale draft copy and tries to seal it too.
	record.Status = "completed"
	err = storeA.UpdateRun(record)

	require.Error(t, err, "storeA must not be able to re-seal a record storeB already sealed")
	require.Contains(t, err.Error(), "sealed")

	// The record on disk must still be storeB's sealed version — sequence 1
	// and storeB's signature — never overwritten by storeA's stale attempt.
	data, err := os.ReadFile(filepath.Join(dir, ".janus", "runlog", record.ID+".json"))
	require.NoError(t, err)

	var onDisk RunRecord
	require.NoError(t, json.Unmarshal(data, &onDisk))

	require.Equal(t, 1, onDisk.Sequence, "the sealed sequence on disk must remain storeB's")
	require.Equal(t, viaB.Signature, onDisk.Signature, "the signature on disk must remain storeB's")
}

// TestSealRollsBackRecordWhenHeadWriteFails exercises the rollback path: if the
// record file is sealed and written successfully but the subsequent head
// checkpoint write fails, the record file must not be left behind sealed while
// the head still points at the previous sequence — that would orphan it
// outside the chain forever, because the next seal would read the stale head
// and hand this same sequence number to a DIFFERENT record.
//
// WriteHead (internal/runlog/chain.go) writes to headPath+".tmp" before
// renaming it onto headPath. Placing a DIRECTORY at the ".tmp" path makes that
// write fail deterministically, without disturbing headPath itself — which
// matters here because this is a genesis seal, and nextChainLocked's ReadHead
// call (against headPath, not headPath+".tmp") must still succeed for the
// record to reach writeRunFileLocked in the first place.
func TestSealRollsBackRecordWhenHeadWriteFails(t *testing.T) {
	store, _ := newSignedStore(t)

	require.NoError(t, os.MkdirAll(store.headPath()+".tmp", 0755))

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	err := store.AddRun(record)
	require.Error(t, err, "a failed head write must surface as an error, never be silently swallowed")

	entries, err := os.ReadDir(store.runlogDir())
	require.NoError(t, err)

	for _, e := range entries {
		if e.IsDir() {
			continue // e.g. the head.json.tmp directory this test deliberately created
		}

		require.NotEqual(t, record.ID+".json", e.Name(),
			"the sealed record file must be rolled back, not orphaned outside the chain "+
				"when the head write fails")
	}
}

// TestSealRestoresExistingDraftWhenHeadWriteFails covers the production path
// TestSealRollsBackRecordWhenHeadWriteFails does not: a record that was
// already persisted as an unsigned DRAFT by AddRun (the "running" -> later
// "completed" lifecycle, not a record that arrives already terminal). Before
// this test's fix, sealLocked's rollback unconditionally os.Remove'd the
// record file on a WriteHead failure — correct only when nothing existed
// before the seal attempt. On this path it instead destroyed the
// already-valid, already-persisted draft while index.json still claimed the
// run existed: an audit record silently gone on a transient I/O failure.
//
// The directory trap is placed at headPath()+".tmp", not headPath() itself.
// Placing it at headPath() would make ReadHead (called by nextChainLocked at
// the top of sealLocked, before the record file is ever touched) fail first,
// so the seal would abort before reaching writeRunFileLocked and this test
// would pass trivially against both the broken and the fixed code. Matching
// TestSealRollsBackRecordWhenHeadWriteFails's proven approach, headPath()+
// ".tmp" lets ReadHead succeed (genesis, no head file yet) so the draft is
// actually overwritten by the sealed version, and only then does WriteHead's
// own temp-file write fail.
func TestSealRestoresExistingDraftWhenHeadWriteFails(t *testing.T) {
	store, _ := newSignedStore(t)

	// AddRun persists a "running" record as an unsigned draft — this is the
	// file that must survive.
	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))
	require.False(t, record.Sealed)

	recordPath := filepath.Join(store.runlogDir(), record.ID+".json")

	draftBefore, err := os.ReadFile(recordPath)
	require.NoError(t, err)

	// Force WriteHead's own temp-file write to fail deterministically.
	require.NoError(t, os.MkdirAll(store.headPath()+".tmp", 0755))

	record.Status = "completed"
	err = store.UpdateRun(record)
	require.Error(t, err, "a failed head write must surface as an error, never be silently swallowed")

	// Read the ON-DISK state directly through a second store instance, not
	// store.GetRun: loadRunLocked can serve from s.cache, which would mask
	// the very bug this test exists to catch (deletion of the file on disk).
	secondStore := NewRunLogStore(store.baseDir, "model.mod")
	require.NoError(t, secondStore.Load())

	reloaded, err := secondStore.GetRun(record.ID)
	require.NoError(t, err, "the draft must still be readable — the audit record must not be destroyed")

	require.Equal(t, "running", reloaded.Status, "the restored file must be the pre-seal draft")
	require.False(t, reloaded.Sealed)
	require.Empty(t, reloaded.Signature)

	draftAfter, err := os.ReadFile(recordPath)
	require.NoError(t, err)
	require.Equal(t, draftBefore, draftAfter, "the restored bytes must exactly match the pre-seal draft")
}

// TestSealRestoresRecordWhenRecordWriteFails pins the failure branch that had no
// rollback at all: the record is already Sealed, sequenced and signed when
// writeRunFileLocked runs, so a disk failure (ENOSPC, EACCES, read-only FS) left
// memory AND s.cache — which holds the caller's very pointer — claiming a sealed,
// signed, sequence-N record while disk still held the unsigned "running" draft.
// Worse, sealLocked's in-memory "already sealed" guard then refused EVERY retry, so
// once the transient error cleared the run's completion could never be recorded for
// the life of the process: the audit trail kept a "running" draft forever and the
// outcome was lost.
//
// The fault is injected by making the runlog directory read-only, which makes
// writeFileAtomic's temp-file write fail deterministically while leaving every read
// (ReadHead, the pre-seal byte snapshot, the flock file, which already exists) working.
func TestSealRestoresRecordWhenRecordWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits, so the write cannot be made to fail this way")
	}

	store, publicKeyPEM := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))
	require.False(t, record.Sealed)

	// t.TempDir cleanup needs the directory writable again regardless of outcome.
	t.Cleanup(func() { _ = os.Chmod(store.runlogDir(), 0755) })
	require.NoError(t, os.Chmod(store.runlogDir(), 0500))

	record.Status = "completed"
	err := store.UpdateRun(record)
	require.Error(t, err, "a failed record write must surface as an error, never be silently swallowed")

	// The caller's struct must be back to its pre-seal state — nothing may claim a
	// chain position or a signature that never reached disk.
	require.False(t, record.Sealed, "a record that was never written must not stay sealed in memory")
	require.Zero(t, record.Sequence, "the chain position must be released")
	require.Empty(t, record.PrevHash)
	require.Empty(t, record.Signature, "the signature must be cleared")
	require.Empty(t, record.SignerFingerprint)
	require.Nil(t, record.SignedAt)

	// The store must not serve a sealed record from its cache — the cache holds the
	// caller's same pointer on this path.
	cached, err := store.GetRun(record.ID)
	require.NoError(t, err)
	require.False(t, cached.Sealed, "GetRun must not serve a sealed record that exists nowhere on disk")

	// Disk truth, read through a second store instance so no cache can mask it.
	diskStore := NewRunLogStore(store.baseDir, "model.mod")
	require.NoError(t, diskStore.Load())

	onDisk, err := diskStore.GetRun(record.ID)
	require.NoError(t, err, "the pre-seal draft must survive")
	require.False(t, onDisk.Sealed)
	require.Equal(t, "running", onDisk.Status)

	head, err := ReadHead(store.headPath())
	require.NoError(t, err)
	require.Nil(t, head, "no head may be written for a seal that failed")

	// The retry must not be permanently blocked: once the fault clears, the run's
	// completion must still be recordable.
	require.NoError(t, os.Chmod(store.runlogDir(), 0755))

	record.Status = "completed"
	require.NoError(t, store.UpdateRun(record), "a transient write failure must not block the retry forever")

	require.True(t, record.Sealed)
	require.Equal(t, 1, record.Sequence)
	require.NoError(t, VerifyRecord(record, publicKeyPEM))
}

// TestSealDoesNotOrphanRecordWhenHeadSigningFails asserts the invariant that must
// hold after ANY failed seal, at every fault point between the record mutation and
// the head write: head.json and the record files agree — there is never a sealed
// record on disk (or served by the store) above the head's sequence, and the
// sequence the failed seal reserved is not consumed, so the NEXT record takes it
// exactly once.
//
// A SignHead failure cannot be injected without adding a production seam whose only
// purpose is to be mocked (signing.Signer holds the parsed key in memory), so the
// invariant is asserted directly, over both faults that CAN be injected on either
// side of the head signing: the record write (read-only directory) and the head
// write (a directory squatting on head.json.tmp). Before the fix the record-write
// case broke the invariant — the store went on serving a sealed sequence-2 record
// that existed nowhere on disk while the head still said 1.
func TestSealDoesNotOrphanRecordWhenHeadSigningFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits, so the record write cannot be made to fail")
	}

	faults := map[string]struct {
		inject func(t *testing.T, store *RunLogStore)
		clear  func(t *testing.T, store *RunLogStore)
	}{
		"record write fails": {
			inject: func(t *testing.T, store *RunLogStore) {
				t.Cleanup(func() { _ = os.Chmod(store.runlogDir(), 0755) })
				require.NoError(t, os.Chmod(store.runlogDir(), 0500))
			},
			clear: func(t *testing.T, store *RunLogStore) {
				require.NoError(t, os.Chmod(store.runlogDir(), 0755))
			},
		},
		"head write fails": {
			inject: func(t *testing.T, store *RunLogStore) {
				require.NoError(t, os.MkdirAll(store.headPath()+".tmp", 0755))
			},
			clear: func(t *testing.T, store *RunLogStore) {
				require.NoError(t, os.RemoveAll(store.headPath()+".tmp"))
			},
		},
	}

	for name, fault := range faults {
		t.Run(name, func(t *testing.T) {
			store, publicKeyPEM := newSignedStore(t)

			// A real chain to lag behind: one sealed record, head at sequence 1.
			first := &RunRecord{ModelFile: "model.mod", Status: "completed"}
			require.NoError(t, store.AddRun(first))
			require.Equal(t, 1, first.Sequence)

			// A second run, in flight as a draft, whose seal will fail.
			second := &RunRecord{ModelFile: "model.mod", Status: "running"}
			require.NoError(t, store.AddRun(second))

			fault.inject(t, store)

			second.Status = "completed"
			require.Error(t, store.UpdateRun(second), "the failed seal must surface as an error")

			fault.clear(t, store)

			// Invariant 1: the head has not moved.
			head, err := ReadHead(store.headPath())
			require.NoError(t, err)
			require.NotNil(t, head)
			require.Equal(t, 1, head.Sequence, "a failed seal must not advance the head")
			require.NoError(t, VerifyHead(head, publicKeyPEM))

			// Invariant 2: no record on disk is sealed above the head's sequence.
			diskStore := NewRunLogStore(store.baseDir, "model.mod")
			require.NoError(t, diskStore.Load())

			onDisk, err := diskStore.GetRun(second.ID)
			require.NoError(t, err, "the pre-seal draft must survive on disk")
			require.False(t, onDisk.Sealed, "a sealed record must never outlive a failed seal")
			require.Equal(t, "running", onDisk.Status, "disk must hold exactly the pre-seal draft")
			require.LessOrEqual(t, onDisk.Sequence, head.Sequence)

			// Invariant 3: the store's own view agrees — nothing sealed above the head.
			served, err := store.GetRun(second.ID)
			require.NoError(t, err)
			require.False(t, served.Sealed, "the store must not serve a sealed record the head does not cover")
			require.False(t, second.Sealed)
			require.Zero(t, second.Sequence)

			// Invariant 4: the reserved sequence was not consumed — the next seal
			// takes sequence 2 exactly once, chained to the record at sequence 1.
			second.Status = "completed"
			require.NoError(t, store.UpdateRun(second))
			require.Equal(t, 2, second.Sequence)

			firstHash, err := RecordHash(first)
			require.NoError(t, err)
			require.Equal(t, firstHash, second.PrevHash, "the chain must be unbroken")
			require.NoError(t, VerifyRecord(second, publicKeyPEM))
		})
	}
}

// TestSealRefusesWhenDiskSaysSealed pins sealLocked's OWN on-disk re-check, which
// UpdateRun's disk guard would otherwise mask: Seal is exported and reachable
// directly, and two RunLogStore instances (the GUI and the MCP daemon) run over the
// same directory. storeB seals a record; storeA, still holding a stale UNSEALED copy
// of that same record, calls Seal directly. Its own in-memory Sealed flag says false
// and its cache agrees, so only a read of the FILE can refuse this — and it must, or
// storeA would re-seal the same ID at a second sequence, overwriting storeB's sealed
// record and dangling anything chained to it.
func TestSealRefusesWhenDiskSaysSealed(t *testing.T) {
	dir := t.TempDir()

	privateKey, _ := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	storeA := NewRunLogStore(dir, "model.mod")
	require.NoError(t, storeA.Load())
	storeA.SetSigner(signer, "johnny@example.com")

	storeB := NewRunLogStore(dir, "model.mod")
	require.NoError(t, storeB.Load())
	storeB.SetSigner(signer, "johnny@example.com")

	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, storeA.AddRun(record))

	viaB, err := storeB.GetRun(record.ID)
	require.NoError(t, err)
	viaB.Status = "completed"
	require.NoError(t, storeB.UpdateRun(viaB))
	require.True(t, viaB.Sealed)
	require.Equal(t, 1, viaB.Sequence)

	// storeA's stale copy still believes it is an unsealed draft.
	require.False(t, record.Sealed)

	record.Status = "completed"
	err = storeA.Seal(record)

	require.Error(t, err, "Seal must be refused when the FILE says the record is already sealed")
	require.Contains(t, err.Error(), "sealed on disk")

	// storeB's sealed version must still be the one on disk, untouched.
	data, err := os.ReadFile(filepath.Join(dir, ".janus", "runlog", record.ID+".json"))
	require.NoError(t, err)

	var onDisk RunRecord
	require.NoError(t, json.Unmarshal(data, &onDisk))
	require.Equal(t, 1, onDisk.Sequence)
	require.Equal(t, viaB.Signature, onDisk.Signature)

	// And the chain must not have advanced behind storeB's back.
	head, err := ReadHead(storeA.headPath())
	require.NoError(t, err)
	require.Equal(t, 1, head.Sequence)
}

// TestSealUpdatesTheIndex pins that the exported Seal, like AddRun and UpdateRun,
// makes the record visible: a record sealed into the chain but absent from
// index.json is invisible to GetRuns/GetAllRuns and so to the GUI, because
// rebuildIndex only runs when the index is missing or corrupt, never merely
// incomplete.
func TestSealUpdatesTheIndex(t *testing.T) {
	store, _ := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))

	record.Status = "completed"
	require.NoError(t, store.Seal(record))
	require.True(t, record.Sealed)

	// A fresh store loads index.json from disk — nothing in this process's memory
	// can mask a missing or stale index entry.
	reloaded := NewRunLogStore(store.baseDir, "model.mod")
	require.NoError(t, reloaded.Load())

	var entry *RunIndexEntry

	for i := range reloaded.index.Entries {
		if reloaded.index.Entries[i].ID == record.ID {
			entry = &reloaded.index.Entries[i]
		}
	}

	require.NotNil(t, entry, "a record sealed via Seal must be present in the index")
	require.Equal(t, "completed", entry.Status, "the index must reflect the sealed status")
	require.True(t, entry.Signed, "the index must reflect that the record is now signed")
}

func TestUnsignedStoreStillWorks(t *testing.T) {
	// No signer configured: records are written, never sealed, never signed.
	store := NewRunLogStore(t.TempDir(), "model.mod")
	require.NoError(t, store.Load())

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	require.Empty(t, record.Signature)
	require.False(t, record.Sealed, "without a signer there is nothing to seal")
}
