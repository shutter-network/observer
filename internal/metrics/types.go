package metrics

import (
	"context"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type DecryptionData struct {
	Key  []byte
	Slot int64
}

type KeyShare struct {
	Share []byte
	Slot  int64
}

type Tx struct {
	EncryptedTx []byte
	DD          *DecryptionData
	KS          *KeyShare
	BlockHash   []byte
}

type TxMapper interface {
	AddTransactionSubmittedEvent(ctx context.Context, tse *data.TransactionSubmittedEvent) error
	AddDecryptionKeyAndMessage(
		ctx context.Context,
		dk *data.DecryptionKey,
		dkm *data.DecryptionKeysMessage,
		dkmdk *data.DecryptionKeysMessageDecryptionKey,
	) error
	AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error
	AddBlock(
		ctx context.Context,
		b *data.Block,
	) error
}
