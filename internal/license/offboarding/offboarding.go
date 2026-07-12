// Package offboarding implements user offboarding & seat reclamation: a license
// is not transferable (it embeds the departing user's key), so offboarding is
// revoke + reclaim. The seat is later filled by a fresh issue to the incoming
// user via the normal request flow.
package offboarding

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Service performs offboarding.
type Service struct {
	db     *db.DB
	admin  cognito.Admin
	logger *log.Logger
}

// NewService creates an offboarding Service.
func NewService(database *db.DB, admin cognito.Admin, logger *log.Logger) *Service {
	return &Service{db: database, admin: admin, logger: logger}
}

// Offboard revokes the user's active license (freeing the seat immediately) and
// active key, deactivates the OrgUser, and disables the user in Cognito. It is
// idempotent — re-running on an already-offboarded user is a no-op.
//
// Billing is unchanged: the seat is reclaimed within the entitlement; reducing
// the Stripe quantity is a separate period-end action. The revoked license JWT
// stays cryptographically valid until exp (no phone-home); the online revocation
// check on /tokens/validate provides the hard cutoff when the ex-user is online.
func (s *Service) Offboard(ctx context.Context, user *models.OrgUser) error {
	err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		reason := models.RevokedReasonOffboarded

		if revErr := tx.Model(&models.IssuedToken{}).
			Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
			Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error; revErr != nil {
			return revErr
		}

		if keyErr := tx.Model(&models.UserPublicKey{}).
			Where("org_user_id = ? AND revoked_at IS NULL", user.ID).
			Update("revoked_at", now).Error; keyErr != nil {
			return keyErr
		}

		return tx.Model(&models.OrgUser{}).Where("id = ?", user.ID).Update("deactivated_at", now).Error
	})
	if err != nil {
		return fmt.Errorf("offboard %d: %w", user.ID, err)
	}

	// Cognito access cutoff (after the DB seat reclamation is committed). Group
	// removal is best-effort; a disable failure is surfaced so the admin can retry
	// (both operations are idempotent).
	if user.Email == "" {
		return nil
	}

	if user.IsAdmin() {
		if rmErr := s.admin.RemoveUserFromGroup(ctx, user.Email, models.OrgUserRoleCustomerAdmin); rmErr != nil {
			s.logger.Printf("offboard: remove group for %s failed (continuing): %v", user.Email, rmErr)
		}
	}

	if disErr := s.admin.DisableUser(ctx, user.Email); disErr != nil {
		return fmt.Errorf("offboard %d: disable cognito user (seat already reclaimed; safe to retry): %w", user.ID, disErr)
	}

	return nil
}
