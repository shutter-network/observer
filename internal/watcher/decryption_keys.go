package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

type DecryptionKeysWatcher struct {
	config                *common.Config
	blocksChannel         chan *BlockReceivedEvent
	decryptionDataChannel chan *DecryptionKeysEvent

	recentBlocksMux sync.Mutex
	recentBlocks    map[uint64]*BlockReceivedEvent
	mostRecentBlock uint64
}

type DecryptionKeysEvent struct {
	Keys []*p2pmsg.Key
	Slot uint64
}

func NewDecryptionKeysWatcher(config *common.Config, blocksChannel chan *BlockReceivedEvent, decryptionDataChannel chan *DecryptionKeysEvent) *DecryptionKeysWatcher {
	return &DecryptionKeysWatcher{
		config:                config,
		blocksChannel:         blocksChannel,
		decryptionDataChannel: decryptionDataChannel,
		recentBlocksMux:       sync.Mutex{},
		recentBlocks:          make(map[uint64]*BlockReceivedEvent),
		mostRecentBlock:       0,
	}
}

func (dkw *DecryptionKeysWatcher) Start(ctx context.Context, runner service.Runner) error {
	p2pService, err := p2p.New(dkw.config.P2P)
	if err != nil {
		return err
	}
	p2pService.AddMessageHandler(dkw)

	runner.Go(func() error { return dkw.insertBlocks(ctx) })

	return runner.StartService(p2pService)
}

func (dkw *DecryptionKeysWatcher) MessagePrototypes() []p2pmsg.Message {
	return []p2pmsg.Message{
		&p2pmsg.DecryptionKeys{},
	}
}

func (dkw *DecryptionKeysWatcher) ValidateMessage(_ context.Context, _ p2pmsg.Message) (pubsub.ValidationResult, error) {
	return pubsub.ValidationAccept, nil
}

func (dkw *DecryptionKeysWatcher) HandleMessage(_ context.Context, msgUntyped p2pmsg.Message) ([]p2pmsg.Message, error) {
	t := time.Now()
	msg := msgUntyped.(*p2pmsg.DecryptionKeys)
	extra := msg.Extra.(*p2pmsg.DecryptionKeys_Gnosis).Gnosis

	dkw.decryptionDataChannel <- &DecryptionKeysEvent{
		Keys: msg.Keys,
		Slot: extra.Slot,
	}

	ev, ok := dkw.getRecentBlock(extra.Slot)
	if !ok {
		log.Warn().
			Uint64("keys-block", extra.Slot).
			Uint64("most-recent-block", dkw.mostRecentBlock).
			Msg("received keys for unknown block")
		return []p2pmsg.Message{}, nil
	}

	dt := t.Sub(ev.Time)
	log.Info().
		Uint64("block", extra.Slot).
		Int("num-keys", len(msg.Keys)).
		Str("latency", fmt.Sprintf("%.2fs", dt.Seconds())).
		Msg("new keys")
	return []p2pmsg.Message{}, nil
}

func (dkw *DecryptionKeysWatcher) insertBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-dkw.blocksChannel:
			if !ok {
				return nil
			}
			dkw.insertBlock(ev)
			dkw.clearOldBlocks(ev)
		}
	}
}

func (dkw *DecryptionKeysWatcher) insertBlock(ev *BlockReceivedEvent) {
	dkw.recentBlocksMux.Lock()
	defer dkw.recentBlocksMux.Unlock()
	dkw.recentBlocks[ev.Header.Number.Uint64()] = ev
	if ev.Header.Number.Uint64() > dkw.mostRecentBlock {
		dkw.mostRecentBlock = ev.Header.Number.Uint64()
	}
}

func (dkw *DecryptionKeysWatcher) clearOldBlocks(latestEv *BlockReceivedEvent) {
	dkw.recentBlocksMux.Lock()
	defer dkw.recentBlocksMux.Unlock()

	tooOld := []uint64{}
	for block := range dkw.recentBlocks {
		if block < latestEv.Header.Number.Uint64()-100 {
			tooOld = append(tooOld, block)
		}
	}
	for _, block := range tooOld {
		delete(dkw.recentBlocks, block)
	}
}

func (dkw *DecryptionKeysWatcher) getRecentBlock(blockNumber uint64) (*BlockReceivedEvent, bool) {
	dkw.recentBlocksMux.Lock()
	defer dkw.recentBlocksMux.Unlock()
	ev, ok := dkw.recentBlocks[blockNumber]
	return ev, ok
}
