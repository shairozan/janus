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

func TestAutoAcceptanceRuleCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Rule Co", CustomerID: "CUST-RULE"}
		require.NoError(t, database.DB.Create(org).Error)
		admin := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-rule-admin", Email: "admin@januspk.com", Role: models.OrgUserRoleCustomerAdmin, Source: models.OrgUserSourceInvited}
		require.NoError(t, database.DB.Create(admin).Error)

		t.Run("create email_domain rule", func(t *testing.T) {
			domain := "januspk.com"
			r := &models.AutoAcceptanceRule{
				OrganizationID:  org.ID,
				RuleType:        models.AutoAcceptanceRuleTypeEmailDomain,
				MatchDomain:     &domain,
				Enabled:         true,
				CreatedByUserID: admin.ID,
			}
			require.NoError(t, database.DB.Create(r).Error)
			assert.NotZero(t, r.ID)
			assert.True(t, r.IsActive())
		})

		t.Run("create seat_threshold rule", func(t *testing.T) {
			max := 50
			r := &models.AutoAcceptanceRule{
				OrganizationID:  org.ID,
				RuleType:        models.AutoAcceptanceRuleTypeSeatThreshold,
				MaxAutoSeats:    &max,
				Enabled:         true,
				CreatedByUserID: admin.ID,
			}
			require.NoError(t, database.DB.Create(r).Error)
			require.NotNil(t, r.MaxAutoSeats)
			assert.Equal(t, 50, *r.MaxAutoSeats)
		})

		t.Run("disabled rule is not active", func(t *testing.T) {
			r := &models.AutoAcceptanceRule{
				OrganizationID:  org.ID,
				RuleType:        models.AutoAcceptanceRuleTypeEmailDomain,
				Enabled:         false,
				CreatedByUserID: admin.ID,
			}
			require.NoError(t, database.DB.Create(r).Error)
			assert.False(t, r.IsActive())
		})
	})
}

func TestSSOConfigurationCRUD(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "SSO Co", CustomerID: "CUST-SSO"}
		require.NoError(t, database.DB.Create(org).Error)

		t.Run("create config with attribute mapping", func(t *testing.T) {
			secret := "enc:abc123"
			cfg := &models.SSOConfiguration{
				OrganizationID:            org.ID,
				ProviderName:              "knomix-okta",
				UserPoolID:                "us-east-2_vFpMlzcTG",
				OIDCIssuer:                "https://knomix.okta.com",
				OIDCClientID:              "client-xyz",
				OIDCClientSecretEncrypted: &secret,
				AttributeMapping:          models.JSONStringMap{"email": "email", "name": "name"},
				CognitoStatus:             models.SSOStatusPending,
			}
			require.NoError(t, database.DB.Create(cfg).Error)
			assert.NotZero(t, cfg.ID)
			assert.True(t, cfg.HasClientSecret())
			assert.Equal(t, "openid email profile", cfg.Scopes, "default scopes applied")
		})

		t.Run("attribute mapping round-trips through JSONB", func(t *testing.T) {
			var cfg models.SSOConfiguration
			require.NoError(t, database.DB.Where("organization_id = ?", org.ID).First(&cfg).Error)
			assert.Equal(t, "email", cfg.AttributeMapping["email"])
			assert.Equal(t, "name", cfg.AttributeMapping["name"])
		})

		t.Run("one config per org (create-once)", func(t *testing.T) {
			dup := &models.SSOConfiguration{
				OrganizationID: org.ID,
				ProviderName:   "second-idp",
				UserPoolID:     "us-east-2_vFpMlzcTG",
				OIDCIssuer:     "https://other.example.com",
				OIDCClientID:   "client-2",
				CognitoStatus:  models.SSOStatusPending,
			}
			err := database.DB.Create(dup).Error
			assert.Error(t, err, "a second SSO config for the same org must be rejected")
		})
	})
}
