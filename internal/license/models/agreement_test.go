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

func TestAgreementCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		// Setup org and license
		org := &models.Organization{Name: "CRUD Org", CustomerID: "CUST-CRUD"}
		require.NoError(t, database.DB.Create(org).Error)

		license := &models.License{
			Name:      "CRUD License",
			Tier:      "crud",
			Features:  models.Features{"local"},
			MSRPCents: 5000,
		}
		require.NoError(t, database.DB.Create(license).Error)

		t.Run("create agreement", func(t *testing.T) {
			agreement := &models.Agreement{
				OrganizationID:    org.ID,
				LicenseID:         license.ID,
				Seats:             10,
				PricePerSeatCents: 5000,
				LicenseModel:      "perpetual",
			}

			err := database.DB.Create(agreement).Error
			require.NoError(t, err)
			assert.NotZero(t, agreement.ID)
			assert.NotZero(t, agreement.ActivatedAt)
		})

		t.Run("read agreement by org and license", func(t *testing.T) {
			var agreement models.Agreement
			err := database.DB.Where("organization_id = ? AND license_id = ? AND deactivated_at IS NULL", org.ID, license.ID).First(&agreement).Error
			require.NoError(t, err)
			assert.Equal(t, org.ID, agreement.OrganizationID)
			assert.Equal(t, license.ID, agreement.LicenseID)
			assert.Equal(t, 10, agreement.Seats)
		})

		t.Run("update agreement", func(t *testing.T) {
			var agreement models.Agreement
			err := database.DB.Where("organization_id = ? AND license_id = ?", org.ID, license.ID).First(&agreement).Error
			require.NoError(t, err)

			agreement.Seats = 20
			agreement.PricePerSeatCents = 4500
			err = database.DB.Save(&agreement).Error
			require.NoError(t, err)

			// Verify update
			var updated models.Agreement
			err = database.DB.First(&updated, agreement.ID).Error
			require.NoError(t, err)
			assert.Equal(t, 20, updated.Seats)
			assert.Equal(t, 4500, updated.PricePerSeatCents)
		})

		t.Run("soft delete agreement", func(t *testing.T) {
			var agreement models.Agreement
			err := database.DB.Where("organization_id = ? AND license_id = ?", org.ID, license.ID).First(&agreement).Error
			require.NoError(t, err)

			// Soft delete
			err = database.DB.Model(&models.Agreement{}).Where("id = ?", agreement.ID).Update("deactivated_at", "NOW()").Error
			require.NoError(t, err)

			// Verify deactivation
			var deactivated models.Agreement
			err = database.DB.First(&deactivated, agreement.ID).Error
			require.NoError(t, err)
			assert.NotNil(t, deactivated.DeactivatedAt)
		})

		t.Run("list agreements by organization", func(t *testing.T) {
			// Create multiple agreements
			license2 := &models.License{
				Name:      "Another License",
				Tier:      "another",
				Features:  models.Features{"grid"},
				MSRPCents: 8000,
			}
			require.NoError(t, database.DB.Create(license2).Error)

			agr2 := &models.Agreement{
				OrganizationID:    org.ID,
				LicenseID:         license2.ID,
				Seats:             5,
				PricePerSeatCents: 8000,
				LicenseModel:      "subscription",
			}
			require.NoError(t, database.DB.Create(agr2).Error)

			// List active agreements
			var agreements []models.Agreement
			err := database.DB.Where("organization_id = ? AND deactivated_at IS NULL", org.ID).Find(&agreements).Error
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(agreements), 1) // At least one active (agr2)
		})
	})
}
