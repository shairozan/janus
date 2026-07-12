// Package signing provides RSA cryptographic signing for run log entries.
// Users generate RSA key pairs externally and submit the public key when
// requesting a license. The public key is embedded in the JWT license claims.
// Janus validates that the configured private key matches the public key
// in the license at startup, then uses the private key to sign run log entries.
package signing

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Signer handles RSA signing operations for run log entries.
type Signer struct {
	privateKey *rsa.PrivateKey
}

// NewSigner creates a new Signer by loading a private key from a PEM file.
// The path can be absolute or have ~ expanded beforehand.
func NewSigner(privateKeyPath string) (*Signer, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	privateKey, err := ParsePrivateKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Signer{
		privateKey: privateKey,
	}, nil
}

// ParsePrivateKeyFromPEM parses an RSA private key from PEM-encoded data.
// Supports both PKCS#1 (RSA PRIVATE KEY) and PKCS#8 (PRIVATE KEY) formats.
func ParsePrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		// PKCS#1 format
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS#1 private key: %w", err)
		}

		return key, nil

	case "PRIVATE KEY":
		// PKCS#8 format
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS#8 private key: %w", err)
		}

		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}

		return rsaKey, nil

	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s (expected RSA PRIVATE KEY or PRIVATE KEY)", block.Type)
	}
}

// LoadPublicKeyFromPEM parses an RSA public key from a PEM-encoded string.
// This is used to parse the public key embedded in the JWT license claims.
func LoadPublicKeyFromPEM(pemData string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("unsupported PEM block type: %s (expected PUBLIC KEY)", block.Type)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return rsaPub, nil
}

// ValidateKeyPair verifies that the signer's private key matches the given public key.
// This is done by signing test data with the private key and verifying with the public key.
func (s *Signer) ValidateKeyPair(publicKey *rsa.PublicKey) error {
	// Create test data to sign
	testData := []byte("janus-key-pair-validation-test")

	// Sign with private key
	signature, err := s.Sign(testData)
	if err != nil {
		return fmt.Errorf("failed to sign test data: %w", err)
	}

	// Verify with public key
	err = Verify(publicKey, testData, signature)
	if err != nil {
		return fmt.Errorf("key pair validation failed: private key does not match public key")
	}

	return nil
}

// Sign creates an RSA-SHA256 signature for the given data.
// Returns the raw signature bytes (not base64 encoded).
func (s *Signer) Sign(data []byte) ([]byte, error) {
	// Hash the data
	hash := sha256.Sum256(data)

	// Sign the hash
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// Verify checks an RSA-SHA256 signature against the given data and public key.
// This function is useful for testing and will be used for on-demand verification.
func Verify(publicKey *rsa.PublicKey, data []byte, signature []byte) error {
	// Hash the data
	hash := sha256.Sum256(data)

	// Verify the signature
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// PublicKey returns the public key corresponding to the signer's private key.
// This can be used for verification or comparison.
func (s *Signer) PublicKey() *rsa.PublicKey {
	return &s.privateKey.PublicKey
}

// PublicKeyPEM returns the public key as a PEM-encoded string.
// This is useful for embedding in run records for later verification.
func (s *Signer) PublicKeyPEM() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&s.privateKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}

	return string(pem.EncodeToMemory(block)), nil
}

// PublicKeyFingerprint returns the SHA256 fingerprint of the public key.
// The fingerprint is returned as a hex-encoded string.
func (s *Signer) PublicKeyFingerprint() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&s.privateKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	hash := sha256.Sum256(pubKeyBytes)

	return fmt.Sprintf("%x", hash), nil
}
