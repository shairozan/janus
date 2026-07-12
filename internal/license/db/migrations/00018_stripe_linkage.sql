-- +goose Up
-- +goose StatementBegin
ALTER TABLE organizations
    ADD COLUMN stripe_customer_id VARCHAR(255);

ALTER TABLE agreements
    ADD COLUMN stripe_subscription_id VARCHAR(255);

COMMENT ON COLUMN organizations.stripe_customer_id IS 'Stripe customer for this org (billing).';
COMMENT ON COLUMN agreements.stripe_subscription_id IS 'Backing Stripe subscription; seat expansion = quantity increase on it.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE organizations DROP COLUMN stripe_customer_id;
ALTER TABLE agreements DROP COLUMN stripe_subscription_id;
-- +goose StatementEnd
