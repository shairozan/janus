-- +goose Up
-- +goose StatementBegin

-- Remove active boolean columns - we use deactivated_at timestamps instead
-- Active state is: deactivated_at IS NULL

ALTER TABLE agreements
  DROP COLUMN IF EXISTS active;

ALTER TABLE organizations
  DROP COLUMN IF EXISTS active;

DROP INDEX IF EXISTS idx_agreements_active_flag;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restore active columns if needed to rollback
ALTER TABLE agreements
  ADD COLUMN active BOOLEAN DEFAULT true;

ALTER TABLE organizations
  ADD COLUMN active BOOLEAN DEFAULT true;

CREATE INDEX idx_agreements_active_flag ON agreements(active) WHERE active = true;

-- +goose StatementEnd
