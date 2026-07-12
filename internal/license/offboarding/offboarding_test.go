//go:build integration && server
// +build integration,server

package offboarding_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/cognito/cognitotest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/offboarding"
)

func TestOffboard(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Off Co", CustomerID: "CUST-OFF"}
		require.NoError(t, database.DB.Create(org).Error)

		seed := func(email, role string) *models.OrgUser {
			u := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-" + email, Email: email, Role: role, Source: models.OrgUserSourceSSO}
			require.NoError(t, database.DB.Create(u).Error)
			require.NoError(t, database.DB.Create(&models.UserPublicKey{OrgUserID: u.ID, PublicKeyPEM: "PEM", Fingerprint: "SHA256:" + email}).Error)
			require.NoError(t, database.DB.Create(&models.IssuedToken{
				JTI: "jti-" + email, OrganizationID: org.ID, OrgUserID: &u.ID, UserEmail: email,
				KeyID: "k", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
			}).Error)

			return u
		}

		ctx := context.Background()

		t.Run("revokes token+key, deactivates user, disables cognito", func(t *testing.T) {
			fake := cognitotest.New()
			svc := offboarding.NewService(database, fake, log.Default())
			user := seed("member@x.com", models.OrgUserRoleMember)

			require.NoError(t, svc.Offboard(ctx, user))

			var tok models.IssuedToken
			require.NoError(t, database.DB.Where("org_user_id = ?", user.ID).First(&tok).Error)
			require.NotNil(t, tok.RevokedAt)
			require.NotNil(t, tok.RevokedReason)
			assert.Equal(t, models.RevokedReasonOffboarded, *tok.RevokedReason)

			var key models.UserPublicKey
			require.NoError(t, database.DB.Where("org_user_id = ?", user.ID).First(&key).Error)
			assert.NotNil(t, key.RevokedAt)

			var reloaded models.OrgUser
			require.NoError(t, database.DB.First(&reloaded, user.ID).Error)
			assert.NotNil(t, reloaded.DeactivatedAt)

			assert.True(t, fake.DisabledUsers["member@x.com"])
		})

		t.Run("admin is removed from customer_admin group", func(t *testing.T) {
			fake := cognitotest.New()
			require.NoError(t, fake.AddUserToGroup(ctx, "admin@x.com", models.OrgUserRoleCustomerAdmin))
			svc := offboarding.NewService(database, fake, log.Default())
			admin := seed("admin@x.com", models.OrgUserRoleCustomerAdmin)

			require.NoError(t, svc.Offboard(ctx, admin))
			assert.NotContains(t, fake.MembershipsOf("admin@x.com"), models.OrgUserRoleCustomerAdmin)
			assert.True(t, fake.DisabledUsers["admin@x.com"])
		})

		t.Run("idempotent re-offboard", func(t *testing.T) {
			fake := cognitotest.New()
			svc := offboarding.NewService(database, fake, log.Default())
			user := seed("again@x.com", models.OrgUserRoleMember)

			require.NoError(t, svc.Offboard(ctx, user))
			require.NoError(t, svc.Offboard(ctx, user), "re-offboarding must be a no-op")
		})
	})
}
