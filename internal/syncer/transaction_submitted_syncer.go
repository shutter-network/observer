package syncer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/observer/common/database"
	"github.com/shutter-network/observer/common/utils"
	"github.com/shutter-network/observer/internal/data"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley"
)

const (
	AssumedReorgDepth    = 10
	maxRequestBlockRange = 10_000
)

type TransactionSubmittedSyncer struct {
	contract             *sequencerBindings.Sequencer
	db                   *pgxpool.Pool
	dbQuery              *data.Queries
	ethClient            *ethclient.Client
	txMapper             metrics.TxMapper
	syncStartBlockNumber uint64
	genesisTimestamp     uint64
	slotDuration         uint64
}

func NewTransactionSubmittedSyncer(
	contract *sequencerBindings.Sequencer,
	db *pgxpool.Pool,
	ethClient *ethclient.Client,
	txMapper metrics.TxMapper,
	syncStartBlockNumber uint64,
	genesisTimestamp uint64,
	slotDuration uint64,
) *TransactionSubmittedSyncer {
	return &TransactionSubmittedSyncer{
		contract:             contract,
		db:                   db,
		dbQuery:              data.New(db),
		ethClient:            ethClient,
		txMapper:             txMapper,
		syncStartBlockNumber: syncStartBlockNumber,
		genesisTimestamp:     genesisTimestamp,
		slotDuration:         slotDuration,
	}
}

func getNumReorgedBlocksForTransactionSubmitted(syncedUntil *data.QueryTransactionSubmittedEventsSyncedUntilRow, header *types.Header) int {
	shouldBeParent := header.Number.Int64() == syncedUntil.BlockNumber+1
	isParent := bytes.Equal(header.ParentHash.Bytes(), syncedUntil.BlockHash)
	isReorg := shouldBeParent && !isParent
	if !isReorg {
		return 0
	}
	// We don't know how deep the reorg is, so we make a conservative guess. Assuming higher depths
	// is safer because it means we resync a little bit more.
	depth := AssumedReorgDepth
	if syncedUntil.BlockNumber < int64(depth) {
		return int(syncedUntil.BlockNumber)
	}
	return depth
}

// resetSyncStatus clears the db from its recent history after a reorg of given depth.
func (ets *TransactionSubmittedSyncer) resetSyncStatus(ctx context.Context, numReorgedBlocks int) error {
	if numReorgedBlocks == 0 {
		return nil
	}

	tx, err := ets.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ets.dbQuery.WithTx(tx)

	syncStatus, err := qtx.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err != nil {
		return fmt.Errorf("failed to query sync status from db in order to reset it, %w", err)
	}
	if syncStatus.BlockNumber < int64(numReorgedBlocks) {
		return fmt.Errorf("detected reorg deeper (%d) than blocks synced (%d)", syncStatus.BlockNumber, numReorgedBlocks)
	}

	deleteFromInclusive := syncStatus.BlockNumber - int64(numReorgedBlocks) + 1

	ids, err := qtx.QueryTranasctionSubmittedEventIDsUsingBlock(ctx, deleteFromInclusive)
	if err != nil {
		return fmt.Errorf("failed to query transaction submitted event ids, %w", err)
	}

	err = qtx.SetTransactionSubmittedEventIDsNullForDecryptedTX(ctx, ids)
	if err != nil {
		return fmt.Errorf("unable to set ids to null, %w", err)
	}

	err = qtx.DeleteTransactionSubmittedEventFromBlockNumber(ctx, deleteFromInclusive)
	if err != nil {
		return fmt.Errorf("failed to delete transaction submitted events from db, %w", err)
	}

	// Currently, we don't have enough information in the db to populate block hash and slot.
	// However, using default values here is fine since the syncer is expected to resync
	// immediately after this function call which will set the correct values. When we do proper
	// reorg handling, we should store the full block data of the previous blocks so that we can
	// avoid this.
	newSyncedUntilBlockNumber := deleteFromInclusive - 1
	err = qtx.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockHash:   []byte{},
		BlockNumber: newSyncedUntilBlockNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to reset transaction submitted event sync status in db, %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit db transaction, %w", err)
	}

	log.Info().
		Int("depth", numReorgedBlocks).
		Int64("previous-synced-until", syncStatus.BlockNumber).
		Int64("new-synced-until", newSyncedUntilBlockNumber).
		Msg("sync status reset due to reorg")
	return nil
}

func (ets *TransactionSubmittedSyncer) HandlePotentialReorg(ctx context.Context, header *types.Header) error {
	syncedUntil, err := ets.dbQuery.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query transaction submitted events sync status, %w", err)
	}

	numReorgedBlocks := getNumReorgedBlocksForTransactionSubmitted(&syncedUntil, header)
	if numReorgedBlocks > 0 {
		return ets.resetSyncStatus(ctx, numReorgedBlocks)
	}
	return nil
}

func (ets *TransactionSubmittedSyncer) Sync(ctx context.Context, header *types.Header) error {
	if err := ets.HandlePotentialReorg(ctx, header); err != nil {
		return err
	}

	syncedUntil, err := ets.dbQuery.QueryTransactionSubmittedEventsSyncedUntil(ctx)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("failed to query transaction submitted events sync status, %v", err)
	}
	var start uint64
	if err == pgx.ErrNoRows {
		start = ets.syncStartBlockNumber
	} else {
		start = uint64(syncedUntil.BlockNumber + 1)
	}
	endBlock := header.Number.Uint64()
	log.Debug().
		Uint64("start-block", start).
		Uint64("end-block", endBlock).
		Msg("syncing transaction submitted events")
	syncRanges := medley.GetSyncRanges(start, endBlock, maxRequestBlockRange)
	for _, r := range syncRanges {
		err = ets.syncRange(ctx, r[0], r[1])
		if err != nil {
			return err
		}
	}
	return nil
}

func (ets *TransactionSubmittedSyncer) syncRange(
	ctx context.Context,
	start,
	end uint64,
) error {
	events, err := ets.fetchEvents(ctx, start, end)
	if err != nil {
		return err
	}
	header, err := ets.ethClient.HeaderByNumber(ctx, new(big.Int).SetUint64(end))
	if err != nil {
		return fmt.Errorf("failed to get execution block header by number, %v", err)
	}
	tx, err := ets.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := ets.dbQuery.WithTx(tx)

	currentBlockNumber := 0
	txIndexInBlock := 0
	if len(events) > 0 {
		currentBlockNumber = int(events[0].Raw.BlockNumber)
	}
	for i, event := range events {
		txSubmittedEventID, err := ets.txMapper.AddTransactionSubmittedEvent(ctx, tx, event)
		if err != nil {
			log.Err(err).Msg("err adding transaction submitted event")
			return err
		}

		log.Info().
			Uint64("block", event.Raw.BlockNumber).
			Hex("encrypted transaction (hex)", event.EncryptedTransaction).
			Msg("new encrypted transaction")

		// check if we are processing the event in the same block
		if currentBlockNumber == int(event.Raw.BlockNumber) {
			// just increment in event in same block so we can compare it with
			// correct transaction index inside the block
			if i > 0 {
				txIndexInBlock += 1
			}
		} else {
			// realistically this condition will hit when currentBlockNumber is less then event.Raw.Blocknumber
			// reset the index and currentBlock since we are encountering a
			// new event which will be compared from the beginning
			txIndexInBlock = 0
			currentBlockNumber = int(event.Raw.BlockNumber)
		}

		// Try to find decryption keys for this event
		dk, err := ets.dbQuery.QueryDecryptionKeyAndMessage(ctx, data.QueryDecryptionKeyAndMessageParams{
			Eon:              database.Uint64ToPgTypeInt8(event.Eon),
			IdentityPreimage: utils.ComputeIdentity(event.IdentityPrefix[:], event.Sender),
		})
		if err != nil {
			// keys not released yet
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			} else {
				log.Err(err).Msg("err querying decryption keys")
				return err
			}
		}

		// If we have decryption keys, try to decrypt and process the transaction
		if len(dk) > 0 {
			decryptionKey := dk[0].Key
			slot := dk[0].Slot
			decryptionKeyID := dk[0].ID

			// Try to decrypt the transaction
			decryptedTx, err := utils.DecryptTransaction(decryptionKey, event.EncryptedTransaction)
			if err != nil {
				// If decryption fails, for some unusual reason throw an error
				return err
			}

			decryptedTxData, err := ets.dbQuery.QueryDecryptedTX(ctx, data.QueryDecryptedTXParams{
				DecryptionKeyID: decryptionKeyID,
				TxHash:          decryptedTx.Hash().Bytes(),
			})
			if err != nil {
				log.Err(err).Msg("err querying decrypted tx from db")
				return err
			}
			// Try to get transaction receipt
			receipt, err := ets.ethClient.TransactionReceipt(ctx, decryptedTx.Hash())
			if err != nil {
				// If receipt not found, check if transaction is pending
				_, isPending, err := ets.ethClient.TransactionByHash(ctx, decryptedTx.Hash())
				if err != nil {
					err = ets.dbQuery.UpdateDecryptedTx(ctx, data.UpdateDecryptedTxParams{
						ID:                          decryptedTxData.ID,
						Slot:                        decryptedTxData.Slot,
						TxIndex:                     int64(event.TxIndex),
						TxHash:                      decryptedTxData.TxHash,
						TxStatus:                    decryptedTxData.TxStatus,
						DecryptionKeyID:             decryptedTxData.DecryptionKeyID,
						TransactionSubmittedEventID: database.Int64ToPgTypeInt8(txSubmittedEventID),
					})
					if err != nil {
						log.Err(err).Msg("failed to update decrypted tx")
						return err
					}
					continue
				}

				if isPending {
					// Transaction is pending
					err = ets.dbQuery.UpdateDecryptedTx(ctx, data.UpdateDecryptedTxParams{
						ID:                          decryptedTxData.ID,
						Slot:                        decryptedTxData.Slot,
						TxIndex:                     int64(event.TxIndex),
						TxHash:                      decryptedTxData.TxHash,
						TxStatus:                    data.TxStatusValPending,
						DecryptionKeyID:             decryptedTxData.DecryptionKeyID,
						TransactionSubmittedEventID: database.Int64ToPgTypeInt8(txSubmittedEventID),
					})
					if err != nil {
						log.Err(err).Msg("failed to update decrypted tx")
						return err
					}
				}
			} else if receipt != nil {
				// Transaction is included and has a receipt
				block, err := ets.ethClient.BlockByNumber(ctx, receipt.BlockNumber)
				if err != nil {
					log.Err(err).Uint64("block-number", receipt.BlockNumber.Uint64()).Msg("failed to retrieve block")
					return err
				}
				inclusionSlot := utils.GetSlotForBlock(block.Header().Time, ets.genesisTimestamp, ets.slotDuration)

				txStatus := data.TxStatusValShieldedinclusion
				log.Info().Uint("tx-index", receipt.TransactionIndex).
					Uint64("inclusion-slot", uint64(slot)).
					Msg("receipt data")

				log.Info().Int("index", txIndexInBlock).
					Int64("inclusion-slot", slot).
					Msg("local data")

				// Check if transaction indices match
				if receipt.TransactionIndex != uint(txIndexInBlock) {
					log.Info().Uint("tx-index", receipt.TransactionIndex).Msg("transaction index mismatch")
					txStatus = data.TxStatusValUnshieldedinclusion
				}
				// Check if slots match
				if inclusionSlot != uint64(slot) {
					log.Info().Int64("slot", slot).Msg("transaction slot mismatch")
					txStatus = data.TxStatusValUnshieldedinclusion
				}

				err = ets.dbQuery.UpdateDecryptedTx(ctx, data.UpdateDecryptedTxParams{
					ID:                          decryptedTxData.ID,
					Slot:                        decryptedTxData.Slot,
					TxIndex:                     int64(event.TxIndex),
					TxHash:                      decryptedTxData.TxHash,
					TxStatus:                    txStatus,
					DecryptionKeyID:             decryptedTxData.DecryptionKeyID,
					TransactionSubmittedEventID: database.Int64ToPgTypeInt8(txSubmittedEventID),
				})
				if err != nil {
					log.Err(err).Msg("failed to update decrypted tx")
					return err
				}
			}
		}
	}
	err = qtx.CreateTransactionSubmittedEventsSyncedUntil(ctx, data.CreateTransactionSubmittedEventsSyncedUntilParams{
		BlockNumber: int64(end),
		BlockHash:   header.Hash().Bytes(),
	})
	if err != nil {
		log.Err(err).Msg("err adding transaction submit event sync until")
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		log.Err(err).Msg("unable to commit db transaction")
		return err
	}
	log.Info().
		Uint64("start-block", start).
		Uint64("end-block", end).
		Int("num-inserted-events", len(events)).
		Msg("synced sequencer contract")
	return nil
}

func (s *TransactionSubmittedSyncer) fetchEvents(
	ctx context.Context,
	start,
	end uint64,
) ([]*sequencerBindings.SequencerTransactionSubmitted, error) {
	opts := bind.FilterOpts{
		Start:   start,
		End:     &end,
		Context: ctx,
	}
	it, err := s.contract.SequencerFilterer.FilterTransactionSubmitted(&opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction submitted events, %v", err)
	}
	events := []*sequencerBindings.SequencerTransactionSubmitted{}
	for it.Next() {
		events = append(events, it.Event)
	}
	if it.Error() != nil {
		return nil, fmt.Errorf("failed to iterate query transaction submitted events, %v", it.Error())
	}
	return events, nil
}

/*
	slot 9  =>   block 9

	slot 10  =>  block 10 => 2 seq     2 dec keys

	slot 11  => block 11  => 8 seq     8 keys yet

	...

	slot 10 => block 10    5 decrypted txs 15 normal txs = 20 txs
	for 5 dec txs =>
*/
