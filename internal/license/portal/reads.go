package portal

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/models"
)

// Profile is the /me response.
type Profile struct {
	ID             int64  `json:"id"`
	OrganizationID int64  `json:"organization_id"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	HasActiveKey   bool   `json:"has_active_key"`
	HasLicense     bool   `json:"has_license"`
	// IsStaff is true when the caller's email domain is in the Janus-staff
	// allowlist. The MeProfile handler sets it (it owns the domain list); it
	// drives the portal's staff "Admin" UI only — the staff endpoints stay
	// independently enforced by requireUserInfo.
	IsStaff bool `json:"is_staff"`
}

// Profile returns the caller's profile (org, role, key/license status).
func (s *Service) Profile(ctx context.Context, user *models.OrgUser) (*Profile, error) {
	_, hasKey, err := s.activeKey(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	var licenseCount int64
	if countErr := s.db.DB.WithContext(ctx).Model(&models.IssuedToken{}).
		Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
		Count(&licenseCount).Error; countErr != nil {
		return nil, countErr
	}

	return &Profile{
		ID:             user.ID,
		OrganizationID: user.OrganizationID,
		Email:          user.Email,
		Role:           user.Role,
		HasActiveKey:   hasKey,
		HasLicense:     licenseCount > 0,
	}, nil
}

// Keys returns the caller's active public key (nil if none) and revoked history,
// newest first.
func (s *Service) Keys(ctx context.Context, user *models.OrgUser) (active *models.UserPublicKey, history []models.UserPublicKey, err error) {
	var keys []models.UserPublicKey
	if listErr := s.db.DB.WithContext(ctx).
		Where("org_user_id = ?", user.ID).
		Order("created_at DESC").
		Find(&keys).Error; listErr != nil {
		return nil, nil, listErr
	}

	for i := range keys {
		if keys[i].IsActive() {
			active = &keys[i]
		} else {
			history = append(history, keys[i])
		}
	}

	return active, history, nil
}

// ListRequests returns the caller's license requests, newest first.
func (s *Service) ListRequests(ctx context.Context, user *models.OrgUser) ([]models.LicenseRequest, error) {
	var reqs []models.LicenseRequest
	if err := s.db.DB.WithContext(ctx).
		Where("org_user_id = ?", user.ID).
		Order("created_at DESC").
		Find(&reqs).Error; err != nil {
		return nil, err
	}

	return reqs, nil
}

// License returns the caller's current active license JWT for download. Returns
// an error if the user has no active license.
func (s *Service) License(ctx context.Context, user *models.OrgUser) (string, error) {
	var tok models.IssuedToken

	err := s.db.DB.WithContext(ctx).
		Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
		First(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("no active license; request one first")
	}

	if err != nil {
		return "", err
	}

	return tok.TokenJWT, nil
}
