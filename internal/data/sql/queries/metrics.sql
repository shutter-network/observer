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
	tx_hash
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING;