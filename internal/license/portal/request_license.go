package portal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/requests"
)

// RequestLicense creates a license request for the user against an agreement and
// evaluates the org's auto-acceptance rules. On auto-approval (and if the user
// has an active key) it issues the license immediately; a full agreement is
// denied and the customer admins are notified.
func (s *Service) RequestLicense(ctx context.Context, user *models.OrgUser, agreementID int64) (*models.LicenseRequest, error) {
	var agr models.Agreement
	if err := s.db.DB.WithContext(ctx).First(&agr, agreementID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("agreement not found")
		}

		return nil, err
	}

	if agr.OrganizationID != user.OrganizationID {
		return nil, fmt.Errorf("agreement does not belong to your organization")
	}

	if !agr.IsActive() {
		return nil, fmt.Errorf("agreement is not active")
	}

	var rules []models.AutoAcceptanceRule
	if err := s.db.DB.WithContext(ctx).
		Where("organization_id = ? AND deactivated_at IS NULL AND enabled = ?", user.OrganizationID, true).
		Find(&rules).Error; err != nil {
		return nil, err
	}

	used, err := requests.ActiveSeats(ctx, s.db, agreementID)
	if err != nil {
		return nil, err
	}

	decision := requests.Decide(requests.Input{
		Email:      user.Email,
		Rules:      rules,
		UsedSeats:  used,
		TotalSeats: agr.Seats,
	})

	activeKey, hasKey, err := s.activeKey(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	lr := &models.LicenseRequest{
		OrganizationID: user.OrganizationID,
		OrgUserID:      user.ID,
		AgreementID:    agreementID,
		Status:         models.LicenseRequestStatusPending,
	}

	switch decision.Outcome {
	case requests.OutcomeDenyNoSeats:
		lr.Status = models.LicenseRequestStatusFailed
		lr.Reason = &decision.Reason
		if createErr := s.db.DB.WithContext(ctx).Create(lr).Error; createErr != nil {
			return nil, createErr
		}

		s.notifyAllocationFailure(ctx, user.OrganizationID, decision.Reason)

		return lr, nil

	case requests.OutcomeApprove:
		if !hasKey {
			reason := "auto-approved; awaiting public key upload"
			lr.Reason = &reason
			if createErr := s.db.DB.WithContext(ctx).Create(lr).Error; createErr != nil {
				return nil, createErr
			}

			return lr, nil
		}

		return s.approveAndIssue(ctx, lr, &agr, user, activeKey, decision.Reason, models.LicenseRequestDecisionAuto, nil)

	case requests.OutcomeManual:
		lr.Reason = &decision.Reason
		if createErr := s.db.DB.WithContext(ctx).Create(lr).Error; createErr != nil {
			return nil, createErr
		}

		return lr, nil
	}

	return nil, fmt.Errorf("unexpected decision outcome %q", decision.Outcome)
}

// approveAndIssue persists an approved license request and issues the license in
// one transaction, linking the issued token back to the request.
func (s *Service) approveAndIssue(
	ctx context.Context,
	lr *models.LicenseRequest,
	agr *models.Agreement,
	user *models.OrgUser,
	key *models.UserPublicKey,
	reason, decisionMode string,
	decidedBy *int64,
) (*models.LicenseRequest, error) {
	now := time.Now()
	lr.Status = models.LicenseRequestStatusApproved
	lr.Decision = &decisionMode
	lr.DecidedAt = &now
	lr.DecidedByUserID = decidedBy
	lr.Reason = &reason
	lr.UserPublicKeyID = &key.ID

	err := s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Save inserts a new auto-approved request (ID == 0) or updates an
		// existing pending one (manual admin approval).
		if saveErr := tx.Save(lr).Error; saveErr != nil {
			return saveErr
		}

		tok, issueErr := s.issueLicense(tx, user, agr, key.PublicKeyPEM)
		if issueErr != nil {
			return issueErr
		}

		return tx.Model(lr).Update("issued_token_id", tok.ID).Error
	})
	if err != nil {
		return nil, err
	}

	return lr, nil
}

func (s *Service) activeKey(ctx context.Context, orgUserID int64) (*models.UserPublicKey, bool, error) {
	var key models.UserPublicKey

	err := s.db.DB.WithContext(ctx).
		Where("org_user_id = ? AND revoked_at IS NULL", orgUserID).
		First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	return &key, true, nil
}

func (s *Service) notifyAllocationFailure(ctx context.Context, orgID int64, reason string) {
	if s.notifier == nil {
		s.logger.Printf("allocation failure for org %d (no notifier wired): %s", orgID, reason)

		return
	}

	s.notifier.NotifyAllocationFailure(ctx, orgID, reason)
}
