package metrics

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	dbTypes "github.com/shutter-network/observer/common/database"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/validatorregistry"
	blst "github.com/supranational/blst/bindings/go"
)

type validatorData struct {
	validatorStatus   string
	validatorValidity data.ValidatorRegistrationValidity
}

func (tm *TxMapperDB) QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx context.Context) (int64, error) {
	data, err := tm.dbQuery.QueryValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil {
		return 0, err
	}
	return data.BlockNumber, nil
}

func (tm *TxMapperDB) UpsertGraffitiIfShutterized(ctx context.Context, validatorIndex int64, graffiti string, blockNumber int64) (bool, error) {
	upserted, err := tm.dbQuery.UpsertGraffitiIfShutterized(ctx, data.UpsertGraffitiIfShutterizedParams{
		ValidatorIndex: dbTypes.Int64ToPgTypeInt8(validatorIndex),
		Graffiti:       graffiti,
		BlockNumber:    blockNumber,
	})
	return upserted, err
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

func validateValidatorRegistryMessageContents(msg *validatorregistry.AggregateRegistrationMessage, chainID uint64, validatorRegistryContractAddress string) data.ValidatorRegistrationValidity {
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