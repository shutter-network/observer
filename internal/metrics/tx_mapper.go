package metrics

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shutter-network/gnosh-metrics/internal/data"
)

type TxMapperMemory struct {
	DecryptionKeysMessages              map[int64]*data.DecryptionKeysMessage
	DecryptionKeys                      map[string]*data.DecryptionKey
	DecryptionKeysMessageDecryptionKeys map[string]*data.DecryptionKeysMessageDecryptionKey
	TransactionSubmittedEvents          map[string]*data.TransactionSubmittedEvent
	DecryptionKeyShare                  map[string]*data.DecryptionKeyShare
	Block                               map[string]*data.Block
	ValidatoryRegistry                  map[string]*data.ValidatorRegistrationMessage
	ethClient                           *ethclient.Client

	mutex sync.Mutex
}

func NewTxMapperMemory(ethClient *ethclient.Client) TxMapper {
	return &TxMapperMemory{
		DecryptionKeysMessages:              make(map[int64]*data.DecryptionKeysMessage),
		DecryptionKeys:                      make(map[string]*data.DecryptionKey),
		DecryptionKeysMessageDecryptionKeys: make(map[string]*data.DecryptionKeysMessageDecryptionKey),
		TransactionSubmittedEvents:          make(map[string]*data.TransactionSubmittedEvent),
		DecryptionKeyShare:                  make(map[string]*data.DecryptionKeyShare),
		Block:                               make(map[string]*data.Block),
		ethClient:                           ethClient,
		ValidatoryRegistry:                  make(map[string]*data.ValidatorRegistrationMessage),
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

func (tm *TxMapperMemory) AddDecryptionKeysAndMessages(
	ctx context.Context,
	dkam *DecKeysAndMessages,
) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.DecryptionKeysMessages[dkam.Slot] = &data.DecryptionKeysMessage{
		Slot:       dkam.Slot,
		InstanceID: dkam.InstanceID,
		Eon:        dkam.Eon,
		TxPointer:  dkam.TxPointer,
	}

	for index, identity := range dkam.Identities {
		dkKey := createDecryptionKeyKey(dkam.Eon, identity)
		tm.DecryptionKeys[dkKey] = &data.DecryptionKey{
			Eon:              dkam.Eon,
			IdentityPreimage: identity,
			Key:              dkam.Keys[index],
		}
	}

	for index, identity := range dkam.Identities {
		dkmdkKey := createDecryptionKeysMessageDecryptionKeyKey(dkam.Slot, dkam.Eon, identity, int64(index))
		tm.DecryptionKeysMessageDecryptionKeys[dkmdkKey] = &data.DecryptionKeysMessageDecryptionKey{
			DecryptionKeysMessageSlot:     dkam.Slot,
			KeyIndex:                      int64(index),
			DecryptionKeyEon:              dkam.Eon,
			DecryptionKeyIdentityPreimage: identity,
		}
	}

	return nil
}

func (tm *TxMapperMemory) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := createDecryptionKeyShareKey(dks.Eon, dks.IdentityPreimage, dks.KeyperIndex)
	tm.DecryptionKeyShare[key] = dks
	return nil
}

func (tm *TxMapperMemory) AddBlock(
	ctx context.Context,
	b *data.Block,
) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.Block[string(b.BlockHash)] = b
	return nil
}

func (tm *TxMapperMemory) QueryBlockNumberFromValidatorRegistry(ctx context.Context) (int64, error) {
	var maxBlock int64
	for _, data := range tm.ValidatoryRegistry {
		if data.EventBlockNumber > maxBlock {
			maxBlock = data.EventBlockNumber
		}
	}
	return maxBlock, nil
}

func (tm *TxMapperMemory) AddValidatorRegistryEvent(ctx context.Context, vr *data.ValidatorRegistrationMessage) error {
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
