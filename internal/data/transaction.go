package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common/database"
)

type TransactionV1 struct {
	ID               int64     `db:"id"`
	EncryptedTx      []byte    `db:"encrypted_tx"`
	DecryptionKey    []byte    `db:"decryption_key"`
	Slot             int64     `db:"slot"`
	BlockHash        []byte    `db:"block_hash"`
	IdentityPreimage []byte    `db:"identity_preimage"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type QueryTransaction struct {
	EncryptedTxs      [][]byte
	DecryptionKeys    [][]byte
	Slots             []int64
	IdentityPreimages [][]byte
	Limit             int32
	Offset            int32
	SortKey           string
	SortDirection     database.SortDirection
	DoLock            bool
}

type TransactionRepo struct {
	*database.TxManager
}

func NewTransactionRepository(db database.DB) *TransactionRepo {
	return &TransactionRepo{
		database.NewTxManager(db),
	}
}

func (tr *TransactionRepo) CreateTransaction(ctx context.Context, tx *TransactionV1) (*TransactionV1, error) {
	rows := tr.GetDB(ctx).QueryRow(ctx,
		`INSERT into transaction 
			(encrypted_tx, 
			decryption_key,
			slot,
			block_hash,
			identity_preimage) 
		VALUES 
			($1, $2, $3, $4, $5) 
		RETURNING
			id,
			encrypted_tx,
			decryption_key,
			slot,
			block_hash,
			identity_preimage,
			created_at`, tx.EncryptedTx, tx.DecryptionKey, tx.Slot, tx.BlockHash, tx.IdentityPreimage)

	err := rows.Scan(&tx.ID, &tx.EncryptedTx, &tx.DecryptionKey, &tx.Slot, &tx.BlockHash, &tx.IdentityPreimage, &tx.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect transaction from insert result : %w", err)
	}
	return tx, nil
}

func (tr *TransactionRepo) QueryTransactions(ctx context.Context, query *QueryTransaction) ([]*TransactionV1, error) {
	var queryBuilder strings.Builder

	queryBuilder.WriteString(`
		SELECT
			id,
			encrypted_tx,
			decryption_key,
			slot,
			block_hash,
			identity_preimage,
			created_at,
			updated_at
		FROM transaction t`)

	var conditions []string
	queryArgs := pgx.NamedArgs{}
	if len(query.EncryptedTxs) > 0 {
		conditions = append(conditions, `t.encrypted_tx=ANY(@EncryptedTXs)`)
		queryArgs["EncryptedTXs"] = query.EncryptedTxs
	}

	if len(query.EncryptedTxs) > 0 {
		conditions = append(conditions, `t.encrypted_tx=ANY(@EncryptedTXs)`)
		queryArgs["EncryptedTXs"] = query.EncryptedTxs
	}

	if len(query.DecryptionKeys) > 0 {
		conditions = append(conditions, `t.decryption_key=ANY(@DecryptionKeys)`)
		queryArgs["DecryptionKeys"] = query.DecryptionKeys
	}

	if len(query.Slots) > 0 {
		conditions = append(conditions, `t.slot=ANY(@Slots)`)
		queryArgs["Slots"] = query.Slots
	}

	if len(query.IdentityPreimages) > 0 {
		conditions = append(conditions, `t.identity_preimage=ANY(@IdentityPreimages)`)
		queryArgs["IdentityPreimages"] = query.IdentityPreimages
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	if len(query.SortKey) > 0 {
		queryBuilder.WriteString(` ORDER BY t.` + query.SortKey)
		if query.SortDirection == database.DESC {
			queryBuilder.WriteString(` DESC`)
		}
	}

	if query.Limit > 0 {
		queryBuilder.WriteString(fmt.Sprintf(` LIMIT %d `, query.Limit))
	}

	if query.Offset > 0 {
		queryBuilder.WriteString(fmt.Sprintf(` OFFSET %d `, query.Offset))
	}

	if query.DoLock {
		queryBuilder.WriteString(" FOR UPDATE ")
	}

	rows, err := tr.GetDB(ctx).Query(ctx, queryBuilder.String(), queryArgs)

	if err != nil {
		return nil, fmt.Errorf("failed to query transaction from DB: %w", err)
	}

	defer rows.Close()

	transactions, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[TransactionV1])
	if err != nil {
		return nil, fmt.Errorf("failed to collect transactions from select result: %w", err)
	}
	return transactions, nil
}

func (tr *TransactionRepo) UpdateTransaction(ctx context.Context, tx *TransactionV1) (*TransactionV1, error) {
	rows := tr.GetDB(ctx).QueryRow(ctx, `
		UPDATE transaction t
		SET encrypted_tx = $2,
			decryption_key = $3,
			slot = $4,
			block_hash = $5,
			identity_preimage = $6
		WHERE 
			id = $1
		RETURNING
			id,
			encrypted_tx,
			decryption_key,
			slot,
			block_hash,
			identity_preimage,
			created_at,
			updated_at`, tx.ID, tx.EncryptedTx, tx.DecryptionKey, tx.Slot, tx.BlockHash, tx.IdentityPreimage)

	err := rows.Scan(&tx.ID, &tx.EncryptedTx, &tx.DecryptionKey, &tx.Slot, &tx.BlockHash, &tx.IdentityPreimage, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect transaction from update result : %w", err)
	}
	return tx, nil
}
