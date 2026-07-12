package models

import (
	"time"
)

// UserPublicKey is a user's RSA public key (PEM), embedded into their license
// JWT for run-log signature verification. A user has exactly one active key at a
// time (enforced by a partial unique index on org_user_id WHERE revoked_at IS
// NULL); prior keys are retained as revoked rows so historical run logs still
// verify against the key they were issued with.
type UserPublicKey struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrgUserID    int64      `gorm:"not null;index:idx_user_public_keys_user" json:"org_user_id" db:"org_user_id"`
	PublicKeyPEM string     `gorm:"type:text;not null" json:"public_key_pem" db:"public_key_pem"`
	Fingerprint  string     `gorm:"type:varchar(100);not null" json:"fingerprint" db:"fingerprint"`
	Title        string     `gorm:"type:varchar(255)" json:"title,omitempty" db:"title"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	RevokedAt    *time.Time `gorm:"default:null" json:"revoked_at,omitempty" db:"revoked_at"`

	// Relationships
	OrgUser *OrgUser `gorm:"foreignKey:OrgUserID" json:"org_user,omitempty"`
}

// TableName overrides the default table name.
func (UserPublicKey) TableName() string {
	return "user_public_keys"
}

// IsActive returns true if the key has not been revoked.
func (k *UserPublicKey) IsActive() bool {
	return k.RevokedAt == nil
}
