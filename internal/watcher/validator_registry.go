package watcher

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	metricsCommon "github.com/shutter-network/observer/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type ValidatorRegistryWatcher struct {
	config                   *metricsCommon.Config
	validatorRegistryChannel chan *validatorRegistryBindings.ValidatorregistryUpdated
	ethClient                *ethclient.Client
	startBlock               int64
}

func NewValidatorRegistryWatcher(
	config *metricsCommon.Config,
	validatorRegistryChannel chan *validatorRegistryBindings.ValidatorregistryUpdated,
	ethClient *ethclient.Client,
	startBlock int64,
) *ValidatorRegistryWatcher {
	return &ValidatorRegistryWatcher{
		config:                   config,
		validatorRegistryChannel: validatorRegistryChannel,
		ethClient:                ethClient,
		startBlock:               startBlock,
	}
}

func (vrw *ValidatorRegistryWatcher) Start(ctx context.Context, runner service.Runner) error {
	newValidatorRegistryUpdatesMsgs := make(chan *validatorRegistryBindings.ValidatorregistryUpdated)
	validatorRegistryContract, err := validatorRegistryBindings.NewValidatorregistry(common.HexToAddress(vrw.config.ValidatorRegistryContractAddress), vrw.ethClient)
	if err != nil {
		return err
	}

	//sync previous blocks which have not been processed yet
	runner.Go(func() error {
		err = vrw.syncPreviousBlocks(ctx, validatorRegistryContract)
		if err != nil {
			log.Err(err).Msg("err syncing previous blocks for validator registry")
			return err
		}
		return nil
	})

	watchOpts := &bind.WatchOpts{Context: ctx, Start: nil}
	sub, err := validatorRegistryContract.WatchUpdated(watchOpts, newValidatorRegistryUpdatesMsgs)
	if err != nil {
		return err
	}
	runner.Defer(sub.Unsubscribe)

	log.Debug().Msg("Successfully subscribed to validator register event")
	runner.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case vruMsg := <-newValidatorRegistryUpdatesMsgs:
				if vruMsg.Raw.BlockNumber > uint64(vrw.startBlock) {
					// process only if block greater then start block
					// since event have been or will be indexed by syncPreviousBlocks
					// till startBlock
					vrw.validatorRegistryChannel <- vruMsg
				}
			case err := <-sub.Err():
				return err
			}
		}
	})
	return nil
}

func (vrw *ValidatorRegistryWatcher) syncPreviousBlocks(
	ctx context.Context,
	validatorRegistryContract *validatorRegistryBindings.Validatorregistry,
) error {
	filterOpts := &bind.FilterOpts{Context: ctx, Start: uint64(vrw.startBlock)}
	events, err := validatorRegistryContract.FilterUpdated(filterOpts)
	if err != nil {
		return err
	}

	for events.Next() {
		event := events.Event
		vrw.validatorRegistryChannel <- event
	}

	if events.Error() != nil {
		return errors.Wrap(events.Error(), "failed to iterate validator registry updated events")
	}

	return nil
}
