package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	dbTypes "github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	gnosis "github.com/shutter-network/rolling-shutter/rolling-shutter/keyperimpl/gnosis"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/validatorregistry"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	blst "github.com/supranational/blst/bindings/go"
)

type TxMapperDB struct {
	db              *pgxpool.Pool
	dbQuery         *data.Queries
	config          *metricsCommon.Config
	ethClient       *ethclient.Client
	beaconAPIClient *beaconapiclient.Client
	chainID         int64
}

func NewTxMapperDB(
	ctx context.Context,
	db *pgxpool.Pool,
	config *metricsCommon.Config,
	ethClient *ethclient.Client,
	beaconAPIClient *beaconapiclient.Client,
	chainID int64,
) TxMapper {
	return &TxMapperDB{
		db:              db,
		dbQuery:         data.New(db),
		config:          config,
		ethClient:       ethClient,
		beaconAPIClient: beaconAPIClient,
		chainID:         chainID,
	}
}

func (tm *TxMapperDB) AddTransactionSubmittedEvent(ctx context.Context, tse *data.TransactionSubmittedEvent) error {
	err := tm.dbQuery.CreateTransactionSubmittedEvent(ctx, data.CreateTransactionSubmittedEventParams{
		EventBlockHash:       tse.EventBlockHash,
		EventBlockNumber:     tse.EventBlockNumber,
		EventTxIndex:         tse.EventTxIndex,
		EventLogIndex:        tse.EventLogIndex,
		Eon:                  tse.Eon,
		TxIndex:              tse.TxIndex,
		IdentityPrefix:       tse.IdentityPrefix,
		Sender:               tse.Sender,
		EncryptedTransaction: tse.EncryptedTransaction,
	})
	if err != nil {
		return err
	}
	metricsEncTxReceived.Inc()
	return nil
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
	if len(decryptionKeyIDs) == 0 {
		log.Debug().Msg("no decryption key was added")
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

	block, err := qtx.QueryBlockFromSlot(ctx, decKeysAndMessages.Slot)
	if err != nil {
		log.Debug().Int64("slot", decKeysAndMessages.Slot).Msg("block not found in AddDecryptedTxFromDecryptionKeys")
		err = tx.Commit(ctx)
		if err != nil {
			log.Err(err).Msg("unable to commit db transaction")
			return err
		}
		for i := 0; i < totalDecKeysAndMessages; i++ {
			metricsDecKeyReceived.Inc()
		}
		return nil
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
		BlockNumber:        block.BlockNumber,
	})
	if err != nil {
		log.Err(err).Int64("slot", decKeysAndMessages.Slot).Msg("failed to process transaction execution")
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Err(err).Msg("unable to commit db transaction")
		return err
	}

	for i := 0; i < totalDecKeysAndMessages; i++ {
		metricsDecKeyReceived.Inc()
	}
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
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := tm.dbQuery.WithTx(tx)
	err = qtx.CreateBlock(ctx, data.CreateBlockParams{
		BlockHash:      b.BlockHash,
		BlockNumber:    b.BlockNumber,
		BlockTimestamp: b.BlockTimestamp,
		TxHash:         b.TxHash,
		Slot:           b.Slot,
	})
	if err != nil {
		return err
	}
	decKeysAndMessages, err := qtx.QueryDecryptionKeysAndMessage(ctx, b.Slot)
	if err != nil {
		return err
	}

	totalDecKeysAndMessages := len(decKeysAndMessages)

	if totalDecKeysAndMessages == 0 {
		log.Debug().Int64("slot", b.Slot).Msg("no decryption keys received yet")
		return tx.Commit(ctx)
	}

	dkam := make([]*DecKeyAndMessage, totalDecKeysAndMessages)
	for index, elem := range decKeysAndMessages {
		dkam[index] = &DecKeyAndMessage{
			Slot:             elem.Slot.Int64,
			TxPointer:        elem.TxPointer.Int64,
			Eon:              elem.Eon.Int64,
			Key:              elem.Key,
			IdentityPreimage: elem.IdentityPreimage,
			KeyIndex:         elem.KeyIndex,
			DecryptionKeyID:  elem.DecryptionKeyID,
		}
	}
	if len(dkam) > 0 {
		dkam = dkam[1:]
	}
	err = tm.processTransactionExecution(ctx, &TxExecution{
		DecKeysAndMessages: dkam,
		BlockNumber:        b.BlockNumber,
	})
	if err != nil {
		log.Err(err).Int64("slot", b.Slot).Msg("failed to process transaction execution")
		return err
	}
	return tx.Commit(ctx)
}

func (tm *TxMapperDB) QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error) {
	blockNumber, err := tm.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil {
		return 0, err
	}
	return blockNumber, nil
}

func (tm *TxMapperDB) AddValidatorRegistryEvent(ctx context.Context, vr *validatorRegistryBindings.ValidatorregistryUpdated) error {
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := tm.dbQuery.WithTx(tx)

	var validator *beaconapiclient.GetValidatorByIndexResponse
	regMessage := &validatorregistry.RegistrationMessage{}
	params := data.CreateValidatorRegistryMessageParams{}
	err = regMessage.Unmarshal(vr.Message)
	if err != nil {
		params.Validity = data.ValidatorRegistrationValidityInvalidmessage
		log.Err(err).Hex("tx-hash", vr.Raw.TxHash.Bytes()).Msg("error unmarshalling registration message")
	} else {
		params.Version = dbTypes.Uint64ToPgTypeInt8(uint64(regMessage.Version))
		params.ValidatorRegistryAddress = regMessage.ValidatorRegistryAddress.Bytes()
		params.ChainID = dbTypes.Uint64ToPgTypeInt8(regMessage.ChainID)
		params.ValidatorIndex = dbTypes.Uint64ToPgTypeInt8(regMessage.ValidatorIndex)
		params.Nonce = dbTypes.Uint64ToPgTypeInt8(regMessage.Nonce)
		params.IsRegisteration = dbTypes.BoolToPgTypeBool(regMessage.IsRegistration)
		params.Validity, validator, err = tm.validateValidatorRegistryEvent(ctx, vr, regMessage, vr.Signature)
		if err != nil {
			log.Err(err).Msg("error validating validator registry events")
			return err
		}
	}

	if params.Validity == data.ValidatorRegistrationValidityValid {
		if validator != nil {
			err := qtx.CreateValidatorStatus(ctx, data.CreateValidatorStatusParams{
				ValidatorIndex: dbTypes.Uint64ToPgTypeInt8(regMessage.ValidatorIndex),
				Status:         validator.Data.Status,
			})
			if err != nil {
				return err
			}
		}
	}

	params.Signature = vr.Signature
	params.EventBlockNumber = int64(vr.Raw.BlockNumber)
	params.EventTxIndex = int64(vr.Raw.TxIndex)
	params.EventLogIndex = int64(vr.Raw.Index)
	err = qtx.CreateValidatorRegistryMessage(ctx, params)

	if err != nil {
		return err
	}
	err = qtx.CreateValidatorRegistryEventsSyncedUntil(ctx, int64(vr.Raw.BlockNumber))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (tm *TxMapperDB) UpdateValidatorStatus(ctx context.Context) error {
	batchSize := 100
	jumpBy := 0
	numWorkers := 5
	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	for {
		// Query a batch of validator statuses
		validatorStatus, err := tm.dbQuery.QueryValidatorStatuses(ctx, data.QueryValidatorStatusesParams{
			Limit:  int32(batchSize),
			Offset: int32(jumpBy),
		})
		if err != nil {
			if err == pgx.ErrNoRows {
				break
			}
			return err
		}

		if len(validatorStatus) == 0 {
			break
		}

		// Launch goroutines to process each status concurrently
		for _, vs := range validatorStatus {
			sem <- struct{}{}
			wg.Add(1)
			go func(vs data.QueryValidatorStatusesRow) {
				defer wg.Done()
				defer func() { <-sem }()

				validatorIndex := uint64(vs.ValidatorIndex.Int64)
				//TODO: should we keep this log or remove it?
				log.Debug().Uint64("validatorIndex", validatorIndex).Msg("validator status being updated")
				validator, err := tm.beaconAPIClient.GetValidatorByIndex(ctx, "head", validatorIndex)
				if err != nil {
					log.Err(err).Uint64("validatorIndex", validatorIndex).Msg("failed to get validator from beacon chain")
					return
				}
				if validator == nil {
					return
				}

				err = tm.dbQuery.CreateValidatorStatus(ctx, data.CreateValidatorStatusParams{
					ValidatorIndex: dbTypes.Uint64ToPgTypeInt8(validatorIndex),
					Status:         validator.Data.Status,
				})
				if err != nil {
					log.Err(err).Uint64("validatorIndex", validatorIndex).Msg("failed to create validator status")
					return
				}
			}(vs)
		}

		wg.Wait()

		// Wait for 3 seconds before processing the next batch
		select {
		case <-ctx.Done():
			return ctx.Err() // Handle context cancellation
		case <-time.After(3 * time.Second):
		}

		jumpBy += batchSize
	}

	return nil
}

func (tm *TxMapperDB) processTransactionExecution(
	ctx context.Context,
	te *TxExecution,
) error {
	totalDecKeysAndMessages := len(te.DecKeysAndMessages)
	if totalDecKeysAndMessages == 0 {
		return nil
	}
	txSubEvents, err := tm.dbQuery.QueryTransactionSubmittedEvent(ctx, data.QueryTransactionSubmittedEventParams{
		Eon:     te.DecKeysAndMessages[0].Eon,
		TxIndex: te.DecKeysAndMessages[0].TxPointer,
		Column3: totalDecKeysAndMessages,
	})
	if err != nil {
		return err
	}

	if len(txSubEvents) != totalDecKeysAndMessages {
		log.Debug().Int("total tx sub events", len(txSubEvents)).
			Int("total decryption keys", totalDecKeysAndMessages).
			Msg("total tx submitted events dont match total decryption keys")
		return nil
	}

	identityPreimageToDecKeyAndMsg := make(map[string]*DecKeyAndMessage)
	for _, dkam := range te.DecKeysAndMessages {
		identityPreimageToDecKeyAndMsg[hex.EncodeToString(dkam.IdentityPreimage)] = dkam
	}

	slot := te.DecKeysAndMessages[0].Slot
	detailedBlock, err := tm.ethClient.BlockByNumber(ctx, big.NewInt(te.BlockNumber))
	if err != nil {
		return err
	}

	var blockTxHashes []common.Hash
	for _, tx := range detailedBlock.Transactions() {
		blockTxHashes = append(blockTxHashes, tx.Hash())
	}

	log.Debug().Int64("block number", detailedBlock.Number().Int64()).
		Int64("slot", slot).
		Msg("block info while processing decrypted transactions")

	for index, txSubEvent := range txSubEvents {
		decryptedTxHash, err := getDecryptedTXHash(txSubEvent, identityPreimageToDecKeyAndMsg)
		if err != nil {
			log.Err(err).Msg("error while trying to get decrypted tx hash")
		}
		decryptionKeyID, err := getDecryptionKeyID(txSubEvent, identityPreimageToDecKeyAndMsg)
		if err != nil {
			log.Err(err).Msg("error while trying to retrieve decryption key ID")
		}
		if index < len(blockTxHashes) {
			log.Debug().
				Str("decryptedTXHash", decryptedTxHash.Hex()).
				Str("blockTxHash", blockTxHashes[index].Hex()).
				Bool("matches", decryptedTxHash.Cmp(blockTxHashes[index]) == 0).
				Msg("comparing tx hash")
			if decryptedTxHash.Cmp(blockTxHashes[index]) == 0 {
				// it means we have it in correct order and the transaction is correct
				err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
					Slot:                        slot,
					TxIndex:                     txSubEvent.TxIndex,
					TxHash:                      decryptedTxHash.Bytes(),
					TxStatus:                    data.TxStatusValIncluded,
					DecryptionKeyID:             pgtype.Int8{Int64: decryptionKeyID, Valid: true},
					TransactionSubmittedEventID: pgtype.Int8{Int64: txSubEvent.ID, Valid: true},
				})
				if err != nil {
					return err
				}
			} else {
				// something went wrong case
				err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
					Slot:                        slot,
					TxIndex:                     txSubEvent.TxIndex,
					TxHash:                      decryptedTxHash.Bytes(),
					TxStatus:                    data.TxStatusValNotincluded,
					DecryptionKeyID:             pgtype.Int8{Int64: decryptionKeyID, Valid: true},
					TransactionSubmittedEventID: pgtype.Int8{Int64: txSubEvent.ID, Valid: true},
				})
				if err != nil {
					return err
				}
			}
		} else {
			// Mark remaining txSubEvents as missing
			log.Debug().Str("txHash", decryptedTxHash.Hex()).
				Msg("decryptedTXHash (missing block transaction)")
			err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
				Slot:     slot,
				TxIndex:  txSubEvent.TxIndex,
				TxHash:   decryptedTxHash.Bytes(),
				TxStatus: data.TxStatusValNotincluded,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (tm *TxMapperDB) validateValidatorRegistryEvent(
	ctx context.Context,
	vr *validatorRegistryBindings.ValidatorregistryUpdated,
	regMessage *validatorregistry.RegistrationMessage,
	blsSignature []byte,
) (data.ValidatorRegistrationValidity, *beaconapiclient.GetValidatorByIndexResponse, error) {
	validity, err := tm.validateValidatorRegistryMessageContents(ctx, vr, regMessage)
	if err != nil {
		return validity, nil, err
	}
	validator, err := tm.beaconAPIClient.GetValidatorByIndex(ctx, "head", regMessage.ValidatorIndex)
	if err != nil {
		return data.ValidatorRegistrationValidityInvalidsignature, nil, errors.Wrapf(err, "failed to get validator %d", regMessage.ValidatorIndex)
	}
	if validity == data.ValidatorRegistrationValidityValid {
		// which means message have been validated and all were passed
		// now we need to check for signature verification
		validity, err = tm.validateBLSSignature(ctx, vr.Signature, regMessage, validator)
		if err != nil {
			return validity, nil, err
		}
	}
	return validity, validator, nil
}

func (tm *TxMapperDB) validateValidatorRegistryMessageContents(
	ctx context.Context,
	vr *validatorRegistryBindings.ValidatorregistryUpdated,
	msg *validatorregistry.RegistrationMessage,
) (data.ValidatorRegistrationValidity, error) {
	validity := data.ValidatorRegistrationValidityValid

	if msg.Version != gnosis.ValidatorRegistrationMessageVersion {
		return data.ValidatorRegistrationValidityInvalidmessage, nil
	}
	if msg.ChainID != uint64(tm.chainID) {
		return data.ValidatorRegistrationValidityInvalidmessage, nil
	}
	if msg.ValidatorRegistryAddress.String() != tm.config.ValidatorRegistryContractAddress {
		return data.ValidatorRegistrationValidityInvalidmessage, nil
	}
	if msg.ValidatorIndex > math.MaxInt64 {
		return data.ValidatorRegistrationValidityInvalidmessage, nil
	}

	nonceBefore, err := tm.dbQuery.QueryValidatorRegistrationMessageNonceBefore(ctx, data.QueryValidatorRegistrationMessageNonceBeforeParams{
		ValidatorIndex:   dbTypes.Uint64ToPgTypeInt8(msg.ValidatorIndex),
		EventBlockNumber: int64(vr.Raw.BlockNumber),
		EventTxIndex:     int64(vr.Raw.TxIndex),
		EventLogIndex:    int64(vr.Raw.Index),
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			// No previous nonce means the message is valid regarding nonce
			nonceBefore = pgtype.Int8{Int64: -1, Valid: true}
		} else {
			return data.ValidatorRegistrationValidityInvalidmessage, errors.Wrapf(err, "failed to query latest nonce for validator %d", msg.ValidatorIndex)
		}
	}

	if msg.Nonce > math.MaxInt64 || int64(msg.Nonce) < nonceBefore.Int64 {
		// new nonce should be less then equals to max int64
		// new should be greater the previous nonce
		return data.ValidatorRegistrationValidityInvalidmessage, nil
	}
	return validity, nil
}

func (tm *TxMapperDB) validateBLSSignature(
	ctx context.Context,
	blsSignature []byte,
	msg *validatorregistry.RegistrationMessage,
	validator *beaconapiclient.GetValidatorByIndexResponse,
) (data.ValidatorRegistrationValidity, error) {
	validity := data.ValidatorRegistrationValidityValid
	if validator == nil {
		//since validator is nil its signature is invalid automatically
		validity = data.ValidatorRegistrationValidityInvalidsignature
	} else {
		pubkey, err := validator.Data.Validator.GetPubkey()
		if err != nil {
			// should we error out here and return?
			log.Err(err).Uint64("validator index", msg.ValidatorIndex).Msg("failed to get pubkey of validator")
			return data.ValidatorRegistrationValidityInvalidsignature, errors.Wrapf(err, "failed to get validator public key %d", msg.ValidatorIndex)
		}
		sig := new(blst.P2Affine).Uncompress(blsSignature)
		if sig == nil {
			validity = data.ValidatorRegistrationValidityInvalidsignature
			log.Warn().
				Uint64("validator index", msg.ValidatorIndex).
				Uint64("nonce", msg.Nonce).
				Msg("ignoring registration message with undecodable signature")
		}
		validSignature := validatorregistry.VerifySignature(sig, pubkey, msg)
		if !validSignature {
			validity = data.ValidatorRegistrationValidityInvalidsignature
			log.Warn().Msg("ignoring registration message with invalid signature")
		}
	}
	return validity, nil
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

func getDecryptedTXHash(
	txSubEvent data.TransactionSubmittedEvent,
	identityPreimageToDecKeyAndMsg map[string]*DecKeyAndMessage,
) (common.Hash, error) {
	identityPreimage := computeIdentity(txSubEvent.IdentityPrefix, common.BytesToAddress(txSubEvent.Sender))
	dkam, ok := identityPreimageToDecKeyAndMsg[hex.EncodeToString(identityPreimage)]
	if !ok {
		return common.Hash{}, fmt.Errorf("identity preimage not found %s", hex.EncodeToString(identityPreimage))
	}
	tx, err := decryptTransaction(dkam.Key, txSubEvent.EncryptedTransaction)
	if err != nil {
		return common.Hash{}, err
	}
	return tx.Hash(), nil
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
