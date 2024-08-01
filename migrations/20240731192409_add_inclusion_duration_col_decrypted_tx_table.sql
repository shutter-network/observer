-- +goose Up
-- +goose StatementBegin
ALTER TABLE decrypted_tx
ADD COLUMN inclusion_delay BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE decrypted_tx
DROP COLUMN inclusion_delay;
-- +goose StatementEnd
