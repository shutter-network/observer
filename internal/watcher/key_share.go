package watcher

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

type KeyShareWatcher struct {
	config          *common.Config
	keyShareChannel chan *KeyShareEvent
}

type KeyShareEvent struct {
	Shares []*p2pmsg.KeyShare
	Slot   uint64
}

func NewKeyShareWatcher(config *common.Config, keyShareChannel chan *KeyShareEvent) *KeyShareWatcher {
	return &KeyShareWatcher{
		config:          config,
		keyShareChannel: keyShareChannel,
	}
}

func (ksw *KeyShareWatcher) Start(ctx context.Context, runner service.Runner) error {
	p2pService, err := p2p.New(ksw.config.P2P)
	if err != nil {
		return err
	}
	p2pService.AddMessageHandler(ksw)

	return runner.StartService(p2pService)
}

func (*KeyShareWatcher) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{&p2pmsg.DecryptionKeyShares{}}
}

func (ksw *KeyShareWatcher) ValidateMessage(_ context.Context, _ p2pmsg.Message) (pubsub.ValidationResult, error) {
	return pubsub.ValidationAccept, nil
}

func (ksw *KeyShareWatcher) HandleMessage(ctx context.Context, msgUntyped p2pmsg.Message) ([]p2pmsg.Message, error) {
	// t := time.Now()
	msg := msgUntyped.(*p2pmsg.DecryptionKeyShares)
	extra := msg.Extra.(*p2pmsg.DecryptionKeyShares_Gnosis).Gnosis

	ksw.keyShareChannel <- &KeyShareEvent{
		Shares: msg.Shares,
		Slot:   extra.Slot,
	}

	log.Info().
		Uint64("slot", extra.Slot).
		Int("num-shares", len(msg.Shares)).
		Msg("received key shares")
	return []p2pmsg.Message{}, nil
}
