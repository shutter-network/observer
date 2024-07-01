package tests

import (
	"math/big"
	"math/rand"

	cryptorand "crypto/rand"

	"github.com/ethereum/go-ethereum/crypto/bls12381"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gotest.tools/assert"
)

var tx = []byte("mimic an evm compatible transaction")

func equalG2(a, b *bls12381.PointG2) bool {
	g2 := bls12381.NewG2()
	return g2.Equal(a, b)
}

var (
	g2Comparer = gocmp.Comparer(equalG2)
)

func (s *TestMetricsSuite) makeKeys() (*shcrypto.EonPublicKey, *shcrypto.EpochSecretKey, *shcrypto.EpochID) {
	s.T().Helper()
	g2 := bls12381.NewG2()
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
	assert.DeepEqual(s.T(), g2.MulScalar(new(bls12381.PointG2), g2.One(), eonSecretKey), (*bls12381.PointG2)(eonPublicKey), g2Comparer)
	epochSecretKey, err := shcrypto.ComputeEpochSecretKey(
		[]int{0, 1},
		[]*shcrypto.EpochSecretKeyShare{epochSecretKeyShares[0], epochSecretKeyShares[1]},
		threshold)
	assert.NilError(s.T(), err)
	return eonPublicKey, epochSecretKey, epochID
}

func (s *TestMetricsSuite) TestAddTxWhenEncryptionTxReceivedFirst() {
	eonPublicKey, decryptionKey, identity := s.makeKeys()

	sigma, err := shcrypto.RandomSigma(cryptorand.Reader)
	s.Require().NoError(err)
	encryptedTransaction := shcrypto.Encrypt(tx, eonPublicKey, identity, sigma)

	encrypedTxBytes := encryptedTransaction.Marshal()

	s.txMapper.AddEncryptedTx(identity.Marshal(), encrypedTxBytes)

	hasComplete, err := s.txMapper.CanBeDecrypted(identity.Marshal())
	s.Require().NoError(err)
	s.Require().False(hasComplete)

	decryptedMessage, err := encryptedTransaction.Decrypt(decryptionKey)
	s.Require().NoError(err)

	s.Require().Equal(tx, decryptedMessage)

	s.txMapper.AddDecryptionData(identity.Marshal(), &metrics.DecryptionData{
		Key:  decryptionKey.Marshal(),
		Slot: rand.Uint64(),
	})

	hasComplete, err = s.txMapper.CanBeDecrypted(identity.Marshal())
	s.Require().NoError(err)
	s.Require().True(hasComplete)
}

func (s *TestMetricsSuite) TestAddTxWhenDecryptionKeysReceivedFirst() {
	eonPublicKey, decryptionKey, identity := s.makeKeys()

	s.txMapper.AddDecryptionData(identity.Marshal(), &metrics.DecryptionData{
		Key:  decryptionKey.Marshal(),
		Slot: rand.Uint64(),
	})

	hasComplete, err := s.txMapper.CanBeDecrypted(identity.Marshal())
	s.Require().NoError(err)
	s.Require().False(hasComplete)

	sigma, err := shcrypto.RandomSigma(cryptorand.Reader)
	s.Require().NoError(err)

	encryptedTransaction := shcrypto.Encrypt(tx, eonPublicKey, identity, sigma)

	encrypedTxBytes := encryptedTransaction.Marshal()

	s.txMapper.AddEncryptedTx(identity.Marshal(), encrypedTxBytes)

	decryptedMessage, err := encryptedTransaction.Decrypt(decryptionKey)
	s.Require().NoError(err)

	s.Require().Equal(tx, decryptedMessage)

	hasComplete, err = s.txMapper.CanBeDecrypted(identity.Marshal())
	s.Require().NoError(err)
	s.Require().True(hasComplete)
}
