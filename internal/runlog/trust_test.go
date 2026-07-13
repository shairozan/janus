//go:build unit
// +build unit

package runlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

func TestForgedRecordIsUntrustedNotValid(t *testing.T) {
	dir := t.TempDir()

	// The legitimate signer.
	goodPriv, goodPub := generateTestKeyPair(t)
	goodPath := filepath.Join(dir, "good.pem")
	writePrivateKeyPEM(t, goodPath, goodPriv)
	goodSigner, err := signing.NewSigner(goodPath)
	require.NoError(t, err)

	// The attacker, with a perfectly valid keypair of their own.
	evilPriv, _ := generateTestKeyPair(t)
	evilPath := filepath.Join(dir, "evil.pem")
	writePrivateKeyPEM(t, evilPath, evilPriv)
	evilSigner, err := signing.NewSigner(evilPath)
	require.NoError(t, err)

	trust, err := NewLicenseTrust(encodePublicKeyPEM(t, goodPub), "johnny@example.com")
	require.NoError(t, err)

	// A record the attacker fabricated and signed with their own key. The signature
	// is cryptographically flawless. The key is simply not ours.
	forged := &RunRecord{ID: "forged", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(forged, evilSigner, "attacker@evil.example"))

	result := VerifyRecordStatus(forged, trust)
	require.Equal(t, VerificationUntrusted, result.Status,
		"a record signed by an unauthorized key must be Untrusted, never Valid")
	require.NotEqual(t, VerificationValid, result.Status)

	// And the genuine article still passes.
	genuine := &RunRecord{ID: "genuine", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(genuine, goodSigner, "johnny@example.com"))
	require.Equal(t, VerificationValid, VerifyRecordStatus(genuine, trust).Status)
}

func TestTamperedRecordIsInvalid(t *testing.T) {
	dir := t.TempDir()
	priv, pub := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, priv)
	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	trust, err := NewLicenseTrust(encodePublicKeyPEM(t, pub), "johnny@example.com")
	require.NoError(t, err)

	record := &RunRecord{ID: "r", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(record, signer, "johnny@example.com"))

	record.Status = "failed" // tamper, without re-signing

	require.Equal(t, VerificationInvalid, VerifyRecordStatus(record, trust).Status)
}

func TestKeyringTrustLoadsFingerprintsFromFile(t *testing.T) {
	dir := t.TempDir()
	_, pub := generateTestKeyPair(t)
	pubPEM := encodePublicKeyPEM(t, pub)

	keyringPath := filepath.Join(dir, "keys.yaml")
	contents := "keys:\n  - email: johnny@example.com\n    public_key: |\n"
	for _, line := range splitLines(pubPEM) {
		contents += "      " + line + "\n"
	}
	require.NoError(t, os.WriteFile(keyringPath, []byte(contents), 0600))

	trust, err := NewKeyringTrust(keyringPath)
	require.NoError(t, err)

	fingerprint, err := fingerprintOfPEM(pubPEM)
	require.NoError(t, err)

	require.True(t, trust.Trusted(fingerprint), "a key in the keyring must be trusted")
	require.False(t, trust.Trusted("not-a-real-fingerprint"))

	identity, ok := trust.Describe(fingerprint)
	require.True(t, ok)
	require.Equal(t, "johnny@example.com", identity.Email)
}

func TestEmptyTrustStoreTrustsNothing(t *testing.T) {
	trust := NewKeySet()
	require.False(t, trust.Trusted("anything"),
		"an empty trust store is not a permissive one")
}

// TestFingerprintOfPEMAgreesWithSigningPackage is the load-bearing assumption of
// the whole trust store: fingerprintOfPEM (here) and signing.Signer's
// PublicKeyFingerprint (used when a record is signed) must produce the SAME
// fingerprint for the SAME key. If they ever disagree, trust.Describe(record.
// SignerFingerprint) never finds anything, and every record silently reports
// Untrusted regardless of who signed it.
func TestFingerprintOfPEMAgreesWithSigningPackage(t *testing.T) {
	dir := t.TempDir()
	priv, _ := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, priv)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	publicKeyPEM, err := signer.PublicKeyPEM()
	require.NoError(t, err)

	wantFingerprint, err := signer.PublicKeyFingerprint()
	require.NoError(t, err)

	gotFingerprint, err := fingerprintOfPEM(publicKeyPEM)
	require.NoError(t, err)

	require.Equal(t, wantFingerprint, gotFingerprint,
		"fingerprintOfPEM must agree with signing.Signer.PublicKeyFingerprint on the same key")
}

// splitLines splits a PEM block into lines for YAML block-scalar indentation.
func splitLines(s string) []string {
	var out []string

	current := ""

	for _, r := range s {
		if r == '\n' {
			if current != "" {
				out = append(out, current)
			}

			current = ""

			continue
		}

		current += string(r)
	}

	if current != "" {
		out = append(out, current)
	}

	return out
}
