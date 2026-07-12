//go:build integration && server
// +build integration,server

package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestUserPublicKeyCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Key Co", CustomerID: "CUST-KEY"}
		require.NoError(t, database.DB.Create(org).Error)
		user := &models.OrgUser{
			OrganizationID: org.ID,
			CognitoSub:     "sub-key-user",
			Email:          "keyholder@januspk.com",
			Role:           models.OrgUserRoleMember,
			Source:         models.OrgUserSourceSSO,
		}
		require.NoError(t, database.DB.Create(user).Error)

		t.Run("create active key", func(t *testing.T) {
			k := &models.UserPublicKey{
				OrgUserID:    user.ID,
				PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\nAAA\n-----END PUBLIC KEY-----",
				Fingerprint:  "SHA256:aaa",
				Title:        "work laptop",
			}
			require.NoError(t, database.DB.Create(k).Error)
			assert.NotZero(t, k.ID)
			assert.True(t, k.IsActive())
		})

		t.Run("second active key for same user is rejected", func(t *testing.T) {
			k2 := &models.UserPublicKey{
				OrgUserID:    user.ID,
				PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\nBBB\n-----END PUBLIC KEY-----",
				Fingerprint:  "SHA256:bbb",
			}
			err := database.DB.Create(k2).Error
			assert.Error(t, err, "one-active-key invariant must reject a second active key")
		})

		t.Run("revoke then add a new active key succeeds", func(t *testing.T) {
			var active models.UserPublicKey
			require.NoError(t, database.DB.
				Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
				First(&active).Error)

			now := time.Now()
			require.NoError(t, database.DB.Model(&models.UserPublicKey{}).
				Where("id = ?", active.ID).
				Update("revoked_at", now).Error)

			k3 := &models.UserPublicKey{
				OrgUserID:    user.ID,
				PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\nCCC\n-----END PUBLIC KEY-----",
				Fingerprint:  "SHA256:ccc",
			}
			require.NoError(t, database.DB.Create(k3).Error, "a new active key is allowed once the prior is revoked")

			// Exactly one active key remains.
			var count int64
			require.NoError(t, database.DB.Model(&models.UserPublicKey{}).
				Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
				Count(&count).Error)
			assert.Equal(t, int64(1), count)
		})
	})
}
