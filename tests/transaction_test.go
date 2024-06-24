package tests

import (
	"context"
	cryptoRand "crypto/rand"
	"math/rand"

	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
)

func (s *TestMetricsSuite) TestEncryptedTransaction() {
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.encryptedTxRepo.CreateEncryptedTx(context.Background(), &data.EncryptedTxV1{
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.Tx, ectx)
	s.Require().Equal(tx.IdentityPreimage, identityPreimage)
}

func (s *TestMetricsSuite) TestQueryEncryptedTransaction() {
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	tx, err := s.encryptedTxRepo.CreateEncryptedTx(context.Background(), &data.EncryptedTxV1{
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.Tx, ectx)
	s.Require().Equal(tx.IdentityPreimage, identityPreimage)

	txs, err := s.encryptedTxRepo.QueryEncryptedTx(context.Background(), &data.QueryEncryptedTx{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	s.Require().NoError(err)
	s.Require().Equal(len(txs), 1)
	s.Require().Equal(txs[0].Tx, ectx)
	s.Require().Equal(txs[0].IdentityPreimage, identityPreimage)

}

func (s *TestMetricsSuite) TestUpdateBlockHashTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	blockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	ctx := context.Background()
	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)
	tx, err := s.encryptedTxRepo.CreateEncryptedTx(ctx, &data.EncryptedTxV1{
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.Tx, ectx)
	s.Require().Equal(tx.IdentityPreimage, identityPreimage)

	dd, err := s.decryptionDataRepo.CreateDecryptionData(ctx, &data.DecryptionDataV1{
		Key:              dk,
		Slot:             slot,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(dd)

	dd.BlockHash = blockHash
	updTx, err := s.decryptionDataRepo.UpdateDecryptionData(ctx, dd)

	s.Require().NoError(err)
	s.Require().Equal(updTx.Key, dk)
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

	dd, err := s.decryptionDataRepo.QueryDecryptionData(context.Background(), &data.QueryDecryptionData{
		IdentityPreimages: [][]byte{identityPreimage},
	})
	s.Require().NoError(err)

	s.Require().Equal(len(dd), 1)
	s.Require().Equal(dd[0].Key, dk)
	s.Require().Equal(dd[0].Slot, slot)
	s.Require().Equal(dd[0].IdentityPreimage, identityPreimage)
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

	err = s.txMapperDB.AddBlockHash(uint64(slot), blockHash)
	s.Require().NoError(err)
}

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := cryptoRand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
