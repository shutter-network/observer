-- +goose Up
ALTER TABLE decrypted_tx
    ADD COLUMN inclusion_position TEXT NOT NULL DEFAULT 'unknown';

ALTER TYPE tx_status_val ADD VALUE IF NOT EXISTS 'tentative shielded inclusion';

-- +goose Down
ALTER TABLE decrypted_tx DROP COLUMN inclusion_position;
-- cannot easily drop enum value; left as-is on down.