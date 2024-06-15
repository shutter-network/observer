package tests

import (
	"context"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

func (s *TestMetricsSuite) TestCreateTransaction() {
	tx, err := s.transactionRepo.CreateTransaction2(context.Background(), &data.TransactionV1{
		EncryptedTx:   []byte{1, 2, 3},
		DecryptionKey: []byte{12, 34},
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)
}
