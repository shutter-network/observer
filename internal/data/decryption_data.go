package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common/database"
)

type DecryptionDataV1 struct {
	ID               int64     `db:"id"`
	Key              []byte    `db:"key"`
	Slot             int64     `db:"slot"`
	BlockHash        []byte    `db:"block_hash"`
	IdentityPreimage []byte    `db:"identity_preimage"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type QueryDecryptionData struct {
	Keys              [][]byte
	Slots             []int64
	IdentityPreimages [][]byte
	Limit             int32
	Offset            int32
	SortKey           string
	SortDirection     database.SortDirection
	DoLock            bool
}

type DecryptionDataRepo struct {
	*database.TxManager
}

func NewDecryptionDataRepository(db database.DB) *DecryptionDataRepo {
	return &DecryptionDataRepo{
		database.NewTxManager(db),
	}
}

func (ddr *DecryptionDataRepo) CreateDecryptionData(ctx context.Context, dd *DecryptionDataV1) (*DecryptionDataV1, error) {
	_, err := ddr.GetDB(ctx).Query(ctx,
		`INSERT into decryption_data
			(key,
			slot,
			block_hash,
			identity_preimage) 
		VALUES 
			($1, $2, $3, $4) 
		ON CONFLICT DO NOTHING
		`, dd.Key, dd.Slot, dd.BlockHash, dd.IdentityPreimage)

	if err != nil {
		return nil, fmt.Errorf("failed to insert decryption data : %w", err)
	}
	return dd, nil
}

func (ddr *DecryptionDataRepo) QueryDecryptionData(ctx context.Context, query *QueryDecryptionData) ([]*DecryptionDataV1, error) {
	var queryBuilder strings.Builder

	queryBuilder.WriteString(`
		SELECT
			id,
			key,
			slot,
			block_hash,
			identity_preimage,
			created_at,
			updated_at
		FROM decryption_data dd`)

	var conditions []string
	queryArgs := pgx.NamedArgs{}

	if len(query.Keys) > 0 {
		conditions = append(conditions, `dd.key=ANY(@Keys)`)
		queryArgs["Keys"] = query.Keys
	}

	if len(query.Slots) > 0 {
		conditions = append(conditions, `dd.slot=ANY(@Slots)`)
		queryArgs["Slots"] = query.Slots
	}

	if len(query.IdentityPreimages) > 0 {
		conditions = append(conditions, `dd.identity_preimage=ANY(@IdentityPreimages)`)
		queryArgs["IdentityPreimages"] = query.IdentityPreimages
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	if len(query.SortKey) > 0 {
		queryBuilder.WriteString(` ORDER BY dd.` + query.SortKey)
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

	rows, err := ddr.GetDB(ctx).Query(ctx, queryBuilder.String(), queryArgs)

	if err != nil {
		return nil, fmt.Errorf("failed to query decryption data from DB: %w", err)
	}

	defer rows.Close()

	decryption_keys, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[DecryptionDataV1])
	if err != nil {
		return nil, fmt.Errorf("failed to collect decryption data from select result: %w", err)
	}
	return decryption_keys, nil
}

func (ddr *DecryptionDataRepo) UpdateDecryptionData(ctx context.Context, dd *DecryptionDataV1) (*DecryptionDataV1, error) {
	rows := ddr.GetDB(ctx).QueryRow(ctx, `
		UPDATE decryption_data dd
		SET key = $2,
			slot = $3,
			block_hash = $4,
			identity_preimage = $5
		WHERE 
			id = $1
		RETURNING
			id,
			key,
			slot,
			block_hash,
			identity_preimage,
			created_at,
			updated_at`, dd.ID, dd.Key, dd.Slot, dd.BlockHash, dd.IdentityPreimage)

	err := rows.Scan(&dd.ID, &dd.Key, &dd.Slot, &dd.BlockHash, &dd.IdentityPreimage, &dd.CreatedAt, &dd.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect decryption data from update result : %w", err)
	}
	return dd, nil
}
