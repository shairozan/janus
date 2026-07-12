-- +goose Up
-- +goose StatementBegin
-- Add GIN index on licenses.features for efficient containment queries
-- This allows queries like: WHERE features @> '["audit"]'::jsonb
CREATE INDEX idx_licenses_features ON licenses USING GIN (features);

-- Add GIN index on agreements for efficient queries joining to licenses
-- This is useful for finding all agreements with specific features
CREATE INDEX idx_agreements_org_active ON agreements(organization_id, license_id)
    WHERE deactivated_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agreements_org_active;
DROP INDEX IF EXISTS idx_licenses_features;
-- +goose StatementEnd