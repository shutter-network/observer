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
INSERT into decryption_keys_message(
    slot,
	instance_id,
	eon,
	tx_pointer
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKey :exec
INSERT into decryption_key(
    eon,
	identity_preimage,
	key
) 
VALUES ($1, $2, $3) 
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKeysMessageDecryptionKey :exec
INSERT into decryption_keys_message_decryption_key(
    decryption_keys_message_slot,
	key_index,
	decryption_key_eon,
	decryption_key_identity_preimage
) 
VALUES ($1, $2, $3, $4) 
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

-- name: QueryBlockFromBlockNumber :exec
SELECT * FROM block
WHERE block_number = $1;

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
    dk.key
FROM decryption_keys_message_decryption_key dkmdk
LEFT JOIN decryption_keys_message dkm ON dkmdk.decryption_keys_message_slot = dkm.slot
LEFT JOIN decryption_key dk ON dkmdk.decryption_key_eon = dk.eon AND dkmdk.decryption_key_identity_preimage = dk.identity_preimage
WHERE dkm.slot = $1;