package models

import (
	"time"
)

// Organization represents a customer organization.
type Organization struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	Name             string     `gorm:"type:varchar(255);not null" json:"name" db:"name"`
	CustomerID       string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"customer_id" db:"customer_id"`
	ContactName      *string    `gorm:"type:varchar(255)" json:"contact_name,omitempty" db:"contact_name"`
	Email            *string    `gorm:"type:varchar(255)" json:"email,omitempty" db:"email"`
	StripeCustomerID *string    `gorm:"type:varchar(255)" json:"stripe_customer_id,omitempty" db:"stripe_customer_id"`
	CreatedAt        time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt        *time.Time `gorm:"default:null" json:"updated_at,omitempty" db:"updated_at"`
	DeactivatedAt    *time.Time `gorm:"default:null" json:"deactivated_at,omitempty" db:"deactivated_at"`
}

// TableName overrides the default table name.
func (Organization) TableName() string {
	return "organizations"
}

// IsActive returns true if the organization is not deactivated.
func (o *Organization) IsActive() bool {
	return o.DeactivatedAt == nil
}
