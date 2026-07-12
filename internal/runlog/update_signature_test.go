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
// AddRun signs unconditionally (store.go), while UpdateRun re-signs only when
// record.Signature == "" — which it never is, because AddRun just set it. The
// mutated record is therefore written back to disk carrying the signature of its
// pre-mutation self.
//
// If that reading is right, the record reloaded from disk fails verification and
// Janus reports its own normal completion path as tampering.
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

	// 1. The run starts. AddRun signs it.
	record := &RunRecord{
		ModelFile: "model.mod",
		Status:    "running",
	}
	require.NoError(t, store.AddRun(record))
	require.NotEmpty(t, record.Signature, "AddRun should have signed the record")

	signatureAtCreation := record.Signature

	// Sanity: as signed, it verifies.
	require.NoError(t,
		VerifyRecord(record, publicKeyPEM),
		"the record should verify immediately after AddRun signed it")

	// 2. The run finishes. The completion path mutates the record and saves it.
	//    This mirrors internal/gui/app.go:4420-4430 and internal/mcpservice/result.go:80.
	record.Status = "completed"
	record.ExitCode = 0
	require.NoError(t, store.UpdateRun(record))

	// 3. Reload from disk — this is what verification actually sees.
	reloaded, err := store.GetRun(record.ID)
	require.NoError(t, err)

	require.Equal(t, "completed", reloaded.Status,
		"the mutation must have been persisted, otherwise this test proves nothing")

	t.Logf("signature at creation:  %.32s...", signatureAtCreation)
	t.Logf("signature after update: %.32s...", reloaded.Signature)
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
