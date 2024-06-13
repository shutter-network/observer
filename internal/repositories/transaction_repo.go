package repositories

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/shutter-network/gnosh-metrics/internal/models"
)

type TransactionRepo struct {
	DB *pgxpool.Pool
}

type ITransactionRepo struct {
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{DB: db}
}

func (r *TransactionRepo) CreateTransaction(ctx context.Context, tx *models.Transaction) error {
	query := `
        INSERT INTO transaction (encrypted_tx, decryption_key, slot)
        VALUES ($1, $2, $3)
        RETURNING id
    `
	err := r.DB.QueryRow(ctx, query, tx.EncryptedTx, tx.DecryptionKey, tx.Slot).Scan(&user.ID)
	if err != nil {
		return err
	}
	return nil
}
