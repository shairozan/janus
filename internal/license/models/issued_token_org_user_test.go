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

// TestIssuedTokenOrgUserLink covers the A1 additions to issued_tokens:
// org_user_id, revoked_reason, and the one-live-token-per-user invariant.
func TestIssuedTokenOrgUserLink(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Tok Co", CustomerID: "CUST-TOK"}
		require.NoError(t, database.DB.Create(org).Error)
		user := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-tok", Email: "tok@januspk.com", Role: models.OrgUserRoleMember, Source: models.OrgUserSourceSSO}
		require.NoError(t, database.DB.Create(user).Error)

		mk := func(jti string) *models.IssuedToken {
			return &models.IssuedToken{
				JTI:            jti,
				OrganizationID: org.ID,
				OrgUserID:      &user.ID,
				UserEmail:      user.Email,
				KeyID:          "key-1",
				IssuedAt:       time.Now(),
				ExpiresAt:      time.Now().Add(365 * 24 * time.Hour),
			}
		}

		t.Run("issue a token linked to an org user", func(t *testing.T) {
			require.NoError(t, database.DB.Create(mk("jti-1")).Error)
		})

		t.Run("second live token for same user is rejected", func(t *testing.T) {
			err := database.DB.Create(mk("jti-2")).Error
			assert.Error(t, err, "one-live-token-per-user invariant must reject a second live token")
		})

		t.Run("revoke with reason, then reissue succeeds", func(t *testing.T) {
			reason := models.RevokedReasonKeyReplaced
			now := time.Now()
			require.NoError(t, database.DB.Model(&models.IssuedToken{}).
				Where("jti = ?", "jti-1").
				Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error)

			require.NoError(t, database.DB.Create(mk("jti-3")).Error, "reissue allowed once prior is revoked")

			var live int64
			require.NoError(t, database.DB.Model(&models.IssuedToken{}).
				Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
				Count(&live).Error)
			assert.Equal(t, int64(1), live)
		})

		t.Run("legacy tokens without org_user_id are unconstrained", func(t *testing.T) {
			// Two NULL-org_user_id tokens must coexist (constraint excludes NULLs).
			for _, jti := range []string{"legacy-1", "legacy-2"} {
				tok := &models.IssuedToken{
					JTI:            jti,
					OrganizationID: org.ID,
					UserEmail:      "legacy@example.com",
					KeyID:          "key-legacy",
					IssuedAt:       time.Now(),
					ExpiresAt:      time.Now().Add(time.Hour),
				}
				require.NoError(t, database.DB.Create(tok).Error)
			}
		})
	})
}
