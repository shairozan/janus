-- +goose Up
-- +goose StatementBegin
-- Persist the signed license JWT so the user can download exactly the artifact
-- that was issued (matching the tracked JTI). The JWT is the user's own license
-- file, not a secret credential.
ALTER TABLE issued_tokens
    ADD COLUMN token_jwt TEXT;

COMMENT ON COLUMN issued_tokens.token_jwt IS 'The signed license JWT, for download via /me/license.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE issued_tokens DROP COLUMN token_jwt;
-- +goose StatementEnd
