// Package sso provisions and records per-organization Cognito identity providers.
// SSO setup is create-once / read-only from the portal: changing a live IdP can
// lock out every SSO user, so mutations are support-only (see A6.1).
package sso

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/models"
)

// maxProviderNameLen is Cognito's identity-provider name limit.
const maxProviderNameLen = 32

// LoginURLFunc builds the hosted-UI deep link that sends users straight to their
// IdP, given the Cognito provider name.
type LoginURLFunc func(providerName string) string

// Service provisions + records SSO configurations.
type Service struct {
	db         *db.DB
	admin      cognito.Admin
	encryptor  *keys.Encryptor
	userPoolID string
	loginURL   LoginURLFunc
	logger     *log.Logger
}

// NewService creates an SSO Service.
func NewService(database *db.DB, admin cognito.Admin, encryptor *keys.Encryptor, userPoolID string, loginURL LoginURLFunc, logger *log.Logger) *Service {
	return &Service{db: database, admin: admin, encryptor: encryptor, userPoolID: userPoolID, loginURL: loginURL, logger: logger}
}

// SetupInput is a request to provision an org's SSO.
type SetupInput struct {
	OrganizationID int64
	ProviderName   string
	OIDCIssuer     string
	ClientID       string
	ClientSecret   string
	Scopes         string
	AttributeMap   map[string]string
}

// Setup provisions the IdP in Cognito and records the SSOConfiguration. It is
// create-once: a second call for an org with an existing config is rejected.
func (s *Service) Setup(ctx context.Context, in SetupInput) (*models.SSOConfiguration, error) {
	if in.ProviderName == "" || len(in.ProviderName) > maxProviderNameLen {
		return nil, fmt.Errorf("provider name must be 1-%d chars", maxProviderNameLen)
	}

	if in.OIDCIssuer == "" || in.ClientID == "" {
		return nil, fmt.Errorf("oidc issuer and client id are required")
	}

	// Create-once guard.
	var existing models.SSOConfiguration
	err := s.db.DB.WithContext(ctx).Where("organization_id = ?", in.OrganizationID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("SSO is already configured for this organization; contact support to change it")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	scopes := in.Scopes
	if scopes == "" {
		scopes = "openid email profile"
	}

	// Provision the IdP in Cognito first; on success, record it.
	if provErr := s.admin.CreateIdentityProvider(ctx, cognito.IdentityProvider{
		Name:         in.ProviderName,
		OIDCIssuer:   in.OIDCIssuer,
		ClientID:     in.ClientID,
		ClientSecret: in.ClientSecret,
		Scopes:       scopes,
		AttributeMap: in.AttributeMap,
	}); provErr != nil {
		return nil, fmt.Errorf("provision identity provider: %w", provErr)
	}

	cfg := &models.SSOConfiguration{
		OrganizationID:   in.OrganizationID,
		ProviderName:     in.ProviderName,
		UserPoolID:       s.userPoolID,
		OIDCIssuer:       in.OIDCIssuer,
		OIDCClientID:     in.ClientID,
		Scopes:           scopes,
		AttributeMapping: models.JSONStringMap(in.AttributeMap),
		CognitoStatus:    models.SSOStatusActive,
		LoginURL:         s.loginURL(in.ProviderName),
	}

	if in.ClientSecret != "" {
		enc, encErr := s.encryptor.Encrypt(in.ClientSecret)
		if encErr != nil {
			return nil, fmt.Errorf("encrypt client secret: %w", encErr)
		}

		cfg.OIDCClientSecretEncrypted = &enc
	}

	if createErr := s.db.DB.WithContext(ctx).Create(cfg).Error; createErr != nil {
		// The IdP was created in Cognito but the record write failed — an orphaned
		// provider that support/reconciliation must clean up.
		s.logger.Printf("sso: IdP %q provisioned but record write failed for org %d: %v", in.ProviderName, in.OrganizationID, createErr)

		return nil, createErr
	}

	return cfg, nil
}

// Get returns the org's SSO configuration (the client secret is never serialized
// — json:"-" — so the read is safe to return directly).
func (s *Service) Get(ctx context.Context, orgID int64) (*models.SSOConfiguration, error) {
	var cfg models.SSOConfiguration

	err := s.db.DB.WithContext(ctx).Where("organization_id = ?", orgID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("no SSO configuration for this organization")
	}

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
