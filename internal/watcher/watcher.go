package watcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

const (
	//chiado network
	ChiadoChainID          = 10200
	ChiadoGenesisTimestamp = 1665396300
	ChiadoSlotDuration     = 5

	//mainnet network
	GnosisMainnetChainID          = 100
	GnosisMainnetGenesisTimestamp = 1638993340
	GnosisMainnetSlotDuration     = 5
)

var (
	GenesisTimestamp int64
	SlotDuration     int64
)

type Watcher struct {
	config *common.Config
}

func New(config *common.Config) *Watcher {
	return &Watcher{
		config: config,
	}
}

func (w *Watcher) Start(ctx context.Context, runner service.Runner) error {
	txSubmittedEventChannel := make(chan *sequencerBindings.SequencerTransactionSubmitted)
	blocksChannel := make(chan *BlockReceivedEvent)
	decryptionDataChannel := make(chan *DecryptionKeysEvent)
	keyShareChannel := make(chan *KeyShareEvent)

	dialer := rpc.WithWebsocketDialer(websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		NetDial: (&net.Dialer{
			Timeout:   45 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
	})
	client, err := rpc.DialOptions(ctx, w.config.RpcURL, dialer)
	if err != nil {
		return err
	}

	ethClient := ethclient.NewClient(client)
	err = setNetworkConfig(ctx, ethClient)
	if err != nil {
		return err
	}
	txMapper, err := getTxMapperImpl(ctx, w.config)
	if err != nil {
		return err
	}
	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel, ethClient)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, txSubmittedEventChannel, ethClient)
	p2pMsgsWatcher := NewP2PMsgsWatcherWatcher(w.config, blocksChannel, decryptionDataChannel, keyShareChannel, txMapper)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher, p2pMsgsWatcher); err != nil {
		return err
	}
	runner.Go(func() error {
		for {
			select {
			case txEvent := <-txSubmittedEventChannel:
				err := txMapper.AddTransactionSubmittedEvent(ctx, &data.TransactionSubmittedEvent{
					EventBlockHash:       txEvent.Raw.BlockHash[:],
					EventBlockNumber:     int64(txEvent.Raw.BlockNumber),
					EventTxIndex:         int64(txEvent.Raw.TxIndex),
					EventLogIndex:        int64(txEvent.Raw.Index),
					Eon:                  int64(txEvent.Eon),
					TxIndex:              int64(txEvent.TxIndex),
					IdentityPrefix:       txEvent.IdentityPrefix[:],
					Sender:               txEvent.Sender[:],
					EncryptedTransaction: txEvent.EncryptedTransaction,
				})
				if err != nil {
					log.Err(err).Msg("err adding encrypting transaction")
					return err
				}
				log.Info().
					Bytes("encrypted transaction", txEvent.EncryptedTransaction).
					Msg("new encrypted transaction")
			case dd := <-decryptionDataChannel:
				for index, key := range dd.Keys {
					err := txMapper.AddDecryptionKeyAndMessage(
						ctx,
						&data.DecryptionKey{
							Eon:              dd.Eon,
							IdentityPreimage: key.Identity,
							Key:              key.Key,
						},
						&data.DecryptionKeysMessage{
							Slot:       dd.Slot,
							InstanceID: dd.InstanceID,
							Eon:        dd.Eon,
							TxPointer:  dd.TxPointer,
						},
						&data.DecryptionKeysMessageDecryptionKey{
							DecryptionKeysMessageSlot:     dd.Slot,
							KeyIndex:                      int64(index),
							DecryptionKeyEon:              dd.Eon,
							DecryptionKeyIdentityPreimage: key.Identity,
						},
					)
					if err != nil {
						log.Err(err).Msg("err adding decryption data")
						return err
					}
					log.Info().
						Bytes("decryption keys", key.Key).
						Int64("slot", dd.Slot).
						Msg("new decryption key")
				}
			case ks := <-keyShareChannel:
				for _, share := range ks.Shares {
					err := txMapper.AddKeyShare(ctx, &data.DecryptionKeyShare{
						Eon:                ks.Eon,
						IdentityPreimage:   share.EpochID,
						KeyperIndex:        ks.KeyperIndex,
						DecryptionKeyShare: share.Share,
						Slot:               ks.Slot,
					})
					if err != nil {
						log.Err(err).Msg("err adding key shares")
						return err
					}
					log.Info().
						Bytes("key shares", share.Share).
						Int64("slot", ks.Slot).
						Msg("new key shares")
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	return nil
}

func getTxMapperImpl(ctx context.Context, config *common.Config) (metrics.TxMapper, error) {
	var txMapper metrics.TxMapper

	if config.NoDB {
		txMapper = metrics.NewTxMapperMemory()
	} else {
		var (
			host     = os.Getenv("DB_HOST")
			port     = os.Getenv("DB_PORT")
			user     = os.Getenv("DB_USER")
			password = os.Getenv("DB_PASSWORD")
			dbName   = os.Getenv("DB_NAME")
			sslMode  = os.Getenv("DB_SSL_MODE")
		)
		dbAddr := fmt.Sprintf("%s:%s", host, port)
		if sslMode == "" {
			sslMode = "disable"
		}
		databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s", user, password, dbAddr, dbName, sslMode)
		dbConfig := common.DBConfig{
			DatabaseURL: databaseURL,
		}
		db, err := database.NewDB(ctx, &dbConfig)
		if err != nil {
			return nil, err
		}

		migrationConn, err := sql.Open("pgx", databaseURL)
		if err != nil {
			return nil, err
		}

		migrationsPath := os.Getenv("MIGRATIONS_PATH")
		if migrationsPath == "" {
			// default to the relative path used in locally
			_, curFile, _, _ := runtime.Caller(0)
			curDir := path.Dir(curFile)
			migrationsPath = curDir + "/../../migrations"
		}

		err = goose.RunContext(ctx, "up", migrationConn, migrationsPath)
		if err != nil {
			return nil, err
		}
		txMapper = metrics.NewTxMapperDB(ctx, db)
	}
	return txMapper, nil
}

func setNetworkConfig(ctx context.Context, ethClient *ethclient.Client) error {
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return err
	}

	switch chainID.Int64() {
	case ChiadoChainID:
		GenesisTimestamp = ChiadoGenesisTimestamp
		SlotDuration = ChiadoSlotDuration
		return nil
	case GnosisMainnetChainID:
		GenesisTimestamp = GnosisMainnetGenesisTimestamp
		SlotDuration = GnosisMainnetSlotDuration
		return nil
	default:
		return errors.New("encountered unsupported chain id")
	}
}
