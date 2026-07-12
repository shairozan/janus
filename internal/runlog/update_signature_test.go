//go:build unit
// +build unit

package runlog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

// TestUpdateRunPreservesSignatureValidity reproduces the add-then-update path that
// every run takes in production: AddRun records the run when it starts, UpdateRun
// records the outcome when it finishes.
//
// Under the draft/seal lifecycle, AddRun writes a still-running record as an
// unsigned DRAFT (it is not yet an audit fact), and UpdateRun seals it — assigns
// its chain position and signs it — only once it reaches a terminal status. This
// is what makes "sign then mutate" impossible by construction: signing is the
// LAST thing that ever happens to a record.
//
// Previously, AddRun signed unconditionally, and UpdateRun re-signed only when
// record.Signature == "" — which was never true, because AddRun had just set it.
// The mutated record was therefore written back to disk carrying the signature of
// its pre-mutation self, and the record reloaded from disk failed verification:
// Janus reported its own normal completion path as tampering.
func TestUpdateRunPreservesSignatureValidity(t *testing.T) {
	dir := t.TempDir()

	privateKey, publicKey := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "signing.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	publicKeyPEM := encodePublicKeyPEM(t, publicKey)

	store := NewRunLogStore(dir, "model.mod")
	require.NoError(t, store.Load())
	store.SetSigner(signer, "johnny@example.com")

	// 1. The run starts. AddRun writes it as an unsigned draft — it is not yet an
	//    audit fact, so there is nothing to sign.
	record := &RunRecord{
		ModelFile: "model.mod",
		Status:    "running",
	}
	require.NoError(t, store.AddRun(record))
	require.Empty(t, record.Signature, "a running record is a draft and must not be signed yet")
	require.False(t, record.Sealed)

	// 2. The run finishes. The completion path mutates the record and saves it.
	//    This mirrors internal/gui/app.go:4420-4430 and internal/mcpservice/result.go:80.
	record.Status = "completed"
	record.ExitCode = 0
	require.NoError(t, store.UpdateRun(record))

	require.True(t, record.Sealed, "reaching a terminal status must seal the record")
	require.NotEmpty(t, record.Signature, "sealing signs the record")

	// 3. Reload from disk — this is what verification actually sees.
	reloaded, err := store.GetRun(record.ID)
	require.NoError(t, err)

	require.Equal(t, "completed", reloaded.Status,
		"the mutation must have been persisted, otherwise this test proves nothing")

	t.Logf("signature after seal:   %.32s...", reloaded.Signature)
	t.Logf("status on disk:         %s", reloaded.Status)

	// THE CLAIM: the persisted record still verifies. If defect 3 is real, it does not.
	err = VerifyRecord(reloaded, publicKeyPEM)
	require.NoError(t, err,
		"record written by the normal AddRun->UpdateRun completion path must still verify; "+
			"a failure here means Janus signs a record, mutates it, saves it without re-signing, "+
			"and then reports its own output as tampered")

	status := VerifyRecordStatus(reloaded, publicKeyPEM)
	require.Equal(t, VerificationValid, status.Status,
		"a normally-completed run must not render as tampered, got %s", status.Status)
}
