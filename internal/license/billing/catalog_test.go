//go:build integration && server
// +build integration,server

package billing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/db/testutil"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestCatalog(t *testing.T) {
	testutil.WithTestDatabase(t, func(t *testing.T, database *db.DB) {
		svc := billing.NewCatalogService(database)
		ctx := context.Background()

		valid := billing.CatalogInput{
			StripeProductID: "prod_a", StripePriceID: "price_a", Nickname: "Pro/seat/yr",
			UnitAmountCents: 120000, Currency: "usd", Interval: models.StripeIntervalYear, Kind: models.StripePriceKindPerSeat,
		}

		t.Run("create + list", func(t *testing.T) {
			p, err := svc.Create(ctx, valid)
			require.NoError(t, err)
			assert.True(t, p.Active)

			prices, err := svc.List(ctx, true)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		})

		t.Run("rejects bad interval/kind", func(t *testing.T) {
			bad := valid
			bad.StripePriceID = "price_b"
			bad.Interval = "weekly"
			_, err := svc.Create(ctx, bad)
			assert.Error(t, err)

			bad.Interval = models.StripeIntervalYear
			bad.Kind = "nonsense"
			_, err = svc.Create(ctx, bad)
			assert.Error(t, err)
		})

		t.Run("deactivate hides from active list", func(t *testing.T) {
			p, err := svc.Create(ctx, billing.CatalogInput{
				StripeProductID: "prod_c", StripePriceID: "price_c", UnitAmountCents: 100,
				Interval: models.StripeIntervalMonth, Kind: models.StripePriceKindFlat,
			})
			require.NoError(t, err)

			require.NoError(t, svc.Deactivate(ctx, p.ID))

			active, err := svc.List(ctx, true)
			require.NoError(t, err)
			for _, pr := range active {
				assert.NotEqual(t, p.ID, pr.ID)
			}

			all, err := svc.List(ctx, false)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(all), 2)
		})
	})
}
