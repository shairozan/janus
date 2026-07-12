//go:build integration && server
// +build integration,server

package portal_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/portal"
)

// testPubKeyPEM is a real RSA public key (PEM), generated at package init so the
// signing.LoadPublicKeyFromPEM validation in ReplaceKey succeeds.
var testPubKeyPEM = mustGenPubKeyPEM()

func mustGenPubKeyPEM() string {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		panic(err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// fakeIssuer returns a canned license with a unique JTI per call.
type fakeIssuer struct {
	mu  sync.Mutex
	seq int
}

func (f *fakeIssuer) IssueLicenseFromAgreement(_ *models.Agreement, _ string, duration time.Duration, _ string) (*jwt.IssuedLicense, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	jti := fmt.Sprintf("jti-%d", f.seq)

	return &jwt.IssuedLicense{
		Token:     "jwt-" + jti,
		JTI:       jti,
		KeyID:     "test-key",
		ExpiresAt: time.Now().Add(duration),
	}, nil
}

type fakeNotifier struct {
	calls int
	last  string
}

func (n *fakeNotifier) NotifyAllocationFailure(_ context.Context, _ int64, reason string) {
	n.calls++
	n.last = reason
}

func setupOrg(t *testing.T, database *db.DB, seats int) (*models.Organization, *models.Agreement, *models.OrgUser) {
	t.Helper()
	org := &models.Organization{Name: "Portal Co", CustomerID: "CUST-" + t.Name()}
	require.NoError(t, database.DB.Create(org).Error)
	lic := &models.License{Name: "L", Tier: "pro", Features: models.Features{"local"}, MSRPCents: 1000}
	require.NoError(t, database.DB.Create(lic).Error)
	agr := &models.Agreement{OrganizationID: org.ID, LicenseID: lic.ID, Seats: seats, PricePerSeatCents: 100, LicenseModel: "subscription"}
	require.NoError(t, database.DB.Create(agr).Error)
	admin := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-admin-" + t.Name(), Email: "admin-" + t.Name() + "@x.com", Role: models.OrgUserRoleCustomerAdmin, Source: models.OrgUserSourceInvited}
	require.NoError(t, database.DB.Create(admin).Error)

	return org, agr, admin
}

func mkUser(t *testing.T, database *db.DB, orgID int64, email string) *models.OrgUser {
	t.Helper()
	u := &models.OrgUser{OrganizationID: orgID, CognitoSub: "sub-" + email, Email: email, Role: models.OrgUserRoleMember, Source: models.OrgUserSourceSSO}
	require.NoError(t, database.DB.Create(u).Error)

	return u
}

func TestReplaceKey(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, _, _ := setupOrg(t, database, 10)
		svc := portal.NewService(database, &fakeIssuer{}, nil, log.Default())
		ctx := context.Background()

		t.Run("uploads first key with fingerprint", func(t *testing.T) {
			user := mkUser(t, database, org.ID, "k1@x.com")
			key, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "laptop")
			require.NoError(t, err)
			assert.NotZero(t, key.ID)
			assert.Contains(t, key.Fingerprint, "SHA256:")
			assert.True(t, key.IsActive())
		})

		t.Run("rejects invalid PEM", func(t *testing.T) {
			user := mkUser(t, database, org.ID, "k2@x.com")
			_, err := svc.ReplaceKey(ctx, user, "not a pem", "")
			assert.Error(t, err)
		})

		t.Run("replacing revokes prior key (one active)", func(t *testing.T) {
			user := mkUser(t, database, org.ID, "k3@x.com")
			_, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "first")
			require.NoError(t, err)
			_, err = svc.ReplaceKey(ctx, user, testPubKeyPEM, "second")
			require.NoError(t, err)

			var active int64
			require.NoError(t, database.DB.Model(&models.UserPublicKey{}).Where("org_user_id = ? AND revoked_at IS NULL", user.ID).Count(&active).Error)
			assert.Equal(t, int64(1), active)
		})
	})
}

func TestRequestLicense_AutoApproveIssues(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, admin := setupOrg(t, database, 10)
		domain := "x.com"
		require.NoError(t, database.DB.Create(&models.AutoAcceptanceRule{
			OrganizationID: org.ID, RuleType: models.AutoAcceptanceRuleTypeEmailDomain, MatchDomain: &domain, Enabled: true, CreatedByUserID: admin.ID,
		}).Error)

		svc := portal.NewService(database, &fakeIssuer{}, nil, log.Default())
		ctx := context.Background()

		user := mkUser(t, database, org.ID, "auto@x.com")
		_, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "k")
		require.NoError(t, err)

		lr, err := svc.RequestLicense(ctx, user, agr.ID)
		require.NoError(t, err)
		assert.Equal(t, models.LicenseRequestStatusApproved, lr.Status)
		require.NotNil(t, lr.IssuedTokenID)

		// a downloadable license now exists
		jwtStr, err := svc.License(ctx, user)
		require.NoError(t, err)
		assert.Contains(t, jwtStr, "jwt-")
	})
}

func TestRequestLicense_SeatFullDeniesAndNotifies(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, admin := setupOrg(t, database, 1) // single seat
		domain := "x.com"
		require.NoError(t, database.DB.Create(&models.AutoAcceptanceRule{
			OrganizationID: org.ID, RuleType: models.AutoAcceptanceRuleTypeEmailDomain, MatchDomain: &domain, Enabled: true, CreatedByUserID: admin.ID,
		}).Error)

		notifier := &fakeNotifier{}
		svc := portal.NewService(database, &fakeIssuer{}, notifier, log.Default())
		ctx := context.Background()

		// consume the only seat
		u1 := mkUser(t, database, org.ID, "first@x.com")
		_, err := svc.ReplaceKey(ctx, u1, testPubKeyPEM, "k")
		require.NoError(t, err)
		_, err = svc.RequestLicense(ctx, u1, agr.ID)
		require.NoError(t, err)

		// second user is denied
		u2 := mkUser(t, database, org.ID, "second@x.com")
		_, err = svc.ReplaceKey(ctx, u2, testPubKeyPEM, "k")
		require.NoError(t, err)
		lr, err := svc.RequestLicense(ctx, u2, agr.ID)
		require.NoError(t, err)

		assert.Equal(t, models.LicenseRequestStatusFailed, lr.Status)
		assert.Equal(t, 1, notifier.calls, "admins must be notified on seat-full denial")
	})
}

func TestRequestLicense_NoRuleIsManual(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, _ := setupOrg(t, database, 10)
		svc := portal.NewService(database, &fakeIssuer{}, nil, log.Default())
		ctx := context.Background()

		user := mkUser(t, database, org.ID, "manual@x.com")
		_, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "k")
		require.NoError(t, err)

		lr, err := svc.RequestLicense(ctx, user, agr.ID)
		require.NoError(t, err)
		assert.Equal(t, models.LicenseRequestStatusPending, lr.Status)
		assert.Nil(t, lr.IssuedTokenID)
	})
}

func TestReplaceKey_ReissuesActiveLicense(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, admin := setupOrg(t, database, 10)
		domain := "x.com"
		require.NoError(t, database.DB.Create(&models.AutoAcceptanceRule{
			OrganizationID: org.ID, RuleType: models.AutoAcceptanceRuleTypeEmailDomain, MatchDomain: &domain, Enabled: true, CreatedByUserID: admin.ID,
		}).Error)
		svc := portal.NewService(database, &fakeIssuer{}, nil, log.Default())
		ctx := context.Background()

		user := mkUser(t, database, org.ID, "reissue@x.com")
		_, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "first")
		require.NoError(t, err)
		lr, err := svc.RequestLicense(ctx, user, agr.ID)
		require.NoError(t, err)
		firstTokenID := *lr.IssuedTokenID

		// replace the key → old license revoked, new one issued
		_, err = svc.ReplaceKey(ctx, user, testPubKeyPEM, "second")
		require.NoError(t, err)

		var live int64
		require.NoError(t, database.DB.Model(&models.IssuedToken{}).Where("org_user_id = ? AND revoked_at IS NULL", user.ID).Count(&live).Error)
		assert.Equal(t, int64(1), live, "exactly one live token after reissue")

		var old models.IssuedToken
		require.NoError(t, database.DB.First(&old, firstTokenID).Error)
		require.NotNil(t, old.RevokedAt)
		require.NotNil(t, old.RevokedReason)
		assert.Equal(t, models.RevokedReasonKeyReplaced, *old.RevokedReason)
	})
}
