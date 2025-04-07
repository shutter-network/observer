-- +goose Up
-- +goose StatementBegin
CREATE TABLE transaction_submitted_events_synced_until(
    enforce_one_row bool PRIMARY KEY DEFAULT true,
    block_hash bytea NOT NULL,
    block_number bigint NOT NULL CHECK (block_number >= 0)
);

ALTER TABLE validator_registry_events_synced_until
ADD COLUMN block_hash BYTEA;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE transaction_submitted_events_synced_until;
-- +goose StatementEnd
