package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/billing"
)

// CatalogList handles GET /api/v1/billing/prices (Janus staff). ?active=false
// includes deactivated prices.
func CatalogList(svc *billing.CatalogService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		activeOnly := r.URL.Query().Get("active") != "false"

		prices, err := svc.List(r.Context(), activeOnly)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list prices")

			return
		}

		writeJSON(w, http.StatusOK, prices)
	}
}

// CatalogCreateBody is the POST /api/v1/billing/prices body.
type CatalogCreateBody struct {
	StripeProductID string `json:"stripe_product_id"`
	StripePriceID   string `json:"stripe_price_id"`
	Nickname        string `json:"nickname"`
	UnitAmountCents int    `json:"unit_amount_cents"`
	Currency        string `json:"currency"`
	Interval        string `json:"interval"`
	Kind            string `json:"kind"`
}

// CatalogCreate handles POST /api/v1/billing/prices (Janus staff).
func CatalogCreate(svc *billing.CatalogService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body CatalogCreateBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		price, err := svc.Create(r.Context(), billing.CatalogInput{
			StripeProductID: body.StripeProductID,
			StripePriceID:   body.StripePriceID,
			Nickname:        body.Nickname,
			UnitAmountCents: body.UnitAmountCents,
			Currency:        body.Currency,
			Interval:        body.Interval,
			Kind:            body.Kind,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		writeJSON(w, http.StatusCreated, price)
	}
}
