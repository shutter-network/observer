package watcher

import (
	"context"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

type P2PMsgsWatcher struct {
	config                *common.Config
	blocksChannel         chan *BlockReceivedEvent
	decryptionDataChannel chan *DecryptionKeysEvent
	keyShareChannel       chan *KeyShareEvent

	recentBlocksMux sync.Mutex
	recentBlocks    map[uint64]*BlockReceivedEvent
	mostRecentBlock uint64
}

type DecryptionKeysEvent struct {
	Eon  int64
	Keys []*p2pmsg.Key
	Slot int64
}

type KeyShareEvent struct {
	Eon         int64
	KeyperIndex int64
	Shares      []*p2pmsg.KeyShare
	Slot        int64
}

func NewP2PMsgsWatcherWatcher(
	config *common.Config,
	blocksChannel chan *BlockReceivedEvent,
	decryptionDataChannel chan *DecryptionKeysEvent,
	keyShareChannel chan *KeyShareEvent,
) *P2PMsgsWatcher {
	return &P2PMsgsWatcher{
		config:                config,
		blocksChannel:         blocksChannel,
		decryptionDataChannel: decryptionDataChannel,
		keyShareChannel:       keyShareChannel,
		recentBlocksMux:       sync.Mutex{},
		recentBlocks:          make(map[uint64]*BlockReceivedEvent),
		mostRecentBlock:       0,
	}
}

func (dkw *P2PMsgsWatcher) Start(ctx context.Context, runner service.Runner) error {
	p2pService, err := p2p.New(dkw.config.P2P)
	if err != nil {
		return err
	}
	p2pService.AddMessageHandler(dkw)

	runner.Go(func() error { return dkw.insertBlocks(ctx) })

	return runner.StartService(p2pService)
}

func (dkw *P2PMsgsWatcher) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKeys{},
		&p2pmsg.DecryptionKeyShares{},
	}
}

func (dkw *P2PMsgsWatcher) ValidateMessage(_ context.Context, _ p2pmsg.Message) (pubsub.ValidationResult, error) {
	return pubsub.ValidationAccept, nil
}

func (dkw *P2PMsgsWatcher) HandleMessage(ctx context.Context, msgUntyped p2pmsg.Message) ([]p2pmsg.Message, error) {
	switch msg := msgUntyped.(type) {
	case *p2pmsg.DecryptionKeys:
		return dkw.handleDecryptionKeyMsg(msg)
	case *p2pmsg.DecryptionKeyShares:
		return dkw.handleKeyShareMsg(msg)
	}
	return []p2pmsg.Message{}, nil
}
