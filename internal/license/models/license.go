package models

import (
	"time"
)

// License represents a license tier/product.
type License struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	Name          string     `gorm:"type:varchar(100);not null" json:"name" db:"name"`
	Tier          string     `gorm:"type:varchar(50);not null" json:"tier" db:"tier"`
	Features      Features   `gorm:"type:jsonb;not null" json:"features" db:"features"`
	MSRPCents     int        `gorm:"column:msrp_cents;not null" json:"msrp_cents" db:"msrp_cents"`
	CreatedAt     time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	DeactivatedAt *time.Time `gorm:"default:null" json:"deactivated_at,omitempty" db:"deactivated_at"`
}

// TableName overrides the default table name.
func (License) TableName() string {
	return "licenses"
}

// IsActive returns true if the license is not deactivated.
func (l *License) IsActive() bool {
	return l.DeactivatedAt == nil
}

// MSRPDollars returns the MSRP in dollars.
func (l *License) MSRPDollars() float64 {
	return float64(l.MSRPCents) / 100.0
}

// ResolveFeatures returns the features as a standard string slice.
func (l *License) ResolveFeatures() []string {
	return []string(l.Features)
}
