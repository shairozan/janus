-- +goose Up
-- +goose StatementBegin
CREATE TABLE issued_tokens (
    id BIGSERIAL PRIMARY KEY,
    jti VARCHAR(100) UNIQUE NOT NULL,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    user_email VARCHAR(255) NOT NULL,
    key_id VARCHAR(50) NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL
);

CREATE INDEX idx_issued_tokens_org ON issued_tokens(organization_id);
CREATE INDEX idx_issued_tokens_jti ON issued_tokens(jti);
CREATE INDEX idx_issued_tokens_active ON issued_tokens(organization_id, revoked_at)
    WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS issued_tokens;
-- +goose StatementEnd