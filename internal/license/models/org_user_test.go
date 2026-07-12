//go:build integration && server
// +build integration,server

package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestOrgUserCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "OrgUser Co", CustomerID: "CUST-ORGUSER"}
		require.NoError(t, database.DB.Create(org).Error)

		t.Run("create org user", func(t *testing.T) {
			u := &models.OrgUser{
				OrganizationID: org.ID,
				CognitoSub:     "sub-abc-123",
				Email:          "admin@januspk.com",
				Role:           models.OrgUserRoleCustomerAdmin,
				Source:         models.OrgUserSourceInvited,
			}
			require.NoError(t, database.DB.Create(u).Error)
			assert.NotZero(t, u.ID)
			assert.NotZero(t, u.CreatedAt)
			assert.True(t, u.IsActive())
			assert.True(t, u.IsAdmin())
		})

		t.Run("read by cognito_sub", func(t *testing.T) {
			var u models.OrgUser
			require.NoError(t, database.DB.Where("cognito_sub = ?", "sub-abc-123").First(&u).Error)
			assert.Equal(t, "admin@januspk.com", u.Email)
			assert.Equal(t, org.ID, u.OrganizationID)
		})

		t.Run("cognito_sub is unique", func(t *testing.T) {
			dup := &models.OrgUser{
				OrganizationID: org.ID,
				CognitoSub:     "sub-abc-123", // same sub
				Email:          "other@januspk.com",
				Role:           models.OrgUserRoleMember,
				Source:         models.OrgUserSourceSSO,
			}
			err := database.DB.Create(dup).Error
			assert.Error(t, err, "duplicate cognito_sub must be rejected")
		})

		t.Run("soft deactivate", func(t *testing.T) {
			var u models.OrgUser
			require.NoError(t, database.DB.Where("cognito_sub = ?", "sub-abc-123").First(&u).Error)
			require.NoError(t, database.DB.Model(&models.OrgUser{}).Where("id = ?", u.ID).Update("deactivated_at", "NOW()").Error)

			var reloaded models.OrgUser
			require.NoError(t, database.DB.First(&reloaded, u.ID).Error)
			assert.NotNil(t, reloaded.DeactivatedAt)
			assert.False(t, reloaded.IsActive())
		})

		t.Run("list active members of an org", func(t *testing.T) {
			member := &models.OrgUser{
				OrganizationID: org.ID,
				CognitoSub:     "sub-member-1",
				Email:          "member@knomix.io",
				Role:           models.OrgUserRoleMember,
				Source:         models.OrgUserSourceSSO,
			}
			require.NoError(t, database.DB.Create(member).Error)

			var users []models.OrgUser
			require.NoError(t, database.DB.
				Where("organization_id = ? AND deactivated_at IS NULL", org.ID).
				Find(&users).Error)
			assert.GreaterOrEqual(t, len(users), 1)
		})
	})
}
