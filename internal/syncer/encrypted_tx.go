package syncer

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley"
)

const (
	AssumedReorgDepth    = 10
	maxRequestBlockRange = 10_000
)

type EncrptedTxSyncer struct {
	contract             *sequencerBindings.Sequencer
	db                   *pgxpool.Pool
	dbQuery              *data.Queries
	ethClient            *ethclient.Client
	txMapper             metrics.TxMapper
	syncStartBlockNumber uint64
}

func (ets *EncrptedTxSyncer) Sync(ctx context.Context, header *types.Header) error {
	// TODO: handle reorgs
	syncedUntil, err := ets.dbQuery.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "failed to query transaction submitted events sync status")
	}
	var start uint64
	if err == pgx.ErrNoRows {
		start = ets.syncStartBlockNumber
	} else {
		start = uint64(syncedUntil.BlockNumber + 1)
	}
	endBlock := header.Number.Uint64()
	log.Debug().
		Uint64("start-block", start).
		Uint64("end-block", endBlock).
		Msg("syncing transaction submitted events")
	syncRanges := medley.GetSyncRanges(start, endBlock, maxRequestBlockRange)
	for _, r := range syncRanges {
		err = ets.syncRange(ctx, r[0], r[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func (ets *EncrptedTxSyncer) syncRange(
	ctx context.Context,
	start,
	end uint64,
) error {
	events, err := ets.fetchEvents(ctx, start, end)
	if err != nil {
		return err
	}
	for _, event := range events {
		err := ets.txMapper.AddTransactionSubmittedEvent(ctx, event)
		if err != nil {
			log.Err(err).Msg("err adding transaction submitted event")
			return nil
		}
		log.Info().
			Uint64("block", event.Raw.BlockNumber).
			Hex("encrypted transaction (hex)", event.EncryptedTransaction).
			Msg("new encrypted transaction")
	}
	log.Info().
		Uint64("start-block", start).
		Uint64("end-block", end).
		Int("num-inserted-events", len(events)).
		Msg("synced registry contract")
	return nil
}

func (s *EncrptedTxSyncer) fetchEvents(
	ctx context.Context,
	start,
	end uint64,
) ([]*sequencerBindings.SequencerTransactionSubmitted, error) {
	opts := bind.FilterOpts{
		Start:   start,
		End:     &end,
		Context: ctx,
	}
	it, err := s.contract.SequencerFilterer.FilterTransactionSubmitted(&opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query transaction submitted events")
	}
	events := []*sequencerBindings.SequencerTransactionSubmitted{}
	for it.Next() {
		events = append(events, it.Event)
	}
	if it.Error() != nil {
		return nil, errors.Wrap(it.Error(), "failed to iterate query transaction submitted events")
	}
	return events, nil
}
