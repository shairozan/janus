package models

import (
	"time"
)

// Agreement represents a license agreement between an organization and a license tier.
type Agreement struct {
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID    int64      `gorm:"not null;index:idx_agreements_org" json:"organization_id" db:"organization_id"`
	LicenseID         int64      `gorm:"not null" json:"license_id" db:"license_id"`
	Seats             int        `gorm:"not null" json:"seats" db:"seats"`
	PricePerSeatCents int        `gorm:"not null" json:"price_per_seat_cents" db:"price_per_seat_cents"`
	LicenseModel      string     `gorm:"type:varchar(20);not null" json:"license_model" db:"license_model"`
	ConcurrentLimit   *int       `gorm:"default:null" json:"concurrent_limit,omitempty" db:"concurrent_limit"`
	ActivatedAt       time.Time  `gorm:"not null;default:now()" json:"activated_at" db:"activated_at"`
	DeactivatedAt     *time.Time `gorm:"default:null;index:idx_agreements_active" json:"deactivated_at,omitempty" db:"deactivated_at"`
	Notes             *string    `gorm:"type:text" json:"notes,omitempty" db:"notes"`
	KeyID             *string    `gorm:"type:varchar(50)" json:"key_id,omitempty" db:"key_id"`

	// Override fields (optional - take priority over license defaults)
	Tier         *string          `gorm:"type:varchar(50);index:idx_agreements_tier" json:"tier,omitempty" db:"tier"`
	MaxSeats     *int             `gorm:"default:null" json:"max_seats,omitempty" db:"max_seats"`
	Features     NullableFeatures `gorm:"type:jsonb" json:"features,omitempty" db:"features"`
	CostPerMonth *float64         `gorm:"type:numeric(10,2)" json:"cost_per_month,omitempty" db:"cost_per_month"`
	StartDate    *time.Time       `gorm:"type:date" json:"start_date,omitempty" db:"start_date"`
	EndDate      *time.Time       `gorm:"type:date" json:"end_date,omitempty" db:"end_date"`
	CreatedAt    time.Time        `gorm:"default:now()" json:"created_at" db:"created_at"`
	UpdatedAt    time.Time        `gorm:"default:now()" json:"updated_at" db:"updated_at"`

	// StripeSubscriptionID links the agreement to its backing Stripe subscription
	// (per-seat quantity). Seat expansion = a quantity increase on this sub.
	StripeSubscriptionID *string `gorm:"type:varchar(255)" json:"stripe_subscription_id,omitempty" db:"stripe_subscription_id"`

	// Relationships (loaded via joins when needed)
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	License      *License      `gorm:"foreignKey:LicenseID" json:"license,omitempty"`
}

// TableName overrides the default table name.
func (Agreement) TableName() string {
	return "agreements"
}

// IsActive returns true if the agreement is not deactivated.
func (a *Agreement) IsActive() bool {
	return a.DeactivatedAt == nil
}

// PricePerSeatDollars returns the price per seat in dollars.
func (a *Agreement) PricePerSeatDollars() float64 {
	return float64(a.PricePerSeatCents) / 100.0
}

// MonthlyRevenue returns the total monthly revenue for this agreement.
func (a *Agreement) MonthlyRevenue() float64 {
	return float64(a.PricePerSeatCents*a.Seats) / 100.0
}

// AnnualRevenue returns the total annual revenue for this agreement.
func (a *Agreement) AnnualRevenue() float64 {
	return a.MonthlyRevenue() * 12
}

// GetTier returns the effective tier for this agreement.
func (a *Agreement) GetTier() *string {
	return a.Tier
}

// GetMaxSeats returns the effective max seats for this agreement.
func (a *Agreement) GetMaxSeats() int {
	if a.MaxSeats != nil {
		return *a.MaxSeats
	}

	return a.Seats
}

// GetFeatures returns the agreement's features if set.
func (a *Agreement) GetFeatures() []string {
	if len(a.Features) > 0 {
		return []string(a.Features)
	}

	return nil
}

// GetCostPerMonth returns the effective monthly cost.
func (a *Agreement) GetCostPerMonth() float64 {
	if a.CostPerMonth != nil {
		return *a.CostPerMonth
	}

	return a.MonthlyRevenue()
}

// GetStartDate returns the agreement start date.
func (a *Agreement) GetStartDate() time.Time {
	if a.StartDate != nil {
		return *a.StartDate
	}

	return a.ActivatedAt
}

// GetEndDate returns the agreement end date if set.
func (a *Agreement) GetEndDate() *time.Time {
	return a.EndDate
}

// IsExpired checks if the agreement has passed its end date.
func (a *Agreement) IsExpired() bool {
	if a.EndDate == nil {
		return false
	}

	return time.Now().After(*a.EndDate)
}
