// Package portal implements the end-user ("/me") business logic of the
// management portal: public-key management, license requests, and license
// issuance/download. HTTP handlers are thin wrappers over this service.
package portal

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/signing"
)

// LicenseIssuer issues a signed license + tracking metadata for an agreement.
type LicenseIssuer interface {
	IssueLicenseFromAgreement(agr *models.Agreement, userEmail string, duration time.Duration, signingPublicKey string) (*jwt.IssuedLicense, error)
}

// Notifier is notified of allocation failures (implemented by A8). A nil
// Notifier disables notifications.
type Notifier interface {
	NotifyAllocationFailure(ctx context.Context, orgID int64, reason string)
}

// Service holds the end-user portal logic.
type Service struct {
	db       *db.DB
	issuer   LicenseIssuer
	notifier Notifier
	logger   *log.Logger
}

// NewService creates a portal Service. notifier may be nil.
func NewService(database *db.DB, issuer LicenseIssuer, notifier Notifier, logger *log.Logger) *Service {
	return &Service{db: database, issuer: issuer, notifier: notifier, logger: logger}
}

// ReplaceKey validates and sets the caller's single active public key, atomically
// revoking the prior key and — if the user holds an active license — revoking that
// license and reissuing a new one bound to the new key (the documented
// replacement transaction).
func (s *Service) ReplaceKey(ctx context.Context, user *models.OrgUser, pem, title string) (*models.UserPublicKey, error) {
	pub, err := signing.LoadPublicKeyFromPEM(pem)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	fp, err := fingerprint(pub)
	if err != nil {
		return nil, err
	}

	var newKey models.UserPublicKey

	err = s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if revErr := tx.Model(&models.UserPublicKey{}).
			Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
			Update("revoked_at", now).Error; revErr != nil {
			return revErr
		}

		newKey = models.UserPublicKey{OrgUserID: user.ID, PublicKeyPEM: pem, Fingerprint: fp, Title: title}
		if createErr := tx.Create(&newKey).Error; createErr != nil {
			return createErr
		}

		return s.reissueActiveLicense(tx, user, pem)
	})
	if err != nil {
		return nil, err
	}

	return &newKey, nil
}

// reissueActiveLicense revokes the user's current active license (if any) and
// reissues one bound to keyPEM, relinking the originating license request.
func (s *Service) reissueActiveLicense(tx *gorm.DB, user *models.OrgUser, keyPEM string) error {
	var current models.IssuedToken

	err := tx.Where("org_user_id = ? AND revoked_at IS NULL", user.ID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	var lr models.LicenseRequest
	if reqErr := tx.Where("issued_token_id = ?", current.ID).First(&lr).Error; reqErr != nil {
		return fmt.Errorf("locate license request for active token: %w", reqErr)
	}

	var agr models.Agreement
	if agrErr := tx.First(&agr, lr.AgreementID).Error; agrErr != nil {
		return agrErr
	}

	reason := models.RevokedReasonKeyReplaced
	if revErr := tx.Model(&models.IssuedToken{}).Where("id = ?", current.ID).
		Updates(map[string]interface{}{"revoked_at": time.Now(), "revoked_reason": reason}).Error; revErr != nil {
		return revErr
	}

	newTok, err := s.issueLicense(tx, user, &agr, keyPEM)
	if err != nil {
		return err
	}

	return tx.Model(&models.LicenseRequest{}).Where("id = ?", lr.ID).Update("issued_token_id", newTok.ID).Error
}

// issueLicense generates a license bound to keyPEM and persists an issued_tokens
// row (JTI for revocation + the JWT for download). Reads (org/license/signing key)
// go through the generator; the row is written on tx.
func (s *Service) issueLicense(tx *gorm.DB, user *models.OrgUser, agr *models.Agreement, keyPEM string) (*models.IssuedToken, error) {
	duration := jwt.LicenseValidity(agr, time.Now())
	if duration <= 0 {
		return nil, fmt.Errorf("agreement term has expired; cannot issue a license")
	}

	issued, err := s.issuer.IssueLicenseFromAgreement(agr, user.Email, duration, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("issue license: %w", err)
	}

	tok := &models.IssuedToken{
		JTI:            issued.JTI,
		OrganizationID: user.OrganizationID,
		OrgUserID:      &user.ID,
		UserEmail:      user.Email,
		KeyID:          issued.KeyID,
		IssuedAt:       time.Now(),
		ExpiresAt:      issued.ExpiresAt,
		TokenJWT:       issued.Token,
	}
	if createErr := tx.Create(tok).Error; createErr != nil {
		return nil, createErr
	}

	return tok, nil
}

func fingerprint(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}

	sum := sha256.Sum256(der)

	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}
