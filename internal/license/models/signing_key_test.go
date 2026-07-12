//go:build integration && server
// +build integration,server

package models_test

import (
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSigningKeyCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Key Test Org", CustomerID: "CUST-KEY"}
		require.NoError(t, database.DB.Create(org).Error)

		t.Run("create signing key", func(t *testing.T) {
			privateKey := "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"
			key := &models.SigningKey{
				KeyID:               "test-key-001",
				OrganizationID:      &org.ID,
				PublicKeyPEM:        "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
				PrivateKeyEncrypted: &privateKey,
				ExpiresAt:           time.Now().Add(365 * 24 * time.Hour),
			}

			err := database.CreateSigningKey(key)
			require.NoError(t, err)
			assert.NotZero(t, key.ID)
		})

		t.Run("read signing key by key_id", func(t *testing.T) {
			key, err := database.GetSigningKeyByID("test-key-001")
			require.NoError(t, err)
			assert.Equal(t, "test-key-001", key.KeyID)
			assert.NotNil(t, key.OrganizationID)
			assert.Equal(t, org.ID, *key.OrganizationID)
		})

		t.Run("get active signing key", func(t *testing.T) {
			key, err := database.GetActiveSigningKey(&org.ID)
			require.NoError(t, err)
			assert.Equal(t, "test-key-001", key.KeyID)
			assert.Nil(t, key.RevokedAt)
		})

		t.Run("revoke signing key", func(t *testing.T) {
			err := database.RevokeSigningKey("test-key-001")
			require.NoError(t, err)

			// Verify revocation
			key, err := database.GetSigningKeyByID("test-key-001")
			require.NoError(t, err)
			assert.NotNil(t, key.RevokedAt)
		})

		t.Run("rotate signing key", func(t *testing.T) {
			// Create initial key
			oldPrivKey := "old-priv"
			oldKey := &models.SigningKey{
				KeyID:               "old-key",
				OrganizationID:      &org.ID,
				PublicKeyPEM:        "old-pub",
				PrivateKeyEncrypted: &oldPrivKey,
				ExpiresAt:           time.Now().Add(365 * 24 * time.Hour),
			}
			require.NoError(t, database.CreateSigningKey(oldKey))

			// Rotate to new key
			newPrivKey := "new-priv"
			newKey := &models.SigningKey{
				KeyID:               "new-key",
				OrganizationID:      &org.ID,
				PublicKeyPEM:        "new-pub",
				PrivateKeyEncrypted: &newPrivKey,
				ExpiresAt:           time.Now().Add(365 * 24 * time.Hour),
			}

			err := database.RotateSigningKey(newKey, "old-key")
			require.NoError(t, err)

			// Verify old key is revoked
			oldKeyAfter, err := database.GetSigningKeyByID("old-key")
			require.NoError(t, err)
			assert.NotNil(t, oldKeyAfter.RevokedAt)

			// Verify new key exists and is active
			newKeyAfter, err := database.GetSigningKeyByID("new-key")
			require.NoError(t, err)
			assert.Nil(t, newKeyAfter.RevokedAt)
		})
	})
}
