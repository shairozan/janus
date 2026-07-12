-- +goose Up
-- +goose StatementBegin
CREATE TABLE stripe_prices (
    id BIGSERIAL PRIMARY KEY,
    stripe_product_id VARCHAR(255) NOT NULL,
    stripe_price_id VARCHAR(255) UNIQUE NOT NULL,
    nickname VARCHAR(255),
    unit_amount_cents INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL,
    interval VARCHAR(20) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stripe_prices_active ON stripe_prices(active) WHERE active = true;

COMMENT ON TABLE stripe_prices IS 'Catalog mapping of logical prices to Stripe identifiers. Operational data — no price IDs in config.';
COMMENT ON COLUMN stripe_prices.interval IS 'month | year | one_time';
COMMENT ON COLUMN stripe_prices.kind IS 'per_seat | flat | concurrent';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS stripe_prices;
-- +goose StatementEnd
