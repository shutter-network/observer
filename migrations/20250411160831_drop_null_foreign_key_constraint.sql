-- +goose Up
-- +goose StatementBegin
ALTER TABLE decrypted_tx
ALTER COLUMN transaction_submitted_event_id DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
