package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

func (pmw *P2PMsgsWatcher) handleDecryptionKeyMsg(msg *p2pmsg.DecryptionKeys) ([]p2pmsg.Message, error) {
	t := time.Now()
	extra := msg.Extra.(*p2pmsg.DecryptionKeys_Gnosis).Gnosis
	pmw.decryptionDataChannel <- &DecryptionKeysEvent{
		Eon:        int64(msg.Eon),
		Keys:       msg.Keys,
		Slot:       int64(extra.Slot),
		InstanceID: int64(msg.InstanceID),
		TxPointer:  int64(extra.TxPointer),
	}

	ev, ok := pmw.getBlockFromSlot(int64(extra.Slot))
	if !ok {
		if mostRecentBlock, ok := pmw.recentBlocks[pmw.mostRecentBlock]; ok {
			mostRecentSlot := uint64(getSlotForBlock(mostRecentBlock.Header))
			if extra.Slot > mostRecentSlot+1 {
				log.Warn().
					Uint64("slot", extra.Slot).
					Uint64("expected-slot", mostRecentSlot+1).
					Uint64("most-recent-block", pmw.mostRecentBlock).
					Msg("received keys for a slot greater then expected slot")
			}
			log.Info().
				Uint64("slot", extra.Slot).
				Int("num-keys", len(msg.Keys)).
				Uint64("most-recent-block", pmw.mostRecentBlock).
				Uint64("most-recent-slot", mostRecentSlot).
				Msg("received keys for future slot")
		}
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

func (pmw *P2PMsgsWatcher) insertBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-pmw.blocksChannel:
			if !ok {
				return nil
			}
			err := pmw.insertBlock(ctx, ev)
			if err != nil {
				return err
			}
			pmw.clearOldBlocks(ev)
		}
	}
}

func (pmw *P2PMsgsWatcher) insertBlock(ctx context.Context, ev *BlockReceivedEvent) error {
	pmw.recentBlocksMux.Lock()
	defer pmw.recentBlocksMux.Unlock()
	pmw.recentBlocks[ev.Header.Number.Uint64()] = ev
	if ev.Header.Number.Uint64() > pmw.mostRecentBlock {
		pmw.mostRecentBlock = ev.Header.Number.Uint64()
	}

	err := pmw.txMapper.AddBlock(ctx, &data.Block{
		BlockHash:      ev.Header.Hash().Bytes(),
		BlockNumber:    ev.Header.Number.Int64(),
		BlockTimestamp: int64(ev.Header.Time),
		TxHash:         ev.Header.TxHash[:],
	})
	if err != nil {
		log.Err(err).Msg("err adding block")
	}
	return err
}

func (pmw *P2PMsgsWatcher) clearOldBlocks(latestEv *BlockReceivedEvent) {
	pmw.recentBlocksMux.Lock()
	defer pmw.recentBlocksMux.Unlock()

	tooOld := []uint64{}
	for block := range pmw.recentBlocks {
		if block < latestEv.Header.Number.Uint64()-100 {
			tooOld = append(tooOld, block)
		}
	}
	for _, block := range tooOld {
		delete(pmw.recentBlocks, block)
	}
}

func (pmw *P2PMsgsWatcher) getBlockFromSlot(slot int64) (*BlockReceivedEvent, bool) {
	pmw.recentBlocksMux.Lock()
	defer pmw.recentBlocksMux.Unlock()

	slotTimestamp := uint64(getSlotTimestamp(slot))
	if ev, ok := pmw.recentBlocks[pmw.mostRecentBlock]; ok {
		if ev.Header.Time == slotTimestamp {
			return ev, ok
		} else if ev.Header.Time < slotTimestamp {
			return nil, false
		}
	}

	for blockNumber := range pmw.recentBlocks {
		if ev, ok := pmw.recentBlocks[blockNumber]; ok {
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
