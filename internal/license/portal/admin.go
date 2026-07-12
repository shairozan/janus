package portal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/requests"
)

// AdminService holds customer-admin operations that aren't license issuance:
// member management (list/promote/demote) and auto-acceptance rule CRUD.
type AdminService struct {
	db     *db.DB
	admin  cognito.Admin
	logger *log.Logger
}

// NewAdminService creates an AdminService.
func NewAdminService(database *db.DB, admin cognito.Admin, logger *log.Logger) *AdminService {
	return &AdminService{db: database, admin: admin, logger: logger}
}

// ListMembers returns the org's active members.
func (a *AdminService) ListMembers(ctx context.Context, orgID int64) ([]models.OrgUser, error) {
	var members []models.OrgUser
	if err := a.db.DB.WithContext(ctx).
		Where("organization_id = ? AND deactivated_at IS NULL", orgID).
		Order("created_at").
		Find(&members).Error; err != nil {
		return nil, err
	}

	return members, nil
}

// Promote grants the customer_admin role: adds the user to the Cognito group
// (authoritative) then updates the persisted role.
func (a *AdminService) Promote(ctx context.Context, orgID, targetUserID int64) (*models.OrgUser, error) {
	return a.setRole(ctx, orgID, targetUserID, models.OrgUserRoleCustomerAdmin, true)
}

// Demote revokes the customer_admin role.
func (a *AdminService) Demote(ctx context.Context, orgID, targetUserID int64) (*models.OrgUser, error) {
	return a.setRole(ctx, orgID, targetUserID, models.OrgUserRoleMember, false)
}

func (a *AdminService) setRole(ctx context.Context, orgID, targetUserID int64, role string, addToGroup bool) (*models.OrgUser, error) {
	user, err := a.loadMember(ctx, orgID, targetUserID)
	if err != nil {
		return nil, err
	}

	// Cognito group is the authoritative admin signal — update it first.
	if addToGroup {
		if grpErr := a.admin.AddUserToGroup(ctx, user.Email, models.OrgUserRoleCustomerAdmin); grpErr != nil {
			return nil, fmt.Errorf("update cognito group: %w", grpErr)
		}
	} else {
		if grpErr := a.admin.RemoveUserFromGroup(ctx, user.Email, models.OrgUserRoleCustomerAdmin); grpErr != nil {
			return nil, fmt.Errorf("update cognito group: %w", grpErr)
		}
	}

	if err := a.db.DB.WithContext(ctx).Model(&models.OrgUser{}).
		Where("id = ?", user.ID).
		Updates(map[string]interface{}{"role": role, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}

	user.Role = role
	a.logger.Printf("role change: org %d user %d → %s", orgID, user.ID, role)

	return user, nil
}

func (a *AdminService) loadMember(ctx context.Context, orgID, userID int64) (*models.OrgUser, error) {
	var user models.OrgUser

	err := a.db.DB.WithContext(ctx).First(&user, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("member not found")
	}

	if err != nil {
		return nil, err
	}

	if user.OrganizationID != orgID {
		return nil, fmt.Errorf("member does not belong to this organization")
	}

	if !user.IsActive() {
		return nil, fmt.Errorf("member is deactivated")
	}

	return &user, nil
}

// Member loads an active org member (scoped to the org).
func (a *AdminService) Member(ctx context.Context, orgID, userID int64) (*models.OrgUser, error) {
	return a.loadMember(ctx, orgID, userID)
}

const defaultActivityLimit = 50

// ListActivity returns the org's most recent audit events (the admin activity log).
func (a *AdminService) ListActivity(ctx context.Context, orgID int64, limit int) ([]models.AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = defaultActivityLimit
	}

	var events []models.AuditEvent
	if err := a.db.DB.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("occurred_at DESC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil, err
	}

	return events, nil
}

// AgreementUsage is an agreement plus its current seat consumption.
type AgreementUsage struct {
	models.Agreement
	UsedSeats int `json:"used_seats"`
}

// ListAgreements returns the org's active agreements with seat usage (used/total),
// for the seat-management screen.
func (a *AdminService) ListAgreements(ctx context.Context, orgID int64) ([]AgreementUsage, error) {
	var agreements []models.Agreement
	if err := a.db.DB.WithContext(ctx).
		Where("organization_id = ? AND deactivated_at IS NULL", orgID).
		Order("created_at").
		Find(&agreements).Error; err != nil {
		return nil, err
	}

	out := make([]AgreementUsage, 0, len(agreements))
	for i := range agreements {
		used, err := requests.ActiveSeats(ctx, a.db, agreements[i].ID)
		if err != nil {
			return nil, err
		}

		out = append(out, AgreementUsage{Agreement: agreements[i], UsedSeats: used})
	}

	return out, nil
}

// ListRequests returns the org's license requests, newest first (the admin queue).
func (a *AdminService) ListRequests(ctx context.Context, orgID int64) ([]models.LicenseRequest, error) {
	var reqs []models.LicenseRequest
	if err := a.db.DB.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&reqs).Error; err != nil {
		return nil, err
	}

	return reqs, nil
}

// RuleInput is a request to create an auto-acceptance rule.
type RuleInput struct {
	AgreementID  *int64
	RuleType     string
	MatchDomain  *string
	MaxAutoSeats *int
}

// CreateRule creates an auto-acceptance rule after validating the type/fields.
func (a *AdminService) CreateRule(ctx context.Context, orgID, createdByUserID int64, in RuleInput) (*models.AutoAcceptanceRule, error) {
	switch in.RuleType {
	case models.AutoAcceptanceRuleTypeEmailDomain:
		if in.MatchDomain == nil || *in.MatchDomain == "" {
			return nil, fmt.Errorf("match_domain is required for an email_domain rule")
		}
	case models.AutoAcceptanceRuleTypeSeatThreshold:
		if in.MaxAutoSeats == nil || *in.MaxAutoSeats <= 0 {
			return nil, fmt.Errorf("max_auto_seats must be > 0 for a seat_threshold rule")
		}
	default:
		return nil, fmt.Errorf("unknown rule_type %q", in.RuleType)
	}

	rule := &models.AutoAcceptanceRule{
		OrganizationID:  orgID,
		AgreementID:     in.AgreementID,
		RuleType:        in.RuleType,
		MatchDomain:     in.MatchDomain,
		MaxAutoSeats:    in.MaxAutoSeats,
		Enabled:         true,
		CreatedByUserID: createdByUserID,
	}
	if err := a.db.DB.WithContext(ctx).Create(rule).Error; err != nil {
		return nil, err
	}

	return rule, nil
}

// ListRules returns the org's active auto-acceptance rules.
func (a *AdminService) ListRules(ctx context.Context, orgID int64) ([]models.AutoAcceptanceRule, error) {
	var rules []models.AutoAcceptanceRule
	if err := a.db.DB.WithContext(ctx).
		Where("organization_id = ? AND deactivated_at IS NULL", orgID).
		Order("created_at").
		Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}

// SetRuleEnabled enables/disables a rule (scoped to the org).
func (a *AdminService) SetRuleEnabled(ctx context.Context, orgID, ruleID int64, enabled bool) (*models.AutoAcceptanceRule, error) {
	rule, err := a.loadRule(ctx, orgID, ruleID)
	if err != nil {
		return nil, err
	}

	if err := a.db.DB.WithContext(ctx).Model(rule).Update("enabled", enabled).Error; err != nil {
		return nil, err
	}

	rule.Enabled = enabled

	return rule, nil
}

// DeleteRule soft-deletes a rule (scoped to the org).
func (a *AdminService) DeleteRule(ctx context.Context, orgID, ruleID int64) error {
	rule, err := a.loadRule(ctx, orgID, ruleID)
	if err != nil {
		return err
	}

	return a.db.DB.WithContext(ctx).Model(rule).Update("deactivated_at", time.Now()).Error
}

func (a *AdminService) loadRule(ctx context.Context, orgID, ruleID int64) (*models.AutoAcceptanceRule, error) {
	var rule models.AutoAcceptanceRule

	err := a.db.DB.WithContext(ctx).First(&rule, ruleID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("rule not found")
	}

	if err != nil {
		return nil, err
	}

	if rule.OrganizationID != orgID {
		return nil, fmt.Errorf("rule does not belong to this organization")
	}

	return &rule, nil
}
