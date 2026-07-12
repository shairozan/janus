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

func TestLicenseRequestCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Req Co", CustomerID: "CUST-REQ"}
		require.NoError(t, database.DB.Create(org).Error)
		license := &models.License{Name: "Req Lic", Tier: "pro", Features: models.Features{"local"}, MSRPCents: 5000}
		require.NoError(t, database.DB.Create(license).Error)
		agr := &models.Agreement{OrganizationID: org.ID, LicenseID: license.ID, Seats: 5, PricePerSeatCents: 5000, LicenseModel: "subscription"}
		require.NoError(t, database.DB.Create(agr).Error)
		user := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-req", Email: "req@januspk.com", Role: models.OrgUserRoleMember, Source: models.OrgUserSourceSSO}
		require.NoError(t, database.DB.Create(user).Error)

		t.Run("create pending request without a key", func(t *testing.T) {
			r := &models.LicenseRequest{
				OrganizationID: org.ID,
				OrgUserID:      user.ID,
				AgreementID:    agr.ID,
				Status:         models.LicenseRequestStatusPending,
			}
			require.NoError(t, database.DB.Create(r).Error)
			assert.NotZero(t, r.ID)
			assert.False(t, r.IsApprovable(), "no key yet → not approvable")
		})

		t.Run("attach key and approve", func(t *testing.T) {
			key := &models.UserPublicKey{OrgUserID: user.ID, PublicKeyPEM: "PEM", Fingerprint: "SHA256:req"}
			require.NoError(t, database.DB.Create(key).Error)

			var r models.LicenseRequest
			require.NoError(t, database.DB.Where("org_user_id = ?", user.ID).First(&r).Error)

			now := time.Now()
			decision := models.LicenseRequestDecisionManual
			require.NoError(t, database.DB.Model(&r).Updates(map[string]interface{}{
				"user_public_key_id": key.ID,
				"status":             models.LicenseRequestStatusApproved,
				"decision":           decision,
				"decided_by_user_id": user.ID,
				"decided_at":         now,
			}).Error)

			var reloaded models.LicenseRequest
			require.NoError(t, database.DB.First(&reloaded, r.ID).Error)
			assert.True(t, reloaded.IsApprovable())
			assert.Equal(t, models.LicenseRequestStatusApproved, reloaded.Status)
			require.NotNil(t, reloaded.Decision)
			assert.Equal(t, models.LicenseRequestDecisionManual, *reloaded.Decision)
		})
	})
}
