package tests

import (
	"context"
	"path"
	"runtime"
	"testing"

	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/stretchr/testify/suite"
)

type TestMetricsSuite struct {
	suite.Suite

	testDB    *common.TestDatabase
	txManager *database.TxManager

	txMapper   metrics.TxMapper
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

	s.txManager = database.NewTxManager(s.testDB.DbInstance)

	s.txMapperDB = metrics.NewTxMapperDB(ctx, s.txManager)
	s.dbQuery = data.New(s.txManager.GetDB(ctx))
}

func (s *TestMetricsSuite) BeforeTest(suitName, testName string) {
	s.txMapper = metrics.NewTxMapperMemory()
}
