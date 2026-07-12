package models

import (
	"time"
)

// OrgUser roles.
const (
	OrgUserRoleCustomerAdmin = "customer_admin"
	OrgUserRoleMember        = "member"
)

// OrgUser sources (how the user entered the system).
const (
	OrgUserSourceInvited = "invited" // native Cognito user (e.g. the first admin)
	OrgUserSourceSSO     = "sso"     // federated via the customer's SSO IdP
)

// OrgUser represents a person within a customer organization, keyed on their
// stable Cognito identity (the `sub` claim). It is the identity anchor that the
// portal's per-user keys, license requests, and audit records hang off of.
type OrgUser struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID int64      `gorm:"not null;index:idx_org_users_org" json:"organization_id" db:"organization_id"`
	CognitoSub     string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"cognito_sub" db:"cognito_sub"`
	Email          string     `gorm:"type:varchar(255);not null" json:"email" db:"email"`
	Role           string     `gorm:"type:varchar(20);not null" json:"role" db:"role"`     // customer_admin | member
	Source         string     `gorm:"type:varchar(20);not null" json:"source" db:"source"` // invited | sso
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `gorm:"not null;default:now()" json:"updated_at" db:"updated_at"`
	DeactivatedAt  *time.Time `gorm:"default:null;index:idx_org_users_active" json:"deactivated_at,omitempty" db:"deactivated_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// TableName overrides the default table name.
func (OrgUser) TableName() string {
	return "org_users"
}

// IsActive returns true if the user is not deactivated.
func (u *OrgUser) IsActive() bool {
	return u.DeactivatedAt == nil
}

// IsAdmin returns true if the user holds the customer_admin role.
func (u *OrgUser) IsAdmin() bool {
	return u.Role == OrgUserRoleCustomerAdmin
}
