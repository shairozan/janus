// Package billingtest provides an in-memory fake of billing.Stripe for tests.
package billingtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/pharmalytica/janus/internal/license/billing"
)

// Fake is an in-memory billing.Stripe.
type Fake struct {
	mu sync.Mutex

	Customers     map[string]bool // customer id -> exists
	Subscriptions map[string]int  // subscription id -> quantity
	FailOn        map[string]error
	WebhookEvent  billing.WebhookEvent // returned by ConstructWebhookEvent
	WebhookErr    error

	custSeq int
	subSeq  int
}

var _ billing.Stripe = (*Fake)(nil)

// New returns an empty Fake.
func New() *Fake {
	return &Fake{
		Customers:     map[string]bool{},
		Subscriptions: map[string]int{},
		FailOn:        map[string]error{},
	}
}

func (f *Fake) fail(method string) error {
	if err, ok := f.FailOn[method]; ok {
		return err
	}

	return nil
}

// CreateCustomer records and returns a new customer id.
func (f *Fake) CreateCustomer(_ context.Context, _, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("CreateCustomer"); err != nil {
		return "", err
	}

	f.custSeq++
	id := fmt.Sprintf("cus_%d", f.custSeq)
	f.Customers[id] = true

	return id, nil
}

// CreateSubscription records the quantity and returns a new subscription id.
func (f *Fake) CreateSubscription(_ context.Context, _, _ string, quantity int, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("CreateSubscription"); err != nil {
		return "", err
	}

	f.subSeq++
	id := fmt.Sprintf("sub_%d", f.subSeq)
	f.Subscriptions[id] = quantity

	return id, nil
}

// UpdateSubscriptionQuantity records the new quantity.
func (f *Fake) UpdateSubscriptionQuantity(_ context.Context, subscriptionID string, quantity int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.fail("UpdateSubscriptionQuantity"); err != nil {
		return err
	}

	if _, ok := f.Subscriptions[subscriptionID]; !ok {
		return fmt.Errorf("unknown subscription %s", subscriptionID)
	}

	f.Subscriptions[subscriptionID] = quantity

	return nil
}

// ConstructWebhookEvent returns the configured event/error.
func (f *Fake) ConstructWebhookEvent(_ []byte, _ string) (billing.WebhookEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.WebhookEvent, f.WebhookErr
}

// QuantityOf returns the recorded quantity for a subscription.
func (f *Fake) QuantityOf(subscriptionID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Subscriptions[subscriptionID]
}
