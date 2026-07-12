//go:build integration && server
// +build integration,server

package billing_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/billing/billingtest"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestSeatExpansion(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		fx := setupBilling(t, database)
		stripe := billingtest.New()

		// a subscription the fake knows about
		subID, err := stripe.CreateSubscription(context.Background(), "cus_1", "price_x", 50, "")
		require.NoError(t, err)

		start := time.Now().AddDate(0, 0, -30)
		end := time.Now().AddDate(0, 0, 335) // ~1yr term, ~335 days remaining
		agr := &models.Agreement{
			OrganizationID: fx.org.ID, LicenseID: 1, Seats: 50, PricePerSeatCents: 120000,
			LicenseModel: "subscription", StartDate: &start, EndDate: &end, StripeSubscriptionID: &subID,
		}
		require.NoError(t, database.DB.Create(agr).Error)

		svc := billing.NewSeatService(database, stripe, log.Default())
		ctx := context.Background()

		t.Run("preview estimates a prorated charge", func(t *testing.T) {
			p, err := svc.Preview(ctx, fx.org.ID, agr.ID, 10)
			require.NoError(t, err)
			assert.Equal(t, 50, p.CurrentSeats)
			assert.Equal(t, 60, p.NewSeats)
			assert.Equal(t, int64(10*120000), p.RecurringDeltaCents)
			// prorated < full recurring (only part of the term remains) but > 0
			assert.Greater(t, p.ProratedChargeCents, int64(0))
			assert.Less(t, p.ProratedChargeCents, p.RecurringDeltaCents)
		})

		t.Run("add bumps subscription quantity + agreement seats", func(t *testing.T) {
			updated, err := svc.Add(ctx, fx.org.ID, agr.ID, 10)
			require.NoError(t, err)
			assert.Equal(t, 60, updated.Seats)
			assert.Equal(t, 60, stripe.QuantityOf(subID))

			var reloaded models.Agreement
			require.NoError(t, database.DB.First(&reloaded, agr.ID).Error)
			assert.Equal(t, 60, reloaded.Seats)
		})

		t.Run("cross-org agreement rejected", func(t *testing.T) {
			_, err := svc.Add(ctx, fx.org.ID+999, agr.ID, 1)
			assert.Error(t, err)
		})
	})
}
