package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

type txKeyType int64

const txKey txKeyType = iota

type DB interface {
	TxStarter
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type TxStarter interface {
	// Begin starts a pseudo nested transaction.
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TxManager struct {
	*txGetter
}

func NewTxManager(db DB) *TxManager {
	return &TxManager{
		txGetter: &txGetter{
			db: db,
		},
	}
}

func (m *TxManager) GetDB(ctx context.Context) DB {
	return m.getDB(ctx)
}

func (m *TxManager) InTx(ctx context.Context, txFunc func(context.Context) error) error {
	tx, err := m.getDB(ctx).Begin(ctx)
	if err != nil {
		return err
	}
	err = txFunc(context.WithValue(ctx, txKey, DB(tx)))
	if err != nil {
		rbErr := tx.Rollback(ctx)
		if rbErr != nil {
			log.Err(rbErr).
				Msg("failed to rollback transaction")
		}
		return err
	}
	return tx.Commit(ctx)
}

type txGetter struct {
	db DB
}

func (g *txGetter) getDB(ctx context.Context) DB {
	if v, ok := ctx.Value(txKey).(DB); ok {
		return v
	}
	return g.db
}
