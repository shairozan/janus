package models

import (
	"time"
)

// StripePrice billing intervals.
const (
	StripeIntervalMonth   = "month"
	StripeIntervalYear    = "year"
	StripeIntervalOneTime = "one_time"
)

// StripePrice kinds.
const (
	StripePriceKindPerSeat    = "per_seat"
	StripePriceKindFlat       = "flat"
	StripePriceKindConcurrent = "concurrent"
)

// StripePrice is the DB-stored catalog mapping of a logical price to its Stripe
// identifiers. Proposals reference catalog rows rather than hardcoded literals,
// so ops can change pricing without a redeploy. Only the Stripe API key/webhook
// secret live in config; all product/price IDs and amounts are rows here.
type StripePrice struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	StripeProductID string    `gorm:"type:varchar(255);not null" json:"stripe_product_id" db:"stripe_product_id"`
	StripePriceID   string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"stripe_price_id" db:"stripe_price_id"`
	Nickname        string    `gorm:"type:varchar(255)" json:"nickname" db:"nickname"`
	UnitAmountCents int       `gorm:"not null" json:"unit_amount_cents" db:"unit_amount_cents"`
	Currency        string    `gorm:"type:varchar(3);not null" json:"currency" db:"currency"`
	Interval        string    `gorm:"type:varchar(20);not null" json:"interval" db:"interval"`
	Kind            string    `gorm:"type:varchar(20);not null" json:"kind" db:"kind"`
	Active          bool      `gorm:"not null" json:"active" db:"active"`
	CreatedAt       time.Time `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `gorm:"not null;default:now()" json:"updated_at" db:"updated_at"`
}

// TableName overrides the default table name.
func (StripePrice) TableName() string {
	return "stripe_prices"
}
