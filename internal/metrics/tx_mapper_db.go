package metrics

import (
	"context"
	"fmt"

	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperDB struct {
	encryptedTxRepo   *data.EncryptedTxRepo
	decrytionDataRepo *data.DecryptionDataRepo
	txManager         *database.TxManager
}

func NewTxMapperDB(
	etr *data.EncryptedTxRepo,
	ddr *data.DecryptionDataRepo,
	txManager *database.TxManager,
) ITxMapper {
	return &TxMapperDB{
		encryptedTxRepo:   etr,
		decrytionDataRepo: ddr,
		txManager:         txManager,
	}
}

func (tm *TxMapperDB) AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error {
	_, err := tm.encryptedTxRepo.CreateEncryptedTx(context.Background(), &data.EncryptedTxV1{
		Tx:               encryptedTx,
		IdentityPreimage: identityPreimage,
	})
	return err
}

func (tm *TxMapperDB) AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error {
	_, err := tm.decrytionDataRepo.CreateDecryptionData(context.Background(), &data.DecryptionDataV1{
		Key:              dd.Key,
		Slot:             int64(dd.Slot),
		IdentityPreimage: identityPreimage,
	})
	return err
}

func (tm *TxMapperDB) AddBlockHash(slot uint64, blockHash []byte) error {
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
	decryptionData[0].BlockHash = blockHash
	_, err = tm.decrytionDataRepo.UpdateDecryptionData(ctx, decryptionData[0])
	return err
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
