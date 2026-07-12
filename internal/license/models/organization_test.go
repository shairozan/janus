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

func TestOrganizationCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		t.Run("create organization", func(t *testing.T) {
			org := &models.Organization{
				Name:       "Test Org",
				CustomerID: "CUST-001",
			}

			err := database.DB.Create(org).Error
			require.NoError(t, err)
			assert.NotZero(t, org.ID)
			assert.NotZero(t, org.CreatedAt)
		})

		t.Run("read organization by ID", func(t *testing.T) {
			var org models.Organization
			err := database.DB.Where("customer_id = ?", "CUST-001").First(&org).Error
			require.NoError(t, err)
			assert.Equal(t, "Test Org", org.Name)
			assert.Equal(t, "CUST-001", org.CustomerID)
		})

		t.Run("update organization", func(t *testing.T) {
			var org models.Organization
			err := database.DB.Where("customer_id = ?", "CUST-001").First(&org).Error
			require.NoError(t, err)

			org.Name = "Updated Org Name"
			err = database.DB.Save(&org).Error
			require.NoError(t, err)

			// Verify update
			var updated models.Organization
			err = database.DB.First(&updated, org.ID).Error
			require.NoError(t, err)
			assert.Equal(t, "Updated Org Name", updated.Name)
		})

		t.Run("soft delete organization", func(t *testing.T) {
			var org models.Organization
			err := database.DB.Where("customer_id = ?", "CUST-001").First(&org).Error
			require.NoError(t, err)

			// Soft delete using Update
			err = database.DB.Model(&models.Organization{}).Where("id = ?", org.ID).Update("deactivated_at", "NOW()").Error
			require.NoError(t, err)

			// Verify deactivation
			var deactivated models.Organization
			err = database.DB.First(&deactivated, org.ID).Error
			require.NoError(t, err)
			assert.NotNil(t, deactivated.DeactivatedAt)
		})

		t.Run("list active organizations", func(t *testing.T) {
			// Create multiple orgs
			org1 := &models.Organization{Name: "Active Org 1", CustomerID: "CUST-ACTIVE-1"}
			org2 := &models.Organization{Name: "Active Org 2", CustomerID: "CUST-ACTIVE-2"}
			require.NoError(t, database.DB.Create(org1).Error)
			require.NoError(t, database.DB.Create(org2).Error)

			// List active organizations
			var orgs []models.Organization
			err := database.DB.Where("deactivated_at IS NULL").Order("created_at DESC").Find(&orgs).Error
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(orgs), 2)
		})
	})
}
