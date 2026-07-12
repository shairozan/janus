package models

import (
	"time"
)

// LicenseRequest statuses.
const (
	LicenseRequestStatusPending  = "pending"
	LicenseRequestStatusApproved = "approved"
	LicenseRequestStatusRejected = "rejected"
	LicenseRequestStatusFailed   = "failed"
)

// LicenseRequest decision modes.
const (
	LicenseRequestDecisionAuto   = "auto"
	LicenseRequestDecisionManual = "manual"
)

// LicenseRequest tracks a user's request for a seat against an agreement and its
// disposition. A request can only be approved once it has both a decision and a
// UserPublicKeyID; issuing the license sets IssuedTokenID.
type LicenseRequest struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID  int64      `gorm:"not null;index:idx_license_requests_org" json:"organization_id" db:"organization_id"`
	OrgUserID       int64      `gorm:"not null;index:idx_license_requests_user" json:"org_user_id" db:"org_user_id"`
	AgreementID     int64      `gorm:"not null" json:"agreement_id" db:"agreement_id"`
	UserPublicKeyID *int64     `gorm:"default:null" json:"user_public_key_id,omitempty" db:"user_public_key_id"`
	Status          string     `gorm:"type:varchar(20);not null" json:"status" db:"status"`
	Decision        *string    `gorm:"type:varchar(10)" json:"decision,omitempty" db:"decision"`
	DecidedByUserID *int64     `gorm:"default:null" json:"decided_by_user_id,omitempty" db:"decided_by_user_id"`
	DecidedAt       *time.Time `gorm:"default:null" json:"decided_at,omitempty" db:"decided_at"`
	Reason          *string    `gorm:"type:text" json:"reason,omitempty" db:"reason"`
	IssuedTokenID   *int64     `gorm:"default:null" json:"issued_token_id,omitempty" db:"issued_token_id"`
	CreatedAt       time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	OrgUser      *OrgUser      `gorm:"foreignKey:OrgUserID" json:"org_user,omitempty"`
	Agreement    *Agreement    `gorm:"foreignKey:AgreementID" json:"agreement,omitempty"`
}

// TableName overrides the default table name.
func (LicenseRequest) TableName() string {
	return "license_requests"
}

// IsApprovable returns true if the request has a public key to embed (a request
// cannot be fulfilled into a license without one).
func (r *LicenseRequest) IsApprovable() bool {
	return r.UserPublicKeyID != nil
}
