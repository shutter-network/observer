package data

import (
	"context"
	"fmt"
	"time"

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

func (tr *TransactionRepo) CreateTransaction(ctx context.Context, tx *TransactionV1) (*TransactionV1, error) {
	rows := tr.GetDB(ctx).QueryRow(ctx,
		`INSERT into transaction 
			(encrypted_tx, 
			decryption_key,
			slot) 
		VALUES 
			($1, $2, $3) 
		RETURNING
			id,
			encrypted_tx,
			decryption_key,
			slot,
			created_at`, tx.EncryptedTx, tx.DecryptionKey, tx.Slot)

	err := rows.Scan(&tx.ID, &tx.EncryptedTx, &tx.DecryptionKey, &tx.Slot, &tx.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect transaction from insert result : %w", err)
	}
	return tx, nil
}
