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

func TestBillingModels(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		org := &models.Organization{Name: "Bill Co", CustomerID: "CUST-BILL"}
		require.NoError(t, database.DB.Create(org).Error)
		admin := &models.OrgUser{OrganizationID: org.ID, CognitoSub: "sub-bill", Email: "bill@januspk.com", Role: models.OrgUserRoleCustomerAdmin, Source: models.OrgUserSourceInvited}
		require.NoError(t, database.DB.Create(admin).Error)

		var price models.StripePrice
		t.Run("create catalog price", func(t *testing.T) {
			price = models.StripePrice{
				StripeProductID: "prod_abc",
				StripePriceID:   "price_abc",
				Nickname:        "Pro per-seat / yr",
				UnitAmountCents: 120000,
				Currency:        "usd",
				Interval:        models.StripeIntervalYear,
				Kind:            models.StripePriceKindPerSeat,
				Active:          true,
			}
			require.NoError(t, database.DB.Create(&price).Error)
			assert.NotZero(t, price.ID)
		})

		t.Run("stripe_price_id is unique", func(t *testing.T) {
			dup := models.StripePrice{
				StripeProductID: "prod_abc", StripePriceID: "price_abc",
				UnitAmountCents: 1, Currency: "usd", Interval: models.StripeIntervalYear, Kind: models.StripePriceKindFlat,
			}
			assert.Error(t, database.DB.Create(&dup).Error)
		})

		var proposal models.AgreementProposal
		t.Run("create proposal with line item referencing catalog", func(t *testing.T) {
			proposal = models.AgreementProposal{
				OrganizationID:    org.ID,
				RequestedByUserID: admin.ID,
				Status:            models.ProposalStatusRequested,
				Seats:             50,
				LicenseModel:      "subscription",
				ValidityDays:      models.DefaultProposalValidityDays,
			}
			require.NoError(t, database.DB.Create(&proposal).Error)
			assert.Equal(t, 365, proposal.ValidityDays)

			li := models.ProposalLineItem{
				ProposalID:  proposal.ID,
				PriceID:     price.ID,
				Quantity:    50,
				AmountCents: 50 * price.UnitAmountCents,
			}
			require.NoError(t, database.DB.Create(&li).Error)
		})

		t.Run("counter proposal links to parent", func(t *testing.T) {
			counter := models.AgreementProposal{
				OrganizationID:    org.ID,
				RequestedByUserID: admin.ID,
				Status:            models.ProposalStatusCountered,
				Seats:             40,
				LicenseModel:      "subscription",
				ValidityDays:      365,
				ParentProposalID:  &proposal.ID,
			}
			require.NoError(t, database.DB.Create(&counter).Error)
			require.NotNil(t, counter.ParentProposalID)
			assert.Equal(t, proposal.ID, *counter.ParentProposalID)
		})

		t.Run("load proposal with line items + price", func(t *testing.T) {
			var loaded models.AgreementProposal
			require.NoError(t, database.DB.Preload("LineItems.Price").First(&loaded, proposal.ID).Error)
			require.Len(t, loaded.LineItems, 1)
			assert.Equal(t, 50, loaded.LineItems[0].Quantity)
			require.NotNil(t, loaded.LineItems[0].Price)
			assert.Equal(t, "price_abc", loaded.LineItems[0].Price.StripePriceID)
		})

		t.Run("stripe linkage columns on org + agreement", func(t *testing.T) {
			cust := "cus_123"
			require.NoError(t, database.DB.Model(org).Update("stripe_customer_id", cust).Error)

			license := &models.License{Name: "Bill Lic", Tier: "pro", Features: models.Features{"local"}, MSRPCents: 5000}
			require.NoError(t, database.DB.Create(license).Error)
			sub := "sub_123"
			agr := &models.Agreement{OrganizationID: org.ID, LicenseID: license.ID, Seats: 50, PricePerSeatCents: 2400, LicenseModel: "subscription", StripeSubscriptionID: &sub}
			require.NoError(t, database.DB.Create(agr).Error)

			var reloadedOrg models.Organization
			require.NoError(t, database.DB.First(&reloadedOrg, org.ID).Error)
			require.NotNil(t, reloadedOrg.StripeCustomerID)
			assert.Equal(t, "cus_123", *reloadedOrg.StripeCustomerID)

			var reloadedAgr models.Agreement
			require.NoError(t, database.DB.First(&reloadedAgr, agr.ID).Error)
			require.NotNil(t, reloadedAgr.StripeSubscriptionID)
			assert.Equal(t, "sub_123", *reloadedAgr.StripeSubscriptionID)
		})
	})
}
