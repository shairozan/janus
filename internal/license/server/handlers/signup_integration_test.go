//go:build integration && server
// +build integration,server

package handlers_test

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing/billingtest"
	"github.com/pharmalytica/janus/internal/license/cognito/cognitotest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/server/handlers"
	"github.com/pharmalytica/janus/internal/license/signup"
)

func postSignup(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/signup", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(rec, req)

	return rec
}

func TestSignup_Integration(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		t.Run("happy path provisions org + admin + Stripe", func(t *testing.T) {
			cog := cognitotest.New()
			stripe := billingtest.New()
			svc := signup.NewService(database, cog, stripe, log.Default())
			h := handlers.Signup(svc, log.Default(), writeJSON, writeError)

			rec := postSignup(h, `{"org_name":"Acme Labs","admin_email":"Owner@Acme.io","contact_name":"Jane Roe"}`)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

			var resp handlers.SignupResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, "ok", resp.Status)
			assert.True(t, strings.HasPrefix(resp.CustomerID, "cust-acme-labs-"), "got %q", resp.CustomerID)

			// Cognito: invited native admin + group membership.
			assert.Contains(t, cog.CreatedUsers, "owner@acme.io") // normalized to lowercase
			assert.Contains(t, cog.MembershipsOf("owner@acme.io"), models.OrgUserRoleCustomerAdmin)

			// DB: org persisted with contact/email; Stripe customer stored.
			var org models.Organization
			require.NoError(t, database.DB.Where("customer_id = ?", resp.CustomerID).First(&org).Error)
			require.NotNil(t, org.Email)
			assert.Equal(t, "owner@acme.io", *org.Email)
			require.NotNil(t, org.ContactName)
			assert.Equal(t, "Jane Roe", *org.ContactName)
			require.NotNil(t, org.StripeCustomerID)
			assert.True(t, stripe.Customers[*org.StripeCustomerID])
		})

		t.Run("idempotent repeat: same admin → 201, one user, one customer", func(t *testing.T) {
			cog := cognitotest.New()
			stripe := billingtest.New()
			svc := signup.NewService(database, cog, stripe, log.Default())
			h := handlers.Signup(svc, log.Default(), writeJSON, writeError)

			body := `{"org_name":"Beta Corp","admin_email":"admin@beta.io"}`

			first := postSignup(h, body)
			require.Equal(t, http.StatusCreated, first.Code)

			second := postSignup(h, body)
			require.Equal(t, http.StatusCreated, second.Code)

			// Identical responses — no account-existence oracle. Note the customer_id
			// differs from the first only if a new org was created; the repeat returns
			// the existing org's id, so the two must match.
			assert.JSONEq(t, first.Body.String(), second.Body.String())

			var users int64
			require.NoError(t, database.DB.Model(&models.OrgUser{}).Where("email = ?", "admin@beta.io").Count(&users).Error)
			assert.Equal(t, int64(1), users, "no duplicate admin OrgUser")
			assert.Len(t, stripe.Customers, 1, "no duplicate Stripe customer")
		})

		t.Run("provisioning failure → generic 500 (no internal leak)", func(t *testing.T) {
			cog := cognitotest.New()
			cog.FailOn["EnsureUser"] = errors.New("cognito pool boom: secret-internal-detail")
			svc := signup.NewService(database, cog, billingtest.New(), log.Default())
			h := handlers.Signup(svc, log.Default(), writeJSON, writeError)

			rec := postSignup(h, `{"org_name":"Gamma","admin_email":"a@gamma.io"}`)
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Contains(t, rec.Body.String(), "signup failed")
			assert.NotContains(t, rec.Body.String(), "secret-internal-detail")
		})
	})
}
