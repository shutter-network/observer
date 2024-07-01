package tests

import (
	"context"
	cryptoRand "crypto/rand"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
)

func (s *TestMetricsSuite) TestCreateTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:      ectx,
		DecryptionKey:    dk,
		Slot:             slot,
		IdentityPreimage: identityPreimage,
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
	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:      ectx,
		DecryptionKey:    dk,
		Slot:             slot,
		BlockHash:        blockHash,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, ectx)
	s.Require().Equal(tx.DecryptionKey, dk)
	s.Require().Equal(tx.Slot, slot)
	s.Require().Equal(tx.BlockHash, blockHash)

	txs, err := s.transactionRepo.QueryTransactions(context.Background(), &data.QueryTransaction{
		IdentityPreimages: [][]byte{identityPreimage},
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

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)
	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:      ectx,
		DecryptionKey:    dk,
		Slot:             slot,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, ectx)
	s.Require().Equal(tx.DecryptionKey, dk)
	s.Require().Equal(tx.Slot, slot)
	s.Require().Empty(tx.BlockHash)

	tx.BlockHash = blockHash
	updTx, err := s.transactionRepo.UpdateTransaction(context.Background(), tx)
	s.Require().NoError(err)
	s.Require().Equal(updTx.EncryptedTx, ectx)
	s.Require().Equal(updTx.DecryptionKey, dk)
	s.Require().Equal(updTx.Slot, slot)
	s.Require().Equal(updTx.BlockHash, blockHash)
}

func (s *TestMetricsSuite) TestAddDecryptionData() {
	slot := rand.Int63()
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	err = s.txMapperDB.AddDecryptionData(identityPreimage, &metrics.DecryptionData{
		Key:  dk,
		Slot: uint64(slot),
	})
	s.Require().NoError(err)

	tx, err := s.transactionRepo.QueryTransactions(context.Background(), &data.QueryTransaction{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	s.Require().NoError(err)

	s.Require().Equal(len(tx), 1)
	s.Require().Equal(tx[0].DecryptionKey, dk)
	s.Require().Equal(tx[0].Slot, slot)
	s.Require().Equal(tx[0].IdentityPreimage, identityPreimage)
}

func (s *TestMetricsSuite) TestAddFullTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	blockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)
	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	err = s.txMapperDB.AddEncryptedTx(identityPreimage, ectx)
	s.Require().NoError(err)

	err = s.txMapperDB.AddDecryptionData(identityPreimage, &metrics.DecryptionData{
		Key:  dk,
		Slot: uint64(slot),
	})
	s.Require().NoError(err)

	err = s.txMapperDB.AddBlockHash(uint64(slot), common.Hash(blockHash))
	s.Require().NoError(err)

	tx, err := s.transactionRepo.QueryTransactions(context.Background(), &data.QueryTransaction{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	s.Require().NoError(err)

	s.Require().Equal(len(tx), 1)
	s.Require().Equal(tx[0].EncryptedTx, ectx)
	s.Require().Equal(tx[0].DecryptionKey, dk)
	s.Require().Equal(tx[0].Slot, slot)
	s.Require().Equal(tx[0].BlockHash, blockHash)
	s.Require().Equal(tx[0].IdentityPreimage, identityPreimage)
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := cryptoRand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
