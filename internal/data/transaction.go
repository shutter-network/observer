package data

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common/database"
)

type TransactionV1 struct {
	ID            int64     `db:"id"`
	EncryptedTx   []byte    `db:"encrypted_tx"`
	DecryptionKey []byte    `db:"decryption_key"`
	Slot          uint64    `db:"slot"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type TransactionRepo struct {
	*database.TxManager
}

func NewTransactionRepository(db database.DB) *TransactionRepo {
	return &TransactionRepo{
		database.NewTxManager(db),
	}
}

func (tr *TransactionRepo) CreateTransaction(ctx context.Context, txs []*TransactionV1) ([]*TransactionV1, error) {
	rows, err := tr.GetDB(ctx).Query(ctx, `
		INSERT INTO transaction 
			(encrypted_tx, 
			decryption_key, 
			slot)
		SELECT 
			t.encrypted_tx,
			t.decryption_key,
			t.slot
		FROM UNNEST(@Transactions::transaction_v1[]) t
		RETURNING
			id,
			encrypted_tx,
			decryption_key,
			slot,
			created_at`,
		pgx.NamedArgs{"Transactions": txs})

	if err != nil {
		return nil, fmt.Errorf("failed to insert transactions in DB: %w", err)
	}

	defer rows.Close()

	txs, err = pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[TransactionV1])
	if err != nil {
		return nil, fmt.Errorf("failed to collect transactions from insert result: %w", err)
	}
	return txs, nil
}

func (tr *TransactionRepo) CreateTransaction2(ctx context.Context, tx *TransactionV1) (*TransactionV1, error) {
	rows, err := tr.GetDB(ctx).Query(ctx, `INSERT into transaction (encrypted_tx, decryption_key) VALUES ($1, $2) RETURNING id`, tx.EncryptedTx, tx.DecryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to insert transactions in DB: %w", err)
	}

	rows.Scan(&tx.ID)

	return tx, nil
}
