-- +goose Up
-- +goose StatementBegin

-- Drop existing FK
ALTER TABLE decryption_keys_message_decryption_key
DROP CONSTRAINT decryption_keys_message_decry_decryption_key_eon_decryptio_fkey;

ALTER TABLE decryption_keys_message_decryption_key
DROP CONSTRAINT decryption_keys_message_decry_decryption_keys_message_slot_fkey;

-- Drop the existing PK constraint.
ALTER TABLE decryption_key
DROP CONSTRAINT decryption_key_pkey;

-- Create new PK constraint and make it auto incrementing
ALTER TABLE decryption_key 
ADD COLUMN id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY;

-- Create a unique constraint on the composite key
ALTER TABLE decryption_key
ADD CONSTRAINT decryption_key_eon_identity_preimage_unique UNIQUE (eon, identity_preimage);

-- Add a new id column to decryption_keys_message_decryption_key to store the FK
ALTER TABLE decryption_keys_message_decryption_key
ADD COLUMN decryption_key_id BIGINT;

-- Populate decryption_keys_message_decryption_key with the correct decryption_key_id values.
UPDATE decryption_keys_message_decryption_key
SET decryption_key_id = d.id
FROM decryption_key d
WHERE decryption_keys_message_decryption_key.decryption_key_eon = d.eon
AND decryption_keys_message_decryption_key.decryption_key_identity_preimage = d.identity_preimage;

-- Alter decryption_keys_message_decryption_key to make decryption_key_id NOT NULL.
ALTER TABLE decryption_keys_message_decryption_key
ALTER COLUMN decryption_key_id SET NOT NULL;

ALTER TABLE decryption_keys_message_decryption_key
ADD CONSTRAINT decryption_keys_message_decryption_key_decryption_key_id_fkey
FOREIGN KEY (decryption_key_id)
REFERENCES decryption_key (id);

ALTER TABLE decryption_keys_message_decryption_key
DROP CONSTRAINT IF EXISTS decryption_keys_message_decryption_key_pkey;

-- Drop the old columns no longer needed.
ALTER TABLE decryption_keys_message_decryption_key
DROP COLUMN IF EXISTS decryption_key_eon,
DROP COLUMN IF EXISTS decryption_key_identity_preimage;

-- Add new PK
ALTER TABLE decryption_keys_message_decryption_key
ADD CONSTRAINT decryption_keys_message_decryption_key_pkey
PRIMARY KEY (decryption_keys_message_slot, decryption_key_id, key_index);

-- Drop the existing PK constraint.
ALTER TABLE transaction_submitted_event
DROP CONSTRAINT transaction_submitted_event_pkey;

ALTER TABLE transaction_submitted_event 
ADD COLUMN id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY;

-- Create a unique constraint on the composite key
ALTER TABLE transaction_submitted_event
ADD CONSTRAINT transaction_submitted_event_block_hash_block_number_tx_index_log_index_unique UNIQUE (event_block_hash, event_block_number, event_tx_index, event_log_index);

-- Add the new column `decryption_key_id`.
ALTER TABLE decrypted_tx
ADD COLUMN decryption_key_id BIGINT;

-- Set `decryption_key_id` as a FK referencing `decryption_key(id)`.
ALTER TABLE decrypted_tx
ADD CONSTRAINT decrypted_tx_decryption_key_id_fkey
FOREIGN KEY (decryption_key_id)
REFERENCES decryption_key (id);

-- Add the new column `transaction_submitted_event_id`.
ALTER TABLE decrypted_tx
ADD COLUMN transaction_submitted_event_id BIGINT;

-- Make the `transaction_submitted_event_id` column a FK referencing `transaction_submitted_event(id)`.
ALTER TABLE decrypted_tx
ADD CONSTRAINT decrypted_tx_transaction_submitted_event_id_fkey
FOREIGN KEY (transaction_submitted_event_id)
REFERENCES transaction_submitted_event (id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
