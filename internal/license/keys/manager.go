package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Manager handles RSA key generation, storage, and rotation.
type Manager struct {
	db        *db.DB
	encryptor *Encryptor
}

// NewManager creates a new key manager with encryption support.
// The encryptionKey should be 32 bytes for AES-256.
func NewManager(database *db.DB, encryptionKey []byte) (*Manager, error) {
	encryptor, err := NewEncryptor(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	return &Manager{
		db:        database,
		encryptor: encryptor,
	}, nil
}

// GenerateMasterKey generates a new master signing key for Janus.
// Master keys are used for per-seat licensing.
func (m *Manager) GenerateMasterKey(keyID string, validityPeriod time.Duration) (*models.SigningKey, error) {
	return m.generateKey(keyID, nil, validityPeriod)
}

// GenerateCustomerKey generates a customer-specific signing key for concurrent licensing.
// Customer keys allow air-gapped customers to sign their own tokens locally.
func (m *Manager) GenerateCustomerKey(keyID string, orgID int64, validityPeriod time.Duration) (*models.SigningKey, error) {
	return m.generateKey(keyID, &orgID, validityPeriod)
}

// generateKey creates an RSA key pair and stores it in the database.
func (m *Manager) generateKey(keyID string, orgID *int64, validityPeriod time.Duration) (*models.SigningKey, error) {
	// Generate RSA key pair (4096 bits for security)
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Encode public key to PEM
	publicKeyPEM, err := encodePublicKeyToPEM(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	// Encode private key to PEM
	privateKeyPEM := encodePrivateKeyToPEM(privateKey)

	// Encrypt private key before storage
	privateKeyEncrypted, err := m.encryptor.Encrypt(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// Create signing key model
	signingKey := &models.SigningKey{
		KeyID:               keyID,
		OrganizationID:      orgID,
		PublicKeyPEM:        publicKeyPEM,
		PrivateKeyEncrypted: &privateKeyEncrypted,
		Algorithm:           "RS256",
		CreatedAt:           time.Now(),
		ExpiresAt:           time.Now().Add(validityPeriod),
	}

	// Store in database
	if err := m.db.CreateSigningKey(signingKey); err != nil {
		return nil, fmt.Errorf("failed to store signing key: %w", err)
	}

	return signingKey, nil
}

// GetActiveSigningKey retrieves the active signing key for an organization (or master key).
func (m *Manager) GetActiveSigningKey(orgID *int64) (*models.SigningKey, error) {
	return m.db.GetActiveSigningKey(orgID)
}

// GetPrivateKey retrieves and decodes a private key from the database.
func (m *Manager) GetPrivateKey(keyID string) (*rsa.PrivateKey, error) {
	signingKey, err := m.db.GetSigningKeyByID(keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve signing key: %w", err)
	}

	if signingKey.IsRevoked() {
		return nil, fmt.Errorf("signing key has been revoked: %s", keyID)
	}

	if signingKey.PrivateKeyEncrypted == nil || *signingKey.PrivateKeyEncrypted == "" {
		return nil, fmt.Errorf("private key not available for key: %s", keyID)
	}

	// Decrypt private key
	privateKeyPEM, err := m.encryptor.Decrypt(*signingKey.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	privateKey, err := decodePrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	return privateKey, nil
}

// GetPublicKey retrieves and decodes a public key from the database.
func (m *Manager) GetPublicKey(keyID string) (*rsa.PublicKey, error) {
	signingKey, err := m.db.GetSigningKeyByID(keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve signing key: %w", err)
	}

	publicKey, err := decodePublicKeyFromPEM(signingKey.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	return publicKey, nil
}

// RotateKey performs key rotation by creating a new key and optionally revoking the old one.
func (m *Manager) RotateKey(newKeyID string, oldKeyID string, orgID *int64, validityPeriod time.Duration) error {
	// Generate new key (this creates it in the database)
	_, err := m.generateKey(newKeyID, orgID, validityPeriod)
	if err != nil {
		return fmt.Errorf("failed to generate new key: %w", err)
	}

	// Revoke old key if one exists (allows grace period for existing tokens)
	if oldKeyID != "" {
		if err := m.db.RevokeSigningKey(oldKeyID); err != nil {
			return fmt.Errorf("failed to revoke old key: %w", err)
		}
	}

	return nil
}

// RevokeKey immediately revokes a key (emergency use only).
func (m *Manager) RevokeKey(keyID string) error {
	return m.db.RevokeSigningKey(keyID)
}

// ExportPublicKeyForEmbedding exports a public key in format suitable for embedding in Janus binary.
func (m *Manager) ExportPublicKeyForEmbedding(keyID string) (string, error) {
	signingKey, err := m.db.GetSigningKeyByID(keyID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve signing key: %w", err)
	}

	return signingKey.PublicKeyPEM, nil
}

// encodePublicKeyToPEM encodes an RSA public key to PEM format.
func encodePublicKeyToPEM(publicKey *rsa.PublicKey) (string, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(publicKeyPEM), nil
}

// encodePrivateKeyToPEM encodes an RSA private key to PEM format.
func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) string {
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return string(privateKeyPEM)
}

// decodePublicKeyFromPEM decodes an RSA public key from PEM format.
func decodePublicKeyFromPEM(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPublicKey, nil
}

// decodePrivateKeyFromPEM decodes an RSA private key from PEM format.
func decodePrivateKeyFromPEM(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}