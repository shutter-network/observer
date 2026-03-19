// Package metrics provides the Prometheus exporter and the transaction mapper logic.
//
// TxMapperDB is the observer's persistence and classification pipeline.
//
// Flow:
// 1. AddTransactionSubmittedEvent stores sequencer tx events.
// 2. AddDecryptionKeysAndMessages decrypts slot txs and creates initial decrypted_tx rows.
// 3. processTransactionExecution sends txs and waits for receipts.
// 4. HandleBlock and receipt handling classify inclusion.
// 5. Validator methods persist validator-related state.
package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	metricsCommon "github.com/shutter-network/observer/common"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
)

const ReceiptWaitTimeout = 1 * time.Hour

var errSendTransaction = errors.New("send transaction failed")

type TxMapperDB struct {
	db               *pgxpool.Pool
	dbQuery          *data.Queries
	config           *metricsCommon.Config
	ethClient        *ethclient.Client
	beaconAPIClient  *beaconapiclient.Client
	chainID          int64
	genesisTimestamp uint64
	slotDuration     uint64
	statusDone       sync.Map
	slotLocks        sync.Map
}

func (tm *TxMapperDB) slotLock(slot int64) *sync.Mutex {
	v, _ := tm.slotLocks.LoadOrStore(slot, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (tm *TxMapperDB) withSlotLock(slot int64, fn func() error) error {
	mu := tm.slotLock(slot)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

type TxEventStore interface {
	AddTransactionSubmittedEvent(ctx context.Context, tx pgx.Tx, st *sequencerBindings.SequencerTransactionSubmitted) error
	AddBlock(ctx context.Context, b *data.Block) error
	AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error
}

type DecryptionStore interface {
	AddDecryptionKeysAndMessages(ctx context.Context, dkam *DecKeysAndMessages) error
}

type BlockClassifier interface {
	HandleBlock(ctx context.Context, blockNumber int64, slot int64, txs types.Transactions) error
}

type ValidatorStore interface {
	QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error)
	AddValidatorRegistryEvent(ctx context.Context, tx pgx.Tx, vr *validatorRegistryBindings.ValidatorregistryUpdated) error
	UpdateValidatorStatus(ctx context.Context) error
	AddProposerDuties(ctx context.Context, epoch uint64) error
	UpsertGraffitiIfShutterized(ctx context.Context, validatorIndex int64, graffiti string, blockNumber int64) (bool, error)
}

type TxMapper interface {
	TxEventStore
	DecryptionStore
	BlockClassifier
	ValidatorStore
}

// markDone records that an inclusion classification was observed for this tx hash.
// Failure and timeout paths should not overwrite rows once a tx is done.
func (tm *TxMapperDB) markDone(hash common.Hash) {
	tm.statusDone.Store(hash.Hex(), struct{}{})
}

// isDone reports whether an inclusion classification was already observed.
func (tm *TxMapperDB) isDone(hash common.Hash) bool {
	_, ok := tm.statusDone.Load(hash.Hex())
	return ok
}

func NewTxMapperDB(
	ctx context.Context,
	db *pgxpool.Pool,
	config *metricsCommon.Config,
	ethClient *ethclient.Client,
	beaconAPIClient *beaconapiclient.Client,
	chainID int64,
	genesisTimestamp uint64,
	slotDuration uint64,
) TxMapper {
	return &TxMapperDB{
		db:               db,
		dbQuery:          data.New(db),
		config:           config,
		ethClient:        ethClient,
		beaconAPIClient:  beaconAPIClient,
		chainID:          chainID,
		genesisTimestamp: genesisTimestamp,
		slotDuration:     slotDuration,
	}
}

func (tm *TxMapperDB) AddTransactionSubmittedEvent(ctx context.Context, tx pgx.Tx, st *sequencerBindings.SequencerTransactionSubmitted) error {
	q := tm.dbQuery
	if tx != nil {
		// Use transaction if available
		q = tm.dbQuery.WithTx(tx)
	}
	err := q.CreateTransactionSubmittedEvent(ctx, data.CreateTransactionSubmittedEventParams{
		EventBlockHash:       st.Raw.BlockHash.Bytes(),
		EventBlockNumber:     int64(st.Raw.BlockNumber),
		EventTxIndex:         int64(st.Raw.TxIndex),
		EventLogIndex:        int64(st.Raw.Index),
		Eon:                  int64(st.Eon),
		TxIndex:              int64(st.TxIndex),
		IdentityPrefix:       st.IdentityPrefix[:],
		Sender:               st.Sender.Bytes(),
		EncryptedTransaction: st.EncryptedTransaction,
		EventTxHash:          st.Raw.TxHash.Bytes(),
	})
	if err != nil {
		return err
	}
	metricsEncTxReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	err := tm.dbQuery.CreateDecryptionKeyShare(ctx, data.CreateDecryptionKeyShareParams{
		Eon:                dks.Eon,
		DecryptionKeyShare: dks.DecryptionKeyShare,
		Slot:               dks.Slot,
		IdentityPreimage:   dks.IdentityPreimage,
		KeyperIndex:        dks.KeyperIndex,
	})
	if err != nil {
		return err
	}
	metricsKeyShareReceived.Inc()
	return nil
}

func (tm *TxMapperDB) AddBlock(
	ctx context.Context,
	b *data.Block,
) error {
	err := tm.dbQuery.CreateBlock(ctx, data.CreateBlockParams{
		BlockHash:      b.BlockHash,
		BlockNumber:    b.BlockNumber,
		BlockTimestamp: b.BlockTimestamp,
		Slot:           b.Slot,
	})
	return err
}