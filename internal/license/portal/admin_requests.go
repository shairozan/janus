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

// ApproveRequest is a customer-admin manual approval of a pending license
// request: it issues the license to the requester (using their active key) and
// records the deciding admin.
func (s *Service) ApproveRequest(ctx context.Context, admin *models.OrgUser, requestID int64) (*models.LicenseRequest, error) {
	lr, requester, agr, err := s.loadPendingRequest(ctx, admin.OrganizationID, requestID)
	if err != nil {
		return nil, err
	}

	key, hasKey, err := s.activeKey(ctx, requester.ID)
	if err != nil {
		return nil, err
	}

	if !hasKey {
		return nil, fmt.Errorf("requester has no active public key; cannot issue a license")
	}

	used, err := requests.ActiveSeats(ctx, s.db, agr.ID)
	if err != nil {
		return nil, err
	}

	if used >= agr.Seats {
		return nil, fmt.Errorf("no seats available on this agreement")
	}

	return s.approveAndIssue(ctx, lr, agr, requester, key, "approved by admin", models.LicenseRequestDecisionManual, &admin.ID)
}

// RejectRequest is a customer-admin rejection of a pending license request.
func (s *Service) RejectRequest(ctx context.Context, admin *models.OrgUser, requestID int64, reason string) (*models.LicenseRequest, error) {
	lr, _, _, err := s.loadPendingRequest(ctx, admin.OrganizationID, requestID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	decision := models.LicenseRequestDecisionManual
	lr.Status = models.LicenseRequestStatusRejected
	lr.Decision = &decision
	lr.DecidedByUserID = &admin.ID
	lr.DecidedAt = &now
	lr.Reason = &reason

	if err := s.db.DB.WithContext(ctx).Save(lr).Error; err != nil {
		return nil, err
	}

	return lr, nil
}

// loadPendingRequest loads a pending request (scoped to the org) plus its
// requester and agreement.
func (s *Service) loadPendingRequest(ctx context.Context, orgID, requestID int64) (*models.LicenseRequest, *models.OrgUser, *models.Agreement, error) {
	var lr models.LicenseRequest
	if err := s.db.DB.WithContext(ctx).First(&lr, requestID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, fmt.Errorf("license request not found")
		}

		return nil, nil, nil, err
	}

	if lr.OrganizationID != orgID {
		return nil, nil, nil, fmt.Errorf("license request does not belong to your organization")
	}

	if lr.Status != models.LicenseRequestStatusPending {
		return nil, nil, nil, fmt.Errorf("license request is not pending (status: %s)", lr.Status)
	}

	var requester models.OrgUser
	if err := s.db.DB.WithContext(ctx).First(&requester, lr.OrgUserID).Error; err != nil {
		return nil, nil, nil, err
	}

	var agr models.Agreement
	if err := s.db.DB.WithContext(ctx).First(&agr, lr.AgreementID).Error; err != nil {
		return nil, nil, nil, err
	}

	return &lr, &requester, &agr, nil
}
