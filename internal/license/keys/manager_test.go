//go:build integration && server
// +build integration,server

package keys

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestManager creates a test manager with containerized PostgreSQL database
func setupTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()

	// Create containerized test database
	database, cleanup := testutil.SetupTestDatabase(t)

	// Generate random encryption key
	encryptionKey := make([]byte, 32)
	_, err := rand.Read(encryptionKey)
	require.NoError(t, err, "failed to generate encryption key")

	// Create manager
	manager, err := NewManager(database, encryptionKey)
	require.NoError(t, err, "failed to create manager")

	return manager, cleanup
}

func TestManager_GenerateMasterKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "test-key-1"
	validityPeriod := 365 * 24 * time.Hour

	key, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	assert.Equal(t, keyID, key.KeyID)
	assert.Nil(t, key.OrganizationID)
	assert.NotEmpty(t, key.PublicKeyPEM)
	assert.NotNil(t, key.PrivateKeyEncrypted)
	assert.Equal(t, "RS256", key.Algorithm)
	assert.False(t, key.CreatedAt.IsZero())
	assert.False(t, key.ExpiresAt.IsZero())
}

func TestManager_GenerateMasterKey_DuplicateKeyID(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "duplicate-key"
	validityPeriod := 365 * 24 * time.Hour

	// Generate first key
	_, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	// Attempt to generate key with same ID should fail
	_, err = manager.GenerateMasterKey(keyID, validityPeriod)
	assert.Error(t, err, "should fail when creating duplicate key ID")
	assert.Contains(t, err.Error(), "failed to store signing key")
}

func TestManager_RotateKey_FirstKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	newKeyID := "key-1"
	oldKeyID := "" // No old key for first rotation
	validityPeriod := 365 * 24 * time.Hour

	// First rotation (no existing key)
	err := manager.RotateKey(newKeyID, oldKeyID, nil, validityPeriod)
	require.NoError(t, err)

	// Verify new key was created
	newKey, err := manager.db.GetSigningKeyByID(newKeyID)
	require.NoError(t, err)
	assert.Equal(t, newKeyID, newKey.KeyID)
	assert.Nil(t, newKey.OrganizationID)
}

func TestManager_RotateKey_WithExistingKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	// Create initial key
	oldKeyID := "key-old"
	validityPeriod := 365 * 24 * time.Hour
	_, err := manager.GenerateMasterKey(oldKeyID, validityPeriod)
	require.NoError(t, err)

	// Rotate to new key
	newKeyID := "key-new"
	err = manager.RotateKey(newKeyID, oldKeyID, nil, validityPeriod)
	require.NoError(t, err)

	// Verify new key exists
	newKey, err := manager.db.GetSigningKeyByID(newKeyID)
	require.NoError(t, err)
	assert.Equal(t, newKeyID, newKey.KeyID)

	// Verify old key still exists
	oldKey, err := manager.db.GetSigningKeyByID(oldKeyID)
	require.NoError(t, err)
	assert.Equal(t, oldKeyID, oldKey.KeyID)
}

func TestManager_RotateKey_MultipleRotations(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	validityPeriod := 365 * 24 * time.Hour

	// First key
	key1 := "key-1"
	err := manager.RotateKey(key1, "", nil, validityPeriod)
	require.NoError(t, err, "first rotation should succeed")

	// Second key
	key2 := "key-2"
	err = manager.RotateKey(key2, key1, nil, validityPeriod)
	require.NoError(t, err, "second rotation should succeed")

	// Third key
	key3 := "key-3"
	err = manager.RotateKey(key3, key2, nil, validityPeriod)
	require.NoError(t, err, "third rotation should succeed")

	// Verify all three keys exist
	_, err = manager.db.GetSigningKeyByID(key1)
	assert.NoError(t, err)
	_, err = manager.db.GetSigningKeyByID(key2)
	assert.NoError(t, err)
	_, err = manager.db.GetSigningKeyByID(key3)
	assert.NoError(t, err)
}

func TestManager_RotateKey_PreventsDuplicateKeyID(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	validityPeriod := 365 * 24 * time.Hour

	// Create initial key
	keyID := "duplicate-test-key"
	err := manager.RotateKey(keyID, "", nil, validityPeriod)
	require.NoError(t, err, "first rotation should succeed")

	// Attempt to rotate to same key ID should fail
	err = manager.RotateKey(keyID, keyID, nil, validityPeriod)
	assert.Error(t, err, "should fail when rotating to existing key ID")
	assert.Contains(t, err.Error(), "failed to generate new key")
}

func TestManager_RotateKey_MasterKeyRotation(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	oldKeyID := "master-key-old"
	newKeyID := "master-key-new"
	validityPeriod := 365 * 24 * time.Hour

	// Create initial master key
	_, err := manager.GenerateMasterKey(oldKeyID, validityPeriod)
	require.NoError(t, err)

	// Rotate master key
	err = manager.RotateKey(newKeyID, oldKeyID, nil, validityPeriod)
	require.NoError(t, err)

	// Verify new master key
	newKey, err := manager.db.GetSigningKeyByID(newKeyID)
	require.NoError(t, err)
	assert.Equal(t, newKeyID, newKey.KeyID)
	assert.Nil(t, newKey.OrganizationID, "should be master key")

	// Verify old key was revoked
	oldKey, err := manager.db.GetSigningKeyByID(oldKeyID)
	require.NoError(t, err)
	assert.True(t, oldKey.IsRevoked(), "old key should be revoked")
}

func TestManager_GetPrivateKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "test-private-key"
	validityPeriod := 365 * 24 * time.Hour

	// Generate key
	_, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	// Retrieve private key
	privateKey, err := manager.GetPrivateKey(keyID)
	require.NoError(t, err)
	assert.NotNil(t, privateKey)
	assert.NotNil(t, privateKey.N)
	assert.NotNil(t, privateKey.D)
}

func TestManager_GetPublicKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "test-public-key"
	validityPeriod := 365 * 24 * time.Hour

	// Generate key
	_, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	// Retrieve public key
	publicKey, err := manager.GetPublicKey(keyID)
	require.NoError(t, err)
	assert.NotNil(t, publicKey)
	assert.NotNil(t, publicKey.N)
	assert.NotNil(t, publicKey.E)
}

func TestManager_RevokeKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "test-revoke-key"
	validityPeriod := 365 * 24 * time.Hour

	// Generate key
	_, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	// Revoke key
	err = manager.RevokeKey(keyID)
	require.NoError(t, err)

	// Verify key is revoked
	key, err := manager.db.GetSigningKeyByID(keyID)
	require.NoError(t, err)
	assert.True(t, key.IsRevoked())

	// Attempting to get private key should fail
	_, err = manager.GetPrivateKey(keyID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestManager_ExportPublicKeyForEmbedding(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	keyID := "test-export-key"
	validityPeriod := 365 * 24 * time.Hour

	// Generate key
	_, err := manager.GenerateMasterKey(keyID, validityPeriod)
	require.NoError(t, err)

	// Export public key
	publicKeyPEM, err := manager.ExportPublicKeyForEmbedding(keyID)
	require.NoError(t, err)
	assert.Contains(t, publicKeyPEM, "-----BEGIN PUBLIC KEY-----")
	assert.Contains(t, publicKeyPEM, "-----END PUBLIC KEY-----")
}

func TestManager_GetActiveSigningKey(t *testing.T) {
	manager, cleanup := setupTestManager(t)
	defer cleanup()

	validityPeriod := 365 * 24 * time.Hour

	// Initially no active key
	_, err := manager.GetActiveSigningKey(nil)
	assert.Error(t, err)

	// Create first key
	key1ID := "key-1"
	_, err = manager.GenerateMasterKey(key1ID, validityPeriod)
	require.NoError(t, err)

	// Should get first key as active
	activeKey, err := manager.GetActiveSigningKey(nil)
	require.NoError(t, err)
	assert.Equal(t, key1ID, activeKey.KeyID)

	// Rotate to new key
	key2ID := "key-2"
	err = manager.RotateKey(key2ID, key1ID, nil, validityPeriod)
	require.NoError(t, err)

	// Should get new key as active
	activeKey, err = manager.GetActiveSigningKey(nil)
	require.NoError(t, err)
	assert.Equal(t, key2ID, activeKey.KeyID)
}
