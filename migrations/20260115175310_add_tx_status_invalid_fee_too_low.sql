-- +goose Up
-- +goose StatementBegin
ALTER TYPE tx_status_val ADD VALUE 'invalid fee too low';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
