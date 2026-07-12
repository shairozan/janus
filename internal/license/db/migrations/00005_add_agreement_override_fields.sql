-- +goose Up
-- +goose StatementBegin

-- Add convenience/override fields to agreements
-- These fields allow per-agreement customization, overriding license defaults
ALTER TABLE agreements
  ADD COLUMN tier VARCHAR(50),              -- Override: can differ from license.tier
  ADD COLUMN max_seats INTEGER,             -- Override: can differ from agreement.seats
  ADD COLUMN features JSONB,                -- Override: can differ from license.features
  ADD COLUMN cost_per_month NUMERIC(10,2), -- Override: alternative to price_per_seat_cents
  ADD COLUMN start_date DATE,              -- Agreement start date
  ADD COLUMN end_date DATE,                -- Agreement end date (NULL = no expiration)
  ADD COLUMN active BOOLEAN DEFAULT true;  -- Convenience: active = (deactivated_at IS NULL)

-- Add convenience fields to organizations
ALTER TABLE organizations
  ADD COLUMN contact_name VARCHAR(255),    -- Primary contact name
  ADD COLUMN email VARCHAR(255),           -- Primary contact email
  ADD COLUMN active BOOLEAN DEFAULT true,  -- Convenience: active = (deactivated_at IS NULL)
  ADD COLUMN updated_at TIMESTAMP DEFAULT NOW(); -- Track updates

-- Create index on agreement tier for filtering
CREATE INDEX idx_agreements_tier ON agreements(tier) WHERE tier IS NOT NULL;

-- Create index on agreement active status
CREATE INDEX idx_agreements_active_flag ON agreements(active) WHERE active = true;

-- Create GIN index on agreement features for fast JSON queries
CREATE INDEX idx_agreements_features ON agreements USING GIN (features) WHERE features IS NOT NULL;

COMMENT ON COLUMN agreements.tier IS 'Optional override of license.tier for this specific agreement';
COMMENT ON COLUMN agreements.max_seats IS 'Optional override - maximum concurrent seats (can differ from seats field)';
COMMENT ON COLUMN agreements.features IS 'Optional override of license.features for this agreement';
COMMENT ON COLUMN agreements.cost_per_month IS 'Optional override - flat monthly cost instead of per-seat pricing';
COMMENT ON COLUMN agreements.start_date IS 'Agreement effective start date';
COMMENT ON COLUMN agreements.end_date IS 'Agreement expiration date (NULL means no expiration)';
COMMENT ON COLUMN agreements.active IS 'Convenience flag - should mirror (deactivated_at IS NULL)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_agreements_tier;
DROP INDEX IF EXISTS idx_agreements_active_flag;
DROP INDEX IF EXISTS idx_agreements_features;

ALTER TABLE agreements
  DROP COLUMN tier,
  DROP COLUMN max_seats,
  DROP COLUMN features,
  DROP COLUMN cost_per_month,
  DROP COLUMN start_date,
  DROP COLUMN end_date,
  DROP COLUMN active;

ALTER TABLE organizations
  DROP COLUMN contact_name,
  DROP COLUMN email,
  DROP COLUMN active,
  DROP COLUMN updated_at;

-- +goose StatementEnd
