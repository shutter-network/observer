-- +goose Up
-- +goose StatementBegin
ALTER TYPE tx_status_val ADD VALUE 'not decrypted';
ALTER TYPE tx_status_val ADD VALUE 'invalid';
ALTER TYPE tx_status_val ADD VALUE 'pending';
ALTER TYPE tx_status_val ADD VALUE 'shielded inclusion';
ALTER TYPE tx_status_val ADD VALUE 'unshielded inclusion';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
