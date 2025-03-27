package watcher

import (
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

func (pmw *P2PMsgsWatcher) handleDecryptionKeyMsg(msg *p2pmsg.DecryptionKeys) ([]p2pmsg.Message, error) {
	extra := msg.Extra.(*p2pmsg.DecryptionKeys_Gnosis).Gnosis
	pmw.decryptionDataChannel <- &DecryptionKeysEvent{
		Eon:        int64(msg.Eon),
		Keys:       msg.Keys,
		Slot:       int64(extra.Slot),
		InstanceID: int64(msg.InstanceId),
		TxPointer:  int64(extra.TxPointer),
	}

	_, ok := pmw.blocksWatcher.getBlockHeaderFromSlot(extra.Slot)
	blocksWatcher := pmw.blocksWatcher
	if !ok {
		if mostRecentBlockHeader, ok := blocksWatcher.recentBlocks[blocksWatcher.mostRecentBlock]; ok {
			mostRecentSlot := uint64(utils.GetSlotForBlock(mostRecentBlockHeader.Time, GenesisTimestamp, SlotDuration))
			if extra.Slot > mostRecentSlot+1 {
				log.Warn().
					Uint64("slot", extra.Slot).
					Uint64("expected-slot", mostRecentSlot+1).
					Uint64("most-recent-block", blocksWatcher.mostRecentBlock).
					Msg("received keys for a slot greater than expected slot")
			}
			log.Info().
				Uint64("slot", extra.Slot).
				Int("num-keys", len(msg.Keys)).
				Uint64("most-recent-block", blocksWatcher.mostRecentBlock).
				Uint64("most-recent-slot", mostRecentSlot).
				Msg("received keys for future slot")
		}
		return []p2pmsg.Message{}, nil
	}

	log.Warn().
		Uint64("slot", extra.Slot).
		Int("num-keys", len(msg.Keys)).
		Msg("received keys for a known slot")
	return []p2pmsg.Message{}, nil
}

func getDecryptionKeysAndIdentities(p2pMsgs []*p2pmsg.Key) ([][]byte, [][]byte) {
	var keys [][]byte
	var identities [][]byte

	for _, msg := range p2pMsgs {
		keys = append(keys, msg.Key)
		identities = append(identities, msg.IdentityPreimage)
	}

	return keys, identities
}
