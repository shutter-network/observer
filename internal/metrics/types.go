package metrics

import (
	"context"

	"github.com/jackc/pgx/v5"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	"github.com/shutter-network/observer/internal/data"
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
	// BlockNumber        int64
	DecKeysAndMessages []*DecKeyAndMessage
}

type TxMapper interface {
	AddTransactionSubmittedEvent(ctx context.Context, tx pgx.Tx, st *sequencerBindings.SequencerTransactionSubmitted) (int64, error)
	AddDecryptionKeysAndMessages(
		ctx context.Context,
		dkam *DecKeysAndMessages,
	) error
	AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error
	AddBlock(
		ctx context.Context,
		b *data.Block,
	) error
	QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error)
	AddValidatorRegistryEvent(ctx context.Context, tx pgx.Tx, vr *validatorRegistryBindings.ValidatorregistryUpdated) error
	UpdateValidatorStatus(ctx context.Context) error
	AddProposerDuties(ctx context.Context, epoch uint64) error
}
