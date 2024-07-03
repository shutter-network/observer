package tests

import (
	"context"
	cryptoRand "crypto/rand"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
)

func (s *TestMetricsSuite) TestEncryptedTransaction() {
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	err = s.dbQuery.CreateEncryptedTx(context.Background(), data.CreateEncryptedTxParams{
		TxIndex:          rand.Int63(),
		Eon:              rand.Int63(),
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
}

func (s *TestMetricsSuite) TestQueryEncryptedTransaction() {
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	txIndex := rand.Int63()
	eon := rand.Int63()
	err = s.dbQuery.CreateEncryptedTx(context.Background(), data.CreateEncryptedTxParams{
		TxIndex:          txIndex,
		Eon:              eon,
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	txs, err := s.dbQuery.QueryEncryptedTx(context.Background(), data.QueryEncryptedTxParams{
		TxIndex: txIndex,
		Eon:     eon,
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

	txIndex := rand.Int63()
	eon := rand.Int63()

	err = s.dbQuery.CreateEncryptedTx(context.Background(), data.CreateEncryptedTxParams{
		TxIndex:          txIndex,
		Eon:              eon,
		Tx:               ectx,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	err = s.dbQuery.CreateDecryptionData(ctx, data.CreateDecryptionDataParams{
		Eon:              eon,
		DecryptionKey:    dk,
		Slot:             slot,
		IdentityPreimage: identityPreimage,
	})
	s.Require().NoError(err)

	err = s.dbQuery.UpdateBlockHash(ctx, data.UpdateBlockHashParams{
		Slot:      slot,
		BlockHash: blockHash,
	})
	s.Require().NoError(err)

	dds, err := s.dbQuery.QueryDecryptionData(ctx, data.QueryDecryptionDataParams{
		IdentityPreimage: identityPreimage,
		Eon:              eon,
	})
	s.Require().NoError(err)

	s.Require().NoError(err)
	s.Require().Equal(dds[0].DecryptionKey, dk)
	s.Require().Equal(dds[0].Slot, slot)
	s.Require().Equal(dds[0].BlockHash, blockHash)
}

func (s *TestMetricsSuite) TestAddDecryptionData() {
	ctx := context.Background()
	slot := rand.Int63()
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()
	err = s.txMapperDB.AddDecryptionData(eon, identityPreimage, &metrics.DecryptionData{
		Key:  dk,
		Slot: slot,
	})
	s.Require().NoError(err)

	dds, err := s.dbQuery.QueryDecryptionData(ctx, data.QueryDecryptionDataParams{
		IdentityPreimage: identityPreimage,
		Eon:              eon,
	})
	s.Require().NoError(err)

	s.Require().Equal(len(dds), 1)
	s.Require().Equal(dds[0].DecryptionKey, dk)
	s.Require().Equal(dds[0].Slot, slot)
	s.Require().Equal(dds[0].IdentityPreimage, identityPreimage)
}

func (s *TestMetricsSuite) TestAddKeyShare() {
	slot := rand.Int63()
	ks, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()
	keyperIndex := rand.Int63()
	err = s.txMapperDB.AddKeyShare(eon, identityPreimage, keyperIndex, &metrics.KeyShare{
		Share: ks,
		Slot:  slot,
	})
	s.Require().NoError(err)

	k, err := s.dbQuery.QueryDecryptionKeyShare(context.Background(), data.QueryDecryptionKeyShareParams{
		Eon:              eon,
		IdentityPreimage: identityPreimage,
		KeyperIndex:      keyperIndex,
	})
	s.Require().NoError(err)

	s.Require().Equal(len(k), 1)
	s.Require().Equal(k[0].DecryptionKeyShare, ks)
	s.Require().Equal(k[0].Slot, slot)
	s.Require().Equal(k[0].IdentityPreimage, identityPreimage)
}

func (s *TestMetricsSuite) TestAddFullTransaction() {
	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	ks, err := generateRandomBytes(32)
	s.Require().NoError(err)
	blockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)
	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()
	txIndex := rand.Int63()
	keyperIndex := rand.Int63()

	err = s.txMapperDB.AddEncryptedTx(txIndex, eon, identityPreimage, ectx)
	s.Require().NoError(err)

	err = s.txMapperDB.AddKeyShare(eon, identityPreimage, keyperIndex, &metrics.KeyShare{
		Share: ks,
		Slot:  slot,
	})
	s.Require().NoError(err)

	err = s.txMapperDB.AddDecryptionData(eon, identityPreimage, &metrics.DecryptionData{
		Key:  dk,
		Slot: slot,
	})
	s.Require().NoError(err)

	err = s.txMapperDB.AddBlockHash(slot, common.Hash(blockHash))
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
