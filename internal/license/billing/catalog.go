// Package billing implements the pricing catalog, proposal/negotiation workflow,
// payments, and seat expansion. Pricing is DB-stored operational data — the only
// Stripe values in config are the API key and webhook secret.
package billing

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// CatalogService manages the stripe_prices catalog (Janus-staff operation).
type CatalogService struct {
	db *db.DB
}

// NewCatalogService creates a CatalogService.
func NewCatalogService(database *db.DB) *CatalogService {
	return &CatalogService{db: database}
}

// CatalogInput is a request to add a catalog price.
type CatalogInput struct {
	StripeProductID string
	StripePriceID   string
	Nickname        string
	UnitAmountCents int
	Currency        string
	Interval        string
	Kind            string
}

// Create adds a price to the catalog after validating the enum fields.
func (c *CatalogService) Create(ctx context.Context, in CatalogInput) (*models.StripePrice, error) {
	if in.StripeProductID == "" || in.StripePriceID == "" {
		return nil, fmt.Errorf("stripe_product_id and stripe_price_id are required")
	}

	if !validInterval(in.Interval) {
		return nil, fmt.Errorf("interval must be one of month/year/one_time")
	}

	if !validKind(in.Kind) {
		return nil, fmt.Errorf("kind must be one of per_seat/flat/concurrent")
	}

	if in.UnitAmountCents < 0 {
		return nil, fmt.Errorf("unit_amount_cents must be >= 0")
	}

	currency := in.Currency
	if currency == "" {
		currency = "usd"
	}

	price := &models.StripePrice{
		StripeProductID: in.StripeProductID,
		StripePriceID:   in.StripePriceID,
		Nickname:        in.Nickname,
		UnitAmountCents: in.UnitAmountCents,
		Currency:        currency,
		Interval:        in.Interval,
		Kind:            in.Kind,
		Active:          true,
	}
	if err := c.db.DB.WithContext(ctx).Create(price).Error; err != nil {
		return nil, err
	}

	return price, nil
}

// List returns catalog prices; activeOnly filters out deactivated ones.
func (c *CatalogService) List(ctx context.Context, activeOnly bool) ([]models.StripePrice, error) {
	q := c.db.DB.WithContext(ctx).Order("created_at DESC")
	if activeOnly {
		q = q.Where("active = ?", true)
	}

	var prices []models.StripePrice
	if err := q.Find(&prices).Error; err != nil {
		return nil, err
	}

	return prices, nil
}

// Get returns a single catalog price by id.
func (c *CatalogService) Get(ctx context.Context, id int64) (*models.StripePrice, error) {
	var price models.StripePrice

	err := c.db.DB.WithContext(ctx).First(&price, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("price not found")
	}

	if err != nil {
		return nil, err
	}

	return &price, nil
}

// Deactivate marks a catalog price inactive (it remains referenced by historical
// proposals/line items).
func (c *CatalogService) Deactivate(ctx context.Context, id int64) error {
	res := c.db.DB.WithContext(ctx).Model(&models.StripePrice{}).Where("id = ?", id).Update("active", false)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return fmt.Errorf("price not found")
	}

	return nil
}

func validInterval(v string) bool {
	switch v {
	case models.StripeIntervalMonth, models.StripeIntervalYear, models.StripeIntervalOneTime:
		return true
	default:
		return false
	}
}

func validKind(v string) bool {
	switch v {
	case models.StripePriceKindPerSeat, models.StripePriceKindFlat, models.StripePriceKindConcurrent:
		return true
	default:
		return false
	}
}
