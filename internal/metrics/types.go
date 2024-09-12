package metrics

import (
	"context"

	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
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

type DecKeyAndMessage struct {
	Slot             int64
	TxPointer        int64
	Eon              int64
	Key              []byte
	IdentityPreimage []byte
	KeyIndex         int64
	DecryptionKeyID  int64
}

type TxExecution struct {
	BlockNumber        int64
	DecKeysAndMessages []*DecKeyAndMessage
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
	BlockExists(ctx context.Context, blockNumber int64) (bool, error)
	QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error)
	AddValidatorRegistryEvent(ctx context.Context, vr *validatorRegistryBindings.ValidatorregistryUpdated) error
	UpdateValidatorStatus(ctx context.Context) error
	AddProposerDuties(ctx context.Context, epoch uint64) error
}
