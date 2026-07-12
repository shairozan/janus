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

func TestActiveSeats(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Seat Co", CustomerID: "CUST-SEAT"}
		require.NoError(t, database.DB.Create(org).Error)
		license := &models.License{Name: "Seat Lic", Tier: "pro", Features: models.Features{"local"}, MSRPCents: 5000}
		require.NoError(t, database.DB.Create(license).Error)
		agr := &models.Agreement{OrganizationID: org.ID, LicenseID: license.ID, Seats: 10, PricePerSeatCents: 2400, LicenseModel: "subscription"}
		require.NoError(t, database.DB.Create(agr).Error)

		// Helper: create a user + an issued token + an approved license_request linking them to the agreement.
		seatFor := func(sub string, revoked bool, expiresIn time.Duration) *models.IssuedToken {
			u := &models.OrgUser{OrganizationID: org.ID, CognitoSub: sub, Email: sub + "@x.com", Role: models.OrgUserRoleMember, Source: models.OrgUserSourceSSO}
			require.NoError(t, database.DB.Create(u).Error)

			tok := &models.IssuedToken{
				JTI: "jti-" + sub, OrganizationID: org.ID, OrgUserID: &u.ID, UserEmail: u.Email,
				KeyID: "k", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(expiresIn),
			}
			if revoked {
				now := time.Now()
				tok.RevokedAt = &now
			}
			require.NoError(t, database.DB.Create(tok).Error)

			lr := &models.LicenseRequest{
				OrganizationID: org.ID, OrgUserID: u.ID, AgreementID: agr.ID,
				Status: models.LicenseRequestStatusApproved, IssuedTokenID: &tok.ID,
			}
			require.NoError(t, database.DB.Create(lr).Error)

			return tok
		}

		ctx := context.Background()

		t.Run("counts only active tokens", func(t *testing.T) {
			seatFor("alice", false, time.Hour) // active
			seatFor("bob", false, time.Hour)   // active
			seatFor("carol", true, time.Hour)  // revoked → not counted
			seatFor("dave", false, -time.Hour) // expired → not counted

			n, err := requests.ActiveSeats(ctx, database, agr.ID)
			require.NoError(t, err)
			assert.Equal(t, 2, n)
		})

		t.Run("zero for an agreement with no seats", func(t *testing.T) {
			other := &models.Agreement{OrganizationID: org.ID, LicenseID: license.ID, Seats: 5, PricePerSeatCents: 2400, LicenseModel: "subscription"}
			require.NoError(t, database.DB.Create(other).Error)

			n, err := requests.ActiveSeats(ctx, database, other.ID)
			require.NoError(t, err)
			assert.Equal(t, 0, n)
		})
	})
}
