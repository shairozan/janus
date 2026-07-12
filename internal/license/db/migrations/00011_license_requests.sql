-- +goose Up
-- +goose StatementBegin
CREATE TABLE license_requests (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    org_user_id BIGINT NOT NULL REFERENCES org_users(id),
    agreement_id BIGINT NOT NULL REFERENCES agreements(id),
    user_public_key_id BIGINT REFERENCES user_public_keys(id),
    status VARCHAR(20) NOT NULL,
    decision VARCHAR(10),
    decided_by_user_id BIGINT REFERENCES org_users(id),
    decided_at TIMESTAMP NULL,
    reason TEXT,
    issued_token_id BIGINT REFERENCES issued_tokens(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_license_requests_org ON license_requests(organization_id);
CREATE INDEX idx_license_requests_user ON license_requests(org_user_id);
CREATE INDEX idx_license_requests_status ON license_requests(organization_id, status);

COMMENT ON TABLE license_requests IS 'A user request for a seat against an agreement, plus its approval disposition.';
COMMENT ON COLUMN license_requests.status IS 'pending | approved | rejected | failed';
COMMENT ON COLUMN license_requests.decision IS 'auto | manual';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS license_requests;
-- +goose StatementEnd
