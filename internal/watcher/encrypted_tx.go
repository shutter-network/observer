package watcher

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type EncryptedTxWatcher struct {
	config                  *metricsCommon.Config
	txSubmittedEventChannel chan *sequencerBindings.SequencerTransactionSubmitted
	ethClient               *ethclient.Client
}

func NewEncryptedTxWatcher(config *metricsCommon.Config, txSubmittedEventChannel chan *sequencerBindings.SequencerTransactionSubmitted, ethClient *ethclient.Client) *EncryptedTxWatcher {
	return &EncryptedTxWatcher{
		config:                  config,
		txSubmittedEventChannel: txSubmittedEventChannel,
		ethClient:               ethClient,
	}
}

func (etw *EncryptedTxWatcher) Start(ctx context.Context, runner service.Runner) error {
	sequencerContract, err := sequencerBindings.NewSequencer(common.HexToAddress(etw.config.SequencerContractAddress), etw.ethClient)
	if err != nil {
		return err
	}
	watchOpts := &bind.WatchOpts{Context: ctx, Start: nil}
	sub, err := sequencerContract.WatchTransactionSubmitted(watchOpts, etw.txSubmittedEventChannel)
	if err != nil {
		return err
	}
	runner.Defer(sub.Unsubscribe)

	log.Debug().Msg("Successfully subscribed to TransactionSubmitted events")
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
