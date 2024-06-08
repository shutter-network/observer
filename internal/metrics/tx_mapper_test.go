package metrics

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto/bls12381"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	"gotest.tools/assert"
)

func makeKeys(t *testing.T) (*shcrypto.EonPublicKey, *shcrypto.EpochSecretKey, *shcrypto.EpochID) {
	t.Helper()
	g2 := bls12381.NewG2()
	n := 3
	threshold := uint64(2)
	epochID := shcrypto.ComputeEpochID([]byte("epoch1"))

	ps := []*shcrypto.Polynomial{}
	gammas := []*shcrypto.Gammas{}
	for i := 0; i < n; i++ {
		p, err := shcrypto.RandomPolynomial(rand.Reader, threshold-1)
		assert.NilError(t, err)
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
	assert.DeepEqual(t, g2.MulScalar(new(bls12381.PointG2), g2.One(), eonSecretKey), (*bls12381.PointG2)(eonPublicKey), gocmp.Comparer(equalG1))
	epochSecretKey, err := shcrypto.ComputeEpochSecretKey(
		[]int{0, 1},
		[]*shcrypto.EpochSecretKeyShare{epochSecretKeyShares[0], epochSecretKeyShares[1]},
		threshold)
	assert.NilError(t, err)
	return eonPublicKey, epochSecretKey, epochID
}

func equalG1(a, b *bls12381.PointG1) bool {
	g1 := bls12381.NewG1()
	return g1.Equal(a, b)
}

func equalG2(a, b *bls12381.PointG2) bool {
	g2 := bls12381.NewG2()
	return g2.Equal(a, b)
}
