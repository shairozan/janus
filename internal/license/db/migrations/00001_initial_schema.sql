-- +goose Up
-- +goose StatementBegin
CREATE TABLE organizations (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    customer_id VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMP NULL
);

CREATE TABLE licenses (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    tier VARCHAR(50) NOT NULL,
    features JSONB NOT NULL,
    msrp_cents INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMP NULL
);

CREATE TABLE agreements (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    license_id BIGINT NOT NULL REFERENCES licenses(id),
    seats INTEGER NOT NULL,

    -- Pricing: Agreement-specific, overrides license MSRP
    price_per_seat_cents INTEGER NOT NULL,

    license_model VARCHAR(20) NOT NULL,
    concurrent_limit INTEGER NULL,

    -- Metadata
    activated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMP NULL,
    notes TEXT NULL,

    -- For concurrent customers
    key_id VARCHAR(50) NULL,

    UNIQUE(organization_id, license_id, activated_at)
);

CREATE INDEX idx_agreements_org ON agreements(organization_id);
CREATE INDEX idx_agreements_active ON agreements(organization_id, deactivated_at)
    WHERE deactivated_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agreements;
DROP TABLE IF EXISTS licenses;
DROP TABLE IF EXISTS organizations;
-- +goose StatementEnd