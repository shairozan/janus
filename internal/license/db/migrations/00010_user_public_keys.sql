-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_public_keys (
    id BIGSERIAL PRIMARY KEY,
    org_user_id BIGINT NOT NULL REFERENCES org_users(id),
    public_key_pem TEXT NOT NULL,
    fingerprint VARCHAR(100) NOT NULL,
    title VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP NULL
);

CREATE INDEX idx_user_public_keys_user ON user_public_keys(org_user_id);

-- A user has at most ONE active (non-revoked) public key at a time.
CREATE UNIQUE INDEX idx_user_public_keys_one_active ON user_public_keys(org_user_id)
    WHERE revoked_at IS NULL;

COMMENT ON TABLE user_public_keys IS 'Per-user RSA public keys (PEM) embedded in license JWTs; one active per user.';
COMMENT ON COLUMN user_public_keys.fingerprint IS 'SHA-256 of the DER bytes, displayed as SHA256:...';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_public_keys;
-- +goose StatementEnd
