-- +goose Up
-- +goose StatementBegin
CREATE TABLE signing_keys (
    id BIGSERIAL PRIMARY KEY,
    key_id VARCHAR(50) UNIQUE NOT NULL,
    organization_id BIGINT NULL REFERENCES organizations(id),
    public_key_pem TEXT NOT NULL,
    private_key_encrypted TEXT NULL,
    algorithm VARCHAR(20) NOT NULL DEFAULT 'RS256',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL
);

CREATE INDEX idx_signing_keys_org ON signing_keys(organization_id);
CREATE INDEX idx_signing_keys_active ON signing_keys(key_id, revoked_at)
    WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS signing_keys;
-- +goose StatementEnd