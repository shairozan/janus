package billing

import (
	"context"
	"fmt"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/reconcile"
)

// EnsureCustomer returns the organization's Stripe customer id, creating the
// customer on first use and persisting it back to the org. It is idempotent: a
// repeat call returns the stored id without touching Stripe, and the create uses
// a deterministic idempotency key (reconcile.Key) so a retry after a crash does
// not double-create. Shared by the proposal payment flow and org signup.
func EnsureCustomer(ctx context.Context, database *db.DB, stripe Stripe, orgID int64) (string, error) {
	var org models.Organization
	if err := database.DB.WithContext(ctx).First(&org, orgID).Error; err != nil {
		return "", err
	}

	if org.StripeCustomerID != nil && *org.StripeCustomerID != "" {
		return *org.StripeCustomerID, nil
	}

	email := ""
	if org.Email != nil {
		email = *org.Email
	}

	customerID, err := stripe.CreateCustomer(ctx, email, org.Name, reconcile.Key("org", fmt.Sprint(orgID), "customer"))
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}

	if err := database.DB.WithContext(ctx).Model(&org).Update("stripe_customer_id", customerID).Error; err != nil {
		return "", err
	}

	return customerID, nil
}
