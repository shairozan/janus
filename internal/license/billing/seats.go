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
)

// SeatService handles seat expansion on an agreement's Stripe subscription.
type SeatService struct {
	db     *db.DB
	stripe Stripe
	logger *log.Logger
}

// NewSeatService creates a SeatService.
func NewSeatService(database *db.DB, stripe Stripe, logger *log.Logger) *SeatService {
	return &SeatService{db: database, stripe: stripe, logger: logger}
}

// SeatPreview is an estimate of a seat addition's cost. The prorated charge is
// computed locally from the agreement term; Stripe applies the authoritative
// proration on Add.
type SeatPreview struct {
	CurrentSeats        int   `json:"current_seats"`
	AddSeats            int   `json:"add_seats"`
	NewSeats            int   `json:"new_seats"`
	ProratedChargeCents int64 `json:"prorated_charge_cents"`
	RecurringDeltaCents int64 `json:"recurring_delta_cents"`
}

// Preview estimates the cost of adding addN seats.
func (s *SeatService) Preview(ctx context.Context, orgID, agreementID int64, addN int) (*SeatPreview, error) {
	agr, err := s.loadAgreement(ctx, orgID, agreementID)
	if err != nil {
		return nil, err
	}

	if addN <= 0 {
		return nil, fmt.Errorf("add must be > 0")
	}

	recurring := int64(addN * agr.PricePerSeatCents)

	return &SeatPreview{
		CurrentSeats:        agr.Seats,
		AddSeats:            addN,
		NewSeats:            agr.Seats + addN,
		ProratedChargeCents: prorate(recurring, agr, time.Now()),
		RecurringDeltaCents: recurring,
	}, nil
}

// Add increases the agreement's seat count: bumps the Stripe subscription
// quantity (Stripe prorates + invoices) and updates Agreement.Seats.
func (s *SeatService) Add(ctx context.Context, orgID, agreementID int64, addN int) (*models.Agreement, error) {
	agr, err := s.loadAgreement(ctx, orgID, agreementID)
	if err != nil {
		return nil, err
	}

	if addN <= 0 {
		return nil, fmt.Errorf("add must be > 0")
	}

	if agr.StripeSubscriptionID == nil || *agr.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("agreement has no Stripe subscription; cannot expand seats")
	}

	newSeats := agr.Seats + addN

	if err := s.stripe.UpdateSubscriptionQuantity(ctx, *agr.StripeSubscriptionID, newSeats); err != nil {
		return nil, fmt.Errorf("update subscription quantity: %w", err)
	}

	if err := s.db.DB.WithContext(ctx).Model(agr).Update("seats", newSeats).Error; err != nil {
		// Stripe quantity is updated but the local seat count write failed — the
		// reconciler can repair from the subscription's quantity.
		s.logger.Printf("seat add: stripe updated to %d but DB write failed for agreement %d: %v", newSeats, agr.ID, err)

		return nil, err
	}

	agr.Seats = newSeats

	return agr, nil
}

func (s *SeatService) loadAgreement(ctx context.Context, orgID, agreementID int64) (*models.Agreement, error) {
	var agr models.Agreement

	err := s.db.DB.WithContext(ctx).First(&agr, agreementID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("agreement not found")
	}

	if err != nil {
		return nil, err
	}

	if agr.OrganizationID != orgID {
		return nil, fmt.Errorf("agreement does not belong to your organization")
	}

	return &agr, nil
}

// prorate returns the fraction of `recurring` for the remaining portion of the
// agreement term. With no EndDate (open-ended) it charges the full recurring
// amount.
func prorate(recurring int64, agr *models.Agreement, now time.Time) int64 {
	if agr.EndDate == nil {
		return recurring
	}

	start := agr.GetStartDate()
	end := *agr.EndDate

	total := end.Sub(start)
	if total <= 0 {
		return 0
	}

	remaining := end.Sub(now)
	if remaining <= 0 {
		return 0
	}

	if remaining > total {
		remaining = total
	}

	return int64(float64(recurring) * (float64(remaining) / float64(total)))
}
