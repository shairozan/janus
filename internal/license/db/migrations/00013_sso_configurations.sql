-- +goose Up
-- +goose StatementBegin
CREATE TABLE sso_configurations (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL UNIQUE REFERENCES organizations(id),
    provider_name VARCHAR(32) NOT NULL,
    user_pool_id VARCHAR(100) NOT NULL,
    oidc_issuer VARCHAR(512) NOT NULL,
    oidc_client_id VARCHAR(255) NOT NULL,
    oidc_client_secret_encrypted TEXT,
    scopes VARCHAR(255) NOT NULL DEFAULT 'openid email profile',
    attribute_mapping JSONB,
    cognito_status VARCHAR(20) NOT NULL,
    login_url VARCHAR(1024),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE sso_configurations IS 'Durable record of a per-org Cognito IdP. Create-once / read-only from the portal; mutations are support-only.';
COMMENT ON COLUMN sso_configurations.provider_name IS 'Cognito identity-provider name (<= 32 chars; org slug or short hash).';
COMMENT ON COLUMN sso_configurations.user_pool_id IS 'Cognito user pool hosting this IdP (per-config for multi-issuer support).';
COMMENT ON COLUMN sso_configurations.oidc_client_secret_encrypted IS 'Write-only, encrypted at rest; never returned by the API.';
COMMENT ON COLUMN sso_configurations.cognito_status IS 'pending | active | failed';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sso_configurations;
-- +goose StatementEnd
