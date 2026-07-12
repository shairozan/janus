//go:build integration && server
// +build integration,server

package sso_test

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/cognito/cognitotest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/sso"
)

func loginURL(provider string) string {
	return "https://auth.januspk.com/oauth2/authorize?identity_provider=" + provider
}

func TestSSOSetup(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "SSO Co", CustomerID: "CUST-SSOSVC"}
		require.NoError(t, database.DB.Create(org).Error)

		enc, err := keys.NewEncryptor([]byte("0123456789abcdef0123456789abcdef"))
		require.NoError(t, err)

		fake := cognitotest.New()
		svc := sso.NewService(database, fake, enc, "us-east-2_pool", loginURL, log.Default())
		ctx := context.Background()

		in := sso.SetupInput{
			OrganizationID: org.ID,
			ProviderName:   "knomix-okta",
			OIDCIssuer:     "https://knomix.okta.com",
			ClientID:       "cid",
			ClientSecret:   "shhh",
			AttributeMap:   map[string]string{"email": "email"},
		}

		t.Run("provisions IdP, encrypts secret, builds login URL", func(t *testing.T) {
			cfg, setupErr := svc.Setup(ctx, in)
			require.NoError(t, setupErr)
			assert.True(t, fake.IdPs["knomix-okta"], "IdP provisioned in Cognito")
			assert.Equal(t, models.SSOStatusActive, cfg.CognitoStatus)
			assert.Contains(t, cfg.LoginURL, "identity_provider=knomix-okta")
			assert.True(t, cfg.HasClientSecret())

			// secret stored encrypted (not plaintext)
			require.NotNil(t, cfg.OIDCClientSecretEncrypted)
			assert.NotEqual(t, "shhh", *cfg.OIDCClientSecretEncrypted)
			dec, decErr := enc.Decrypt(*cfg.OIDCClientSecretEncrypted)
			require.NoError(t, decErr)
			assert.Equal(t, "shhh", dec)
		})

		t.Run("create-once: second setup is rejected", func(t *testing.T) {
			_, setupErr := svc.Setup(ctx, in)
			assert.Error(t, setupErr)
		})

		t.Run("get returns the config", func(t *testing.T) {
			cfg, getErr := svc.Get(ctx, org.ID)
			require.NoError(t, getErr)
			assert.Equal(t, "knomix-okta", cfg.ProviderName)
			assert.Equal(t, "us-east-2_pool", cfg.UserPoolID)
		})
	})

	t.Run("rejects over-long provider name", func(t *testing.T) {
		svc := sso.NewService(nil, cognitotest.New(), nil, "p", loginURL, log.Default())
		_, err := svc.Setup(context.Background(), sso.SetupInput{
			OrganizationID: 1, ProviderName: "this-name-is-definitely-longer-than-32-chars", OIDCIssuer: "https://x", ClientID: "c",
		})
		assert.Error(t, err)
	})
}
