package syncer

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley"
)

type ValidatorRegistrySyncer struct {
	contract             *validatorRegistryBindings.Validatorregistry
	db                   *pgxpool.Pool
	dbQuery              *data.Queries
	ethClient            *ethclient.Client
	txMapper             metrics.TxMapper
	syncStartBlockNumber uint64
}

func NewValidatorRegistrySyncer(
	contract *validatorRegistryBindings.Validatorregistry,
	db *pgxpool.Pool,
	ethClient *ethclient.Client,
	txMapper metrics.TxMapper,
	syncStartBlockNumber uint64,
) *ValidatorRegistrySyncer {
	return &ValidatorRegistrySyncer{
		contract:             contract,
		db:                   db,
		dbQuery:              data.New(db),
		ethClient:            ethClient,
		txMapper:             txMapper,
		syncStartBlockNumber: syncStartBlockNumber,
	}
}

func getNumReorgedBlocksForValidatorRegistrations(syncedUntil *data.QueryValidatorRegistryEventsSyncedUntilRow, header *types.Header) int {
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
func (vts *ValidatorRegistrySyncer) resetSyncStatus(ctx context.Context, numReorgedBlocks int) error {
	if numReorgedBlocks == 0 {
		return nil
	}

	tx, err := vts.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := vts.dbQuery.WithTx(tx)

	syncStatus, err := qtx.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil {
		return fmt.Errorf("failed to query sync status from db in order to reset it, %w", err)
	}
	if syncStatus.BlockNumber < int64(numReorgedBlocks) {
		return fmt.Errorf("detected reorg deeper (%d) than blocks synced (%d)", syncStatus.BlockNumber, numReorgedBlocks)
	}

	deleteFromInclusive := syncStatus.BlockNumber - int64(numReorgedBlocks) + 1

	err = qtx.DeleteValidatorRegistrationMessageFromBlockNumber(ctx, deleteFromInclusive)
	if err != nil {
		return fmt.Errorf("failed to delete validator registration event from db, %w", err)
	}

	// Currently, we don't have enough information in the db to populate block hash and slot.
	// However, using default values here is fine since the syncer is expected to resync
	// immediately after this function call which will set the correct values. When we do proper
	// reorg handling, we should store the full block data of the previous blocks so that we can
	// avoid this.
	newSyncedUntilBlockNumber := deleteFromInclusive - 1
	err = qtx.CreateValidatorRegistryEventsSyncedUntil(ctx, data.CreateValidatorRegistryEventsSyncedUntilParams{
		BlockHash:   []byte{},
		BlockNumber: newSyncedUntilBlockNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to reset validator registration event sync status in db, %w", err)
	}
	log.Info().
		Int("depth", numReorgedBlocks).
		Int64("previous-synced-until", syncStatus.BlockNumber).
		Int64("new-synced-until", newSyncedUntilBlockNumber).
		Msg("sync status reset due to reorg")
	return nil
}

func (vts *ValidatorRegistrySyncer) handlePotentialReorg(ctx context.Context, header *types.Header) error {
	syncedUntil, err := vts.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query validator registry events sync status, %w", err)
	}

	numReorgedBlocks := getNumReorgedBlocksForValidatorRegistrations(&syncedUntil, header)
	if numReorgedBlocks > 0 {
		return vts.resetSyncStatus(ctx, numReorgedBlocks)
	}
	return nil
}

func (vts *ValidatorRegistrySyncer) Sync(ctx context.Context, header *types.Header) error {
	if err := vts.handlePotentialReorg(ctx, header); err != nil {
		return err
	}

	syncedUntil, err := vts.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("failed to query validator registry sync status, %v", err)
	}
	var start uint64
	if err == pgx.ErrNoRows {
		start = vts.syncStartBlockNumber
	} else {
		start = uint64(syncedUntil.BlockNumber + 1)
	}
	endBlock := header.Number.Uint64()
	log.Debug().
		Uint64("start-block", start).
		Uint64("end-block", endBlock).
		Msg("syncing validator registry updated events")
	syncRanges := medley.GetSyncRanges(start, endBlock, maxRequestBlockRange)
	for _, r := range syncRanges {
		err = vts.syncRange(ctx, r[0], r[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func (ets *ValidatorRegistrySyncer) syncRange(
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
		return fmt.Errorf("failed to get execution block header by number, %v", err)
	}
	tx, err := ets.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ets.dbQuery.WithTx(tx)

	for _, event := range events {
		err := ets.txMapper.AddValidatorRegistryEvent(ctx, tx, event)
		if err != nil {
			log.Err(err).Msg("err adding validator registry updated event")
			return err
		}
		log.Info().
			Uint64("block", event.Raw.BlockNumber).
			Msg("new validator registry updated message")
	}

	err = qtx.CreateValidatorRegistryEventsSyncedUntil(ctx, data.CreateValidatorRegistryEventsSyncedUntilParams{
		BlockNumber: int64(end),
		BlockHash:   header.Hash().Bytes(),
	})
	if err != nil {
		log.Err(err).Msg("err adding validator registry event sync until")
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
		Msg("synced validator registry contract")
	return nil
}

func (s *ValidatorRegistrySyncer) fetchEvents(
	ctx context.Context,
	start,
	end uint64,
) ([]*validatorRegistryBindings.ValidatorregistryUpdated, error) {
	opts := bind.FilterOpts{
		Start:   start,
		End:     &end,
		Context: ctx,
	}
	it, err := s.contract.ValidatorregistryFilterer.FilterUpdated(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query validator registry updated events, %v", err)
	}
	events := []*validatorRegistryBindings.ValidatorregistryUpdated{}
	for it.Next() {
		events = append(events, it.Event)
	}
	if it.Error() != nil {
		return nil, fmt.Errorf("failed to iterate query validator registry updated events, %v", it.Error())
	}
	return events, nil
}
