package metrics

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/shutter/shlib/shcrypto"
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

func (tm *TxMapperDB) AddDecryptionKeysAndMessages(
	ctx context.Context,
	decKeysAndMessages *DecKeysAndMessages,
) error {
	if len(decKeysAndMessages.Keys) == 0 {
		return nil
	}
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := tm.dbQuery.WithTx(tx)

	eons, slots, instanceIDs, txPointers, keyIndexes := getDecryptionMessageInfos(decKeysAndMessages)
	decryptionKeyIDs, err := qtx.CreateDecryptionKeys(ctx, data.CreateDecryptionKeysParams{
		Column1: eons,
		Column2: decKeysAndMessages.Identities,
		Column3: decKeysAndMessages.Keys,
	})
	if err != nil {
		return err
	}
	if len(decryptionKeyIDs) == 0 || len(decryptionKeyIDs) != len(slots) {
		log.Debug().Msg("no new decryption key was added")
		return nil
	}
	err = qtx.CreateDecryptionKeyMessages(ctx, data.CreateDecryptionKeyMessagesParams{
		Column1: slots,
		Column2: instanceIDs,
		Column3: eons,
		Column4: txPointers,
	})
	if err != nil {
		return err
	}

	err = qtx.CreateDecryptionKeysMessageDecryptionKey(ctx, data.CreateDecryptionKeysMessageDecryptionKeyParams{
		Column1: slots,
		Column2: keyIndexes,
		Column3: decryptionKeyIDs,
	})
	if err != nil {
		return err
	}

	totalDecKeysAndMessages := len(decKeysAndMessages.Keys)
	err = tx.Commit(ctx)
	if err != nil {
		log.Err(err).Msg("unable to commit db transaction")
		return err
	}

	for i := 0; i < totalDecKeysAndMessages; i++ {
		metricsDecKeyReceived.Inc()
	}

	dkam := make([]*DecKeyAndMessage, totalDecKeysAndMessages)
	for index, key := range decKeysAndMessages.Keys {
		identityPreimage := decKeysAndMessages.Identities[index]
		dkam[index] = &DecKeyAndMessage{
			Slot:             decKeysAndMessages.Slot,
			TxPointer:        decKeysAndMessages.TxPointer,
			Eon:              decKeysAndMessages.Eon,
			Key:              key,
			IdentityPreimage: identityPreimage,
			KeyIndex:         int64(index),
			DecryptionKeyID:  decryptionKeyIDs[index],
		}
	}

	if len(dkam) > 0 {
		dkam = dkam[1:]
	}
	err = tm.processTransactionExecution(ctx, &TxExecution{
		DecKeysAndMessages: dkam,
	})
	if err != nil {
		log.Err(err).Int64("slot", decKeysAndMessages.Slot).Msg("failed to process transaction execution")
		return err
	}
	return nil
}

func getDecryptionMessageInfos(dkam *DecKeysAndMessages) ([]int64, []int64, []int64, []int64, []int64) {
	eons := make([]int64, len(dkam.Keys))
	slots := make([]int64, len(dkam.Keys))
	instanceIDs := make([]int64, len(dkam.Keys))
	txPointers := make([]int64, len(dkam.Keys))
	keyIndexes := make([]int64, len(dkam.Keys))

	for index := range dkam.Keys {
		eons[index] = dkam.Eon
		slots[index] = dkam.Slot
		instanceIDs[index] = dkam.InstanceID
		txPointers[index] = dkam.TxPointer
		keyIndexes[index] = int64(index)
	}

	return eons, slots, instanceIDs, txPointers, keyIndexes
}

func computeIdentity(prefix []byte, sender common.Address) []byte {
	imageBytes := append(prefix, sender.Bytes()...)
	return imageBytes
}

func getDecryptedTX(
	txSubEvent data.TransactionSubmittedEvent,
	identityPreimageToDecKeyAndMsg map[string]*DecKeyAndMessage,
) (*types.Transaction, error) {
	identityPreimage := computeIdentity(txSubEvent.IdentityPrefix, common.BytesToAddress(txSubEvent.Sender))
	dkam, ok := identityPreimageToDecKeyAndMsg[hex.EncodeToString(identityPreimage)]
	if !ok {
		return nil, fmt.Errorf("identity preimage not found %s", hex.EncodeToString(identityPreimage))
	}
	tx, err := decryptTransaction(dkam.Key, txSubEvent.EncryptedTransaction)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func getDecryptionKeyID(
	txSubEvent data.TransactionSubmittedEvent,
	identityPreimageToDecKeyAndMsg map[string]*DecKeyAndMessage,
) (int64, error) {
	identityPreimage := computeIdentity(txSubEvent.IdentityPrefix, common.BytesToAddress(txSubEvent.Sender))
	dkam, ok := identityPreimageToDecKeyAndMsg[hex.EncodeToString(identityPreimage)]
	if !ok {
		return 0, fmt.Errorf("identity preimage not found %s", hex.EncodeToString(identityPreimage))
	}
	return dkam.DecryptionKeyID, nil
}

func decryptTransaction(key []byte, encrypted []byte) (*types.Transaction, error) {
	decryptionKey := new(shcrypto.EpochSecretKey)
	err := decryptionKey.Unmarshal(key)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid decryption key")
	}
	encryptedMsg := new(shcrypto.EncryptedMessage)
	err = encryptedMsg.Unmarshal(encrypted)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid encrypted msg")
	}
	decryptedMsg, err := encryptedMsg.Decrypt(decryptionKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decrypt message")
	}

	tx := new(types.Transaction)
	err = tx.UnmarshalBinary(decryptedMsg)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal decrypted message to transaction type")
	}
	return tx, nil
}