package tests

import (
	"context"
	"math/big"
	"math/rand"

	cryptorand "crypto/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	blst "github.com/supranational/blst/bindings/go"
	"gotest.tools/assert"
)

var tx = []byte("mimic an evm compatible transaction")

func bigToScalar(i *big.Int) *blst.Scalar {
	b := make([]byte, 32)
	i.FillBytes(b)
	s := new(blst.Scalar)
	s.FromBEndian(b)
	return s
}

func generateP2(i *big.Int) *blst.P2Affine {
	s := bigToScalar(i)
	return blst.P2Generator().Mult(s).ToAffine()
}

func (s *TestMetricsSuite) makeKeys() (*shcrypto.EonPublicKey, *shcrypto.EpochSecretKey, *shcrypto.EpochID) {
	s.T().Helper()
	n := 3
	threshold := uint64(2)
	epochID := shcrypto.ComputeEpochID([]byte("epoch1"))

	ps := []*shcrypto.Polynomial{}
	gammas := []*shcrypto.Gammas{}
	for i := 0; i < n; i++ {
		p, err := shcrypto.RandomPolynomial(cryptorand.Reader, threshold-1)
		assert.NilError(s.T(), err)
		ps = append(ps, p)
		gammas = append(gammas, p.Gammas())
	}

	eonSecretKeyShares := []*shcrypto.EonSecretKeyShare{}
	epochSecretKeyShares := []*shcrypto.EpochSecretKeyShare{}
	eonSecretKey := big.NewInt(0)
	for i := 0; i < n; i++ {
		eonSecretKey.Add(eonSecretKey, ps[i].Eval(big.NewInt(0)))

		ss := []*big.Int{}
		for j := 0; j < n; j++ {
			s := ps[j].EvalForKeyper(i)
			ss = append(ss, s)
		}
		eonSecretKeyShares = append(eonSecretKeyShares, shcrypto.ComputeEonSecretKeyShare(ss))
		_ = shcrypto.ComputeEonPublicKeyShare(i, gammas)
		epochSecretKeyShares = append(epochSecretKeyShares, shcrypto.ComputeEpochSecretKeyShare(eonSecretKeyShares[i], epochID))
	}
	eonPublicKey := shcrypto.ComputeEonPublicKey(gammas)
	eonPublicKeyExp := (*shcrypto.EonPublicKey)(generateP2(eonSecretKey))
	assert.Assert(s.T(), eonPublicKey.Equal(eonPublicKeyExp))
	epochSecretKey, err := shcrypto.ComputeEpochSecretKey(
		[]int{0, 1},
		[]*shcrypto.EpochSecretKeyShare{epochSecretKeyShares[0], epochSecretKeyShares[1]},
		threshold)
	assert.NilError(s.T(), err)

	return eonPublicKey, epochSecretKey, epochID
}

func (s *TestMetricsSuite) TestAddTransactionSubmittedEventAndDecryptionData() {
	eonPublicKey, decryptionKey, identity := s.makeKeys()

	sigma, err := shcrypto.RandomSigma(cryptorand.Reader)
	s.Require().NoError(err)
	encryptedTransaction := shcrypto.Encrypt(tx, eonPublicKey, identity, sigma)

	encrypedTxBytes := encryptedTransaction.Marshal()

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
	slot := rand.Int63()
	instanceID := rand.Int63()
	txPointer := rand.Int63()

	eventTxHash, err := generateRandomBytes(32)
	s.Require().NoError(err)

	_, err = s.txMapperDB.AddTransactionSubmittedEvent(ctx, nil, &sequencer.SequencerTransactionSubmitted{
		Eon:                  uint64(eon),
		TxIndex:              uint64(txIndex),
		IdentityPrefix:       [32]byte(identityPrefix),
		Sender:               common.Address(sender),
		EncryptedTransaction: encrypedTxBytes,
		Raw: types.Log{
			BlockHash:   common.Hash(eventBlockHash),
			BlockNumber: uint64(eventBlockNumber),
			TxIndex:     uint(eventTxIndex),
			Index:       uint(eventLogIndex),
			TxHash:      common.Hash(eventTxHash),
		},
	})
	s.Require().NoError(err)

	decryptedMessage, err := encryptedTransaction.Decrypt(decryptionKey)
	s.Require().NoError(err)

	s.Require().Equal(tx, decryptedMessage)

	err = s.txMapperDB.AddDecryptionKeysAndMessages(ctx, &metrics.DecKeysAndMessages{
		Eon:        eon,
		Keys:       [][]byte{decryptionKey.Marshal()},
		Identities: [][]byte{identity.Marshal()},
		Slot:       slot,
		InstanceID: instanceID,
		TxPointer:  txPointer,
	})
	s.Require().NoError(err)
}
