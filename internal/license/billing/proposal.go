package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/reconcile"
)

// ProposalService runs the agreement proposal/negotiation lifecycle and its
// Stripe-backed fulfilment.
type ProposalService struct {
	db     *db.DB
	stripe Stripe
	logger *log.Logger
}

// NewProposalService creates a ProposalService.
func NewProposalService(database *db.DB, stripe Stripe, logger *log.Logger) *ProposalService {
	return &ProposalService{db: database, stripe: stripe, logger: logger}
}

// RequestInput describes the terms a customer requests (or counters with).
type RequestInput struct {
	Tier         *string
	Seats        int
	LicenseModel string
	ValidityDays int
}

// OfferLineItem references a catalog price + quantity for a staff offer.
type OfferLineItem struct {
	PriceID  int64 // stripe_prices.id (catalog row)
	Quantity int
}

// OfferInput is a Janus-staff priced offer.
type OfferInput struct {
	LineItems  []OfferLineItem
	ValidUntil *time.Time
	Message    string
}

// Request creates a proposal in the `requested` state (customer admin).
func (p *ProposalService) Request(ctx context.Context, orgID, byUserID int64, in RequestInput) (*models.AgreementProposal, error) {
	if in.Seats <= 0 || in.LicenseModel == "" {
		return nil, fmt.Errorf("seats (>0) and license_model are required")
	}

	validity := in.ValidityDays
	if validity <= 0 {
		validity = models.DefaultProposalValidityDays
	}

	pr := &models.AgreementProposal{
		OrganizationID:    orgID,
		RequestedByUserID: byUserID,
		Status:            models.ProposalStatusRequested,
		Tier:              in.Tier,
		Seats:             in.Seats,
		LicenseModel:      in.LicenseModel,
		ValidityDays:      validity,
	}
	if err := p.db.DB.WithContext(ctx).Create(pr).Error; err != nil {
		return nil, err
	}

	return pr, nil
}

// List returns the org's proposals, newest first, each with its line items and
// catalog price (for the onboarding/negotiation screen).
func (p *ProposalService) List(ctx context.Context, orgID int64) ([]models.AgreementProposal, error) {
	var proposals []models.AgreementProposal
	if err := p.db.DB.WithContext(ctx).
		Preload("LineItems.Price").
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&proposals).Error; err != nil {
		return nil, err
	}

	return proposals, nil
}

// Get returns a single proposal (with line items + price) scoped to the org.
func (p *ProposalService) Get(ctx context.Context, orgID, proposalID int64) (*models.AgreementProposal, error) {
	var pr models.AgreementProposal

	err := p.db.DB.WithContext(ctx).
		Preload("LineItems.Price").
		First(&pr, proposalID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("proposal not found")
	}

	if err != nil {
		return nil, err
	}

	if pr.OrganizationID != orgID {
		return nil, fmt.Errorf("proposal does not belong to your organization")
	}

	return &pr, nil
}

// Offer prices a requested/countered proposal from catalog rows (Janus staff).
func (p *ProposalService) Offer(ctx context.Context, proposalID int64, in OfferInput) (*models.AgreementProposal, error) {
	if len(in.LineItems) == 0 {
		return nil, fmt.Errorf("at least one line item is required")
	}

	pr, err := p.load(ctx, proposalID)
	if err != nil {
		return nil, err
	}

	if pr.Status != models.ProposalStatusRequested && pr.Status != models.ProposalStatusCountered {
		return nil, fmt.Errorf("proposal is not awaiting an offer (status: %s)", pr.Status)
	}

	err = p.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if delErr := tx.Where("proposal_id = ?", pr.ID).Delete(&models.ProposalLineItem{}).Error; delErr != nil {
			return delErr
		}

		total := 0
		for _, li := range in.LineItems {
			var price models.StripePrice
			if priceErr := tx.First(&price, li.PriceID).Error; priceErr != nil {
				return fmt.Errorf("catalog price %d: %w", li.PriceID, priceErr)
			}

			amount := price.UnitAmountCents * li.Quantity
			total += amount

			if liErr := tx.Create(&models.ProposalLineItem{
				ProposalID: pr.ID, PriceID: price.ID, Quantity: li.Quantity, AmountCents: amount,
			}).Error; liErr != nil {
				return liErr
			}
		}

		return tx.Model(pr).Updates(map[string]interface{}{
			"status":             models.ProposalStatusProposed,
			"total_amount_cents": total,
			"valid_until":        in.ValidUntil,
			"message":            nullableString(in.Message),
		}).Error
	})
	if err != nil {
		return nil, err
	}

	return p.loadWithItems(ctx, proposalID)
}

// Counter creates a new versioned proposal (status `countered`) linked to its
// predecessor, carrying the customer's revised terms.
func (p *ProposalService) Counter(ctx context.Context, orgID, proposalID int64, in RequestInput, message string) (*models.AgreementProposal, error) {
	parent, err := p.loadScoped(ctx, orgID, proposalID)
	if err != nil {
		return nil, err
	}

	if parent.Status != models.ProposalStatusProposed {
		return nil, fmt.Errorf("only a proposed offer can be countered (status: %s)", parent.Status)
	}

	validity := in.ValidityDays
	if validity <= 0 {
		validity = parent.ValidityDays
	}

	counter := &models.AgreementProposal{
		OrganizationID:    orgID,
		RequestedByUserID: parent.RequestedByUserID,
		Status:            models.ProposalStatusCountered,
		Tier:              in.Tier,
		Seats:             in.Seats,
		LicenseModel:      in.LicenseModel,
		ValidityDays:      validity,
		ParentProposalID:  &parent.ID,
		Message:           nullableString(message),
	}
	if err := p.db.DB.WithContext(ctx).Create(counter).Error; err != nil {
		return nil, err
	}

	return counter, nil
}

// Accept marks a proposed offer accepted (customer admin).
func (p *ProposalService) Accept(ctx context.Context, orgID, proposalID int64) (*models.AgreementProposal, error) {
	pr, err := p.loadScoped(ctx, orgID, proposalID)
	if err != nil {
		return nil, err
	}

	if pr.Status != models.ProposalStatusProposed {
		return nil, fmt.Errorf("only a proposed offer can be accepted (status: %s)", pr.Status)
	}

	if err := p.db.DB.WithContext(ctx).Model(pr).Update("status", models.ProposalStatusAccepted).Error; err != nil {
		return nil, err
	}

	pr.Status = models.ProposalStatusAccepted

	return pr, nil
}

// Pay creates the Stripe customer (if needed) + subscription and activates the
// Agreement, moving the proposal to `paid`. Webhook confirmation flips it to
// `fulfilled` (see FulfillBySubscription).
func (p *ProposalService) Pay(ctx context.Context, orgID, proposalID int64) (*models.AgreementProposal, error) {
	pr, err := p.loadScoped(ctx, orgID, proposalID)
	if err != nil {
		return nil, err
	}

	if pr.Status != models.ProposalStatusAccepted {
		return nil, fmt.Errorf("proposal must be accepted before payment (status: %s)", pr.Status)
	}

	perSeatPrice, err := p.perSeatPrice(ctx, pr.ID)
	if err != nil {
		return nil, err
	}

	license, err := p.licenseForTier(ctx, pr.Tier)
	if err != nil {
		return nil, err
	}

	customerID, err := p.ensureCustomer(ctx, orgID)
	if err != nil {
		return nil, err
	}

	subID, err := p.stripe.CreateSubscription(ctx, customerID, perSeatPrice.StripePriceID, pr.Seats,
		reconcile.Key("proposal", fmt.Sprint(pr.ID), "subscription"))
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	now := time.Now()
	end := now.AddDate(0, 0, pr.ValidityDays)

	err = p.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		agr := &models.Agreement{
			OrganizationID:       orgID,
			LicenseID:            license.ID,
			Seats:                pr.Seats,
			PricePerSeatCents:    perSeatPrice.UnitAmountCents,
			LicenseModel:         pr.LicenseModel,
			Tier:                 pr.Tier,
			StartDate:            &now,
			EndDate:              &end,
			StripeSubscriptionID: &subID,
		}
		if agrErr := tx.Create(agr).Error; agrErr != nil {
			return agrErr
		}

		return tx.Model(pr).Updates(map[string]interface{}{
			"status":                 models.ProposalStatusPaid,
			"resulting_agreement_id": agr.ID,
		}).Error
	})
	if err != nil {
		return nil, err
	}

	return p.load(ctx, proposalID)
}

// FulfillBySubscription is invoked by the Stripe webhook (invoice.paid): it flips
// the proposal backing the given subscription from `paid` to `fulfilled`.
func (p *ProposalService) FulfillBySubscription(ctx context.Context, subscriptionID string) error {
	var agr models.Agreement
	err := p.db.DB.WithContext(ctx).Where("stripe_subscription_id = ?", subscriptionID).First(&agr).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p.logger.Printf("webhook: no agreement for subscription %s (ignoring)", subscriptionID)

		return nil
	}

	if err != nil {
		return err
	}

	res := p.db.DB.WithContext(ctx).Model(&models.AgreementProposal{}).
		Where("resulting_agreement_id = ? AND status = ?", agr.ID, models.ProposalStatusPaid).
		Update("status", models.ProposalStatusFulfilled)

	return res.Error
}

func (p *ProposalService) ensureCustomer(ctx context.Context, orgID int64) (string, error) {
	return EnsureCustomer(ctx, p.db, p.stripe, orgID)
}

func (p *ProposalService) perSeatPrice(ctx context.Context, proposalID int64) (*models.StripePrice, error) {
	var items []models.ProposalLineItem
	if err := p.db.DB.WithContext(ctx).Preload("Price").Where("proposal_id = ?", proposalID).Find(&items).Error; err != nil {
		return nil, err
	}

	for i := range items {
		if items[i].Price != nil && items[i].Price.Kind == models.StripePriceKindPerSeat {
			return items[i].Price, nil
		}
	}

	return nil, fmt.Errorf("proposal has no per-seat price to subscribe to")
}

func (p *ProposalService) licenseForTier(ctx context.Context, tier *string) (*models.License, error) {
	var license models.License

	q := p.db.DB.WithContext(ctx).Where("deactivated_at IS NULL")
	if tier != nil && *tier != "" {
		q = q.Where("tier = ?", *tier)
	}

	if err := q.Order("id").First(&license).Error; err != nil {
		return nil, fmt.Errorf("no license found for tier: %w", err)
	}

	return &license, nil
}

func (p *ProposalService) load(ctx context.Context, id int64) (*models.AgreementProposal, error) {
	var pr models.AgreementProposal

	err := p.db.DB.WithContext(ctx).First(&pr, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("proposal not found")
	}

	if err != nil {
		return nil, err
	}

	return &pr, nil
}

func (p *ProposalService) loadScoped(ctx context.Context, orgID, id int64) (*models.AgreementProposal, error) {
	pr, err := p.load(ctx, id)
	if err != nil {
		return nil, err
	}

	if pr.OrganizationID != orgID {
		return nil, fmt.Errorf("proposal does not belong to your organization")
	}

	return pr, nil
}

func (p *ProposalService) loadWithItems(ctx context.Context, id int64) (*models.AgreementProposal, error) {
	var pr models.AgreementProposal
	if err := p.db.DB.WithContext(ctx).Preload("LineItems.Price").First(&pr, id).Error; err != nil {
		return nil, err
	}

	return &pr, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
