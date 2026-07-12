-- +goose Up
-- +goose StatementBegin
CREATE TABLE auto_acceptance_rules (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id),
    agreement_id BIGINT REFERENCES agreements(id),
    rule_type VARCHAR(20) NOT NULL,
    match_domain VARCHAR(255),
    max_auto_seats INTEGER,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by_user_id BIGINT NOT NULL REFERENCES org_users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deactivated_at TIMESTAMP NULL
);

CREATE INDEX idx_auto_rules_org ON auto_acceptance_rules(organization_id);
CREATE INDEX idx_auto_rules_active ON auto_acceptance_rules(organization_id)
    WHERE deactivated_at IS NULL AND enabled = true;

COMMENT ON TABLE auto_acceptance_rules IS 'Admin-defined rules for auto-approving license requests.';
COMMENT ON COLUMN auto_acceptance_rules.rule_type IS 'email_domain | seat_threshold';
COMMENT ON COLUMN auto_acceptance_rules.match_domain IS 'For email_domain: auto-approve requesters whose email matches.';
COMMENT ON COLUMN auto_acceptance_rules.max_auto_seats IS 'For seat_threshold: auto-approve until N seats consumed, then manual.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auto_acceptance_rules;
-- +goose StatementEnd
