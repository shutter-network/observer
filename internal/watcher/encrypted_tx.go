package watcher

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
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
	runner.Go(func() error {
		sequencerContract, err := sequencerBindings.NewSequencer(common.HexToAddress(etw.config.ContractAddress), etw.ethClient)
		if err != nil {
			return err
		}
		watchOpts := &bind.WatchOpts{Context: ctx, Start: nil}

		sub, err := sequencerContract.WatchTransactionSubmitted(watchOpts, etw.txSubmittedEventChannel)
		if err != nil {
			return err
		}
		defer sub.Unsubscribe()
		return nil
	})
	return nil
}
