package watcher

import (
	"context"
	"math"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

type P2PMsgsWatcher struct {
	config                *common.Config
	decryptionDataChannel chan *DecryptionKeysEvent
	keyShareChannel       chan *KeyShareEvent
	blocksWatcher         *BlocksWatcher
}

type DecryptionKeysEvent struct {
	Eon        int64
	Keys       []*p2pmsg.Key
	Slot       int64
	InstanceID int64
	TxPointer  int64
}

type KeyShareEvent struct {
	Eon         int64
	KeyperIndex int64
	Shares      []*p2pmsg.KeyShare
	Slot        int64
}

func NewP2PMsgsWatcherWatcher(
	config *common.Config,
	decryptionDataChannel chan *DecryptionKeysEvent,
	keyShareChannel chan *KeyShareEvent,
	blocksWatcher *BlocksWatcher,
) *P2PMsgsWatcher {
	return &P2PMsgsWatcher{
		config:                config,
		decryptionDataChannel: decryptionDataChannel,
		keyShareChannel:       keyShareChannel,
		blocksWatcher:         blocksWatcher,
	}
}

func (pmw *P2PMsgsWatcher) Start(ctx context.Context, runner service.Runner) error {
	p2pService, err := p2p.New(pmw.config.P2P)
	if err != nil {
		return err
	}
	p2pService.AddMessageHandler(pmw)

	return runner.StartService(p2pService)
}

func (pmw *P2PMsgsWatcher) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKeys{},
		&p2pmsg.DecryptionKeyShares{},
	}
}

func (pmw *P2PMsgsWatcher) ValidateMessage(_ context.Context, msgUntyped p2pmsg.Message) (pubsub.ValidationResult, error) {
	switch msg := msgUntyped.(type) {
	case *p2pmsg.DecryptionKeys:
		extra := msg.Extra.(*p2pmsg.DecryptionKeys_Gnosis).Gnosis
		if extra == nil {
			log.Warn().
				Int("num-keys", len(msg.Keys)).
				Uint64("most-recent-block", pmw.blocksWatcher.mostRecentBlock).
				Msg("received DecryptionKeys without any slot")
			return pubsub.ValidationReject, nil
		}
		if msg.Eon > math.MaxInt64 {
			return pubsub.ValidationReject, errors.Errorf("eon %d overflows int64", msg.Eon)
		}
		if len(msg.Keys) == 0 {
			return pubsub.ValidationReject, errors.New("no keys in message")
		}
	case *p2pmsg.DecryptionKeyShares:
		extra := msg.Extra.(*p2pmsg.DecryptionKeyShares_Gnosis).Gnosis
		if extra == nil {
			log.Warn().
				Int("num-keyshares", len(msg.Shares)).
				Uint64("most-recent-block", pmw.blocksWatcher.mostRecentBlock).
				Msg("received DecryptionKeyShares without any slot")
			return pubsub.ValidationReject, nil
		}
	}
	return pubsub.ValidationAccept, nil
}

func (pmw *P2PMsgsWatcher) HandleMessage(ctx context.Context, msgUntyped p2pmsg.Message) ([]p2pmsg.Message, error) {
	switch msg := msgUntyped.(type) {
	case *p2pmsg.DecryptionKeys:
		return pmw.handleDecryptionKeyMsg(msg)
	case *p2pmsg.DecryptionKeyShares:
		return pmw.handleKeyShareMsg(msg)
	}
	return []p2pmsg.Message{}, nil
}
