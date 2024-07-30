-- name: CreateTransactionSubmittedEvent :exec
INSERT into transaction_submitted_event (
    event_block_hash, 
	event_block_number,
	event_tx_index,
	event_log_index,
	eon,
	tx_index,
	identity_prefix,
	sender,
	encrypted_transaction
) 
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKeyMessage :exec
WITH data (slot, instance_id, eon, tx_pointer) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[]), 
    unnest($4::BIGINT[])
)
INSERT INTO decryption_keys_message (slot, instance_id, eon, tx_pointer)
SELECT * FROM data
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKey :exec
WITH data (eon, identity_preimage, key) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BYTEA[]), 
    unnest($3::BYTEA[])
)
INSERT INTO decryption_key (eon, identity_preimage, key)
SELECT * FROM data 
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKeysMessageDecryptionKey :exec
WITH data (decryption_keys_message_slot, key_index, decryption_key_eon, decryption_key_identity_preimage) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[]), 
    unnest($4::BYTEA[])
)
INSERT INTO decryption_keys_message_decryption_key (decryption_keys_message_slot, key_index, decryption_key_eon, decryption_key_identity_preimage)
SELECT * FROM data
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKeyShare :exec
INSERT into decryption_key_share(
	eon,
	identity_preimage,
	keyper_index,
    decryption_key_share,
	slot
) 
VALUES ($1, $2, $3, $4, $5) 
ON CONFLICT DO NOTHING;

-- name: QueryDecryptionKeyShare :many
SELECT * FROM decryption_key_share
WHERE eon = $1 AND identity_preimage = $2 AND keyper_index = $3;

-- name: CreateBlock :exec
INSERT into block(
	block_hash,
	block_number,
	block_timestamp,
	tx_hash,
	slot
) 
VALUES ($1, $2, $3, $4, $5) 
ON CONFLICT DO NOTHING;

-- name: QueryBlockFromSlot :one
SELECT * FROM block
WHERE slot = $1 FOR UPDATE;

-- name: CreateDecryptedTX :exec
INSERT into decrypted_tx(
	slot,
	tx_index,
	tx_hash,
	tx_status
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING;

-- name: QueryDecryptionKeysAndMessage :many
SELECT
    dkm.slot, dkm.tx_pointer, dkm.eon, 
    dk.key, dk.identity_preimage, dkmdk.key_index
FROM decryption_keys_message_decryption_key dkmdk
LEFT JOIN decryption_keys_message dkm ON dkmdk.decryption_keys_message_slot = dkm.slot
LEFT JOIN decryption_key dk ON dkmdk.decryption_key_eon = dk.eon AND dkmdk.decryption_key_identity_preimage = dk.identity_preimage
WHERE dkm.slot = $1 ORDER BY dkmdk.key_index ASC;

-- name: QueryTransactionSubmittedEvent :many
SELECT * FROM transaction_submitted_event
WHERE eon = $1 AND tx_index >= $2 AND tx_index < $2 + $3 ORDER BY tx_index ASC;