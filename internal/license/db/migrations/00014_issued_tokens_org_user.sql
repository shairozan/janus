-- +goose Up
-- +goose StatementBegin
ALTER TABLE issued_tokens
    ADD COLUMN org_user_id BIGINT REFERENCES org_users(id),
    ADD COLUMN revoked_reason VARCHAR(20);

CREATE INDEX idx_issued_tokens_org_user ON issued_tokens(org_user_id);

-- One live (non-revoked) token per org user → keeps seat accounting exact.
-- NULL org_user_id rows (legacy/system tokens) are excluded from the constraint.
CREATE UNIQUE INDEX idx_issued_tokens_one_live_per_user ON issued_tokens(org_user_id)
    WHERE revoked_at IS NULL AND org_user_id IS NOT NULL;

COMMENT ON COLUMN issued_tokens.org_user_id IS 'The OrgUser (seat) this license was issued to; NULL for legacy/system tokens.';
COMMENT ON COLUMN issued_tokens.revoked_reason IS 'offboarded | key_replaced | non_payment | manual';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_issued_tokens_one_live_per_user;
DROP INDEX IF EXISTS idx_issued_tokens_org_user;
ALTER TABLE issued_tokens
    DROP COLUMN org_user_id,
    DROP COLUMN revoked_reason;
-- +goose StatementEnd
