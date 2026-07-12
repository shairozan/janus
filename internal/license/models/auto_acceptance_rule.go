package models

import (
	"time"
)

// AutoAcceptanceRule types.
const (
	AutoAcceptanceRuleTypeEmailDomain   = "email_domain"
	AutoAcceptanceRuleTypeSeatThreshold = "seat_threshold"
)

// AutoAcceptanceRule is an admin-defined rule for auto-approving license
// requests. Any rule evaluation that fails must notify all customer admins
// (reqs. 11 & 14).
type AutoAcceptanceRule struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID  int64      `gorm:"not null;index:idx_auto_rules_org" json:"organization_id" db:"organization_id"`
	AgreementID     *int64     `gorm:"default:null" json:"agreement_id,omitempty" db:"agreement_id"`
	RuleType        string     `gorm:"type:varchar(20);not null" json:"rule_type" db:"rule_type"`
	MatchDomain     *string    `gorm:"type:varchar(255)" json:"match_domain,omitempty" db:"match_domain"`
	MaxAutoSeats    *int       `gorm:"default:null" json:"max_auto_seats,omitempty" db:"max_auto_seats"`
	Enabled         bool       `gorm:"not null" json:"enabled" db:"enabled"`
	CreatedByUserID int64      `gorm:"not null" json:"created_by_user_id" db:"created_by_user_id"`
	CreatedAt       time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	DeactivatedAt   *time.Time `gorm:"default:null" json:"deactivated_at,omitempty" db:"deactivated_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Agreement    *Agreement    `gorm:"foreignKey:AgreementID" json:"agreement,omitempty"`
}

// TableName overrides the default table name.
func (AutoAcceptanceRule) TableName() string {
	return "auto_acceptance_rules"
}

// IsActive returns true if the rule is enabled and not deactivated.
func (r *AutoAcceptanceRule) IsActive() bool {
	return r.Enabled && r.DeactivatedAt == nil
}
