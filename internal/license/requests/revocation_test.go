//go:build integration && server
// +build integration,server

package requests_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/requests"
)

func TestIsTokenRevoked(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Rev Co", CustomerID: "CUST-REV"}
		require.NoError(t, database.DB.Create(org).Error)

		tok := &models.IssuedToken{
			JTI: "jti-live", OrganizationID: org.ID, UserEmail: "u@x.com",
			KeyID: "k", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, database.DB.Create(tok).Error)

		ctx := context.Background()

		t.Run("active token is not revoked", func(t *testing.T) {
			revoked, err := requests.IsTokenRevoked(ctx, database, "jti-live")
			require.NoError(t, err)
			assert.False(t, revoked)
		})

		t.Run("unknown JTI is not revoked (permissive)", func(t *testing.T) {
			revoked, err := requests.IsTokenRevoked(ctx, database, "jti-does-not-exist")
			require.NoError(t, err)
			assert.False(t, revoked)
		})

		t.Run("revoked token reports revoked", func(t *testing.T) {
			now := time.Now()
			reason := models.RevokedReasonOffboarded
			require.NoError(t, database.DB.Model(&models.IssuedToken{}).
				Where("jti = ?", "jti-live").
				Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error)

			revoked, err := requests.IsTokenRevoked(ctx, database, "jti-live")
			require.NoError(t, err)
			assert.True(t, revoked)
		})
	})
}
