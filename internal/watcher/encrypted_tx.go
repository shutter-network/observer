package watcher

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/identitypreimage"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type EncryptedTxWatcher struct {
	config             *metricsCommon.Config
	encryptedTxChannel chan *EncryptedTxReceivedEvent
	ethClient          *ethclient.Client
}

type EncryptedTxReceivedEvent struct {
	IdentityPrefix [32]byte
	Sender         common.Address
	Tx             []byte
	Time           time.Time
}

func NewEncryptedTxWatcher(config *metricsCommon.Config, encryptedTxChannel chan *EncryptedTxReceivedEvent, ethClient *ethclient.Client) *EncryptedTxWatcher {
	return &EncryptedTxWatcher{
		config:             config,
		encryptedTxChannel: encryptedTxChannel,
		ethClient:          ethClient,
	}
}

func (etw *EncryptedTxWatcher) Start(ctx context.Context, runner service.Runner) error {
	runner.Go(func() error {
		sequencerContract, err := sequencerBindings.NewSequencer(common.HexToAddress(etw.config.ContractAddress), etw.ethClient)
		if err != nil {
			return err
		}

		ctx := context.Background()
		watchOpts := &bind.WatchOpts{Context: ctx, Start: nil}

		txSubmittedEventChannel := make(chan *sequencerBindings.SequencerTransactionSubmitted)

		sub, err := sequencerContract.WatchTransactionSubmitted(watchOpts, txSubmittedEventChannel)
		if err != nil {
			return err
		}
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return err
			case event := <-txSubmittedEventChannel:
				ev := &EncryptedTxReceivedEvent{
					IdentityPrefix: event.IdentityPrefix,
					Sender:         event.Sender,
					Tx:             event.EncryptedTransaction,
					Time:           time.Now(),
				}
				etw.encryptedTxChannel <- ev
			case err := <-sub.Err():
				return err
			}
		}
	})
	return nil
}

func computeIdentityPreimage(identityPrefix []byte, sender common.Address) []byte {
	imageBytes := append(identityPrefix, sender.Bytes()...)
	epochID := shcrypto.ComputeEpochID(identitypreimage.IdentityPreimage(imageBytes).Bytes())
	return epochID.Marshal()
}
