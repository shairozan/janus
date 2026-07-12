//go:build unit
// +build unit

package runlog

import (
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

func TestUnsignedStoreStillWorks(t *testing.T) {
	// No signer configured: records are written, never sealed, never signed.
	store := NewRunLogStore(t.TempDir(), "model.mod")
	require.NoError(t, store.Load())

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	require.Empty(t, record.Signature)
	require.False(t, record.Sealed, "without a signer there is nothing to seal")
}
