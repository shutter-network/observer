package watcher

import (
	"context"
	"fmt"

	"github.com/shutter-network/gnosh-metrics/common"
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
	encryptedTxChannel := make(chan *EncryptedTxReceivedEvent)
	blocksChannel := make(chan *BlockReceivedEvent)

	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, encryptedTxChannel)
	// decryptionKeysWatcher := NewDecryptionKeysWatcher(w.config, blocksChannel)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher); err != nil {
		return err
	}

	for {
		select {
		case block := <-blocksChannel:
			fmt.Println("blocks", block)
		case enTx := <-encryptedTxChannel:
			fmt.Println("transactions", enTx)
		}
	}
}
