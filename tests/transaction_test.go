package tests

import (
	"context"
	"math/rand"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

func (s *TestMetricsSuite) TestCreateTransaction() {
	slot := rand.Int63()
	tx, err := s.transactionRepo.CreateTransaction(context.Background(), &data.TransactionV1{
		EncryptedTx:   []byte{1, 2, 3},
		DecryptionKey: []byte{12, 34},
		Slot:          slot,
	})
	s.Require().NoError(err)
	s.Require().NotNil(tx)

	s.Require().Equal(tx.EncryptedTx, []byte{1, 2, 3})
	s.Require().Equal(tx.DecryptionKey, []byte{12, 34})
	s.Require().Equal(tx.Slot, slot)
	s.Require().Empty(tx.BlockHash)
}
