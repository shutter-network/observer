-- +goose Up
-- +goose StatementBegin
ALTER TABLE decrypted_tx
ADD COLUMN block_number BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
