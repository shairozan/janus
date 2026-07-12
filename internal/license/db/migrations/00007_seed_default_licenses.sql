-- +goose Up
-- +goose StatementBegin

-- Insert default license tiers for common use cases
INSERT INTO licenses (name, tier, features, msrp_cents, created_at) VALUES
    ('Starter License', 'starter', '["basic", "local"]'::jsonb, 0, NOW()),
    ('Professional License', 'professional', '["basic", "local", "grid"]'::jsonb, 5000, NOW()),
    ('Enterprise License', 'enterprise', '["basic", "local", "grid", "audit", "validation"]'::jsonb, 10000, NOW())
ON CONFLICT DO NOTHING;

COMMENT ON TABLE licenses IS 'License tiers define the product offerings and features available';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove the seeded licenses (only if no agreements reference them)
DELETE FROM licenses WHERE tier IN ('starter', 'professional', 'enterprise') AND name LIKE '%License';

-- +goose StatementEnd
