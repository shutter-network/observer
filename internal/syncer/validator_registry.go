package syncer

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
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

func (vts *ValidatorRegistrySyncer) Sync(ctx context.Context, header *types.Header) error {
	// TODO: handle reorgs
	syncedUntil, err := vts.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "failed to query validator registry sync status")
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
		return errors.Wrap(err, "failed to get execution block header by number")
	}
	for _, event := range events {
		err := ets.txMapper.AddValidatorRegistryEvent(ctx, event)
		if err != nil {
			log.Err(err).Msg("err adding validator registry updated event")
			return err
		}
		log.Info().
			Uint64("block", event.Raw.BlockNumber).
			Msg("new validator registry updated message")
	}

	err = ets.dbQuery.CreateValidatorRegistryEventsSyncedUntil(ctx, data.CreateValidatorRegistryEventsSyncedUntilParams{
		BlockNumber: int64(end),
		BlockHash:   header.Hash().Bytes(),
	})
	if err != nil {
		log.Err(err).Msg("err adding validator registry event sync until")
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
		return nil, errors.Wrap(err, "failed to query validator registry updated events")
	}
	events := []*validatorRegistryBindings.ValidatorregistryUpdated{}
	for it.Next() {
		events = append(events, it.Event)
	}
	if it.Error() != nil {
		return nil, errors.Wrap(it.Error(), "failed to iterate query validator registry updated events")
	}
	return events, nil
}
