package watcher

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type Watcher struct {
	config *common.Config
}

func New(config *common.Config) *Watcher {
	return &Watcher{
		config: config,
	}
}

func (w *Watcher) Start(_ context.Context, runner service.Runner) error {
	txMapper := metrics.NewTxMapper()
	encryptedTxChannel := make(chan *EncryptedTxReceivedEvent)
	blocksChannel := make(chan *BlockReceivedEvent)
	decryptionDataChannel := make(chan *DecryptionKeysEvent)

	ethClient, err := ethclient.Dial(w.config.RpcURL)
	if err != nil {
		return err
	}

	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel, ethClient)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, encryptedTxChannel, ethClient)
	decryptionKeysWatcher := NewDecryptionKeysWatcher(w.config, blocksChannel, decryptionDataChannel)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher, decryptionKeysWatcher); err != nil {
		return err
	}

	for {
		select {
		//TODO: clean memory if !txMapper.CanBeDecrypted for more then 30 mins
		case enTx := <-encryptedTxChannel:
			identityPreimage := computeIdentityPreimage(enTx.IdentityPrefix[:], enTx.Sender)
			txMapper.AddEncryptedTx(identityPreimage, enTx.Tx)
			log.Info().
				Bytes("encrypted transaction", enTx.Tx).
				Msg("new encrypted transaction")

		case dd := <-decryptionDataChannel:
			for _, key := range dd.Keys {
				txMapper.AddDecryptionData(string(key.Identity), &metrics.DecryptionData{
					Key:  key.Key,
					Slot: dd.Slot,
				})
				log.Info().
					Bytes("decryption keys", key.Key).
					Uint64("slot", dd.Slot).
					Msg("new decryption key")
			}
		}
	}
}
