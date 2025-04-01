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
	encrypted_transaction,
	event_tx_hash
) 
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT DO NOTHING;

-- name: CreateDecryptionKeyMessages :exec
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

-- name: CreateDecryptionKeys :many
WITH data (eon, identity_preimage, key) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BYTEA[]), 
    unnest($3::BYTEA[])
),
inserted AS (
  INSERT INTO decryption_key (eon, identity_preimage, key)
  SELECT * FROM data 
  ON CONFLICT DO NOTHING
  RETURNING id
)
SELECT * FROM inserted;

-- name: CreateDecryptionKeysMessageDecryptionKey :exec
WITH data (decryption_keys_message_slot, key_index, decryption_key_id) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[])
)
INSERT INTO decryption_keys_message_decryption_key (decryption_keys_message_slot, key_index, decryption_key_id)
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
	slot
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING;

-- name: QueryBlockFromSlot :one
SELECT * FROM block
WHERE slot = $1 FOR UPDATE;

-- name: CreateDecryptedTX :exec
INSERT into decrypted_tx(
	slot,
	tx_index,
	tx_hash,
	tx_status,
	decryption_key_id,
	transaction_submitted_event_id
) 
VALUES ($1, $2, $3, $4, $5, $6) 
ON CONFLICT DO NOTHING;

-- name: CreateValidatorRegistryMessage :exec
INSERT into validator_registration_message(
	version,
	chain_id,
	validator_registry_address,
	validator_index,
	nonce,
	is_registeration,
	signature,
	event_block_number,
	event_tx_index,
	event_log_index,
	validity
) 
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) 
ON CONFLICT DO NOTHING;

-- name: QueryDecryptionKeysAndMessage :many
SELECT
    dkm.slot, dkm.tx_pointer, dkm.eon, 
    dk.key, dk.identity_preimage, 
	dkmdk.key_index, dkmdk.decryption_key_id
FROM decryption_keys_message_decryption_key dkmdk
LEFT JOIN decryption_keys_message dkm ON dkmdk.decryption_keys_message_slot = dkm.slot
LEFT JOIN decryption_key dk ON dkmdk.decryption_key_id = dk.id
WHERE dkm.slot = $1 ORDER BY dkmdk.key_index ASC;

-- name: QueryTransactionSubmittedEvent :many
SELECT * FROM transaction_submitted_event
WHERE eon = $1 AND tx_index >= $2 AND tx_index < $2 + $3 ORDER BY tx_index ASC;

-- name: CreateValidatorRegistryEventsSyncedUntil :exec
INSERT INTO validator_registry_events_synced_until (block_hash, block_number) VALUES ($1, $2)
ON CONFLICT (enforce_one_row) DO UPDATE
SET block_hash = $1, block_number = $2;

-- name: QueryValidatorRegistrationMessageNonceBefore :one 
SELECT nonce FROM validator_registration_message WHERE validator_index = $1 AND event_block_number <= $2 AND event_tx_index <= $3 AND event_log_index <= $4 ORDER BY event_block_number DESC, event_tx_index DESC, event_log_index DESC FOR UPDATE;

-- name: QueryValidatorRegistryEventsSyncedUntil :one
SELECT  block_hash, block_number FROM validator_registry_events_synced_until LIMIT 1;

-- name: CreateValidatorStatus :exec
INSERT into validator_status(
	validator_index,
	status
) 
VALUES ($1, $2) 
ON CONFLICT (validator_index) DO UPDATE
SET status = $2;

-- name: QueryValidatorStatuses :many
SELECT validator_index, status FROM validator_status
LIMIT $1 OFFSET $2;

-- name: CreateProposerDuties :exec
WITH data (public_key, validator_index, slot) AS (
  SELECT 
    unnest($1::TEXT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[])
)
INSERT INTO proposer_duties (public_key, validator_index, slot)
SELECT * FROM data
ON CONFLICT DO NOTHING;

-- name: UpsertTX :exec
INSERT INTO decrypted_tx (
	slot, 
	tx_index, 
	tx_hash, 
	tx_status, 
	decryption_key_id, 
	transaction_submitted_event_id, 
	block_number
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (slot, tx_index) 
DO UPDATE
SET tx_status = $4,
    block_number = $7,
    updated_at = NOW();

-- name: CreateTransactionSubmittedEventsSyncedUntil :exec
INSERT INTO transaction_submitted_events_synced_until (block_hash, block_number) VALUES ($1, $2)
ON CONFLICT (enforce_one_row) DO UPDATE
SET block_hash = $1, block_number = $2;

-- name: QueryTransactionSubmittedEventsSyncedUntil :one
SELECT  block_hash, block_number FROM transaction_submitted_events_synced_until LIMIT 1;

-- name: DeleteDecryptedTxFromBlockNumber :exec
DELETE FROM decrypted_tx WHERE block_number >= $1;

-- name: DeleteTransactionSubmittedEventsFromBlockNumber :exec
DELETE FROM transaction_submitted_event WHERE event_block_number >= $1;