package metrics

import (
	"context"
	"fmt"
	"sync"

	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperMemory struct {
	DecryptionKeysMessages              map[int64]*data.DecryptionKeysMessage
	DecryptionKeys                      map[string]*data.DecryptionKey
	DecryptionKeysMessageDecryptionKeys map[string]*data.DecryptionKeysMessageDecryptionKey
	TransactionSubmittedEvents          map[string]*data.TransactionSubmittedEvent
	DecryptionKeyShare                  map[string]*data.DecryptionKeyShare
	Block                               map[string]*data.Block
	mutex                               sync.Mutex
}

func NewTxMapperMemory() TxMapper {
	return &TxMapperMemory{
		DecryptionKeysMessages:              make(map[int64]*data.DecryptionKeysMessage),
		DecryptionKeys:                      make(map[string]*data.DecryptionKey),
		DecryptionKeysMessageDecryptionKeys: make(map[string]*data.DecryptionKeysMessageDecryptionKey),
		TransactionSubmittedEvents:          make(map[string]*data.TransactionSubmittedEvent),
		DecryptionKeyShare:                  make(map[string]*data.DecryptionKeyShare),
		Block:                               make(map[string]*data.Block),
		mutex:                               sync.Mutex{},
	}
}

func (tm *TxMapperMemory) AddTransactionSubmittedEvent(ctx context.Context, tse *data.TransactionSubmittedEvent) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := createTransactionSubmittedEventKey(tse.EventBlockHash, tse.EventBlockNumber, tse.EventTxIndex, tse.EventLogIndex)
	tm.TransactionSubmittedEvents[key] = tse
	return nil
}

func (tm *TxMapperMemory) AddDecryptionKeyAndMessage(
	ctx context.Context,
	dk *data.DecryptionKey,
	dkm *data.DecryptionKeysMessage,
	dkmdk *data.DecryptionKeysMessageDecryptionKey,
) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.DecryptionKeysMessages[dkm.Slot] = dkm

	dkKey := createDecryptionKeyKey(dk.Eon, dk.IdentityPreimage)
	tm.DecryptionKeys[dkKey] = dk

	dkmdkKey := createDecryptionKeysMessageDecryptionKeyKey(dkmdk.DecryptionKeysMessageSlot, dkmdk.DecryptionKeyEon, dkmdk.DecryptionKeyIdentityPreimage, dkmdk.KeyIndex)
	tm.DecryptionKeysMessageDecryptionKeys[dkmdkKey] = dkmdk

	return nil
}

func (tm *TxMapperMemory) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := createDecryptionKeyShareKey(dks.Eon, dks.IdentityPreimage, dks.KeyperIndex)
	tm.DecryptionKeyShare[key] = dks
	return nil
}

func (tm *TxMapperMemory) AddBlock(ctx context.Context, b *data.Block) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.Block[string(b.BlockHash)] = b
	return nil
}

func createTransactionSubmittedEventKey(eventBlockHash []byte, eventBlockNumber int64, eventTxIndex int64, eventLogIndex int64) string {
	return fmt.Sprintf("%x_%d_%d_%d", eventBlockHash, eventBlockNumber, eventTxIndex, eventLogIndex)
}

func createDecryptionKeyKey(eon int64, identityPreimage []byte) string {
	return fmt.Sprintf("%d_%x", eon, identityPreimage)
}

func createDecryptionKeysMessageDecryptionKeyKey(slot int64, eon int64, identityPreimage []byte, keyIndex int64) string {
	return fmt.Sprintf("%d_%d_%x_%d", slot, eon, identityPreimage, keyIndex)
}

func createDecryptionKeyShareKey(eon int64, identityPreimage []byte, keyperIndex int64) string {
	return fmt.Sprintf("%d_%x_%d", eon, identityPreimage, keyperIndex)
}
