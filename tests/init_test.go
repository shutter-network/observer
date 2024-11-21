package tests

import (
	"context"
	"math/rand"
	"path"
	"runtime"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/stretchr/testify/suite"
)

const ValidatorRegistryContract = "0xefCC23E71f6bA9B22C4D28F7588141d44496A6D6"

type TestMetricsSuite struct {
	suite.Suite

	testDB *common.TestDatabase

	txMapperDB metrics.TxMapper
	dbQuery    *data.Queries
}

func TestMain(t *testing.T) {
	suite.Run(t, new(TestMetricsSuite))
}

func (s *TestMetricsSuite) TearDownAllSuite() {
	s.testDB.TearDown()
}

func (s *TestMetricsSuite) SetupSuite() {
	ctx := context.Background()
	_, curFile, _, _ := runtime.Caller(0)
	curDir := path.Dir(curFile)

	migrationsPath := curDir + "/../migrations"
	s.testDB = common.SetupTestDatabase(migrationsPath)

	s.txMapperDB = metrics.NewTxMapperDB(ctx, s.testDB.DbInstance, &common.Config{}, &ethclient.Client{}, &beaconapiclient.Client{}, 1, rand.Uint64(), rand.Uint64())
	s.dbQuery = data.New(s.testDB.DbInstance)
}
