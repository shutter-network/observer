-- +goose Up
-- +goose StatementBegin

ALTER TABLE transaction_submitted_event
ADD COLUMN event_tx_hash BYTEA DEFAULT '\x';

ALTER TABLE transaction_submitted_event
ALTER COLUMN event_tx_hash SET NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
