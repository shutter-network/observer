package tests

import (
	"context"
	cryptoRand "crypto/rand"
	"math/rand"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

func (s *TestMetricsSuite) TestCreateTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:   ectx,
		DecryptionKey: dk,
		Slot:          slot,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, ectx)
	s.Require().Equal(tx.DecryptionKey, dk)
	s.Require().Equal(tx.Slot, slot)
	s.Require().Empty(tx.BlockHash)
}

func (s *TestMetricsSuite) TestQueryTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	blockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:   ectx,
		DecryptionKey: dk,
		Slot:          slot,
		BlockHash:     blockHash,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, ectx)
	s.Require().Equal(tx.DecryptionKey, dk)
	s.Require().Equal(tx.Slot, slot)
	s.Require().Equal(tx.BlockHash, blockHash)

	txs, err := s.transactionRepo.QueryTransactions(context.Background(), &data.QueryTransaction{
		EncryptedTxs: [][]byte{ectx},
	})
	s.Require().NoError(err)
	s.Require().Equal(len(txs), 1)
	s.Require().Equal(txs[0].EncryptedTx, ectx)
	s.Require().Equal(txs[0].DecryptionKey, dk)
	s.Require().Equal(txs[0].Slot, slot)
	s.Require().Equal(txs[0].BlockHash, blockHash)
}

func (s *TestMetricsSuite) TestUpdateTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	blockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:   ectx,
		DecryptionKey: dk,
		Slot:          slot,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, ectx)
	s.Require().Equal(tx.DecryptionKey, dk)
	s.Require().Equal(tx.Slot, slot)
	s.Require().Empty(tx.BlockHash)

	updTx, err := s.transactionRepo.UpdateTransaction(context.Background(), &data.TransactionV1{
		ID:            tx.ID,
		EncryptedTx:   ectx,
		DecryptionKey: dk,
		Slot:          slot,
		BlockHash:     blockHash,
	})
	s.Require().NoError(err)
	s.Require().Equal(updTx.EncryptedTx, ectx)
	s.Require().Equal(updTx.DecryptionKey, dk)
	s.Require().Equal(updTx.Slot, slot)
	s.Require().Equal(updTx.BlockHash, blockHash)
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := cryptoRand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
