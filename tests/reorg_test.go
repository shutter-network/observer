package tests

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shutter-network/observer/internal/data"
)

func (s *TestMetricsSuite) TestHandlePotentialReorgTXSubmittedEvent() {
	ctx := context.Background()
	// Setup initial sync status
	blockHash := [32]byte{1, 2, 3}
	err := s.dbQuery.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockHash:   blockHash[:],
		BlockNumber: 99,
	})
	s.Require().NoError(err)

	// Create header with matching parent hash
	header := &types.Header{
		Number:     big.NewInt(100),
		ParentHash: blockHash,
	}

	// no reorg
	err = s.txSubmittedSyncer.HandlePotentialReorg(ctx, header)
	s.Require().NoError(err)

	blockHash = [32]byte{1, 2, 3}
	err = s.dbQuery.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockHash:   blockHash[:],
		BlockNumber: 99,
	})
	s.Require().NoError(err)

	header = &types.Header{
		Number:     big.NewInt(100),
		ParentHash: [32]byte{4, 5, 6},
	}

	// reorg
	err = s.txSubmittedSyncer.HandlePotentialReorg(ctx, header)
	s.Require().NoError(err)

	syncStatus, err := s.dbQuery.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	s.Require().NoError(err)
	s.Require().Equal(int64(89), syncStatus.BlockNumber)
	s.Require().Empty(syncStatus.BlockHash)
}
