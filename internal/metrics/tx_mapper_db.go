package metrics

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperDB struct {
	db      *pgxpool.Pool
	dbQuery *data.Queries
}

func NewTxMapperDB(
	ctx context.Context,
	db *pgxpool.Pool,
) TxMapper {
	return &TxMapperDB{
		db:      db,
		dbQuery: data.New(db),
	}
}

func (tm *TxMapperDB) AddTransactionSubmittedEvent(ctx context.Context, tse *data.TransactionSubmittedEvent) error {
	err := tm.dbQuery.CreateTransactionSubmittedEvent(ctx, data.CreateTransactionSubmittedEventParams{
		EventBlockHash:       tse.EventBlockHash,
		EventBlockNumber:     tse.EventBlockNumber,
		EventTxIndex:         tse.EventTxIndex,
		EventLogIndex:        tse.EventLogIndex,
		Eon:                  tse.Eon,
		TxIndex:              tse.TxIndex,
		IdentityPrefix:       tse.IdentityPrefix,
		Sender:               tse.Sender,
		EncryptedTransaction: tse.EncryptedTransaction,
	})
	if err != nil {
		return err
	}
	metricsEncTxReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddDecryptionKeyAndMessage(
	ctx context.Context,
	dk *data.DecryptionKey,
	dkm *data.DecryptionKeysMessage,
	dkmdk *data.DecryptionKeysMessageDecryptionKey,
) error {
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := tm.dbQuery.WithTx(tx)
	err = qtx.CreateDecryptionKey(ctx, data.CreateDecryptionKeyParams{
		Eon:              dk.Eon,
		IdentityPreimage: dk.IdentityPreimage,
		Key:              dk.Key,
	})
	if err != nil {
		return err
	}

	err = qtx.CreateDecryptionKeyMessage(ctx, data.CreateDecryptionKeyMessageParams{
		Slot:       dkm.Slot,
		InstanceID: dkm.InstanceID,
		Eon:        dkm.Eon,
		TxPointer:  dkm.TxPointer,
	})
	if err != nil {
		return err
	}

	err = qtx.CreateDecryptionKeysMessageDecryptionKey(ctx, data.CreateDecryptionKeysMessageDecryptionKeyParams{
		DecryptionKeysMessageSlot:     dkmdk.DecryptionKeysMessageSlot,
		KeyIndex:                      dkmdk.KeyIndex,
		DecryptionKeyEon:              dkmdk.DecryptionKeyEon,
		DecryptionKeyIdentityPreimage: dkmdk.DecryptionKeyIdentityPreimage,
	})
	if err != nil {
		return err
	}
	metricsDecKeyReceived.Inc()
	return tx.Commit(ctx)
}

func (tm *TxMapperDB) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	err := tm.dbQuery.CreateDecryptionKeyShare(context.Background(), data.CreateDecryptionKeyShareParams{
		Eon:                dks.Eon,
		DecryptionKeyShare: dks.DecryptionKeyShare,
		Slot:               dks.Slot,
		IdentityPreimage:   dks.IdentityPreimage,
		KeyperIndex:        dks.KeyperIndex,
	})
	if err != nil {
		return err
	}
	metricsKeyShareReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddBlock(ctx context.Context, b *data.Block) error {
	err := tm.dbQuery.CreateBlock(context.Background(), data.CreateBlockParams{
		BlockHash:      b.BlockHash,
		BlockNumber:    b.BlockNumber,
		BlockTimestamp: b.BlockTimestamp,
		TxHash:         b.TxHash,
	})
	return err
}

// func (tm *TxMapperDB) CanBeDecrypted(txIndex int64, eon int64, identityPreimage []byte) (bool, error) {
// 	encryptedTx, err := tm.dbQuery.QueryEncryptedTx(context.Background(), data.QueryEncryptedTxParams{
// 		TxIndex: txIndex,
// 		Eon:     eon,
// 	})
// 	if err != nil {
// 		return false, fmt.Errorf("error querying encrypted transaction: %w", err)
// 	}

// 	decryptionData, err := tm.dbQuery.QueryDecryptionData(context.Background(), data.QueryDecryptionDataParams{
// 		Eon:              eon,
// 		IdentityPreimage: identityPreimage,
// 	})
// 	if err != nil {
// 		return false, fmt.Errorf("error querying decryption data: %w", err)
// 	}

// 	if len(encryptedTx) == 0 || len(decryptionData) == 0 {
// 		return false, nil
// 	}
// 	return true, nil
// }
