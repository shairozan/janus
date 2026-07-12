//go:build unit
// +build unit

package runlog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

func TestRecordHashIsStableAndContentSensitive(t *testing.T) {
	a := &RunRecord{ID: "r1", Status: "completed", Sequence: 1}
	b := &RunRecord{ID: "r1", Status: "completed", Sequence: 1}

	ha, err := RecordHash(a)
	require.NoError(t, err)
	hb, err := RecordHash(b)
	require.NoError(t, err)
	require.Equal(t, ha, hb, "identical records must hash identically")
	require.Len(t, ha, 64, "hex sha256 is 64 chars")

	b.Status = "failed"
	hc, err := RecordHash(b)
	require.NoError(t, err)
	require.NotEqual(t, ha, hc, "a content change must change the hash")
}

func TestRecordHashCoversTheSignature(t *testing.T) {
	// PrevHash must chain the previous record's signature too, otherwise an
	// attacker could swap a signature without breaking the chain.
	a := &RunRecord{ID: "r1", Status: "completed", Signature: "sigA"}
	before, err := RecordHash(a)
	require.NoError(t, err)

	a.Signature = "sigB"
	after, err := RecordHash(a)
	require.NoError(t, err)

	require.NotEqual(t, before, after, "RecordHash must cover the signature field")
}

func TestHeadSignVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	privateKey, publicKey := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)
	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)
	publicKeyPEM := encodePublicKeyPEM(t, publicKey)

	h := &Head{Sequence: 7, TipHash: "abc123"}
	require.NoError(t, SignHead(h, signer))
	require.NotEmpty(t, h.Signature)
	require.NotEmpty(t, h.SignerFingerprint)

	require.NoError(t, VerifyHead(h, publicKeyPEM), "a freshly signed head must verify")

	// Truncation attempt: rewind the tip without re-signing.
	h.Sequence = 5
	require.Error(t, VerifyHead(h, publicKeyPEM), "a mutated head must not verify")
}

func TestReadHeadMissingFileIsGenesis(t *testing.T) {
	h, err := ReadHead(filepath.Join(t.TempDir(), "head.json"))
	require.NoError(t, err, "a missing head is genesis, not an error")
	require.Nil(t, h)
}

func TestWriteThenReadHead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "head.json")
	require.NoError(t, WriteHead(path, &Head{Sequence: 3, TipHash: "deadbeef", Signature: "s"}))

	got, err := ReadHead(path)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 3, got.Sequence)
	require.Equal(t, "deadbeef", got.TipHash)
}
