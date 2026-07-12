package billing

import "context"

// Stripe is the narrow interface over the Stripe operations the billing engine
// needs. The concrete client wraps stripe-go; tests use a fake (no live Stripe —
// IQ/OQ boundary). Pass idempotency keys (reconcile.Key) so retries don't
// double-create.
type Stripe interface {
	// CreateCustomer creates a Stripe customer and returns its id.
	CreateCustomer(ctx context.Context, email, name, idempotencyKey string) (customerID string, err error)
	// CreateSubscription creates a per-seat subscription and returns its id.
	CreateSubscription(ctx context.Context, customerID, priceID string, quantity int, idempotencyKey string) (subscriptionID string, err error)
	// UpdateSubscriptionQuantity changes the seat quantity (Stripe prorates the
	// charge automatically — proration_behavior=always_invoice).
	UpdateSubscriptionQuantity(ctx context.Context, subscriptionID string, quantity int) error
	// ConstructWebhookEvent verifies the webhook signature and returns the event.
	ConstructWebhookEvent(payload []byte, sigHeader string) (WebhookEvent, error)
}

// WebhookEvent is the minimal shape of a verified Stripe webhook event.
type WebhookEvent struct {
	Type     string            // e.g. "invoice.paid"
	ObjectID string            // the subscription/invoice id
	Metadata map[string]string // our metadata (e.g. proposal_id)
}

// Well-known webhook event types we act on.
const (
	WebhookInvoicePaid          = "invoice.paid"
	WebhookInvoicePaymentFailed = "invoice.payment_failed"
)
