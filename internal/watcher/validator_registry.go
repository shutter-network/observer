package watcher

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type ValidatorRegisteryWatcher struct {
	config                   *metricsCommon.Config
	validatorRegistryChannel chan *validatorRegistryBindings.ValidatorregistryUpdated
	ethClient                *ethclient.Client
	startBlock               *uint64
}

func NewValidatorRegisteryWatcher(
	config *metricsCommon.Config,
	validatorRegistryChannel chan *validatorRegistryBindings.ValidatorregistryUpdated,
	ethClient *ethclient.Client,
	startBlock *uint64,
) *ValidatorRegisteryWatcher {
	return &ValidatorRegisteryWatcher{
		config:                   config,
		validatorRegistryChannel: validatorRegistryChannel,
		ethClient:                ethClient,
		startBlock:               startBlock,
	}
}

func (etw *ValidatorRegisteryWatcher) Start(ctx context.Context, runner service.Runner) error {
	validatorRegistryContract, err := validatorRegistryBindings.NewValidatorregistry(common.HexToAddress(etw.config.ContractAddress), etw.ethClient)
	if err != nil {
		return err
	}
	watchOpts := &bind.WatchOpts{Context: ctx, Start: etw.startBlock}
	sub, err := validatorRegistryContract.WatchUpdated(watchOpts, etw.validatorRegistryChannel)
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
			case err := <-sub.Err():
				return err
			}
		}
	})
	return nil
}
