package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/observer/internal/data"
)

type TxExecution struct {
	// BlockNumber        int64
	DecKeysAndMessages []*DecKeyAndMessage
}

type txExecutionJob struct {
	expectedIndex   int
	decryptedTx     *types.Transaction
	txSubEvent      data.TransactionSubmittedEvent
	decryptionKeyID int64
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
	expectedIdx := 0
	jobs := make([]txExecutionJob, 0, len(txSubEvents))

	err = tm.withSlotLock(slot, func() error {
		for _, txSubEvent := range txSubEvents {
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
					InclusionPosition:           InclPosUnknown,
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

			err = tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
				Slot:                        slot,
				TxIndex:                     txSubEvent.TxIndex,
				TxHash:                      decryptedTx.Hash().Bytes(),
				TxStatus:                    data.TxStatusValPending,
				InclusionPosition:           InclPosUnknown,
				DecryptionKeyID:             decryptionKeyID,
				TransactionSubmittedEventID: txSubEvent.ID,
			})
			if err != nil {
				log.Err(err).Msg("failed to create decrypted tx")
				continue
			}

			jobs = append(jobs, txExecutionJob{
				expectedIndex:   expectedIdx,
				decryptedTx:     decryptedTx,
				txSubEvent:      txSubEvent,
				decryptionKeyID: decryptionKeyID,
			})
			expectedIdx++
		}

		return nil
	})
	if err != nil {
		return err
	}

	// If the block already exists for this slot, classify now that rows are present.
	tm.maybeHandleStoredBlock(ctx, slot)

	var wg sync.WaitGroup
	for _, job := range jobs {
		currExpected := job.expectedIndex
		decryptedTx := job.decryptedTx
		txSubEvent := job.txSubEvent
		decryptionKeyID := job.decryptionKeyID

		txErrorSignalCh := make(chan error, 1)
		wg.Add(2)

		go func(ctx context.Context, decryptedTx *types.Transaction, txSubEvent data.TransactionSubmittedEvent, slot int64, decryptionKeyID int64, txErrorSignalCh chan error) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				txErrorSignalCh <- fmt.Errorf("transaction send cancelled due to context: %w", ctx.Err())
				return
			case <-time.After(time.Duration(tm.config.InclusionDelay) * time.Second):
				if tm.isDone(decryptedTx.Hash()) {
					return
				}

				if err := tm.ethClient.SendTransaction(ctx, decryptedTx); err != nil {
					log.Err(err).Msg("failed to send transaction")
					if err.Error() == "AlreadyKnown" {
						log.Debug().Hex("tx-hash", decryptedTx.Hash().Bytes()).Msg("already known")
						return
					}

					txStatus := data.TxStatusValInvalid
					if isFeeTooLowError(err) {
						txStatus = data.TxStatusValInvalidfeetoolow
					}
					err := tm.withSlotLock(slot, func() error {
						if tm.isDone(decryptedTx.Hash()) {
							return nil
						}

						return tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
							Slot:                        slot,
							TxIndex:                     txSubEvent.TxIndex,
							TxHash:                      decryptedTx.Hash().Bytes(),
							TxStatus:                    txStatus,
							InclusionPosition:           InclPosUnknown,
							DecryptionKeyID:             decryptionKeyID,
							TransactionSubmittedEventID: txSubEvent.ID,
							BlockNumber:                 pgtype.Int8{},
						})
					})
					if err != nil {
						log.Err(err).Msg("failed to upsert decrypted tx")
					}
					txErrorSignalCh <- fmt.Errorf("%w: %v", errSendTransaction, err)
					return
				}

				log.Info().Hex("tx-hash", decryptedTx.Hash().Bytes()).Msg("transaction sent")
			}
		}(ctx, decryptedTx, txSubEvent, slot, decryptionKeyID, txErrorSignalCh)

		go func(ctx context.Context, expectedIndex int, txHash common.Hash, txIndex int64, slot int64, decryptionKeyID int64, txSubEventID int64, txErrorSignalCh chan error) {
			defer wg.Done()

			if tm.isDone(txHash) {
				return
			}

			receipt, err := tm.waitForReceiptWithTimeout(ctx, txHash, ReceiptWaitTimeout, txErrorSignalCh)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, errSendTransaction) {
					log.Debug().Hex("tx-hash", txHash.Bytes()).Err(err).Msg("receipt wait stopped after send failure")
					return
				}

				log.Err(err).Hex("tx-hash", txHash.Bytes()).Msg("receipt wait failed")
				err := tm.withSlotLock(slot, func() error {
					if tm.isDone(txHash) {
						return nil
					}

					return tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
						Slot:                        slot,
						TxIndex:                     txIndex,
						TxHash:                      txHash[:],
						TxStatus:                    data.TxStatusValNotincluded,
						InclusionPosition:           InclPosUnknown,
						DecryptionKeyID:             decryptionKeyID,
						TransactionSubmittedEventID: txSubEventID,
						BlockNumber:                 pgtype.Int8{},
					})
				})
				if err != nil {
					log.Err(err).Msg("failed to upsert decrypted tx")
				}
				return
			}

			// Block classification may have completed while we were waiting.
			if tm.isDone(txHash) {
				return
			}

			log.Info().Hex("tx-hash", receipt.TxHash.Bytes()).
				Uint64("receipt-status", receipt.Status).
				Msg("transaction receipt found")

			block, err := tm.ethClient.BlockByNumber(ctx, receipt.BlockNumber)
			if err != nil {
				log.Err(err).Uint64("block-number", receipt.BlockNumber.Uint64()).Msg("failed to retrieve block")
				return
			}

			// Block classification may have completed while we were fetching the block.
			if tm.isDone(txHash) {
				return
			}

			inclusionSlot := utils.GetSlotForBlock(block.Header().Time, tm.genesisTimestamp, tm.slotDuration)
			txStatus, inclusionPosition := data.TxStatusValShieldedinclusion, InclPosUnknown
			if inclusionSlot != uint64(slot) {
				txStatus = data.TxStatusValUnshieldedinclusion
				inclusionPosition = InclPosWrong
			} else {
				txStatus, inclusionPosition = classifyInclusion(expectedIndex, receipt.TransactionIndex)
			}

			log.Info().
				Int64("expected-slot", slot).
				Uint64("receipt-slot", inclusionSlot).
				Uint("expected-index", uint(expectedIndex)).
				Uint("receipt-index", receipt.TransactionIndex).
				Hex("tx-hash", receipt.TxHash.Bytes()).
				Str("inclusion_position", inclusionPosition).
				Str("tx_status", string(txStatus)).
				Msg("transaction receipt classified")

			err = tm.withSlotLock(slot, func() error {
				if tm.isDone(txHash) {
					return nil
				}

				if err := tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
					Slot:                        slot,
					TxIndex:                     txIndex,
					TxHash:                      receipt.TxHash.Bytes(),
					TxStatus:                    txStatus,
					InclusionPosition:           inclusionPosition,
					DecryptionKeyID:             decryptionKeyID,
					TransactionSubmittedEventID: txSubEventID,
					BlockNumber:                 pgtype.Int8{Int64: receipt.BlockNumber.Int64(), Valid: true},
				}); err != nil {
					return err
				}

				tm.markDone(txHash)
				return nil
			})
			if err != nil {
				log.Err(err).Msg("failed to update decrypted tx")
			}
		}(ctx, currExpected, decryptedTx.Hash(), txSubEvent.TxIndex, slot, decryptionKeyID, txSubEvent.ID, txErrorSignalCh)
	}

	wg.Wait()
	return nil
}

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

func (tm *TxMapperDB) waitForReceipt(ctx context.Context, txHash common.Hash, txErrorSignalCh chan error) (*types.Receipt, error) {
	for {
		if tm.isDone(txHash) {
			return nil, context.Canceled
		}

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
		if errors.Is(err, ethereum.NotFound) || err == ethereum.NotFound {
			// If the receipt is not found, continue polling
			time.Sleep(3 * time.Second)
			continue
		} else if err != nil {
			return nil, err
		}

		return receipt, nil
	}
}

func isFeeTooLowError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "feetoolow") ||
		strings.Contains(strings.ToLower(err.Error()), "underpriced") ||
		strings.Contains(strings.ToLower(err.Error()), "maxfeepergaslessthanblockbasefee")
}