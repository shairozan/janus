//go:build integration && server
// +build integration,server

package models_test

import (
	"testing"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		t.Run("create license", func(t *testing.T) {
			license := &models.License{
				Name:      "Test License",
				Tier:      "test",
				Features:  models.Features{"feature1", "feature2"},
				MSRPCents: 9999,
			}

			err := database.DB.Create(license).Error
			require.NoError(t, err)
			assert.NotZero(t, license.ID)
			assert.NotZero(t, license.CreatedAt)
		})

		t.Run("read license by tier", func(t *testing.T) {
			var license models.License
			err := database.DB.Where("tier = ? AND deactivated_at IS NULL", "test").First(&license).Error
			require.NoError(t, err)
			assert.Equal(t, "Test License", license.Name)
			assert.Equal(t, 9999, license.MSRPCents)
			assert.Contains(t, license.Features, "feature1")
		})

		t.Run("update license", func(t *testing.T) {
			var license models.License
			err := database.DB.Where("tier = ?", "test").First(&license).Error
			require.NoError(t, err)

			license.MSRPCents = 12000
			license.Features = models.Features{"feature1", "feature2", "feature3"}
			err = database.DB.Save(&license).Error
			require.NoError(t, err)

			// Verify update
			var updated models.License
			err = database.DB.First(&updated, license.ID).Error
			require.NoError(t, err)
			assert.Equal(t, 12000, updated.MSRPCents)
			assert.Contains(t, updated.Features, "feature3")
		})

		t.Run("soft delete license", func(t *testing.T) {
			var license models.License
			err := database.DB.Where("tier = ?", "test").First(&license).Error
			require.NoError(t, err)

			// Soft delete
			err = database.DB.Model(&models.License{}).Where("id = ?", license.ID).Update("deactivated_at", "NOW()").Error
			require.NoError(t, err)

			// Verify deactivation
			var deactivated models.License
			err = database.DB.First(&deactivated, license.ID).Error
			require.NoError(t, err)
			assert.NotNil(t, deactivated.DeactivatedAt)
		})
	})
}
