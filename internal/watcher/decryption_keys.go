package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common/utils"
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

	ev, ok := pmw.getBlockReceivedEventFromSlot(extra.Slot)
	if !ok {
		if mostRecentBlock, ok := pmw.recentBlocks[pmw.mostRecentBlock]; ok {
			mostRecentSlot := uint64(utils.GetSlotForBlock(mostRecentBlock.Header.Time, GenesisTimestamp, SlotDuration))
			if extra.Slot > mostRecentSlot+1 {
				log.Warn().
					Uint64("slot", extra.Slot).
					Uint64("expected-slot", mostRecentSlot+1).
					Uint64("most-recent-block", pmw.mostRecentBlock).
					Msg("received keys for a slot greater than expected slot")
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
		log.Info().Msg("polling insertBlocks, for new head channel")
		select {
		case <-ctx.Done():
			log.Info().Msg("context cancelled for insertBlocks, no new head")
			return ctx.Err()
		case ev, ok := <-pmw.blocksChannel:
			if !ok {
				log.Info().Msg("returning NIL from insertBlocks, no new head")
				return nil
			}
			log.Info().
				Hex("block-hash", ev.Header.Hash().Bytes()).
				Int64("block-number", ev.Header.Number.Int64()).
				Msg("planning to insertBlock, with new head")
			err := pmw.insertBlock(ctx, ev)
			if err != nil {
				log.Info().Err(err).Msg("error in insertBlock, no new head")
				return err
			}
			log.Info().Msg("calling ClearOldBlocks, before new head")
			pmw.clearOldBlocks(ev)
		}
	}
}

func (pmw *P2PMsgsWatcher) insertBlock(ctx context.Context, ev *BlockReceivedEvent) error {
	log.Info().
		Hex("block-hash", ev.Header.Hash().Bytes()).
		Int64("block-number", ev.Header.Number.Int64()).
		Msg("trying to obtain lock for insertBlock new head")
	pmw.recentBlocksMux.Lock()
	log.Info().
		Hex("block-hash", ev.Header.Hash().Bytes()).
		Int64("block-number", ev.Header.Number.Int64()).
		Msg("obtained lock for insertBlock new head")
	defer pmw.recentBlocksMux.Unlock()
	pmw.recentBlocks[ev.Header.Number.Uint64()] = ev
	if ev.Header.Number.Uint64() > pmw.mostRecentBlock {
		pmw.mostRecentBlock = ev.Header.Number.Uint64()
	}

	err := pmw.txMapper.AddBlock(ctx, &data.Block{
		BlockHash:      ev.Header.Hash().Bytes(),
		BlockNumber:    ev.Header.Number.Int64(),
		BlockTimestamp: int64(ev.Header.Time),
		Slot:           int64(utils.GetSlotForBlock(ev.Header.Time, GenesisTimestamp, SlotDuration)),
	})
	if err != nil {
		log.Err(err).Msg("err adding block")
	}
	return err
}

func (pmw *P2PMsgsWatcher) clearOldBlocks(latestEv *BlockReceivedEvent) {
	log.Info().
		Hex("block-hash", latestEv.Header.Hash().Bytes()).
		Int64("block-number", latestEv.Header.Number.Int64()).
		Msg("trying to obtain lock for clearOldBlocks new head")
	pmw.recentBlocksMux.Lock()
	log.Info().
		Hex("block-hash", latestEv.Header.Hash().Bytes()).
		Int64("block-number", latestEv.Header.Number.Int64()).
		Msg("obtained lock for clearOldBlocks new head")
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

func (pmw *P2PMsgsWatcher) getBlockReceivedEventFromSlot(slot uint64) (*BlockReceivedEvent, bool) {
	pmw.recentBlocksMux.Lock()
	defer pmw.recentBlocksMux.Unlock()

	slotTimestamp := utils.GetSlotTimestamp(slot, GenesisTimestamp, SlotDuration)
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

func getDecryptionKeysAndIdentities(p2pMsgs []*p2pmsg.Key) ([][]byte, [][]byte) {
	var keys [][]byte
	var identities [][]byte

	for _, msg := range p2pMsgs {
		keys = append(keys, msg.Key)
		identities = append(identities, msg.Identity)
	}

	return keys, identities
}
