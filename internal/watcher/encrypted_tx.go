package watcher

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
)

type EncryptedTxWatcher struct {
	config             *metricsCommon.Config
	encryptedTxChannel chan *EncryptedTxReceivedEvent
	ethClient          *ethclient.Client
}

type EncryptedTxReceivedEvent struct {
	Tx   []byte
	Time time.Time
}

func NewEncryptedTxWatcher(config *metricsCommon.Config, encryptedTxChannel chan *EncryptedTxReceivedEvent) *EncryptedTxWatcher {
	return &EncryptedTxWatcher{
		config:             config,
		encryptedTxChannel: encryptedTxChannel,
	}
}

func (etw *EncryptedTxWatcher) Start(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		sequencerContract, err := sequencerBindings.NewSequencer(common.HexToAddress(etw.config.ContractAddress), etw.ethClient)

		if err != nil {
			errChan <- err
			return
		}

		ctx := context.Background()
		watchOpts := &bind.WatchOpts{Context: ctx, Start: nil}

		txSubmittedEventChannel := make(chan *sequencerBindings.SequencerTransactionSubmitted)

		sub, err := sequencerContract.WatchTransactionSubmitted(watchOpts, txSubmittedEventChannel)
		if err != nil {
			errChan <- err
			return
		}
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				errChan <- err
				return
			case event := <-txSubmittedEventChannel:
				ev := &EncryptedTxReceivedEvent{
					Tx:   event.EncryptedTransaction,
					Time: time.Now(),
				}
				etw.encryptedTxChannel <- ev
			case err := <-sub.Err():
				errChan <- err
				return
			}
		}
	}()

	err := <-errChan
	return err
}
