package metrics

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperDB struct {
	dbQuery *data.Queries
}

func NewTxMapperDB(
	ctx context.Context,
	txManager *database.TxManager,
) TxMapper {
	return &TxMapperDB{
		dbQuery: data.New(txManager.GetDB(ctx)),
	}
}

func (tm *TxMapperDB) AddEncryptedTx(txIndex int64, eon int64, identityPreimage []byte, encryptedTx []byte) error {
	err := tm.dbQuery.CreateEncryptedTx(context.Background(), data.CreateEncryptedTxParams{
		TxIndex:          txIndex,
		Eon:              eon,
		Tx:               encryptedTx,
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return err
	}
	metricsEncTxReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddDecryptionData(eon int64, identityPreimage []byte, dd *DecryptionData) error {
	err := tm.dbQuery.CreateDecryptionData(context.Background(), data.CreateDecryptionDataParams{
		Eon:              eon,
		DecryptionKey:    dd.Key,
		Slot:             int64(dd.Slot),
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return err
	}
	metricsDecKeyReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddKeyShare(eon int64, identityPreimage []byte, keyperIndex int64, ks *KeyShare) error {
	err := tm.dbQuery.CreateDecryptionKeyShare(context.Background(), data.CreateDecryptionKeyShareParams{
		Eon:                eon,
		DecryptionKeyShare: ks.Share,
		Slot:               int64(ks.Slot),
		IdentityPreimage:   identityPreimage,
		KeyperIndex:        keyperIndex,
	})
	if err != nil {
		return err
	}
	metricsKeyShareReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddBlockHash(slot int64, blockHash common.Hash) error {
	ctx := context.Background()
	err := tm.dbQuery.UpdateBlockHash(ctx, data.UpdateBlockHashParams{
		Slot:      slot,
		BlockHash: blockHash.Bytes(),
	})
	if err != nil {
		return err
	}
	metricsShutterTxIncludedInBlock.Inc()
	return nil
}

func (tm *TxMapperDB) CanBeDecrypted(txIndex int64, eon int64, identityPreimage []byte) (bool, error) {
	encryptedTx, err := tm.dbQuery.QueryEncryptedTx(context.Background(), data.QueryEncryptedTxParams{
		TxIndex: txIndex,
		Eon:     eon,
	})
	if err != nil {
		return false, fmt.Errorf("error querying encrypted transaction: %w", err)
	}

	decryptionData, err := tm.dbQuery.QueryDecryptionData(context.Background(), data.QueryDecryptionDataParams{
		Eon:              eon,
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return false, fmt.Errorf("error querying decryption data: %w", err)
	}

	if len(encryptedTx) == 0 || len(decryptionData) == 0 {
		return false, nil
	}
	return true, nil
}
