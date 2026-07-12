// Package signup orchestrates organization signup and first-admin bootstrap.
//
// The first admin is a NATIVE Cognito user (not federated) — this breaks the
// chicken-and-egg of "you need an admin to configure SSO, but SSO is how users
// log in." Every Cognito step is idempotent and the whole flow is retry-safe:
// re-running with the same admin email returns the existing org/admin rather
// than creating duplicates.
package signup

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// Service performs org signup + first-admin bootstrap.
type Service struct {
	db     *db.DB
	admin  cognito.Admin
	stripe billing.Stripe
	logger *log.Logger
}

// NewService creates a signup Service. stripe may be nil, in which case signup
// creates the org + admin but provisions no Stripe customer (non-billing setups
// and unit paths); when present, the org's billing customer is created eagerly.
func NewService(database *db.DB, admin cognito.Admin, stripe billing.Stripe, logger *log.Logger) *Service {
	return &Service{db: database, admin: admin, stripe: stripe, logger: logger}
}

// Input is a signup request.
type Input struct {
	OrgName     string
	CustomerID  string
	AdminEmail  string
	ContactName string
}

// Result is the created (or already-existing) organization and its first admin.
type Result struct {
	Organization *models.Organization
	Admin        *models.OrgUser
}

// SignUp provisions the first admin in Cognito (native user + customer_admin
// group) and persists the Organization + admin OrgUser. It is idempotent: a
// repeat call for the same admin returns the existing records.
func (s *Service) SignUp(ctx context.Context, in Input) (*Result, error) {
	if in.OrgName == "" || in.CustomerID == "" || in.AdminEmail == "" {
		return nil, fmt.Errorf("org name, customer id, and admin email are required")
	}

	// Cognito (all idempotent): ensure the group, ensure the native admin user,
	// add the user to the group.
	if err := s.admin.EnsureGroup(ctx, models.OrgUserRoleCustomerAdmin); err != nil {
		return nil, fmt.Errorf("ensure customer_admin group: %w", err)
	}

	sub, err := s.admin.EnsureUser(ctx, in.AdminEmail)
	if err != nil {
		return nil, fmt.Errorf("ensure admin user: %w", err)
	}

	if err := s.admin.AddUserToGroup(ctx, in.AdminEmail, models.OrgUserRoleCustomerAdmin); err != nil {
		return nil, fmt.Errorf("add admin to group: %w", err)
	}

	// Idempotency: if this Cognito sub already maps to an OrgUser, signup
	// completed on a prior attempt — return the existing records.
	existingUser, existingOrg, err := s.lookupExisting(ctx, sub)
	if err != nil {
		return nil, err
	}

	if existingUser != nil {
		if err := s.ensureBilling(ctx, existingOrg); err != nil {
			return nil, err
		}

		return &Result{Organization: existingOrg, Admin: existingUser}, nil
	}

	// Persist org + admin atomically.
	var org models.Organization

	var admin models.OrgUser

	adminEmail := in.AdminEmail

	err = s.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		org = models.Organization{Name: in.OrgName, CustomerID: in.CustomerID, Email: &adminEmail}
		if in.ContactName != "" {
			contact := in.ContactName
			org.ContactName = &contact
		}
		if createErr := tx.Create(&org).Error; createErr != nil {
			return fmt.Errorf("create organization: %w", createErr)
		}

		admin = models.OrgUser{
			OrganizationID: org.ID,
			CognitoSub:     sub,
			Email:          in.AdminEmail,
			Role:           models.OrgUserRoleCustomerAdmin,
			Source:         models.OrgUserSourceInvited,
		}
		if createErr := tx.Create(&admin).Error; createErr != nil {
			return fmt.Errorf("create admin org user: %w", createErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.Printf("signup: created organization %q (id %d) with first admin %s", org.Name, org.ID, in.AdminEmail)

	if err := s.ensureBilling(ctx, &org); err != nil {
		return nil, err
	}

	return &Result{Organization: &org, Admin: &admin}, nil
}

// ensureBilling creates the org's Stripe customer (idempotent) and reflects the
// resulting id back onto the org so the caller's Result carries it. It is a no-op
// when Stripe is not configured. Runs outside the org-create transaction because
// it makes a network call.
func (s *Service) ensureBilling(ctx context.Context, org *models.Organization) error {
	if s.stripe == nil {
		return nil
	}

	customerID, err := billing.EnsureCustomer(ctx, s.db, s.stripe, org.ID)
	if err != nil {
		return fmt.Errorf("ensure stripe customer: %w", err)
	}

	org.StripeCustomerID = &customerID

	return nil
}

func (s *Service) lookupExisting(ctx context.Context, sub string) (*models.OrgUser, *models.Organization, error) {
	var user models.OrgUser

	err := s.db.DB.WithContext(ctx).Where("cognito_sub = ?", sub).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("lookup existing org user: %w", err)
	}

	var org models.Organization
	if err := s.db.DB.WithContext(ctx).First(&org, user.OrganizationID).Error; err != nil {
		return nil, nil, fmt.Errorf("load organization: %w", err)
	}

	return &user, &org, nil
}
