package models

import (
	"time"
)

// SigningKey represents an RSA key pair for signing JWTs.
type SigningKey struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	KeyID               string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"key_id" db:"key_id"`
	OrganizationID      *int64     `gorm:"index:idx_signing_keys_org" json:"organization_id,omitempty" db:"organization_id"`
	PublicKeyPEM        string     `gorm:"type:text;not null" json:"public_key_pem" db:"public_key_pem"`
	PrivateKeyEncrypted *string    `gorm:"type:text" json:"private_key_encrypted,omitempty" db:"private_key_encrypted"`
	Algorithm           string     `gorm:"type:varchar(20);not null;default:'RS256'" json:"algorithm" db:"algorithm"`
	CreatedAt           time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	ExpiresAt           time.Time  `gorm:"not null" json:"expires_at" db:"expires_at"`
	RevokedAt           *time.Time `gorm:"default:null;index:idx_signing_keys_active" json:"revoked_at,omitempty" db:"revoked_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// TableName overrides the default table name.
func (SigningKey) TableName() string {
	return "signing_keys"
}

// IsActive returns true if the key is not revoked and not expired.
func (sk *SigningKey) IsActive() bool {
	return sk.RevokedAt == nil && sk.ExpiresAt.After(time.Now())
}

// IsExpired returns true if the key has expired.
func (sk *SigningKey) IsExpired() bool {
	return sk.ExpiresAt.Before(time.Now())
}

// IsRevoked returns true if the key has been revoked.
func (sk *SigningKey) IsRevoked() bool {
	return sk.RevokedAt != nil
}

// IsMasterKey returns true if this is a Janus master key (not customer-specific).
func (sk *SigningKey) IsMasterKey() bool {
	return sk.OrganizationID == nil
}
