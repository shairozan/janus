//go:build integration && server
// +build integration,server

package signup_test

import (
	"context"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing/billingtest"
	"github.com/pharmalytica/janus/internal/license/cognito/cognitotest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/signup"
)

func TestSignUp(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		input := signup.Input{OrgName: "Knomix", CustomerID: "CUST-KNOMIX", AdminEmail: "owner@knomix.io"}

		t.Run("happy path provisions Cognito + DB + Stripe", func(t *testing.T) {
			fake := cognitotest.New()
			stripe := billingtest.New()
			svc := signup.NewService(database, fake, stripe, log.Default())

			res, err := svc.SignUp(context.Background(), input)
			require.NoError(t, err)

			// DB: org + admin org user
			require.NotNil(t, res.Organization)
			require.NotNil(t, res.Admin)
			assert.Equal(t, "Knomix", res.Organization.Name)
			assert.Equal(t, models.OrgUserRoleCustomerAdmin, res.Admin.Role)
			assert.Equal(t, models.OrgUserSourceInvited, res.Admin.Source)
			assert.Equal(t, "sub-owner@knomix.io", res.Admin.CognitoSub)

			// Cognito: group ensured, user created, membership added
			assert.True(t, fake.Groups[models.OrgUserRoleCustomerAdmin])
			assert.Contains(t, fake.CreatedUsers, "owner@knomix.io")
			assert.Contains(t, fake.MembershipsOf("owner@knomix.io"), models.OrgUserRoleCustomerAdmin)

			// Stripe: a customer was created and reflected onto the org.
			require.NotNil(t, res.Organization.StripeCustomerID)
			assert.True(t, stripe.Customers[*res.Organization.StripeCustomerID])
			assert.Len(t, stripe.Customers, 1)
		})

		t.Run("idempotent on retry (same admin → no duplicate)", func(t *testing.T) {
			fake := cognitotest.New()
			stripe := billingtest.New()
			svc := signup.NewService(database, fake, stripe, log.Default())

			first, err := svc.SignUp(context.Background(), input)
			require.NoError(t, err)

			second, err := svc.SignUp(context.Background(), input)
			require.NoError(t, err)
			assert.Equal(t, first.Admin.ID, second.Admin.ID, "retry must return the same admin")

			var count int64
			require.NoError(t, database.DB.Model(&models.OrgUser{}).Where("cognito_sub = ?", "sub-owner@knomix.io").Count(&count).Error)
			assert.Equal(t, int64(1), count, "no duplicate OrgUser")
			assert.Empty(t, stripe.Customers, "existing org's Stripe customer must be reused (persisted id), not re-created")
		})
	})
}

func TestSignUp_CognitoFailureAbortsBeforeDB(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fake := cognitotest.New()
		fake.FailOn["EnsureUser"] = errors.New("cognito unavailable")
		svc := signup.NewService(database, fake, billingtest.New(), log.Default())

		_, err := svc.SignUp(context.Background(), signup.Input{OrgName: "Foo", CustomerID: "CUST-FOO", AdminEmail: "a@foo.com"})
		require.Error(t, err)

		var count int64
		require.NoError(t, database.DB.Model(&models.Organization{}).Where("customer_id = ?", "CUST-FOO").Count(&count).Error)
		assert.Equal(t, int64(0), count, "no org should be created if Cognito fails")
	})
}

func TestSignUp_Validation(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		svc := signup.NewService(database, cognitotest.New(), billingtest.New(), log.Default())
		_, err := svc.SignUp(context.Background(), signup.Input{OrgName: "", CustomerID: "", AdminEmail: ""})
		assert.Error(t, err)
	})
}
