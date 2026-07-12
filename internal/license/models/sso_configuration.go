package models

import (
	"time"
)

// SSOConfiguration provisioning statuses.
const (
	SSOStatusPending = "pending"
	SSOStatusActive  = "active"
	SSOStatusFailed  = "failed"
)

// SSOConfiguration is the durable lifecycle record of a per-org Cognito identity
// provider. It is create-once / read-only from the portal's perspective —
// changes go through internal/support tooling because editing a live IdP can
// lock out every SSO user in the org.
//
// The client secret is write-only: it is encrypted at rest and never serialized
// (json:"-"); responses expose only the HasClientSecret presence flag.
type SSOConfiguration struct {
	ID                        int64         `gorm:"primaryKey;autoIncrement" json:"id" db:"id"`
	OrganizationID            int64         `gorm:"not null;uniqueIndex:idx_sso_config_org" json:"organization_id" db:"organization_id"`
	ProviderName              string        `gorm:"type:varchar(32);not null" json:"provider_name" db:"provider_name"`
	UserPoolID                string        `gorm:"type:varchar(100);not null" json:"user_pool_id" db:"user_pool_id"`
	OIDCIssuer                string        `gorm:"column:oidc_issuer;type:varchar(512);not null" json:"oidc_issuer" db:"oidc_issuer"`
	OIDCClientID              string        `gorm:"column:oidc_client_id;type:varchar(255);not null" json:"oidc_client_id" db:"oidc_client_id"`
	OIDCClientSecretEncrypted *string       `gorm:"column:oidc_client_secret_encrypted;type:text" json:"-" db:"oidc_client_secret_encrypted"`
	Scopes                    string        `gorm:"type:varchar(255);not null;default:'openid email profile'" json:"scopes" db:"scopes"`
	AttributeMapping          JSONStringMap `gorm:"type:jsonb" json:"attribute_mapping,omitempty" db:"attribute_mapping"`
	CognitoStatus             string        `gorm:"type:varchar(20);not null" json:"cognito_status" db:"cognito_status"`
	LoginURL                  string        `gorm:"type:varchar(1024)" json:"login_url" db:"login_url"`
	CreatedAt                 time.Time     `gorm:"not null;default:now()" json:"created_at" db:"created_at"`
	UpdatedAt                 time.Time     `gorm:"not null;default:now()" json:"updated_at" db:"updated_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

// TableName overrides the default table name.
func (SSOConfiguration) TableName() string {
	return "sso_configurations"
}

// HasClientSecret reports whether a client secret is stored — the only secret
// detail safe to expose in an API response.
func (c *SSOConfiguration) HasClientSecret() bool {
	return c.OIDCClientSecretEncrypted != nil && *c.OIDCClientSecretEncrypted != ""
}
