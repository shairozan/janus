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

func TestUnsignedStoreStillWorks(t *testing.T) {
	// No signer configured: records are written, never sealed, never signed.
	store := NewRunLogStore(t.TempDir(), "model.mod")
	require.NoError(t, store.Load())

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	require.Empty(t, record.Signature)
	require.False(t, record.Sealed, "without a signer there is nothing to seal")
}
