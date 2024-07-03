package metrics

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperDB struct {
	encryptedTxRepo   *data.EncryptedTxRepo
	decrytionDataRepo *data.DecryptionDataRepo
	keyShareRepo      *data.KeyShareRepo
	txManager         *database.TxManager
}

func NewTxMapperDB(
	etr *data.EncryptedTxRepo,
	ddr *data.DecryptionDataRepo,
	ksr *data.KeyShareRepo,
	txManager *database.TxManager,
) TxMapper {
	return &TxMapperDB{
		encryptedTxRepo:   etr,
		decrytionDataRepo: ddr,
		keyShareRepo:      ksr,
		txManager:         txManager,
	}
}

func (tm *TxMapperDB) AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error {
	_, err := tm.encryptedTxRepo.CreateEncryptedTx(context.Background(), &data.EncryptedTxV1{
		Tx:               encryptedTx,
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return err
	}
	metricsEncTxReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error {
	_, err := tm.decrytionDataRepo.CreateDecryptionData(context.Background(), &data.DecryptionDataV1{
		Key:              dd.Key,
		Slot:             int64(dd.Slot),
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return err
	}
	metricsDecKeyReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddKeyShare(identityPreimage []byte, ks *KeyShare) error {
	_, err := tm.keyShareRepo.CreateKeyShare(context.Background(), &data.KeyShareV1{
		KeyShare:         ks.Share,
		Slot:             int64(ks.Slot),
		IdentityPreimage: identityPreimage,
	})
	if err != nil {
		return err
	}
	metricsKeyShareReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddBlockHash(slot uint64, blockHash common.Hash) error {
	ctx := context.Background()
	decryptionData, err := tm.decrytionDataRepo.QueryDecryptionData(ctx, &data.QueryDecryptionData{
		Slots:  []int64{int64(slot)},
		DoLock: true,
	})

	if err != nil {
		return fmt.Errorf("error querying decryption data: %w", err)
	}

	if len(decryptionData) == 0 {
		return nil
	}
	decryptionData[0].BlockHash = blockHash.Bytes()
	_, err = tm.decrytionDataRepo.UpdateDecryptionData(ctx, decryptionData[0])
	if err != nil {
		return err
	}
	metricsShutterTxIncludedInBlock.Inc()
	return nil
}

func (tm *TxMapperDB) CanBeDecrypted(identityPreimage []byte) (bool, error) {
	encryptedTx, err := tm.encryptedTxRepo.QueryEncryptedTx(context.Background(), &data.QueryEncryptedTx{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	if err != nil {
		return false, fmt.Errorf("error querying encrypted transaction: %w", err)
	}

	decryptionData, err := tm.decrytionDataRepo.QueryDecryptionData(context.Background(), &data.QueryDecryptionData{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	if err != nil {
		return false, fmt.Errorf("error querying decryption data: %w", err)
	}

	if len(encryptedTx) == 0 || len(decryptionData) == 0 {
		return false, nil
	}
	return true, nil
}
