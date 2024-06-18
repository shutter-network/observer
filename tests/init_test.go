package tests

import (
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

	txMapper        metrics.ITxMapper
	txMapperDB      metrics.ITxMapper
	transactionRepo *data.TransactionRepo
}

func TestMain(t *testing.T) {
	suite.Run(t, new(TestMetricsSuite))
}

func (s *TestMetricsSuite) TearDownAllSuite() {
	s.testDB.TearDown()
}

func (s *TestMetricsSuite) SetupSuite() {
	_, curFile, _, _ := runtime.Caller(0)
	curDir := path.Dir(curFile)

	migrationsPath := curDir + "/../migrations"
	s.testDB = common.SetupTestDatabase(migrationsPath)

	s.txManager = database.NewTxManager(s.testDB.DbInstance)
	s.transactionRepo = data.NewTransactionRepository(s.testDB.DbInstance)

	s.txMapperDB = metrics.NewTxMapperDB(s.transactionRepo, s.txManager)
}

func (s *TestMetricsSuite) BeforeTest(suitName, testName string) {
	s.txMapper = metrics.NewTxMapper()
}
