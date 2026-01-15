package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	metricsCommon "github.com/shutter-network/observer/common"
	dbTypes "github.com/shutter-network/observer/common/database"
	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/validatorregistry"
	"github.com/shutter-network/shutter/shlib/shcrypto"
	blst "github.com/supranational/blst/bindings/go"
)

const ReceiptWaitTimeout = 1 * time.Hour

type TxMapperDB struct {
	db               *pgxpool.Pool
	dbQuery          *data.Queries
	config           *metricsCommon.Config
	ethClient        *ethclient.Client
	beaconAPIClient  *beaconapiclient.Client
	chainID          int64
	genesisTimestamp uint64
	slotDuration     uint64
}

type validatorData struct {
	validatorStatus   string
	validatorValidity data.ValidatorRegistrationValidity
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

func (tm *TxMapperDB) QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error) {
	data, err := tm.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil {
		return 0, err
	}
	return data.BlockNumber, nil
}

func (tm *TxMapperDB) AddValidatorRegistryEvent(ctx context.Context, tx pgx.Tx, vr *validatorRegistryBindings.ValidatorregistryUpdated) error {
	regMessage := &validatorregistry.AggregateRegistrationMessage{}
	err := regMessage.Unmarshal(vr.Message)
	if err != nil {
		log.Err(err).Hex("tx-hash", vr.Raw.TxHash.Bytes()).Msg("error unmarshalling registration message")
	} else {
		validatorIDtoValidity, err := tm.validateValidatorRegistryEvent(ctx, vr, regMessage, uint64(tm.chainID), tm.config.ValidatorRegistryContractAddress)
		if err != nil {
			log.Err(err).Msg("error validating validator registry events")
			return err
		}

		q := tm.dbQuery
		if tx != nil {
			// Use transaction if available
			q = tm.dbQuery.WithTx(tx)
		}

		for validatorID, validatorData := range validatorIDtoValidity {
			err := q.CreateValidatorRegistryMessage(ctx, data.CreateValidatorRegistryMessageParams{
				Version:                  dbTypes.Uint64ToPgTypeInt8(uint64(regMessage.Version)),
				ChainID:                  dbTypes.Uint64ToPgTypeInt8(regMessage.ChainID),
				ValidatorRegistryAddress: regMessage.ValidatorRegistryAddress.Bytes(),
				ValidatorIndex:           dbTypes.Int64ToPgTypeInt8(validatorID),
				Nonce:                    dbTypes.Uint64ToPgTypeInt8(uint64(regMessage.Nonce)),
				IsRegisteration:          dbTypes.BoolToPgTypeBool(regMessage.IsRegistration),
				Signature:                vr.Signature,
				EventBlockNumber:         int64(vr.Raw.BlockNumber),
				EventTxIndex:             int64(vr.Raw.TxIndex),
				EventLogIndex:            int64(vr.Raw.Index),
				Validity:                 validatorData.validatorValidity,
			})
			if err != nil {
				return err
			}

			if validatorData.validatorValidity == data.ValidatorRegistrationValidityValid &&
				validatorData.validatorStatus != "" {
				err := q.CreateValidatorStatus(ctx, data.CreateValidatorStatusParams{
					ValidatorIndex: dbTypes.Int64ToPgTypeInt8(validatorID),
					Status:         validatorData.validatorStatus,
				})
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
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
			if errors.Is(err, pgx.ErrNoRows) {
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

func (tm *TxMapperDB) AddProposerDuties(ctx context.Context, epoch uint64) error {
	proposerDuties, err := tm.beaconAPIClient.GetProposerDutiesByEpoch(ctx, epoch)
	if err != nil {
		return err
	}
	if proposerDuties == nil {
		return errors.Errorf("no proposer duties found for epoch %d", epoch)
	}

	log.Info().Uint64("epoch", epoch).Msg("processing proposer duties")

	publicKeys := make([]string, len(proposerDuties.Data))
	validatorIndices := make([]int64, len(proposerDuties.Data))
	slots := make([]int64, len(proposerDuties.Data))

	for i := 0; i < len(proposerDuties.Data); i++ {
		publicKeys[i] = proposerDuties.Data[i].Pubkey
		validatorIndices[i] = int64(proposerDuties.Data[i].ValidatorIndex)
		slots[i] = int64(proposerDuties.Data[i].Slot)
	}

	err = tm.dbQuery.CreateProposerDuties(ctx, data.CreateProposerDutiesParams{
		Column1: publicKeys,
		Column2: validatorIndices,
		Column3: slots,
	})
	return err
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

	var wg sync.WaitGroup
	for index, txSubEvent := range txSubEvents {
		decryptionKeyID, err := getDecryptionKeyID(txSubEvent, identityPreimageToDecKeyAndMsg)
		if err != nil {
			log.Err(err).Msg("error while trying to retrieve decryption key ID")
			continue
		}
		decryptedTx, err := getDecryptedTX(txSubEvent, identityPreimageToDecKeyAndMsg)
		if err != nil {
			log.Err(err).Msg("error while trying to get decrypted tx hash")
			err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
				Slot:                        slot,
				TxIndex:                     txSubEvent.TxIndex,
				TxHash:                      common.Hash{}.Bytes(),
				TxStatus:                    data.TxStatusValNotdecrypted,
				DecryptionKeyID:             decryptionKeyID,
				TransactionSubmittedEventID: txSubEvent.ID,
			})
			if err != nil {
				log.Err(err).Msg("failed to create decrypted tx")
			}
			continue
		}

		log.Info().Uint64("gas", decryptedTx.Gas()).
			Uint64("gas-price", decryptedTx.GasPrice().Uint64()).
			Uint64("cost", decryptedTx.Cost().Uint64()).
			Uint64("max-priority-fee-per-gas", decryptedTx.GasTipCap().Uint64()).
			Uint64("max-fee-per-gas", decryptedTx.GasFeeCap().Uint64()).
			Uint8("tx-type", decryptedTx.Type()).
			Msg("tx-data")

		// channel to propagate errors between goroutines
		txErrorSignalCh := make(chan error, 1)

		wg.Add(2)

		// First goroutine: Send transaction
		go func(ctx context.Context, inclusionDelay int64, decryptedTx *types.Transaction, txSubEvent data.TransactionSubmittedEvent, slot int64, decryptionKeyID int64, txErrorSignalCh chan error) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				txErrorSignalCh <- fmt.Errorf("transaction send cancelled due to context: %w", ctx.Err())
				return
			case <-time.After(time.Duration(tm.config.InclusionDelay) * time.Second):
				if err := tm.ethClient.SendTransaction(ctx, decryptedTx); err != nil {
					log.Err(err).Msg("failed to send transaction")
					if err.Error() == "AlreadyKnown" {
						log.Debug().Hex("tx-hash", decryptedTx.Hash().Bytes()).Msg("already known")
						err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
							Slot:                        slot,
							TxIndex:                     txSubEvent.TxIndex,
							TxHash:                      decryptedTx.Hash().Bytes(),
							TxStatus:                    data.TxStatusValPending,
							DecryptionKeyID:             decryptionKeyID,
							TransactionSubmittedEventID: txSubEvent.ID,
						})
						if err != nil {
							txErrorSignalCh <- fmt.Errorf("failed to create decrypted tx: %w", err)
							return
						}
					} else {
						txStatus := data.TxStatusValInvalid
						if isFeeTooLowError(err) {
							txStatus = data.TxStatusValInvalidfeetoolow
						}
						err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
							Slot:                        slot,
							TxIndex:                     txSubEvent.TxIndex,
							TxHash:                      decryptedTx.Hash().Bytes(),
							TxStatus:                    txStatus,
							DecryptionKeyID:             decryptionKeyID,
							TransactionSubmittedEventID: txSubEvent.ID,
						})
						if err != nil {
							log.Err(err).Msg("failed to create decrypted tx")
						}
						txErrorSignalCh <- fmt.Errorf("failed to send transaction: %w", err)
						return
					}
				} else {
					log.Info().Hex("tx-hash", decryptedTx.Hash().Bytes()).Msg("transaction sent")
					err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
						Slot:                        slot,
						TxIndex:                     txSubEvent.TxIndex,
						TxHash:                      decryptedTx.Hash().Bytes(),
						TxStatus:                    data.TxStatusValPending,
						DecryptionKeyID:             decryptionKeyID,
						TransactionSubmittedEventID: txSubEvent.ID,
					})
					if err != nil {
						txErrorSignalCh <- fmt.Errorf("failed to create decrypted tx: %w", err)
						return
					}
				}
			}
		}(ctx, tm.config.InclusionDelay, decryptedTx, txSubEvent, slot, decryptionKeyID, txErrorSignalCh)

		// Second goroutine: Wait for receipt
		go func(ctx context.Context, index int, txHash common.Hash, txIndex int64, slot int64, decryptionKeyID int64, txSubEventID int64, txErrorSignalCh chan error) {
			defer wg.Done()

			// Wait for the receipt with a timeout
			receipt, err := tm.waitForReceiptWithTimeout(ctx, txHash, ReceiptWaitTimeout, txErrorSignalCh)
			if err != nil {
				log.Err(err).Msg("")
				// update/create status to not included
				err := tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
					Slot:                        slot,
					TxIndex:                     txIndex,
					TxHash:                      txHash[:],
					TxStatus:                    data.TxStatusValNotincluded,
					DecryptionKeyID:             decryptionKeyID,
					TransactionSubmittedEventID: txSubEventID,
				})
				if err != nil {
					log.Err(err).Msg("failed to upsert decrypted tx")
				}
				return
			}

			// receipt found
			log.Info().Hex("tx-hash", receipt.TxHash.Bytes()).
				Uint64("receipt-status", receipt.Status).
				Msg("transaction receipt found")

			block, err := tm.ethClient.BlockByNumber(ctx, receipt.BlockNumber)
			if err != nil {
				log.Err(err).Uint64("block-number", receipt.BlockNumber.Uint64()).Msg("failed to retrieve block")
				return
			}

			inclusionSlot := utils.GetSlotForBlock(block.Header().Time, tm.genesisTimestamp, tm.slotDuration)
			txStatus := data.TxStatusValShieldedinclusion

			log.Info().Uint("tx-index", receipt.TransactionIndex).
				Uint64("inclusion-slot", inclusionSlot).
				Msg("receipt data")

			log.Info().Int("index", index).
				Int64("inclusion-slot", slot).
				Msg("local data")

			if receipt.TransactionIndex != uint(index) {
				log.Info().Uint("tx-index", receipt.TransactionIndex).Msg("transaction index mismatch")
				txStatus = data.TxStatusValUnshieldedinclusion
			}
			if inclusionSlot != uint64(slot) {
				log.Info().Int64("slot", slot).Msg("transaction slot mismatch")
				txStatus = data.TxStatusValUnshieldedinclusion
			}

			err = tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
				Slot:                        slot,
				TxIndex:                     txIndex,
				TxHash:                      receipt.TxHash.Bytes(),
				TxStatus:                    txStatus,
				DecryptionKeyID:             decryptionKeyID,
				TransactionSubmittedEventID: txSubEventID,
				BlockNumber:                 pgtype.Int8{Int64: receipt.BlockNumber.Int64(), Valid: true},
			})
			if err != nil {
				log.Err(err).Msg("failed to update decrypted tx")
			}
		}(ctx, index, decryptedTx.Hash(), txSubEvent.TxIndex, slot, decryptionKeyID, txSubEvent.ID, txErrorSignalCh)
	}

	// Wait for all routines to end
	wg.Wait()
	return nil
}

func (tm *TxMapperDB) validateValidatorRegistryEvent(
	ctx context.Context,
	vr *validatorRegistryBindings.ValidatorregistryUpdated,
	regMessage *validatorregistry.AggregateRegistrationMessage,
	chainID uint64,
	validatorRegistryContractAddress string,
) (map[int64]*validatorData, error) {
	staticRegistrationMessageValidity := validateValidatorRegistryMessageContents(regMessage, chainID, validatorRegistryContractAddress)

	var publicKeys []*blst.P1Affine
	var validators []*beaconapiclient.GetValidatorByIndexResponse
	validatorIDtoValidity := make(map[int64]*validatorData)

	for _, validatorIndex := range regMessage.ValidatorIndices() {
		validatorIDtoValidity[validatorIndex] = &validatorData{validatorValidity: staticRegistrationMessageValidity}
		nonceBefore, err := tm.dbQuery.QueryValidatorRegistrationMessageNonceBefore(ctx, data.QueryValidatorRegistrationMessageNonceBeforeParams{
			ValidatorIndex:   dbTypes.Int64ToPgTypeInt8(validatorIndex),
			EventBlockNumber: int64(vr.Raw.BlockNumber),
			EventTxIndex:     int64(vr.Raw.TxIndex),
			EventLogIndex:    int64(vr.Raw.Index),
		})

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// No previous nonce means the message is valid regarding nonce
				nonceBefore = pgtype.Int8{Int64: -1, Valid: true}
			} else {
				return nil, errors.Wrapf(err, "failed to query latest nonce for validator %d", validatorIndex)
			}
		}

		if regMessage.Nonce > math.MaxInt32 || int64(regMessage.Nonce) <= nonceBefore.Int64 {
			// skip the validator
			log.Warn().
				Uint32("nonce", regMessage.Nonce).
				Int64("before-nonce", nonceBefore.Int64).
				Msg("ignoring validator with invalid nonce")
			validatorIDtoValidity[validatorIndex].validatorValidity = data.ValidatorRegistrationValidityInvalidmessage
			continue
		}
		validator, err := tm.beaconAPIClient.GetValidatorByIndex(ctx, "head", uint64(validatorIndex))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get validator %d", validatorIndex)
		}
		if validator == nil {
			// validator not found
			log.Warn().Msg("registration message for unknown validator")
			validatorIDtoValidity[validatorIndex].validatorValidity = data.ValidatorRegistrationValidityInvalidmessage
			continue
		}
		validatorIDtoValidity[validatorIndex].validatorStatus = validator.Data.Status
		publicKey, err := validator.Data.Validator.GetPubkey()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get public key of validator %d", validatorIndex)
		}
		publicKeys = append(publicKeys, publicKey)
		validators = append(validators, validator)
	}
	if len(publicKeys) > 0 {
		// now we need to check for signature verification depending on the message version
		sig := new(blst.P2Affine).Uncompress(vr.Signature)
		if sig == nil {
			return nil, fmt.Errorf("ignoring registration message with undecodable signature")
		}

		if regMessage.Version == validatorregistry.LegacyValidatorRegistrationMessageVersion {
			regMessage := new(validatorregistry.LegacyRegistrationMessage)
			err := regMessage.Unmarshal(vr.Message)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to unmarshal legacy registration message")
			}
			if valid := validatorregistry.VerifySignature(sig, publicKeys[0], regMessage); !valid {
				validatorIDtoValidity[int64(validators[0].Data.Index)].validatorValidity = data.ValidatorRegistrationValidityInvalidsignature
				log.Warn().Msg("invalid legacy registration message with invalid signature")
			}
		} else {
			if valid := validatorregistry.VerifyAggregateSignature(sig, publicKeys, regMessage); !valid {
				for _, validator := range validators {
					validatorIDtoValidity[int64(validator.Data.Index)].validatorValidity = data.ValidatorRegistrationValidityInvalidsignature
				}
				log.Warn().Msg("invalid aggregate registration message with invalid signature")
			}
		}
	}
	return validatorIDtoValidity, nil
}

func validateValidatorRegistryMessageContents(
	msg *validatorregistry.AggregateRegistrationMessage,
	chainID uint64,
	validatorRegistryContractAddress string,
) data.ValidatorRegistrationValidity {
	validity := data.ValidatorRegistrationValidityValid
	if msg.Version != validatorregistry.AggregateValidatorRegistrationMessageVersion &&
		msg.Version != validatorregistry.LegacyValidatorRegistrationMessageVersion {
		return data.ValidatorRegistrationValidityInvalidmessage
	}
	if msg.ChainID != chainID {
		return data.ValidatorRegistrationValidityInvalidmessage
	}
	if msg.ValidatorRegistryAddress.String() != validatorRegistryContractAddress {
		return data.ValidatorRegistrationValidityInvalidmessage
	}
	if msg.ValidatorIndex > math.MaxInt64 {
		return data.ValidatorRegistrationValidityInvalidmessage
	}
	return validity
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

func isFeeTooLowError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "fee too low") ||
		strings.Contains(strings.ToLower(err.Error()), "transaction underpriced")
}

// waitForReceiptWithTimeout waits for a transaction receipt with a provided timeout.
func (tm *TxMapperDB) waitForReceiptWithTimeout(ctx context.Context, txHash common.Hash, receiptWaitTimeout time.Duration, txErrorSignalCh chan error) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, receiptWaitTimeout)
	defer cancel()

	// wait for the transaction receipt
	receipt, err := tm.waitForReceipt(ctx, txHash, txErrorSignalCh)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", txHash.Hex(), err)
	}
	return receipt, nil
}

// waitForReceipt polls the Ethereum network for the transaction receipt until it's available or the context is canceled.
func (tm *TxMapperDB) waitForReceipt(ctx context.Context, txHash common.Hash, txErrorSignalCh chan error) (*types.Receipt, error) {
	for {
		// check if the context has been canceled or timed out
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-txErrorSignalCh: // Listen for errors from the sending goroutine
			if err != nil {
				return nil, err
			}
		default:
		}

		// query for the transaction receipt
		receipt, err := tm.ethClient.TransactionReceipt(ctx, txHash)
		if err == ethereum.NotFound {
			// If the receipt is not found, continue polling
			time.Sleep(3 * time.Second)
			continue
		} else if err != nil {
			return nil, err
		}

		return receipt, nil
	}
}

func (tm *TxMapperDB) UpsertGraffitiIfShutterized(ctx context.Context, validatorIndex int64, graffiti string, blockNumber int64) (bool, error) {
	upserted, err := tm.dbQuery.UpsertGraffitiIfShutterized(ctx, data.UpsertGraffitiIfShutterizedParams{
		ValidatorIndex: dbTypes.Int64ToPgTypeInt8(validatorIndex),
		Graffiti:       graffiti,
		BlockNumber:    blockNumber,
	})
	return upserted, err
}
