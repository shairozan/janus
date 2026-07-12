-- +goose Up
-- +goose StatementBegin
CREATE TABLE proposal_line_items (
    id BIGSERIAL PRIMARY KEY,
    proposal_id BIGINT NOT NULL REFERENCES agreement_proposals(id) ON DELETE CASCADE,
    price_id BIGINT NOT NULL REFERENCES stripe_prices(id),
    quantity INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL
);

CREATE INDEX idx_proposal_line_items_proposal ON proposal_line_items(proposal_id);

COMMENT ON TABLE proposal_line_items IS 'Priced lines of a proposal, referencing catalog stripe_prices.id; amount_cents is a snapshot.';
COMMENT ON COLUMN proposal_line_items.price_id IS 'FK to stripe_prices.id (the catalog row), not a Stripe price string.';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS proposal_line_items;
-- +goose StatementEnd
