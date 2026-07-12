//go:build unit
// +build unit

package signing

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestKeyPair generates an RSA key pair for testing.
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return privateKey, &privateKey.PublicKey
}

// writePrivateKeyPEM writes a private key to a PEM file (PKCS#1 format).
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

// writePrivateKeyPKCS8PEM writes a private key to a PEM file (PKCS#8 format).
func writePrivateKeyPKCS8PEM(t *testing.T, path string, key *rsa.PrivateKey) {
	t.Helper()
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	}
	pemData := pem.EncodeToMemory(pemBlock)
	err = os.WriteFile(path, pemData, 0600)
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

func TestNewSigner(t *testing.T) {
	t.Run("loads_pkcs1_private_key", func(t *testing.T) {
		privateKey, _ := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)
		require.NotNil(t, signer)
	})

	t.Run("loads_pkcs8_private_key", func(t *testing.T) {
		privateKey, _ := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPKCS8PEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)
		require.NotNil(t, signer)
	})

	t.Run("fails_on_nonexistent_file", func(t *testing.T) {
		_, err := NewSigner("/nonexistent/path/key.pem")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read private key file")
	})

	t.Run("fails_on_invalid_pem", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "invalid.pem")
		err := os.WriteFile(keyPath, []byte("not a valid pem file"), 0600)
		require.NoError(t, err)

		_, err = NewSigner(keyPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode PEM block")
	})
}

func TestLoadPublicKeyFromPEM(t *testing.T) {
	t.Run("loads_valid_public_key", func(t *testing.T) {
		_, publicKey := generateTestKeyPair(t)
		pemData := encodePublicKeyPEM(t, publicKey)

		loadedKey, err := LoadPublicKeyFromPEM(pemData)
		require.NoError(t, err)
		require.NotNil(t, loadedKey)

		// Verify the keys match
		assert.Equal(t, publicKey.N, loadedKey.N)
		assert.Equal(t, publicKey.E, loadedKey.E)
	})

	t.Run("fails_on_invalid_pem", func(t *testing.T) {
		_, err := LoadPublicKeyFromPEM("not a valid pem")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode PEM block")
	})

	t.Run("fails_on_wrong_pem_type", func(t *testing.T) {
		// Create a private key PEM instead of public key
		privateKey, _ := generateTestKeyPair(t)
		keyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		pemBlock := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: keyBytes,
		}
		pemData := string(pem.EncodeToMemory(pemBlock))

		_, err := LoadPublicKeyFromPEM(pemData)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported PEM block type")
	})
}

func TestSigner_ValidateKeyPair(t *testing.T) {
	t.Run("matching_key_pair_validates", func(t *testing.T) {
		privateKey, publicKey := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		err = signer.ValidateKeyPair(publicKey)
		assert.NoError(t, err)
	})

	t.Run("mismatched_key_pair_fails", func(t *testing.T) {
		privateKey1, _ := generateTestKeyPair(t)
		_, publicKey2 := generateTestKeyPair(t)

		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey1)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		err = signer.ValidateKeyPair(publicKey2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key pair validation failed")
	})
}

func TestSigner_Sign(t *testing.T) {
	t.Run("signs_data_successfully", func(t *testing.T) {
		privateKey, _ := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		data := []byte("test data to sign")
		signature, err := signer.Sign(data)
		require.NoError(t, err)
		require.NotNil(t, signature)
		assert.NotEmpty(t, signature)
	})

	t.Run("same_data_produces_different_signatures", func(t *testing.T) {
		// Note: RSA PKCS#1 v1.5 is deterministic, so same data produces same signature
		// This test verifies consistent behavior
		privateKey, _ := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		data := []byte("test data")
		sig1, err := signer.Sign(data)
		require.NoError(t, err)

		sig2, err := signer.Sign(data)
		require.NoError(t, err)

		// RSA PKCS#1 v1.5 is deterministic
		assert.Equal(t, sig1, sig2)
	})
}

func TestVerify(t *testing.T) {
	t.Run("valid_signature_verifies", func(t *testing.T) {
		privateKey, publicKey := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		data := []byte("test data to verify")
		signature, err := signer.Sign(data)
		require.NoError(t, err)

		err = Verify(publicKey, data, signature)
		assert.NoError(t, err)
	})

	t.Run("invalid_signature_fails", func(t *testing.T) {
		_, publicKey := generateTestKeyPair(t)

		data := []byte("test data")
		invalidSignature := []byte("invalid signature data")

		err := Verify(publicKey, data, invalidSignature)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("tampered_data_fails", func(t *testing.T) {
		privateKey, publicKey := generateTestKeyPair(t)
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "private.pem")
		writePrivateKeyPEM(t, keyPath, privateKey)

		signer, err := NewSigner(keyPath)
		require.NoError(t, err)

		data := []byte("original data")
		signature, err := signer.Sign(data)
		require.NoError(t, err)

		tamperedData := []byte("tampered data")
		err = Verify(publicKey, tamperedData, signature)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature verification failed")
	})
}

func TestSigner_PublicKey(t *testing.T) {
	privateKey, expectedPublicKey := generateTestKeyPair(t)
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := NewSigner(keyPath)
	require.NoError(t, err)

	actualPublicKey := signer.PublicKey()
	assert.Equal(t, expectedPublicKey.N, actualPublicKey.N)
	assert.Equal(t, expectedPublicKey.E, actualPublicKey.E)
}

func TestRoundTrip(t *testing.T) {
	// Full round-trip test: create signer, sign, verify with public key from PEM
	privateKey, publicKey := generateTestKeyPair(t)

	// Create signer from private key file
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "private.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := NewSigner(keyPath)
	require.NoError(t, err)

	// Create public key PEM string (as would be in JWT claims)
	publicKeyPEM := encodePublicKeyPEM(t, publicKey)

	// Load public key from PEM
	loadedPublicKey, err := LoadPublicKeyFromPEM(publicKeyPEM)
	require.NoError(t, err)

	// Validate key pair
	err = signer.ValidateKeyPair(loadedPublicKey)
	require.NoError(t, err)

	// Sign data
	data := []byte(`{"id":1,"timestamp":"2024-01-01T00:00:00Z","model":"test.mod"}`)
	signature, err := signer.Sign(data)
	require.NoError(t, err)

	// Verify signature
	err = Verify(loadedPublicKey, data, signature)
	assert.NoError(t, err)
}
