//go:build integration && server
// +build integration,server

package portal_test

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
	"github.com/pharmalytica/janus/internal/license/portal"
)

func TestAdminApproveReject(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, admin := setupOrg(t, database, 10)
		svc := portal.NewService(database, &fakeIssuer{}, nil, log.Default())
		ctx := context.Background()

		// a pending request (no rules → manual)
		user := mkUser(t, database, org.ID, "pending@x.com")
		_, err := svc.ReplaceKey(ctx, user, testPubKeyPEM, "k")
		require.NoError(t, err)
		lr, err := svc.RequestLicense(ctx, user, agr.ID)
		require.NoError(t, err)
		require.Equal(t, models.LicenseRequestStatusPending, lr.Status)

		t.Run("approve issues the license", func(t *testing.T) {
			approved, approveErr := svc.ApproveRequest(ctx, admin, lr.ID)
			require.NoError(t, approveErr)
			assert.Equal(t, models.LicenseRequestStatusApproved, approved.Status)
			require.NotNil(t, approved.IssuedTokenID)
			require.NotNil(t, approved.DecidedByUserID)
			assert.Equal(t, admin.ID, *approved.DecidedByUserID)
		})

		t.Run("approving a non-pending request fails", func(t *testing.T) {
			_, approveErr := svc.ApproveRequest(ctx, admin, lr.ID)
			assert.Error(t, approveErr)
		})

		t.Run("reject sets status", func(t *testing.T) {
			u2 := mkUser(t, database, org.ID, "reject@x.com")
			_, e := svc.ReplaceKey(ctx, u2, testPubKeyPEM, "k")
			require.NoError(t, e)
			r2, e := svc.RequestLicense(ctx, u2, agr.ID)
			require.NoError(t, e)

			rejected, e := svc.RejectRequest(ctx, admin, r2.ID, "not this cycle")
			require.NoError(t, e)
			assert.Equal(t, models.LicenseRequestStatusRejected, rejected.Status)
		})
	})
}

func TestAdminMembers(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, _, _ := setupOrg(t, database, 10)
		fake := cognitotest.New()
		admin := portal.NewAdminService(database, fake, log.Default())
		ctx := context.Background()

		member := mkUser(t, database, org.ID, "member@x.com")

		t.Run("promote adds cognito group + role", func(t *testing.T) {
			u, err := admin.Promote(ctx, org.ID, member.ID)
			require.NoError(t, err)
			assert.Equal(t, models.OrgUserRoleCustomerAdmin, u.Role)
			assert.Contains(t, fake.MembershipsOf("member@x.com"), models.OrgUserRoleCustomerAdmin)
		})

		t.Run("demote removes cognito group + role", func(t *testing.T) {
			u, err := admin.Demote(ctx, org.ID, member.ID)
			require.NoError(t, err)
			assert.Equal(t, models.OrgUserRoleMember, u.Role)
			assert.NotContains(t, fake.MembershipsOf("member@x.com"), models.OrgUserRoleCustomerAdmin)
		})

		t.Run("cross-org member is rejected", func(t *testing.T) {
			otherOrg := &models.Organization{Name: "Other", CustomerID: "CUST-OTHER-ADM"}
			require.NoError(t, database.DB.Create(otherOrg).Error)
			outsider := mkUser(t, database, otherOrg.ID, "outsider@x.com")

			_, err := admin.Promote(ctx, org.ID, outsider.ID)
			assert.Error(t, err)
		})

		t.Run("lists active members", func(t *testing.T) {
			members, err := admin.ListMembers(ctx, org.ID)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(members), 1)
		})
	})
}

func TestAdminReads(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, agr, actor := setupOrg(t, database, 10)
		admin := portal.NewAdminService(database, cognitotest.New(), log.Default())
		ctx := context.Background()

		t.Run("ListActivity returns the org's audit events newest-first", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				rid := "r"
				require.NoError(t, database.DB.Create(&models.AuditEvent{
					OrganizationID: &org.ID, ActorOrgUserID: &actor.ID, ActorLabel: actor.Email,
					Resource: models.AuditResourceLicense, Action: models.AuditActionApprove,
					ResourceID: &rid, ResultStatus: 200,
				}).Error)
			}

			events, err := admin.ListActivity(ctx, org.ID, 0)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(events), 3)
			assert.Equal(t, models.AuditResourceLicense, events[0].Resource)
		})

		t.Run("ListAgreements reports seat usage", func(t *testing.T) {
			// consume one seat on the agreement (user + active token via approved request)
			u := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-seat", Email: "seat@x.com", Role: models.OrgUserRoleMember, Source: models.OrgUserSourceSSO}
			require.NoError(t, database.DB.Create(u).Error)
			tok := &models.IssuedToken{JTI: "jti-seat", OrganizationID: org.ID, OrgUserID: &u.ID, UserEmail: u.Email, KeyID: "k", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
			require.NoError(t, database.DB.Create(tok).Error)
			require.NoError(t, database.DB.Create(&models.LicenseRequest{OrganizationID: org.ID, OrgUserID: u.ID, AgreementID: agr.ID, Status: models.LicenseRequestStatusApproved, IssuedTokenID: &tok.ID}).Error)

			usage, err := admin.ListAgreements(ctx, org.ID)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(usage), 1)

			var found bool
			for _, a := range usage {
				if a.ID == agr.ID {
					found = true
					assert.Equal(t, 10, a.Seats)
					assert.Equal(t, 1, a.UsedSeats)
				}
			}
			assert.True(t, found, "the agreement should be listed with usage")
		})
	})
}

func TestAdminRules(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org, _, creator := setupOrg(t, database, 10)
		admin := portal.NewAdminService(database, cognitotest.New(), log.Default())
		ctx := context.Background()

		t.Run("create email_domain rule", func(t *testing.T) {
			domain := "knomix.io"
			rule, err := admin.CreateRule(ctx, org.ID, creator.ID, portal.RuleInput{
				RuleType: models.AutoAcceptanceRuleTypeEmailDomain, MatchDomain: &domain,
			})
			require.NoError(t, err)
			assert.True(t, rule.Enabled)
		})

		t.Run("email_domain rule without domain is rejected", func(t *testing.T) {
			_, err := admin.CreateRule(ctx, org.ID, creator.ID, portal.RuleInput{
				RuleType: models.AutoAcceptanceRuleTypeEmailDomain,
			})
			assert.Error(t, err)
		})

		t.Run("disable then delete", func(t *testing.T) {
			max := 5
			rule, err := admin.CreateRule(ctx, org.ID, creator.ID, portal.RuleInput{
				RuleType: models.AutoAcceptanceRuleTypeSeatThreshold, MaxAutoSeats: &max,
			})
			require.NoError(t, err)

			disabled, err := admin.SetRuleEnabled(ctx, org.ID, rule.ID, false)
			require.NoError(t, err)
			assert.False(t, disabled.Enabled)

			require.NoError(t, admin.DeleteRule(ctx, org.ID, rule.ID))

			rules, err := admin.ListRules(ctx, org.ID)
			require.NoError(t, err)
			for _, r := range rules {
				assert.NotEqual(t, rule.ID, r.ID, "deleted rule must not appear in active list")
			}
		})
	})
}
