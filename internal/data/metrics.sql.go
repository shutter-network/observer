// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: metrics.sql

package data

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createBlock = `-- name: CreateBlock :exec
INSERT into block(
	block_hash,
	block_number,
	block_timestamp,
	tx_hash,
	slot
) 
VALUES ($1, $2, $3, $4, $5) 
ON CONFLICT DO NOTHING
`

type CreateBlockParams struct {
	BlockHash      []byte
	BlockNumber    int64
	BlockTimestamp int64
	TxHash         []byte
	Slot           int64
}

func (q *Queries) CreateBlock(ctx context.Context, arg CreateBlockParams) error {
	_, err := q.db.Exec(ctx, createBlock,
		arg.BlockHash,
		arg.BlockNumber,
		arg.BlockTimestamp,
		arg.TxHash,
		arg.Slot,
	)
	return err
}

const createDecryptedTX = `-- name: CreateDecryptedTX :exec
INSERT into decrypted_tx(
	slot,
	tx_index,
	tx_hash,
	tx_status
) 
VALUES ($1, $2, $3, $4) 
ON CONFLICT DO NOTHING
`

type CreateDecryptedTXParams struct {
	Slot     int64
	TxIndex  int64
	TxHash   []byte
	TxStatus TxStatusVal
}

func (q *Queries) CreateDecryptedTX(ctx context.Context, arg CreateDecryptedTXParams) error {
	_, err := q.db.Exec(ctx, createDecryptedTX,
		arg.Slot,
		arg.TxIndex,
		arg.TxHash,
		arg.TxStatus,
	)
	return err
}

const createDecryptionKey = `-- name: CreateDecryptionKey :exec
WITH data (eon, identity_preimage, key) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BYTEA[]), 
    unnest($3::BYTEA[])
)
INSERT INTO decryption_key (eon, identity_preimage, key)
SELECT eon, identity_preimage, key FROM data 
ON CONFLICT DO NOTHING
`

type CreateDecryptionKeyParams struct {
	Column1 []int64
	Column2 [][]byte
	Column3 [][]byte
}

func (q *Queries) CreateDecryptionKey(ctx context.Context, arg CreateDecryptionKeyParams) error {
	_, err := q.db.Exec(ctx, createDecryptionKey, arg.Column1, arg.Column2, arg.Column3)
	return err
}

const createDecryptionKeyMessage = `-- name: CreateDecryptionKeyMessage :exec
WITH data (slot, instance_id, eon, tx_pointer) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[]), 
    unnest($4::BIGINT[])
)
INSERT INTO decryption_keys_message (slot, instance_id, eon, tx_pointer)
SELECT slot, instance_id, eon, tx_pointer FROM data
ON CONFLICT DO NOTHING
`

type CreateDecryptionKeyMessageParams struct {
	Column1 []int64
	Column2 []int64
	Column3 []int64
	Column4 []int64
}

func (q *Queries) CreateDecryptionKeyMessage(ctx context.Context, arg CreateDecryptionKeyMessageParams) error {
	_, err := q.db.Exec(ctx, createDecryptionKeyMessage,
		arg.Column1,
		arg.Column2,
		arg.Column3,
		arg.Column4,
	)
	return err
}

const createDecryptionKeyShare = `-- name: CreateDecryptionKeyShare :exec
INSERT into decryption_key_share(
	eon,
	identity_preimage,
	keyper_index,
    decryption_key_share,
	slot
) 
VALUES ($1, $2, $3, $4, $5) 
ON CONFLICT DO NOTHING
`

type CreateDecryptionKeyShareParams struct {
	Eon                int64
	IdentityPreimage   []byte
	KeyperIndex        int64
	DecryptionKeyShare []byte
	Slot               int64
}

func (q *Queries) CreateDecryptionKeyShare(ctx context.Context, arg CreateDecryptionKeyShareParams) error {
	_, err := q.db.Exec(ctx, createDecryptionKeyShare,
		arg.Eon,
		arg.IdentityPreimage,
		arg.KeyperIndex,
		arg.DecryptionKeyShare,
		arg.Slot,
	)
	return err
}

const createDecryptionKeysMessageDecryptionKey = `-- name: CreateDecryptionKeysMessageDecryptionKey :exec
WITH data (decryption_keys_message_slot, key_index, decryption_key_eon, decryption_key_identity_preimage) AS (
  SELECT 
    unnest($1::BIGINT[]), 
    unnest($2::BIGINT[]), 
    unnest($3::BIGINT[]), 
    unnest($4::BYTEA[])
)
INSERT INTO decryption_keys_message_decryption_key (decryption_keys_message_slot, key_index, decryption_key_eon, decryption_key_identity_preimage)
SELECT decryption_keys_message_slot, key_index, decryption_key_eon, decryption_key_identity_preimage FROM data
ON CONFLICT DO NOTHING
`

type CreateDecryptionKeysMessageDecryptionKeyParams struct {
	Column1 []int64
	Column2 []int64
	Column3 []int64
	Column4 [][]byte
}

func (q *Queries) CreateDecryptionKeysMessageDecryptionKey(ctx context.Context, arg CreateDecryptionKeysMessageDecryptionKeyParams) error {
	_, err := q.db.Exec(ctx, createDecryptionKeysMessageDecryptionKey,
		arg.Column1,
		arg.Column2,
		arg.Column3,
		arg.Column4,
	)
	return err
}

const createTransactionSubmittedEvent = `-- name: CreateTransactionSubmittedEvent :exec
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
ON CONFLICT DO NOTHING
`

type CreateTransactionSubmittedEventParams struct {
	EventBlockHash       []byte
	EventBlockNumber     int64
	EventTxIndex         int64
	EventLogIndex        int64
	Eon                  int64
	TxIndex              int64
	IdentityPrefix       []byte
	Sender               []byte
	EncryptedTransaction []byte
}

func (q *Queries) CreateTransactionSubmittedEvent(ctx context.Context, arg CreateTransactionSubmittedEventParams) error {
	_, err := q.db.Exec(ctx, createTransactionSubmittedEvent,
		arg.EventBlockHash,
		arg.EventBlockNumber,
		arg.EventTxIndex,
		arg.EventLogIndex,
		arg.Eon,
		arg.TxIndex,
		arg.IdentityPrefix,
		arg.Sender,
		arg.EncryptedTransaction,
	)
	return err
}

const createValidatorRegistryEventsSyncedUntil = `-- name: CreateValidatorRegistryEventsSyncedUntil :exec
INSERT INTO validator_registry_events_synced_until (block_number) VALUES ($1)
ON CONFLICT (enforce_one_row) DO UPDATE
SET block_number = $1
`

func (q *Queries) CreateValidatorRegistryEventsSyncedUntil(ctx context.Context, blockNumber int64) error {
	_, err := q.db.Exec(ctx, createValidatorRegistryEventsSyncedUntil, blockNumber)
	return err
}

const createValidatorRegistryMessage = `-- name: CreateValidatorRegistryMessage :exec
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
ON CONFLICT DO NOTHING
`

type CreateValidatorRegistryMessageParams struct {
	Version                  pgtype.Int8
	ChainID                  pgtype.Int8
	ValidatorRegistryAddress []byte
	ValidatorIndex           pgtype.Int8
	Nonce                    pgtype.Int8
	IsRegisteration          pgtype.Bool
	Signature                []byte
	EventBlockNumber         int64
	EventTxIndex             int64
	EventLogIndex            int64
	Validity                 ValidatorRegistrationValidity
}

func (q *Queries) CreateValidatorRegistryMessage(ctx context.Context, arg CreateValidatorRegistryMessageParams) error {
	_, err := q.db.Exec(ctx, createValidatorRegistryMessage,
		arg.Version,
		arg.ChainID,
		arg.ValidatorRegistryAddress,
		arg.ValidatorIndex,
		arg.Nonce,
		arg.IsRegisteration,
		arg.Signature,
		arg.EventBlockNumber,
		arg.EventTxIndex,
		arg.EventLogIndex,
		arg.Validity,
	)
	return err
}

const queryBlockFromSlot = `-- name: QueryBlockFromSlot :one
SELECT block_hash, block_number, block_timestamp, tx_hash, created_at, updated_at, slot FROM block
WHERE slot = $1 FOR UPDATE
`

func (q *Queries) QueryBlockFromSlot(ctx context.Context, slot int64) (Block, error) {
	row := q.db.QueryRow(ctx, queryBlockFromSlot, slot)
	var i Block
	err := row.Scan(
		&i.BlockHash,
		&i.BlockNumber,
		&i.BlockTimestamp,
		&i.TxHash,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Slot,
	)
	return i, err
}

const queryDecryptionKeyShare = `-- name: QueryDecryptionKeyShare :many
SELECT eon, identity_preimage, keyper_index, decryption_key_share, slot, created_at, updated_at FROM decryption_key_share
WHERE eon = $1 AND identity_preimage = $2 AND keyper_index = $3
`

type QueryDecryptionKeyShareParams struct {
	Eon              int64
	IdentityPreimage []byte
	KeyperIndex      int64
}

func (q *Queries) QueryDecryptionKeyShare(ctx context.Context, arg QueryDecryptionKeyShareParams) ([]DecryptionKeyShare, error) {
	rows, err := q.db.Query(ctx, queryDecryptionKeyShare, arg.Eon, arg.IdentityPreimage, arg.KeyperIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DecryptionKeyShare
	for rows.Next() {
		var i DecryptionKeyShare
		if err := rows.Scan(
			&i.Eon,
			&i.IdentityPreimage,
			&i.KeyperIndex,
			&i.DecryptionKeyShare,
			&i.Slot,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const queryDecryptionKeysAndMessage = `-- name: QueryDecryptionKeysAndMessage :many
SELECT
    dkm.slot, dkm.tx_pointer, dkm.eon, 
    dk.key, dk.identity_preimage, dkmdk.key_index
FROM decryption_keys_message_decryption_key dkmdk
LEFT JOIN decryption_keys_message dkm ON dkmdk.decryption_keys_message_slot = dkm.slot
LEFT JOIN decryption_key dk ON dkmdk.decryption_key_eon = dk.eon AND dkmdk.decryption_key_identity_preimage = dk.identity_preimage
WHERE dkm.slot = $1 ORDER BY dkmdk.key_index ASC
`

type QueryDecryptionKeysAndMessageRow struct {
	Slot             pgtype.Int8
	TxPointer        pgtype.Int8
	Eon              pgtype.Int8
	Key              []byte
	IdentityPreimage []byte
	KeyIndex         int64
}

func (q *Queries) QueryDecryptionKeysAndMessage(ctx context.Context, slot int64) ([]QueryDecryptionKeysAndMessageRow, error) {
	rows, err := q.db.Query(ctx, queryDecryptionKeysAndMessage, slot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []QueryDecryptionKeysAndMessageRow
	for rows.Next() {
		var i QueryDecryptionKeysAndMessageRow
		if err := rows.Scan(
			&i.Slot,
			&i.TxPointer,
			&i.Eon,
			&i.Key,
			&i.IdentityPreimage,
			&i.KeyIndex,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const queryTransactionSubmittedEvent = `-- name: QueryTransactionSubmittedEvent :many
SELECT event_block_hash, event_block_number, event_tx_index, event_log_index, eon, tx_index, identity_prefix, sender, encrypted_transaction, created_at, updated_at FROM transaction_submitted_event
WHERE eon = $1 AND tx_index >= $2 AND tx_index < $2 + $3 ORDER BY tx_index ASC
`

type QueryTransactionSubmittedEventParams struct {
	Eon     int64
	TxIndex int64
	Column3 interface{}
}

func (q *Queries) QueryTransactionSubmittedEvent(ctx context.Context, arg QueryTransactionSubmittedEventParams) ([]TransactionSubmittedEvent, error) {
	rows, err := q.db.Query(ctx, queryTransactionSubmittedEvent, arg.Eon, arg.TxIndex, arg.Column3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TransactionSubmittedEvent
	for rows.Next() {
		var i TransactionSubmittedEvent
		if err := rows.Scan(
			&i.EventBlockHash,
			&i.EventBlockNumber,
			&i.EventTxIndex,
			&i.EventLogIndex,
			&i.Eon,
			&i.TxIndex,
			&i.IdentityPrefix,
			&i.Sender,
			&i.EncryptedTransaction,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const queryValidatorRegistrationMessageNonceBefore = `-- name: QueryValidatorRegistrationMessageNonceBefore :one
SELECT nonce FROM validator_registration_message WHERE validator_index = $1 AND event_block_number <= $2 AND event_tx_index <= $3 AND event_log_index <= $4 ORDER BY event_block_number DESC, event_tx_index DESC, event_log_index DESC FOR UPDATE
`

type QueryValidatorRegistrationMessageNonceBeforeParams struct {
	ValidatorIndex   pgtype.Int8
	EventBlockNumber int64
	EventTxIndex     int64
	EventLogIndex    int64
}

func (q *Queries) QueryValidatorRegistrationMessageNonceBefore(ctx context.Context, arg QueryValidatorRegistrationMessageNonceBeforeParams) (pgtype.Int8, error) {
	row := q.db.QueryRow(ctx, queryValidatorRegistrationMessageNonceBefore,
		arg.ValidatorIndex,
		arg.EventBlockNumber,
		arg.EventTxIndex,
		arg.EventLogIndex,
	)
	var nonce pgtype.Int8
	err := row.Scan(&nonce)
	return nonce, err
}

const queryValidatorRegistryEventsSyncedUntil = `-- name: QueryValidatorRegistryEventsSyncedUntil :one
SELECT block_number FROM validator_registry_events_synced_until LIMIT 1
`

func (q *Queries) QueryValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, queryValidatorRegistryEventsSyncedUntil)
	var block_number int64
	err := row.Scan(&block_number)
	return block_number, err
}
