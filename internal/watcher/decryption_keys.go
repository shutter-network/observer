package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

func (dkw *P2PMsgsWatcher) handleDecryptionKeyMsg(msg *p2pmsg.DecryptionKeys) ([]p2pmsg.Message, error) {
	t := time.Now()

	extra := msg.Extra.(*p2pmsg.DecryptionKeys_Gnosis).Gnosis
	if extra == nil {
		log.Warn().
			Int("num-keys", len(msg.Keys)).
			Uint64("most-recent-block", dkw.mostRecentBlock).
			Msg("received keys without any slot")
		return []p2pmsg.Message{}, nil
	}
	dkw.decryptionDataChannel <- &DecryptionKeysEvent{
		Eon:  int64(msg.Eon),
		Keys: msg.Keys,
		Slot: int64(extra.Slot),
	}

	ev, ok := dkw.getBlockFromSlot(int64(extra.Slot))
	if !ok {
		mostRecentBlock := dkw.recentBlocks[dkw.mostRecentBlock]
		mostRecentSlot := uint64(getSlotForBlock(mostRecentBlock.Header))

		if extra.Slot > mostRecentSlot+1 {
			log.Warn().
				Uint64("slot", extra.Slot).
				Uint64("expected-slot", mostRecentSlot+1).
				Uint64("most-recent-block", dkw.mostRecentBlock).
				Msg("received keys for a slot greater then expected slot")
		}
		log.Info().
			Uint64("slot", extra.Slot).
			Int("num-keys", len(msg.Keys)).
			Uint64("most-recent-block", dkw.mostRecentBlock).
			Uint64("most-recent-slot", mostRecentSlot).
			Msg("received keys for future slot")
		return []p2pmsg.Message{}, nil
	}

	dt := t.Sub(ev.Time)
	log.Warn().
		Uint64("slot", extra.Slot).
		Int("num-keys", len(msg.Keys)).
		Str("latency", fmt.Sprintf("%.2fs", dt.Seconds())).
		Msg("received keys for a known slot")
	return []p2pmsg.Message{}, nil
}

func (dkw *P2PMsgsWatcher) insertBlocks(ctx context.Context) error {
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

func (dkw *P2PMsgsWatcher) insertBlock(ev *BlockReceivedEvent) {
	dkw.recentBlocksMux.Lock()
	defer dkw.recentBlocksMux.Unlock()
	dkw.recentBlocks[ev.Header.Number.Uint64()] = ev
	if ev.Header.Number.Uint64() > dkw.mostRecentBlock {
		dkw.mostRecentBlock = ev.Header.Number.Uint64()
	}
}

func (dkw *P2PMsgsWatcher) clearOldBlocks(latestEv *BlockReceivedEvent) {
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

func (dkw *P2PMsgsWatcher) getBlockFromSlot(slot int64) (*BlockReceivedEvent, bool) {
	dkw.recentBlocksMux.Lock()
	defer dkw.recentBlocksMux.Unlock()

	slotTimestamp := uint64(getSlotTimestamp(slot))
	if ev, ok := dkw.recentBlocks[dkw.mostRecentBlock]; ok {
		if ev.Header.Time == slotTimestamp {
			return ev, ok
		} else if ev.Header.Time < slotTimestamp {
			return nil, false
		}
	}

	for blockNumber := range dkw.recentBlocks {
		if ev, ok := dkw.recentBlocks[blockNumber]; ok {
			if ev.Header.Time == slotTimestamp {
				return ev, ok
			}
		}
	}

	return nil, false
}

func getSlotTimestamp(slot int64) int64 {
	return GenesisTimestamp + (slot)*SlotDuration
}

func getSlotForBlock(blockHeader *types.Header) int64 {
	return (int64(blockHeader.Time) - GenesisTimestamp) / SlotDuration
}
