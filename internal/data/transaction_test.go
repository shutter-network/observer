package data

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/stretchr/testify/assert"
)

var testDbInstance *pgx.Conn

func TestMain(m *testing.M) {
	ctx := context.Background()
	testDB := common.SetupTestDatabase()
	testDbInstance = testDB.DbInstance
	defer testDB.TearDown(ctx)
	os.Exit(m.Run())
}

func TestCreateUser(t *testing.T) {
	ds := NewTransactionRepository(testDbInstance)

	tx, err := ds.CreateTransaction2(context.Background(), &TransactionV1{EncryptedTx: []byte{1, 2, 3}, DecryptionKey: []byte{12, 34}})
	assert.NoError(t, err)
	assert.NotNil(t, tx)
}
