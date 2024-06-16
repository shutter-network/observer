package tests

import (
	"context"
	"math/rand"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

func (s *TestMetricsSuite) TestCreateTransaction() {
	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:   []byte{1, 2, 3},
		DecryptionKey: []byte{12, 34},
		Slot:          rand.Uint64(),
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)
}
