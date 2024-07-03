-- name: CreateEncryptedTx :exec
INSERT into encrypted_tx (
    tx_index, 
	eon,
	tx,
	identity_preimage
) 
VALUES ($1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: QueryEncryptedTx :many
SELECT * FROM encrypted_tx
WHERE tx_index = $1 AND eon = $2;

-- name: CreateDecryptionData :exec
INSERT into decryption_data(
    eon,
	identity_preimage,
	decryption_key,
	slot
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING;

-- name: UpdateBlockHash :exec
UPDATE decryption_data
SET block_hash = $2
WHERE slot = $1;

-- name: QueryDecryptionData :many
SELECT * FROM decryption_data
WHERE eon = $1 AND identity_preimage = $2;

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
