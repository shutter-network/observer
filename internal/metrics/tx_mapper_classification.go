package metrics

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/shutter-network/observer/internal/data"
)

const (
	InclPosUnknown = "unknown"
	InclPosExact   = "exact"
	InclPosLater   = "later"
	InclPosEarlier = "earlier"
	InclPosWrong   = "wrong_slot"
)

func classifyInclusion(expectedIndex int, receiptIndex uint) (data.TxStatusVal, string) {
	switch {
	case receiptIndex == uint(expectedIndex):
		return data.TxStatusValShieldedinclusion, InclPosExact
	case receiptIndex > uint(expectedIndex):
		return data.TxStatusValUnshieldedinclusion, InclPosLater
	default:
		return data.TxStatusValTentativeshieldedinclusion, InclPosEarlier
	}
}

type batchEntry struct {
	hash             common.Hash
	status           data.TxStatusVal
	txIndex          int64
	decryptionKeyID  int64
	submittedEventID int64
}

func classifyWithPredecessors(expectedPos, blockPos int, entries []batchEntry) (data.TxStatusVal,
	string) {
	// later index always unshielded
	if blockPos > expectedPos {
		return data.TxStatusValUnshieldedinclusion, InclPosLater
	}

	pos := InclPosExact
	if blockPos < expectedPos {
		pos = InclPosEarlier
	}

	badPredecessor := false
	allShieldedBefore := true
	seenTentative := false
	for i := 0; i < expectedPos; i++ {
		switch entries[i].status {
		case data.TxStatusValShieldedinclusion:
			// ok
		case data.TxStatusValTentativeshieldedinclusion:
			allShieldedBefore = false
			seenTentative = true
		case data.TxStatusValUnshieldedinclusion:
			badPredecessor = true
		default:
			allShieldedBefore = false
		}
		if badPredecessor {
			break
		}
	}

	if badPredecessor {
		return data.TxStatusValUnshieldedinclusion, pos
	}
	if pos == InclPosExact && allShieldedBefore {
		return data.TxStatusValShieldedinclusion, pos
	}
	// exact with tentative predecessor, or earlier with good predecessors -> ambiguous (tentative)
	_ = seenTentative // kept for readability; not used in decision
	return data.TxStatusValTentativeshieldedinclusion, pos
}

// HandleBlock performs predecessor-aware classification at block arrival.
// Receipt-based classification is a fallback and skips if a tx is already finalized.
func (tm *TxMapperDB) HandleBlock(ctx context.Context, blockNumber int64, slot int64, txs types.Transactions) error {
	if len(txs) == 0 {
		return nil
	}

	log.Debug().
		Int64("slot", slot).
		Int64("block_number", blockNumber).
		Int("num_txs", len(txs)).
		Msg("handling block for tx classification")

	return tm.withSlotLock(slot, func() error {
		rows, err := tm.db.Query(ctx, `
			SELECT tx_hash, tx_index, tx_status, decryption_key_id, transaction_submitted_event_id
			FROM decrypted_tx
			WHERE slot = $1
			  AND tx_hash <> '\x00'
			ORDER BY tx_index`, slot)
		if err != nil {
			return err
		}
		defer rows.Close()

		var entries []batchEntry
		indexByHash := make(map[string]int)

		for rows.Next() {
			var (
				hashBytes []byte
				txIdx     int64
				status    data.TxStatusVal
				decID     int64
				subID     int64
			)
			if err := rows.Scan(&hashBytes, &txIdx, &status, &decID, &subID); err != nil {
				return err
			}
			h := common.BytesToHash(hashBytes)
			indexByHash[h.Hex()] = len(entries)
			entries = append(entries, batchEntry{
				hash:             h,
				status:           status,
				txIndex:          txIdx,
				decryptionKeyID:  decID,
				submittedEventID: subID,
			})
		}

		log.Debug().
			Int64("slot", slot).
			Int("num_candidates", len(entries)).
			Msg("loaded decrypted tx candidates for block classification")

		if len(entries) == 0 {
			return nil
		}

		for blockPos, tx := range txs {
			h := tx.Hash()
			expectedPos, ok := indexByHash[h.Hex()]
			if !ok {
				continue
			}

			status, pos := classifyWithPredecessors(expectedPos, blockPos, entries)
			entries[expectedPos].status = status

			log.Debug().
				Int64("slot", slot).
				Int64("block_number", blockNumber).
				Int("block_pos", blockPos).
				Int("expected_pos", expectedPos).
				Int64("tx_index", entries[expectedPos].txIndex).
				Hex("tx_hash", h.Bytes()).
				Str("tx_status", string(status)).
				Str("inclusion_position", pos).
				Msg("classified tx from block body")

			if err := tm.dbQuery.UpsertTX(ctx, data.UpsertTXParams{
				Slot:                        slot,
				TxIndex:                     entries[expectedPos].txIndex,
				TxHash:                      h.Bytes(),
				TxStatus:                    status,
				InclusionPosition:           pos,
				DecryptionKeyID:             entries[expectedPos].decryptionKeyID,
				TransactionSubmittedEventID: entries[expectedPos].submittedEventID,
				BlockNumber:                 pgtype.Int8{Int64: blockNumber, Valid: true},
			}); err != nil {
				log.Err(err).Hex("tx-hash", h.Bytes()).Msg("failed to upsert tx from block body")
				continue
			}

			tm.markDone(h)
		}

		return nil
	})
}

func (tm *TxMapperDB) maybeHandleStoredBlock(ctx context.Context, slot int64) {
	storedBlock, err := tm.dbQuery.QueryBlockFromSlot(ctx, slot)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) { // handling the error check properly
			log.Err(err).Int64("slot", slot).Msg("failed to query stored block")
		}
		return
	}

	block, err := tm.ethClient.BlockByNumber(ctx, big.NewInt(storedBlock.BlockNumber))
	if err != nil {
		log.Err(err).
			Int64("slot", slot).
			Int64("block_number", storedBlock.BlockNumber).
			Msg("failed to fetch stored block")
		return
	}

	if err := tm.HandleBlock(ctx, storedBlock.BlockNumber, slot, block.Transactions()); err != nil {
		log.Err(err).
			Int64("slot", slot).
			Int64("block_number", storedBlock.BlockNumber).
			Msg("failed to handle stored block after decryption")
	}
}