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

type DecKeysAndMessages struct {
	Eon        int64
	Keys       [][]byte
	Identities [][]byte
	Slot       int64
	InstanceID int64
	TxPointer  int64
}

type TxMapper interface {
	AddTransactionSubmittedEvent(ctx context.Context, tse *data.TransactionSubmittedEvent) error
	AddDecryptionKeysAndMessages(
		ctx context.Context,
		dkam *DecKeysAndMessages,
	) error
	AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error
	AddBlock(
		ctx context.Context,
		b *data.Block,
	) error
}
