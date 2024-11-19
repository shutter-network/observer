package tests

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	observerCommon "github.com/shutter-network/gnosh-metrics/common"
	dbTypes "github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/validatorregistry"
	blst "github.com/supranational/blst/bindings/go"
	"gotest.tools/assert"
)

func (s *TestMetricsSuite) TestAggregateValidatorRegistrationMessage() {
	ctx := context.Background()

	validatorIndex := rand.Int63()
	blockNumber := rand.Int63()
	txIndex := rand.Uint64()
	index := rand.Uint64()

	msg := &validatorregistry.AggregateRegistrationMessage{
		Version:                  0,
		ChainID:                  2,
		ValidatorRegistryAddress: common.HexToAddress(ValidatorRegistryContract),
		ValidatorIndex:           uint64(validatorIndex),
		Nonce:                    0,
		Count:                    1,
		IsRegistration:           true,
	}

	var ikm [32]byte
	var sks []*blst.SecretKey
	var pks []*blst.P1Affine
	for i := 0; i < int(msg.Count); i++ {
		privkey := blst.KeyGen(ikm[:])
		pubkey := new(blst.P1Affine).From(privkey)
		sks = append(sks, privkey)
		pks = append(pks, pubkey)
	}

	sig := validatorregistry.CreateAggregateSignature(sks, msg)
	url := mockBeaconClient(s.T(), hex.EncodeToString(pks[0].Compress()))

	cl, err := beaconapiclient.New(url)
	s.Require().NoError(err)

	event := validatorRegistryBindings.ValidatorregistryUpdated{
		Signature: sig.Compress(),
		Message:   msg.Marshal(),
		Raw: types.Log{
			Address:     common.HexToAddress(ValidatorRegistryContract),
			BlockNumber: uint64(blockNumber),
			TxIndex:     uint(txIndex),
			Index:       uint(index),
		},
	}
	s.txMapperDB = metrics.NewTxMapperDB(ctx, s.testDB.DbInstance, &observerCommon.Config{ValidatorRegistryContractAddress: ValidatorRegistryContract}, &ethclient.Client{}, cl, 2, rand.Uint64(), rand.Uint64())
	err = s.txMapperDB.AddValidatorRegistryEvent(ctx, &event)
	s.Require().NoError(err)

	currentNonce, err := s.dbQuery.QueryValidatorRegistrationMessageNonceBefore(ctx, data.QueryValidatorRegistrationMessageNonceBeforeParams{
		ValidatorIndex:   dbTypes.Int64ToPgTypeInt8(validatorIndex),
		EventBlockNumber: int64(blockNumber),
		EventTxIndex:     int64(txIndex),
		EventLogIndex:    int64(index),
	})
	s.Require().NoError(err)
	s.Require().Equal(currentNonce.Int64, int64(0))
}

func (s *TestMetricsSuite) TestLegacyValidatorRegistrationMessage() {
	ctx := context.Background()

	validatorIndex := rand.Int63()
	blockNumber := rand.Int63()
	txIndex := rand.Uint64()
	index := rand.Uint64()

	msg := &validatorregistry.LegacyRegistrationMessage{
		Version:                  1,
		ChainID:                  2,
		ValidatorRegistryAddress: common.HexToAddress(ValidatorRegistryContract),
		ValidatorIndex:           uint64(validatorIndex),
		Nonce:                    1,
		IsRegistration:           true,
	}

	var ikm [32]byte
	privkey := blst.KeyGen(ikm[:])
	pubkey := new(blst.P1Affine).From(privkey)

	sig := validatorregistry.CreateSignature(privkey, msg)
	url := mockBeaconClient(s.T(), hex.EncodeToString(pubkey.Compress()))

	cl, err := beaconapiclient.New(url)
	s.Require().NoError(err)

	event := validatorRegistryBindings.ValidatorregistryUpdated{
		Signature: sig.Compress(),
		Message:   msg.Marshal(),
		Raw: types.Log{
			Address:     common.HexToAddress(ValidatorRegistryContract),
			BlockNumber: uint64(blockNumber),
			TxIndex:     uint(txIndex),
			Index:       uint(index),
		},
	}
	s.txMapperDB = metrics.NewTxMapperDB(ctx, s.testDB.DbInstance, &observerCommon.Config{ValidatorRegistryContractAddress: ValidatorRegistryContract}, &ethclient.Client{}, cl, 2, rand.Uint64(), rand.Uint64())
	err = s.txMapperDB.AddValidatorRegistryEvent(ctx, &event)
	s.Require().NoError(err)

	currentNonce, err := s.dbQuery.QueryValidatorRegistrationMessageNonceBefore(ctx, data.QueryValidatorRegistrationMessageNonceBeforeParams{
		ValidatorIndex:   dbTypes.Int64ToPgTypeInt8(validatorIndex),
		EventBlockNumber: int64(blockNumber),
		EventTxIndex:     int64(txIndex),
		EventLogIndex:    int64(index),
	})
	s.Require().NoError(err)
	s.Require().Equal(currentNonce.Int64, int64(1))
}

func mockBeaconClient(t *testing.T, pubKeyHex string) string {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x := beaconapiclient.GetValidatorByIndexResponse{
			Finalized: true,
			Data: beaconapiclient.ValidatorData{
				Validator: beaconapiclient.Validator{
					PubkeyHex: pubKeyHex,
				},
			},
		}
		res, err := json.Marshal(x)
		assert.NilError(t, err)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(res)
		assert.NilError(t, err)
	})).URL
}
