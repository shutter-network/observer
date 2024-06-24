package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common/database"
)

type KeyShareV1 struct {
	ID               int64     `db:"id"`
	KeyShare         []byte    `db:"key_share"`
	Slot             int64     `db:"slot"`
	IdentityPreimage []byte    `db:"identity_preimage"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

type QueryKeyShares struct {
	KeyShares         [][]byte
	Slots             []int64
	IdentityPreimages [][]byte
	Limit             int32
	Offset            int32
	SortKey           string
	SortDirection     database.SortDirection
	DoLock            bool
}

type KeyShareRepo struct {
	*database.TxManager
}

func NewKeyShareRepository(db database.DB) *KeyShareRepo {
	return &KeyShareRepo{
		database.NewTxManager(db),
	}
}

func (ksr *KeyShareRepo) CreateKeyShare(ctx context.Context, kk *KeyShareV1) (*KeyShareV1, error) {
	rows := ksr.GetDB(ctx).QueryRow(ctx,
		`INSERT into key_share
			(key_share,
			slot,
			identity_preimage) 
		VALUES 
			($1, $2, $3) 
		RETURNING
			id,
			key_share,
			slot,
			identity_preimage,
			created_at`, kk.KeyShare, kk.Slot, kk.IdentityPreimage)

	err := rows.Scan(&kk.ID, &kk.KeyShare, &kk.Slot, &kk.IdentityPreimage, &kk.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to collect key share from insert result : %w", err)
	}
	return kk, nil
}

func (ksr *KeyShareRepo) QueryKeyShares(ctx context.Context, query *QueryKeyShares) ([]*KeyShareV1, error) {
	var queryBuilder strings.Builder

	queryBuilder.WriteString(`
		SELECT
			id,
			key_share,
			slot,
			identity_preimage,
			created_at,
			updated_at
		FROM key_share ks`)

	var conditions []string
	queryArgs := pgx.NamedArgs{}

	if len(query.KeyShares) > 0 {
		conditions = append(conditions, `ks.key=ANY(@KeyShares)`)
		queryArgs["KeyShares"] = query.KeyShares
	}

	if len(query.Slots) > 0 {
		conditions = append(conditions, `ks.slot=ANY(@Slots)`)
		queryArgs["Slots"] = query.Slots
	}

	if len(query.IdentityPreimages) > 0 {
		conditions = append(conditions, `ks.identity_preimage=ANY(@IdentityPreimages)`)
		queryArgs["IdentityPreimages"] = query.IdentityPreimages
	}

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	if len(query.SortKey) > 0 {
		queryBuilder.WriteString(` ORDER BY ks.` + query.SortKey)
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

	rows, err := ksr.GetDB(ctx).Query(ctx, queryBuilder.String(), queryArgs)

	if err != nil {
		return nil, fmt.Errorf("failed to query key shares from DB: %w", err)
	}

	defer rows.Close()

	key_shares, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[KeyShareV1])
	if err != nil {
		return nil, fmt.Errorf("failed to collect key shares from select result: %w", err)
	}
	return key_shares, nil
}
