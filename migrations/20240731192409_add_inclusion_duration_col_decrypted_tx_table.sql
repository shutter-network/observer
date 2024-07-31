-- +goose Up
-- +goose StatementBegin
ALTER TABLE decrypted_tx
ADD COLUMN inclusion_duration BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
