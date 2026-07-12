-- +goose Up
-- +goose StatementBegin

-- Add timestamp tracking fields to agreements table
ALTER TABLE agreements
  ADD COLUMN created_at TIMESTAMP DEFAULT NOW(),
  ADD COLUMN updated_at TIMESTAMP DEFAULT NOW();

COMMENT ON COLUMN agreements.created_at IS 'Timestamp when the agreement record was created';
COMMENT ON COLUMN agreements.updated_at IS 'Timestamp when the agreement record was last updated';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE agreements
  DROP COLUMN created_at,
  DROP COLUMN updated_at;

-- +goose StatementEnd
