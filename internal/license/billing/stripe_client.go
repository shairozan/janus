package billing

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeClient is the production Stripe, backed by stripe-go. It is a thin
// wrapper validated in staging (not unit-tested — IQ/OQ boundary); the billing
// services are tested against the fake.
type StripeClient struct {
	sc            *stripe.Client
	webhookSecret string
}

var _ Stripe = (*StripeClient)(nil)

// NewStripeClient creates a Stripe client. apiKey + webhookSecret are the only
// Stripe secrets (SOPS); all pricing is DB-stored.
func NewStripeClient(apiKey, webhookSecret string) *StripeClient {
	return &StripeClient{sc: stripe.NewClient(apiKey), webhookSecret: webhookSecret}
}

// CreateCustomer creates a Stripe customer.
func (c *StripeClient) CreateCustomer(ctx context.Context, email, name, idempotencyKey string) (string, error) {
	params := &stripe.CustomerCreateParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}

	cust, err := c.sc.V1Customers.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}

	return cust.ID, nil
}

// CreateSubscription creates a per-seat subscription.
func (c *StripeClient) CreateSubscription(ctx context.Context, customerID, priceID string, quantity int, idempotencyKey string) (string, error) {
	params := &stripe.SubscriptionCreateParams{
		Customer: stripe.String(customerID),
		Items: []*stripe.SubscriptionCreateItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(int64(quantity))},
		},
	}
	if idempotencyKey != "" {
		params.IdempotencyKey = stripe.String(idempotencyKey)
	}

	sub, err := c.sc.V1Subscriptions.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("stripe create subscription: %w", err)
	}

	return sub.ID, nil
}

// UpdateSubscriptionQuantity sets the seat quantity on the subscription's first
// item; Stripe prorates the change and invoices it immediately.
func (c *StripeClient) UpdateSubscriptionQuantity(ctx context.Context, subscriptionID string, quantity int) error {
	sub, err := c.sc.V1Subscriptions.Retrieve(ctx, subscriptionID, nil)
	if err != nil {
		return fmt.Errorf("stripe retrieve subscription: %w", err)
	}

	if sub.Items == nil || len(sub.Items.Data) == 0 {
		return fmt.Errorf("subscription %s has no items", subscriptionID)
	}

	params := &stripe.SubscriptionUpdateParams{
		Items: []*stripe.SubscriptionUpdateItemParams{
			{ID: stripe.String(sub.Items.Data[0].ID), Quantity: stripe.Int64(int64(quantity))},
		},
		ProrationBehavior: stripe.String("always_invoice"),
	}

	if _, err := c.sc.V1Subscriptions.Update(ctx, subscriptionID, params); err != nil {
		return fmt.Errorf("stripe update subscription quantity: %w", err)
	}

	return nil
}

// ConstructWebhookEvent verifies the signature and extracts the event type +
// the subscription id (invoice events carry "subscription") + metadata.
func (c *StripeClient) ConstructWebhookEvent(payload []byte, sigHeader string) (WebhookEvent, error) {
	event, err := webhook.ConstructEvent(payload, sigHeader, c.webhookSecret)
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("verify webhook signature: %w", err)
	}

	out := WebhookEvent{Type: string(event.Type), Metadata: map[string]string{}}

	obj := event.Data.Object
	// invoice events: the subscription id is the correlation key.
	if sub, ok := obj["subscription"].(string); ok && sub != "" {
		out.ObjectID = sub
	} else if id, ok := obj["id"].(string); ok {
		out.ObjectID = id
	}

	if md, ok := obj["metadata"].(map[string]interface{}); ok {
		for k, v := range md {
			if s, ok := v.(string); ok {
				out.Metadata[k] = s
			}
		}
	}

	return out, nil
}
