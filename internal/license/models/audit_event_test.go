//go:build integration && server
// +build integration,server

package models_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestAuditEvent(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Audit Co", CustomerID: "CUST-AUDIT"}
		require.NoError(t, database.DB.Create(org).Error)
		actor := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-audit", Email: "actor@januspk.com", Role: models.OrgUserRoleCustomerAdmin, Source: models.OrgUserSourceInvited}
		require.NoError(t, database.DB.Create(actor).Error)

		t.Run("insert assigns UUIDv7 and stores body", func(t *testing.T) {
			rid := "42"
			e := &models.AuditEvent{
				OrganizationID: &org.ID,
				ActorOrgUserID: &actor.ID,
				ActorLabel:     actor.Email,
				Resource:       models.AuditResourceLicense,
				Action:         models.AuditActionAllocate,
				ResourceID:     &rid,
				RequestMethod:  "POST",
				RequestPath:    "/api/v1/orgs/1/license-requests/5/approve",
				ResultStatus:   200,
				Body:           models.JSONObject{"agreement_id": float64(42), "note": "User X assigned license Y"},
			}
			require.NoError(t, database.DB.Create(e).Error)
			assert.NotEqual(t, uuid.Nil, e.ID, "UUIDv7 assigned by BeforeCreate")
			assert.False(t, e.OccurredAt.IsZero())
		})

		t.Run("JSONB body containment query (GIN)", func(t *testing.T) {
			var count int64
			require.NoError(t, database.DB.Model(&models.AuditEvent{}).
				Where(`body @> ?`, `{"agreement_id": 42}`).
				Count(&count).Error)
			assert.GreaterOrEqual(t, count, int64(1))
		})

		t.Run("query by resource + action", func(t *testing.T) {
			var events []models.AuditEvent
			require.NoError(t, database.DB.
				Where("resource = ? AND action = ?", models.AuditResourceLicense, models.AuditActionAllocate).
				Find(&events).Error)
			assert.GreaterOrEqual(t, len(events), 1)
			assert.Equal(t, "actor@januspk.com", events[0].ActorLabel)
		})
	})
}
