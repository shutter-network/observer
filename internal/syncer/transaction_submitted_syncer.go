package syncer

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/observer/common/database"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley"
)

const (
	AssumedReorgDepth    = 10
	maxRequestBlockRange = 10_000
)

type TransactionSubmittedSyncer struct {
	contract             *sequencerBindings.Sequencer
	db                   *pgxpool.Pool
	dbQuery              *data.Queries
	ethClient            *ethclient.Client
	txMapper             metrics.TxMapper
	syncStartBlockNumber uint64
}

func NewTransactionSubmittedSyncer(
	contract *sequencerBindings.Sequencer,
	db *pgxpool.Pool,
	ethClient *ethclient.Client,
	txMapper metrics.TxMapper,
	syncStartBlockNumber uint64,
) *TransactionSubmittedSyncer {
	return &TransactionSubmittedSyncer{
		contract:             contract,
		db:                   db,
		dbQuery:              data.New(db),
		ethClient:            ethClient,
		txMapper:             txMapper,
		syncStartBlockNumber: syncStartBlockNumber,
	}
}

func getNumReorgedBlocksForTransactionSubmitted(syncedUntil *data.QueryTransactionSubmittedEventsSyncedUntilRow, header *types.Header) int {
	shouldBeParent := header.Number.Int64() == syncedUntil.BlockNumber+1
	isParent := bytes.Equal(header.ParentHash.Bytes(), syncedUntil.BlockHash)
	isReorg := shouldBeParent && !isParent
	if !isReorg {
		return 0
	}
	// We don't know how deep the reorg is, so we make a conservative guess. Assuming higher depths
	// is safer because it means we resync a little bit more.
	depth := AssumedReorgDepth
	if syncedUntil.BlockNumber < int64(depth) {
		return int(syncedUntil.BlockNumber)
	}
	return depth
}

// resetSyncStatus clears the db from its recent history after a reorg of given depth.
func (ets *TransactionSubmittedSyncer) resetSyncStatus(ctx context.Context, numReorgedBlocks int) error {
	if numReorgedBlocks == 0 {
		return nil
	}

	tx, err := ets.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ets.dbQuery.WithTx(tx)

	syncStatus, err := qtx.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query sync status from db in order to reset it")
	}
	if syncStatus.BlockNumber < int64(numReorgedBlocks) {
		return errors.Wrapf(err, "detected reorg deeper (%d) than blocks synced (%d)", syncStatus.BlockNumber, numReorgedBlocks)
	}

	deleteFromInclusive := syncStatus.BlockNumber - int64(numReorgedBlocks) + 1

	err = qtx.DeleteDecryptedTxFromBlockNumber(ctx, database.Int64ToPgTypeInt8(deleteFromInclusive))
	if err != nil {
		return errors.Wrap(err, "failed to delete decrypted tx from db")
	}

	err = qtx.DeleteTransactionSubmittedEventFromBlockNumber(ctx, deleteFromInclusive)
	if err != nil {
		return errors.Wrap(err, "failed to delete transaction submitted events from db")
	}

	// Currently, we don't have enough information in the db to populate block hash and slot.
	// However, using default values here is fine since the syncer is expected to resync
	// immediately after this function call which will set the correct values. When we do proper
	// reorg handling, we should store the full block data of the previous blocks so that we can
	// avoid this.
	newSyncedUntilBlockNumber := deleteFromInclusive - 1
	err = qtx.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockHash:   []byte{},
		BlockNumber: newSyncedUntilBlockNumber,
	})
	if err != nil {
		return errors.Wrap(err, "failed to reset transaction submitted event sync status in db")
	}
	log.Info().
		Int("depth", numReorgedBlocks).
		Int64("previous-synced-until", syncStatus.BlockNumber).
		Int64("new-synced-until", newSyncedUntilBlockNumber).
		Msg("sync status reset due to reorg")
	return nil
}

func (ets *TransactionSubmittedSyncer) handlePotentialReorg(ctx context.Context, header *types.Header) error {
	syncedUntil, err := ets.dbQuery.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to query transaction submitted events sync status")
	}

	numReorgedBlocks := getNumReorgedBlocksForTransactionSubmitted(&syncedUntil, header)
	if numReorgedBlocks > 0 {
		return ets.resetSyncStatus(ctx, numReorgedBlocks)
	}
	return nil
}

func (ets *TransactionSubmittedSyncer) Sync(ctx context.Context, header *types.Header) error {
	if err := ets.handlePotentialReorg(ctx, header); err != nil {
		return err
	}

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

func (ets *TransactionSubmittedSyncer) syncRange(
	ctx context.Context,
	start,
	end uint64,
) error {
	events, err := ets.fetchEvents(ctx, start, end)
	if err != nil {
		return err
	}
	header, err := ets.ethClient.HeaderByNumber(ctx, new(big.Int).SetUint64(end))
	if err != nil {
		return errors.Wrap(err, "failed to get execution block header by number")
	}
	tx, err := ets.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ets.dbQuery.WithTx(tx)
	for _, event := range events {
		err := ets.txMapper.AddTransactionSubmittedEvent(ctx, tx, event)
		if err != nil {
			log.Err(err).Msg("err adding transaction submitted event")
			return err
		}
		log.Info().
			Uint64("block", event.Raw.BlockNumber).
			Hex("encrypted transaction (hex)", event.EncryptedTransaction).
			Msg("new encrypted transaction")
	}
	err = qtx.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockNumber: int64(end),
		BlockHash:   header.Hash().Bytes(),
	})
	if err != nil {
		log.Err(err).Msg("err adding transaction submit event sync until")
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Err(err).Msg("unable to commit db transaction")
		return err
	}
	log.Info().
		Uint64("start-block", start).
		Uint64("end-block", end).
		Int("num-inserted-events", len(events)).
		Msg("synced sequencer contract")
	return nil
}

func (s *TransactionSubmittedSyncer) fetchEvents(
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
