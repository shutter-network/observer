package metrics

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/shutter/shlib/shcrypto"
)

type TxMapperDB struct {
	db        *pgxpool.Pool
	dbQuery   *data.Queries
	ethClient *ethclient.Client
}

func NewTxMapperDB(
	ctx context.Context,
	db *pgxpool.Pool,
	ethClient *ethclient.Client,
) TxMapper {
	return &TxMapperDB{
		db:        db,
		dbQuery:   data.New(db),
		ethClient: ethClient,
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
	dkam *DecKeysAndMessages,
) error {
	if len(dkam.Keys) == 0 {
		return nil
	}
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := tm.dbQuery.WithTx(tx)

	eons, slots, instanceIDs, txPointers, keyIndexes := getDecryptionMessageInfos(dkam)
	err = qtx.CreateDecryptionKey(ctx, data.CreateDecryptionKeyParams{
		Column1: eons,
		Column2: dkam.Identities,
		Column3: dkam.Keys,
	})
	if err != nil {
		return err
	}
	err = qtx.CreateDecryptionKeyMessage(ctx, data.CreateDecryptionKeyMessageParams{
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
		Column3: eons,
		Column4: dkam.Identities,
	})
	if err != nil {
		return err
	}
	metricsDecKeyReceived.Inc()
	return tx.Commit(ctx)
}

func (tm *TxMapperDB) AddKeyShare(ctx context.Context, dks *data.DecryptionKeyShare) error {
	err := tm.dbQuery.CreateDecryptionKeyShare(context.Background(), data.CreateDecryptionKeyShareParams{
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
		TxHash:         b.TxHash,
		Slot:           b.Slot,
	})
	if err != nil {
		return err
	}
	detailedBlock, err := tm.ethClient.BlockByNumber(ctx, big.NewInt(b.BlockNumber))
	if err != nil {
		return err
	}

	decKeysAndMessages, err := tm.dbQuery.QueryDecryptionKeysAndMessage(ctx, b.Slot)
	if err != nil {
		return err
	}
	totalDecKeysAndMessages := len(decKeysAndMessages)
	if totalDecKeysAndMessages == 0 {
		log.Debug().Int64("slot", b.Slot).Msg("no decryption keys received yet")
		return nil
	}

	txSubEvents, err := tm.dbQuery.QueryTransactionSubmittedEvent(ctx, data.QueryTransactionSubmittedEventParams{
		Eon:     decKeysAndMessages[0].Eon.Int64,
		TxIndex: decKeysAndMessages[0].TxPointer.Int64,
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
	for _, txSubEvent := range txSubEvents {
		fmt.Println("txSubEvent 1", txSubEvent.TxIndex, txSubEvent.Eon)
	}

	for _, dkam := range decKeysAndMessages {
		fmt.Println("dkam 1", dkam.TxPointer, dkam.KeyIndex, dkam.Eon)
	}

	decryptedTXHash := make([]common.Hash, len(txSubEvents))

	for index, txSubEvent := range txSubEvents {
		encryptedMsg := new(shcrypto.EncryptedMessage)
		err = encryptedMsg.Unmarshal(txSubEvent.EncryptedTransaction)
		if err != nil {
			log.Err(err).
				Int64("tx index", txSubEvent.TxIndex).
				Int64("slot", b.Slot).
				Msg("invalid encrypted message")
			decryptedTXHash[index] = common.Hash{}
			continue
		}
		dkam := decKeysAndMessages[index]
		tx, err := getDecryptedTX(dkam.Key, encryptedMsg)
		if err != nil {
			log.Err(err).
				Int64("tx index", txSubEvent.TxIndex).
				Int64("slot", b.Slot).
				Msg("invalid decrypted transaction")
			decryptedTXHash[index] = common.Hash{}
			continue
		}
		decryptedTXHash[index] = tx.Hash()
	}
	return tm.processDecryptedTransactions(ctx, decryptedTXHash, txSubEvents, detailedBlock, b.Slot)
}

func getDecryptedTX(key []byte, encryptedMsg *shcrypto.EncryptedMessage) (*types.Transaction, error) {
	decryptionKey := new(shcrypto.EpochSecretKey)
	err := decryptionKey.Unmarshal(key)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid decryption key")
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

func (tm *TxMapperDB) processDecryptedTransactions(
	ctx context.Context,
	decryptedTXHash []common.Hash,
	txSubEvents []data.TransactionSubmittedEvent,
	detailedBlock *types.Block,
	slot int64,
) error {
	var blockTxHashes []common.Hash
	for _, tx := range detailedBlock.Transactions() {
		blockTxHashes = append(blockTxHashes, tx.Hash())
	}

	log.Debug().Int64("block number", detailedBlock.Number().Int64()).
		Int64("slot", slot).
		Msg("block info while processing decrypted transactions")

	for index, txSubEvent := range txSubEvents {
		if index < len(blockTxHashes) {
			log.Debug().
				Str("decryptedTXHash", decryptedTXHash[index].Hex()).
				Str("blockTxHash", blockTxHashes[index].Hex()).
				Bool("matches", decryptedTXHash[index].Cmp(blockTxHashes[index]) == 0).
				Msg("comparing tx hash")
			if decryptedTXHash[index].Cmp(blockTxHashes[index]) == 0 {
				// it means we have it in correct order and the transaction is correct
				err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
					Slot:     slot,
					TxIndex:  txSubEvent.TxIndex,
					TxHash:   decryptedTXHash[index].Bytes(),
					TxStatus: data.TxStatusValIncluded,
				})
				if err != nil {
					return err
				}
			} else {
				// something went wrong case
				err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
					Slot:     slot,
					TxIndex:  txSubEvent.TxIndex,
					TxHash:   decryptedTXHash[index].Bytes(),
					TxStatus: data.TxStatusValNotincluded,
				})
				if err != nil {
					return err
				}
			}
		} else {
			// Mark remaining txSubEvents as missing
			log.Debug().Str("txHash", decryptedTXHash[index].Hex()).
				Msg("decryptedTXHash (missing block transaction)")
			err := tm.dbQuery.CreateDecryptedTX(ctx, data.CreateDecryptedTXParams{
				Slot:     slot,
				TxIndex:  txSubEvent.TxIndex,
				TxHash:   decryptedTXHash[index].Bytes(),
				TxStatus: data.TxStatusValNotincluded,
			})
			if err != nil {
				return err
			}
		}
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
		txPointers[index] = dkam.TxPointer
		keyIndexes[index] = int64(index)
	}

	return eons, slots, instanceIDs, txPointers, keyIndexes
}
