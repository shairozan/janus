-- +goose Up
-- +goose StatementBegin
CREATE TABLE agreement_proposals (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    requested_by_user_id BIGINT NOT NULL REFERENCES org_users(id),
    status VARCHAR(20) NOT NULL,
    tier VARCHAR(50),
    seats INTEGER NOT NULL,
    license_model VARCHAR(20) NOT NULL,
    validity_days INTEGER NOT NULL DEFAULT 365,
    total_amount_cents INTEGER NOT NULL DEFAULT 0,
    stripe_quote_id VARCHAR(255),
    valid_until TIMESTAMP NULL,
    message TEXT,
    parent_proposal_id BIGINT REFERENCES agreement_proposals(id),
    resulting_agreement_id BIGINT REFERENCES agreements(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_proposals_org ON agreement_proposals(organization_id);
CREATE INDEX idx_proposals_status ON agreement_proposals(organization_id, status);

COMMENT ON TABLE agreement_proposals IS 'Request → propose → negotiate → pay → fulfil lifecycle that yields an Agreement.';
COMMENT ON COLUMN agreement_proposals.validity_days IS 'License term basis (default 365) → sets agreement Start/EndDate → license exp.';
COMMENT ON COLUMN agreement_proposals.parent_proposal_id IS 'A counter-offer points at the proposal it supersedes.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agreement_proposals;
-- +goose StatementEnd
