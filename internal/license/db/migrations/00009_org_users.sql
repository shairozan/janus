-- +goose Up
-- +goose StatementBegin
CREATE TABLE org_users (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    cognito_sub VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL,
    source VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMP NULL
);

CREATE INDEX idx_org_users_org ON org_users(organization_id);
CREATE INDEX idx_org_users_active ON org_users(organization_id, deactivated_at)
    WHERE deactivated_at IS NULL;

COMMENT ON TABLE org_users IS 'A person within a customer organization, keyed on their Cognito sub.';
COMMENT ON COLUMN org_users.cognito_sub IS 'Stable Cognito subject identifier (the `sub` claim).';
COMMENT ON COLUMN org_users.role IS 'customer_admin | member (mirrors the Cognito group).';
COMMENT ON COLUMN org_users.source IS 'invited (native Cognito user) | sso (federated).';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS org_users;
-- +goose StatementEnd
