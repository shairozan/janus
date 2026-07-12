package models

import (
	"time"
)

// IssuedToken revocation reasons.
const (
	RevokedReasonOffboarded  = "offboarded"
	RevokedReasonKeyReplaced = "key_replaced"
	RevokedReasonNonPayment  = "non_payment"
	RevokedReasonManual      = "manual"
)

// IssuedToken tracks JWT tokens for revocation purposes.
type IssuedToken struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	JTI            string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"jti"`
	OrganizationID int64      `gorm:"not null;index:idx_issued_tokens_org" json:"organization_id"`
	OrgUserID      *int64     `gorm:"index:idx_issued_tokens_org_user" json:"org_user_id,omitempty"`
	UserEmail      string     `gorm:"type:varchar(255);not null" json:"user_email"`
	KeyID          string     `gorm:"type:varchar(50);not null" json:"key_id"`
	IssuedAt       time.Time  `gorm:"not null" json:"issued_at"`
	ExpiresAt      time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt      *time.Time `gorm:"default:null;index:idx_issued_tokens_active" json:"revoked_at,omitempty"`
	RevokedReason  *string    `gorm:"type:varchar(20)" json:"revoked_reason,omitempty"`
	TokenJWT       string     `gorm:"type:text" json:"-"` // the signed license, for download

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	OrgUser      *OrgUser      `gorm:"foreignKey:OrgUserID" json:"org_user,omitempty"`
}

// TableName overrides the default table name.
func (IssuedToken) TableName() string {
	return "issued_tokens"
}

// IsRevoked returns true if the token has been revoked.
func (it *IssuedToken) IsRevoked() bool {
	return it.RevokedAt != nil
}

// IsExpired returns true if the token has expired.
func (it *IssuedToken) IsExpired() bool {
	return it.ExpiresAt.Before(time.Now())
}

// IsActive returns true if the token is not revoked and not expired.
func (it *IssuedToken) IsActive() bool {
	return !it.IsRevoked() && !it.IsExpired()
}
