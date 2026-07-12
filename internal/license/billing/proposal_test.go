//go:build integration && server
// +build integration,server

package billing_test

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/billing/billingtest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

type billingFixture struct {
	org     *models.Organization
	admin   *models.OrgUser
	perSeat *models.StripePrice
	tier    string
}

func setupBilling(t *testing.T, database *db.DB) billingFixture {
	t.Helper()
	org := &models.Organization{Name: "Bill Co", CustomerID: "CUST-" + t.Name()}
	require.NoError(t, database.DB.Create(org).Error)
	admin := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-" + t.Name(), Email: "admin@x.com", Role: models.OrgUserRoleCustomerAdmin, Source: models.OrgUserSourceInvited}
	require.NoError(t, database.DB.Create(admin).Error)
	lic := &models.License{Name: "Pro", Tier: "pro", Features: models.Features{"local"}, MSRPCents: 120000}
	require.NoError(t, database.DB.Create(lic).Error)
	price := &models.StripePrice{StripeProductID: "prod_x", StripePriceID: "price_x", UnitAmountCents: 120000, Currency: "usd", Interval: models.StripeIntervalYear, Kind: models.StripePriceKindPerSeat, Active: true}
	require.NoError(t, database.DB.Create(price).Error)

	return billingFixture{org: org, admin: admin, perSeat: price, tier: "pro"}
}

func TestProposalLifecycle(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fx := setupBilling(t, database)
		stripe := billingtest.New()
		svc := billing.NewProposalService(database, stripe, log.Default())
		ctx := context.Background()

		tier := fx.tier
		pr, err := svc.Request(ctx, fx.org.ID, fx.admin.ID, billing.RequestInput{Tier: &tier, Seats: 50, LicenseModel: "subscription"})
		require.NoError(t, err)
		assert.Equal(t, models.ProposalStatusRequested, pr.Status)

		offered, err := svc.Offer(ctx, pr.ID, billing.OfferInput{
			LineItems: []billing.OfferLineItem{{PriceID: fx.perSeat.ID, Quantity: 50}},
			Message:   "here's our offer",
		})
		require.NoError(t, err)
		assert.Equal(t, models.ProposalStatusProposed, offered.Status)
		assert.Equal(t, 50*120000, offered.TotalAmountCents)
		require.Len(t, offered.LineItems, 1)

		accepted, err := svc.Accept(ctx, fx.org.ID, pr.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ProposalStatusAccepted, accepted.Status)

		paid, err := svc.Pay(ctx, fx.org.ID, pr.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ProposalStatusPaid, paid.Status)
		require.NotNil(t, paid.ResultingAgreementID)

		// Agreement created + linked to the fake subscription; quantity = seats.
		var agr models.Agreement
		require.NoError(t, database.DB.First(&agr, *paid.ResultingAgreementID).Error)
		require.NotNil(t, agr.StripeSubscriptionID)
		assert.Equal(t, 50, stripe.QuantityOf(*agr.StripeSubscriptionID))
		assert.Equal(t, 50, agr.Seats)

		// org now has a Stripe customer
		var org models.Organization
		require.NoError(t, database.DB.First(&org, fx.org.ID).Error)
		require.NotNil(t, org.StripeCustomerID)

		// webhook fulfilment
		require.NoError(t, svc.FulfillBySubscription(ctx, *agr.StripeSubscriptionID))
		var fulfilled models.AgreementProposal
		require.NoError(t, database.DB.First(&fulfilled, pr.ID).Error)
		assert.Equal(t, models.ProposalStatusFulfilled, fulfilled.Status)
	})
}

func TestProposalCounter(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fx := setupBilling(t, database)
		svc := billing.NewProposalService(database, billingtest.New(), log.Default())
		ctx := context.Background()
		tier := fx.tier

		pr, err := svc.Request(ctx, fx.org.ID, fx.admin.ID, billing.RequestInput{Tier: &tier, Seats: 50, LicenseModel: "subscription"})
		require.NoError(t, err)
		_, err = svc.Offer(ctx, pr.ID, billing.OfferInput{LineItems: []billing.OfferLineItem{{PriceID: fx.perSeat.ID, Quantity: 50}}})
		require.NoError(t, err)

		counter, err := svc.Counter(ctx, fx.org.ID, pr.ID, billing.RequestInput{Tier: &tier, Seats: 40, LicenseModel: "subscription"}, "too many seats")
		require.NoError(t, err)
		assert.Equal(t, models.ProposalStatusCountered, counter.Status)
		require.NotNil(t, counter.ParentProposalID)
		assert.Equal(t, pr.ID, *counter.ParentProposalID)
		assert.Equal(t, 40, counter.Seats)
	})
}

func TestPayRequiresAccepted(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fx := setupBilling(t, database)
		svc := billing.NewProposalService(database, billingtest.New(), log.Default())
		ctx := context.Background()
		tier := fx.tier

		pr, err := svc.Request(ctx, fx.org.ID, fx.admin.ID, billing.RequestInput{Tier: &tier, Seats: 50, LicenseModel: "subscription"})
		require.NoError(t, err)

		_, err = svc.Pay(ctx, fx.org.ID, pr.ID)
		assert.Error(t, err, "cannot pay a proposal that isn't accepted")
	})
}

func TestProposalReads(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fx := setupBilling(t, database)
		svc := billing.NewProposalService(database, billingtest.New(), log.Default())
		ctx := context.Background()
		tier := fx.tier

		pr, err := svc.Request(ctx, fx.org.ID, fx.admin.ID, billing.RequestInput{Tier: &tier, Seats: 50, LicenseModel: "subscription"})
		require.NoError(t, err)
		_, err = svc.Offer(ctx, pr.ID, billing.OfferInput{
			LineItems: []billing.OfferLineItem{{PriceID: fx.perSeat.ID, Quantity: 50}},
		})
		require.NoError(t, err)

		t.Run("Get returns the proposal with line items + price", func(t *testing.T) {
			got, getErr := svc.Get(ctx, fx.org.ID, pr.ID)
			require.NoError(t, getErr)
			assert.Equal(t, models.ProposalStatusProposed, got.Status)
			require.Len(t, got.LineItems, 1)
			require.NotNil(t, got.LineItems[0].Price)
			assert.Equal(t, fx.perSeat.StripePriceID, got.LineItems[0].Price.StripePriceID)
		})

		t.Run("Get is org-scoped", func(t *testing.T) {
			other := &models.Organization{Name: "Other", CustomerID: "CUST-OTHER-PROP"}
			require.NoError(t, database.DB.Create(other).Error)
			_, getErr := svc.Get(ctx, other.ID, pr.ID)
			assert.Error(t, getErr, "a different org must not read this proposal")
		})

		t.Run("List returns the org's proposals newest-first", func(t *testing.T) {
			list, listErr := svc.List(ctx, fx.org.ID)
			require.NoError(t, listErr)
			require.GreaterOrEqual(t, len(list), 1)
			assert.Equal(t, pr.ID, list[0].ID)
			require.Len(t, list[0].LineItems, 1)
		})
	})
}
