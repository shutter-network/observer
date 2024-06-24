package metrics

import (
	"context"
	"fmt"

	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperDB struct {
	transactionRepo *data.TransactionRepo
	txManager       *database.TxManager
}

func NewTxMapperDB(
	tr *data.TransactionRepo,
	txManager *database.TxManager,
) ITxMapper {
	return &TxMapperDB{
		transactionRepo: tr,
		txManager:       txManager,
	}
}

func (tm *TxMapperDB) AddEncryptedTx(identityPreimage []byte, encryptedTx []byte) error {
	err := tm.txManager.InTx(context.Background(), func(ctx context.Context) error {
		transactions, err := tm.transactionRepo.QueryTransactions(ctx, &data.QueryTransaction{
			IdentityPreimages: [][]byte{identityPreimage},
			DoLock:            true,
		})
		if err != nil {
			return fmt.Errorf("error querying transaction: %w", err)
		}

		if len(transactions) == 0 {
			_, err = tm.transactionRepo.CreateTransaction(ctx, &data.TransactionV1{
				EncryptedTx:      encryptedTx,
				IdentityPreimage: identityPreimage,
			})
			return err
		}
		transactions[0].EncryptedTx = encryptedTx
		_, err = tm.transactionRepo.UpdateTransaction(ctx, transactions[0])
		return err
	})
	return err
}

func (tm *TxMapperDB) AddDecryptionData(identityPreimage []byte, dd *DecryptionData) error {
	err := tm.txManager.InTx(context.Background(), func(ctx context.Context) error {
		transactions, err := tm.transactionRepo.QueryTransactions(ctx, &data.QueryTransaction{
			IdentityPreimages: [][]byte{identityPreimage},
			DoLock:            true,
		})

		if err != nil {
			return fmt.Errorf("error querying transaction: %w", err)
		}

		if len(transactions) == 0 {
			_, err := tm.transactionRepo.CreateTransaction(ctx, &data.TransactionV1{
				DecryptionKey:    dd.Key,
				Slot:             int64(dd.Slot),
				IdentityPreimage: identityPreimage,
			})
			return err
		}
		transactions[0].DecryptionKey = dd.Key
		transactions[0].Slot = int64(dd.Slot)
		_, err = tm.transactionRepo.UpdateTransaction(ctx, transactions[0])
		return err
	})
	return err
}

func (tm *TxMapperDB) AddBlockHash(slot uint64, blockHash []byte) error {
	err := tm.txManager.InTx(context.Background(), func(ctx context.Context) error {
		transactions, err := tm.transactionRepo.QueryTransactions(ctx, &data.QueryTransaction{
			Slots:  []int64{int64(slot)},
			DoLock: true,
		})

		if err != nil {
			return fmt.Errorf("error querying transaction: %w", err)
		}

		if len(transactions) == 0 {
			return nil
		}
		transactions[0].BlockHash = blockHash
		_, err = tm.transactionRepo.UpdateTransaction(ctx, transactions[0])
		return err
	})
	return err
}

func (tm *TxMapperDB) CanBeDecrypted(identityPreimage []byte) (bool, error) {
	transactions, err := tm.transactionRepo.QueryTransactions(context.Background(), &data.QueryTransaction{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	if err != nil {
		return false, fmt.Errorf("error querying transaction: %w", err)
	}

	if len(transactions) == 0 {
		return false, nil
	}
	return true, nil
}
