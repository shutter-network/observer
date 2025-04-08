package tests

import (
	"context"
	cryptoRand "crypto/rand"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
)

func (s *TestMetricsSuite) TestEncryptedTransaction() {
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)

	ctx := context.Background()
	txIndex := rand.Int63()
	eon := rand.Int63()
	eventBlockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)
	eventBlockNumber := rand.Int63()
	eventTxIndex := rand.Int63()
	eventLogIndex := rand.Int63()
	identityPrefix, err := generateRandomBytes(32)
	s.Require().NoError(err)
	sender, err := generateRandomBytes(20)
	s.Require().NoError(err)

	eventTxHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	err = s.dbQuery.CreateTransactionSubmittedEvent(ctx, data.CreateTransactionSubmittedEventParams{
		TxIndex:              txIndex,
		Eon:                  eon,
		EventBlockHash:       eventBlockHash,
		EventBlockNumber:     eventBlockNumber,
		EventTxIndex:         eventTxIndex,
		EventLogIndex:        eventLogIndex,
		IdentityPrefix:       identityPrefix,
		Sender:               sender,
		EncryptedTransaction: ectx,
		EventTxHash:          eventTxHash,
	})
	s.Require().NoError(err)
}

func (s *TestMetricsSuite) TestAddDecryptionData() {
	ctx := context.Background()
	slot := rand.Int63()
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()

	instanceID := rand.Int63()
	txPointer := rand.Int63()

	err = s.txMapperDB.AddDecryptionKeysAndMessages(ctx, &metrics.DecKeysAndMessages{
		Eon:        eon,
		Keys:       [][]byte{dk},
		Identities: [][]byte{identityPreimage},
		Slot:       slot,
		InstanceID: instanceID,
		TxPointer:  txPointer,
	})
	s.Require().NoError(err)
}

func (s *TestMetricsSuite) TestAddKeyShare() {
	ctx := context.Background()
	slot := rand.Int63()
	ks, err := generateRandomBytes(32)
	s.Require().NoError(err)

	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()
	keyperIndex := rand.Int63()
	err = s.txMapperDB.AddKeyShare(ctx, &data.DecryptionKeyShare{
		Eon:                eon,
		IdentityPreimage:   identityPreimage,
		KeyperIndex:        keyperIndex,
		DecryptionKeyShare: ks,
		Slot:               slot,
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
	ctx := context.Background()

	slot := rand.Int63()
	ectx, err := generateRandomBytes(32)
	s.Require().NoError(err)
	dk, err := generateRandomBytes(32)
	s.Require().NoError(err)
	ks, err := generateRandomBytes(32)
	s.Require().NoError(err)
	identityPreimage, err := generateRandomBytes(32)
	s.Require().NoError(err)

	eon := rand.Int63()
	txIndex := rand.Int63()
	keyperIndex := rand.Int63()

	eventBlockHash, err := generateRandomBytes(32)
	s.Require().NoError(err)
	eventBlockNumber := rand.Int63()
	eventTxIndex := rand.Int63()
	eventLogIndex := rand.Int63()
	identityPrefix, err := generateRandomBytes(32)
	s.Require().NoError(err)
	sender, err := generateRandomBytes(20)
	s.Require().NoError(err)
	instanceID := rand.Int63()
	txPointer := rand.Int63()

	eventTxHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	s.txMapperDB.AddTransactionSubmittedEvent(ctx, nil, &sequencer.SequencerTransactionSubmitted{
		Eon:                  uint64(eon),
		TxIndex:              uint64(txIndex),
		IdentityPrefix:       [32]byte(identityPrefix),
		Sender:               common.Address(sender),
		EncryptedTransaction: ectx,
		Raw: types.Log{
			BlockHash:   common.Hash(eventBlockHash),
			BlockNumber: uint64(eventBlockNumber),
			TxIndex:     uint(eventTxIndex),
			Index:       uint(eventLogIndex),
			TxHash:      common.Hash(eventTxHash),
		},
	})

	s.Require().NoError(err)

	err = s.txMapperDB.AddKeyShare(ctx, &data.DecryptionKeyShare{
		Eon:                eon,
		IdentityPreimage:   identityPreimage,
		KeyperIndex:        keyperIndex,
		DecryptionKeyShare: ks,
		Slot:               slot,
	})
	s.Require().NoError(err)

	err = s.txMapperDB.AddDecryptionKeysAndMessages(ctx, &metrics.DecKeysAndMessages{
		Eon:        eon,
		Keys:       [][]byte{dk},
		Identities: [][]byte{identityPreimage},
		Slot:       slot,
		InstanceID: instanceID,
		TxPointer:  txPointer,
	})
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
