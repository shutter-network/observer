-- +goose Up
-- +goose StatementBegin
ALTER TABLE block DROP COLUMN tx_hash;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
