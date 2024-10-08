-- +goose Up
-- +goose StatementBegin
UPDATE decrypted_tx
SET tx_status = 'shielded inclusion'
WHERE tx_status = 'included';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
