package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common/database"
)

type EncryptedTxV1 struct {
	ID               int64     `db:"id"`
	Tx               []byte    `db:"tx"`
	IdentityPreimage []byte    `db:"identity_preimage"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type QueryEncryptedTx struct {
	Txs               [][]byte
	IdentityPreimages [][]byte
	Limit             int32
	Offset            int32
	SortKey           string
	SortDirection     database.SortDirection
	DoLock            bool
}

type EncryptedTxRepo struct {
	*database.TxManager
}

func NewEncryptedTxRepository(db database.DB) *EncryptedTxRepo {
	return &EncryptedTxRepo{
		database.NewTxManager(db),
	}
}

func (etr *EncryptedTxRepo) CreateEncryptedTx(ctx context.Context, tx *EncryptedTxV1) (*EncryptedTxV1, error) {
	rows := etr.GetDB(ctx).QueryRow(ctx,
		`INSERT into encrypted_tx 
			(tx, 
			identity_preimage) 
		VALUES 
			($1, $2) 
		RETURNING
			id,
			tx,
			identity_preimage,
			created_at`, tx.Tx, tx.IdentityPreimage)

	err := rows.Scan(&tx.ID, &tx.Tx, &tx.IdentityPreimage, &tx.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect encrypted tx from insert result : %w", err)
	}
	return tx, nil
}

func (etr *EncryptedTxRepo) QueryEncryptedTx(ctx context.Context, query *QueryEncryptedTx) ([]*EncryptedTxV1, error) {
	var queryBuilder strings.Builder

	queryBuilder.WriteString(`
		SELECT
			id,
			tx,
			identity_preimage,
			created_at,
			updated_at
		FROM encrypted_tx et`)

	var conditions []string
	queryArgs := pgx.NamedArgs{}
	if len(query.Txs) > 0 {
		conditions = append(conditions, `et.tx=ANY(@TXs)`)
		queryArgs["TXs"] = query.Txs
	}

	if len(query.IdentityPreimages) > 0 {
		conditions = append(conditions, `et.identity_preimage=ANY(@IdentityPreimages)`)
		queryArgs["IdentityPreimages"] = query.IdentityPreimages
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	if len(query.SortKey) > 0 {
		queryBuilder.WriteString(` ORDER BY et.` + query.SortKey)
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

	rows, err := etr.GetDB(ctx).Query(ctx, queryBuilder.String(), queryArgs)

	if err != nil {
		return nil, fmt.Errorf("failed to query encrypted txs from DB: %w", err)
	}

	defer rows.Close()

	encrypted_txs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[EncryptedTxV1])
	if err != nil {
		return nil, fmt.Errorf("failed to collect encrypted txs from select result: %w", err)
	}
	return encrypted_txs, nil
}
