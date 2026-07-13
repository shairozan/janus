//go:build unit
// +build unit

package runlog

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

// generateTestKeyPair generates an RSA key pair for testing.
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return privateKey, &privateKey.PublicKey
}

// writePrivateKeyPEM writes a private key to a PEM file.
func writePrivateKeyPEM(t *testing.T, path string, key *rsa.PrivateKey) {
	t.Helper()
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	pemData := pem.EncodeToMemory(pemBlock)
	err := os.WriteFile(path, pemData, 0600)
	require.NoError(t, err)
}

// encodePublicKeyPEM encodes a public key to PEM string.
func encodePublicKeyPEM(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)
	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}

	return string(pem.EncodeToMemory(pemBlock))
}

// createTestSigner creates a signer for testing.
func createTestSigner(t *testing.T) (*signing.Signer, string) {
	t.Helper()
	privateKey, publicKey := generateTestKeyPair(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	publicKeyPEM := encodePublicKeyPEM(t, publicKey)

	return signer, publicKeyPEM
}

// createTestRecord creates a sample run record for testing.
func createTestRecord() *RunRecord {
	return &RunRecord{
		ID:         "test-run-id",
		Timestamp:  time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		ModelFile:  "acop.mod",
		Command:    "nmfe75 acop.mod acop.lst",
		ExitCode:   0,
		IsParallel: true,
		Cores:      4,
		IsGrid:     false,
		Status:     "completed",
	}
}

func TestSignRecord(t *testing.T) {
	t.Run("signs_record_successfully", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		// Signature should be set
		assert.NotEmpty(t, record.Signature)
	})

	t.Run("signature_is_base64_encoded", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		// Should be valid base64
		_, err = os.ReadFile("/dev/null") // dummy to make linter happy
		_ = err
		// The signature is base64 encoded, verify by checking it's non-empty
		// and doesn't contain non-base64 characters
		assert.NotEmpty(t, record.Signature)
		assert.Regexp(t, `^[A-Za-z0-9+/]+=*$`, record.Signature)
	})

	t.Run("same_record_produces_same_signature", func(t *testing.T) {
		signer, _ := createTestSigner(t)

		record1 := createTestRecord()
		record2 := createTestRecord()

		err := SignRecord(record1, signer)
		require.NoError(t, err)

		err = SignRecord(record2, signer)
		require.NoError(t, err)

		// Same data should produce same signature (RSA PKCS#1 v1.5 is deterministic)
		assert.Equal(t, record1.Signature, record2.Signature)
	})

	t.Run("different_records_produce_different_signatures", func(t *testing.T) {
		signer, _ := createTestSigner(t)

		record1 := createTestRecord()
		record1.ID = "test-uuid-1"

		record2 := createTestRecord()
		record2.ID = "test-uuid-2" // Different ID

		err := SignRecord(record1, signer)
		require.NoError(t, err)

		err = SignRecord(record2, signer)
		require.NoError(t, err)

		// Different data should produce different signatures
		assert.NotEqual(t, record1.Signature, record2.Signature)
	})

	t.Run("signature_excludes_itself", func(t *testing.T) {
		signer, _ := createTestSigner(t)

		// Sign a record
		record := createTestRecord()
		err := SignRecord(record, signer)
		require.NoError(t, err)
		firstSignature := record.Signature

		// Sign again - should get same result because signature field is excluded
		err = SignRecord(record, signer)
		require.NoError(t, err)

		assert.Equal(t, firstSignature, record.Signature)
	})
}

func TestVerifyRecord(t *testing.T) {
	t.Run("valid_signature_verifies", func(t *testing.T) {
		signer, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		err = VerifyRecord(record, publicKeyPEM)
		assert.NoError(t, err)
	})

	t.Run("unsigned_record_fails", func(t *testing.T) {
		_, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()
		// Don't sign it

		err := VerifyRecord(record, publicKeyPEM)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "record has no signature")
	})

	t.Run("tampered_record_fails", func(t *testing.T) {
		signer, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		// Tamper with the record
		record.ExitCode = 999

		err = VerifyRecord(record, publicKeyPEM)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("wrong_public_key_fails_for_legacy_records", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		// Clear the embedded public key to simulate legacy record
		record.SignerPublicKey = ""

		// Use a different public key as fallback
		_, wrongPublicKey := generateTestKeyPair(t)
		wrongPublicKeyPEM := encodePublicKeyPEM(t, wrongPublicKey)

		err = VerifyRecord(record, wrongPublicKeyPEM)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("verifies_against_given_key_not_embedded_key", func(t *testing.T) {
		// VerifyRecord must never trust the key embedded in the record itself —
		// that field is data the record supplies about itself, and treating it as
		// authority is exactly how a forged record vouches for its own
		// authenticity. It verifies against exactly the key the caller passes.
		signer, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)

		// Record has an embedded public key, but it must be irrelevant here.
		assert.NotEmpty(t, record.SignerPublicKey)

		_, wrongPublicKey := generateTestKeyPair(t)
		wrongPublicKeyPEM := encodePublicKeyPEM(t, wrongPublicKey)

		// A wrong key passed explicitly must fail, even though the embedded key
		// (ignored here) would have verified.
		err = VerifyRecord(record, wrongPublicKeyPEM)
		require.Error(t, err)

		// The correct key, passed explicitly, still verifies.
		err = VerifyRecord(record, publicKeyPEM)
		assert.NoError(t, err)
	})
}

func TestSignRecordWithInfo(t *testing.T) {
	t.Run("stores_signer_info", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()
		email := "test@example.com"

		err := SignRecordWithInfo(record, signer, email)
		require.NoError(t, err)

		// All signer info should be set
		assert.NotEmpty(t, record.Signature)
		assert.NotEmpty(t, record.SignerPublicKey)
		assert.NotEmpty(t, record.SignerFingerprint)
		assert.Equal(t, email, record.SignerEmail)
		assert.NotNil(t, record.SignedAt)
	})

	t.Run("empty_email_is_allowed", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecordWithInfo(record, signer, "")
		require.NoError(t, err)

		assert.NotEmpty(t, record.Signature)
		assert.NotEmpty(t, record.SignerPublicKey)
		assert.Empty(t, record.SignerEmail)
	})
}

func TestVerifyRecordStatus(t *testing.T) {
	t.Run("valid_signature_returns_valid_status", func(t *testing.T) {
		signer, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()

		err := SignRecordWithInfo(record, signer, "user@example.com")
		require.NoError(t, err)

		trust, err := NewLicenseTrust(publicKeyPEM, "user@example.com")
		require.NoError(t, err)

		result := VerifyRecordStatus(record, trust)
		assert.Equal(t, VerificationValid, result.Status)
		assert.Contains(t, result.Message, "Signed by user@example.com")
		assert.Equal(t, "user@example.com", result.Signer)
	})

	t.Run("unsigned_record_returns_unsigned_status", func(t *testing.T) {
		record := createTestRecord()

		result := VerifyRecordStatus(record, nil)
		assert.Equal(t, VerificationUnsigned, result.Status)
		assert.Equal(t, "Unsigned", result.Message)
	})

	t.Run("tampered_record_returns_invalid_status", func(t *testing.T) {
		signer, publicKeyPEM := createTestSigner(t)
		record := createTestRecord()

		err := SignRecordWithInfo(record, signer, "user@example.com")
		require.NoError(t, err)

		trust, err := NewLicenseTrust(publicKeyPEM, "user@example.com")
		require.NoError(t, err)

		// Tamper with the record
		record.ExitCode = 999

		result := VerifyRecordStatus(record, trust)
		assert.Equal(t, VerificationInvalid, result.Status)
		assert.Contains(t, result.Message, "invalid")
	})

	t.Run("nil_trust_store_returns_unverifiable_even_with_embedded_key", func(t *testing.T) {
		// A nil trust store must never be treated as "trust the embedded key" —
		// that would resurrect the self-referential fallback this feature closes.
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecord(record, signer)
		require.NoError(t, err)
		require.NotEmpty(t, record.SignerPublicKey)

		result := VerifyRecordStatus(record, nil)
		assert.Equal(t, VerificationUnverifiable, result.Status)
		assert.Contains(t, result.Message, "no trust anchor configured")
	})

	t.Run("unauthorized_key_returns_untrusted_not_unverifiable", func(t *testing.T) {
		signer, _ := createTestSigner(t)
		record := createTestRecord()

		err := SignRecordWithInfo(record, signer, "user@example.com")
		require.NoError(t, err)

		// A trust store that authorizes a completely different key.
		_, otherPublicKey := generateTestKeyPair(t)
		trust, err := NewLicenseTrust(encodePublicKeyPEM(t, otherPublicKey), "someone-else@example.com")
		require.NoError(t, err)

		result := VerifyRecordStatus(record, trust)
		assert.Equal(t, VerificationUntrusted, result.Status)
		assert.Contains(t, result.Message, "UNAUTHORIZED")
	})
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	signer, publicKeyPEM := createTestSigner(t)

	// Create a record with various fields populated
	record := &RunRecord{
		ID:                    "test-uuid-042",
		Timestamp:             time.Now().UTC(),
		ModelFile:             "complex_model.mod",
		Command:               "nmfe75 complex_model.mod complex_model.lst -parafile=parallel.pnm",
		ExitCode:              0,
		IsParallel:            true,
		Cores:                 8,
		IsGrid:                true,
		Status:                "completed",
		StdoutCompressed:      "H4sIAAAAAAAA/0tJTc5IzcnJVwQAAAD//wMA",
		StderrCompressed:      "",
		DescriptionCompressed: "H4sIAAAAAAAA/0tNSyzKTgYAAAD//wMA",
	}

	// Sign
	err := SignRecord(record, signer)
	require.NoError(t, err)

	// Verify
	err = VerifyRecord(record, publicKeyPEM)
	assert.NoError(t, err)
}
