package watcher

import (
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2pmsg"
)

func (dkw *P2PMsgsWatcher) handleKeyShareMsg(msg *p2pmsg.DecryptionKeyShares) ([]p2pmsg.Message, error) {
	extra := msg.Extra.(*p2pmsg.DecryptionKeyShares_Gnosis).Gnosis

	dkw.keyShareChannel <- &KeyShareEvent{
		Shares: msg.Shares,
		Slot:   extra.Slot,
	}

	log.Info().
		Uint64("slot", extra.Slot).
		Int("num-shares", len(msg.Shares)).
		Msg("received key shares")
	return []p2pmsg.Message{}, nil
}
