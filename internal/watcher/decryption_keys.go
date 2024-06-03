package watcher

import (
	"context"

	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type DecryptionKeysWatcher struct {
	config        *common.Config
	blocksChannel chan *BlockReceivedEvent
}

func NewDecryptionKeysWatcher(config *common.Config, blocksChannel chan *BlockReceivedEvent) *DecryptionKeysWatcher {
	return &DecryptionKeysWatcher{
		config:        config,
		blocksChannel: blocksChannel,
	}
}

func (w *DecryptionKeysWatcher) Start(ctx context.Context, runner service.Runner) error {
	p2pService, err := p2p.New(w.config.P2P)
	if err != nil {
		return err
	}
	p2pService.AddMessageHandler(w)

	return runner.StartService(p2pService)
}
