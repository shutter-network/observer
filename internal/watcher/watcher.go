package watcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
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
	GenesisTimestamp = 0
	SlotDuration     = 0
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
	encryptedTxChannel := make(chan *EncryptedTxReceivedEvent)
	blocksChannel := make(chan *BlockReceivedEvent)
	decryptionDataChannel := make(chan *DecryptionKeysEvent)
	keyShareChannel := make(chan *KeyShareEvent)

	ethClient, err := ethclient.Dial(w.config.RpcURL)
	if err != nil {
		return err
	}

	err = setNetworkConfig(ctx, ethClient)
	if err != nil {
		return err
	}
	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel, ethClient)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, encryptedTxChannel, ethClient)
	decryptionKeysWatcher := NewP2PMsgsWatcherWatcher(w.config, blocksChannel, decryptionDataChannel, keyShareChannel)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher, decryptionKeysWatcher); err != nil {
		return err
	}

	txMapper, err := getTxMapperImpl(w.config)
	if err != nil {
		return err
	}

	// runner.Go(func() error {
	for {
		select {
		case block := <-blocksChannel:
			slot := getSlotForBlock(block.Header)
			err := txMapper.AddBlockHash(slot, block.Header.Hash())
			if err != nil {
				log.Err(err).Msg("err adding block hash")
				return err
			}
		case enTx := <-encryptedTxChannel:
			identityPreimage := computeIdentityPreimage(enTx.IdentityPrefix[:], enTx.Sender)
			err := txMapper.AddEncryptedTx(identityPreimage, enTx.Tx)
			if err != nil {
				log.Err(err).Msg("err adding encrypting transaction")
				return err
			}
			log.Info().
				Bytes("encrypted transaction", enTx.Tx).
				Msg("new encrypted transaction")
		case dd := <-decryptionDataChannel:
			for _, key := range dd.Keys {
				err := txMapper.AddDecryptionData(key.Identity, &metrics.DecryptionData{
					Key:  key.Key,
					Slot: dd.Slot,
				})
				if err != nil {
					log.Err(err).Msg("err adding decryption data")
					return err
				}
				log.Info().
					Bytes("decryption keys", key.Key).
					Uint64("slot", dd.Slot).
					Msg("new decryption key")
			}
		case ks := <-keyShareChannel:
			for _, share := range ks.Shares {
				err := txMapper.AddKeyShare(share.EpochID, &metrics.KeyShare{
					Share: share.Share,
					Slot:  ks.Slot,
				})
				if err != nil {
					log.Err(err).Msg("err adding key shares")
					return err
				}
				log.Info().
					Bytes("key shares", share.Share).
					Uint64("slot", ks.Slot).
					Msg("new key shares")
			}
		}
	}
	// })
	// return nil
}

func getTxMapperImpl(config *common.Config) (metrics.TxMapper, error) {
	var txMapper metrics.TxMapper

	if config.NoDB {
		txMapper = metrics.NewTxMapperMemory()
	} else {
		ctx := context.Background()
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
		_, curFile, _, _ := runtime.Caller(0)
		curDir := path.Dir(curFile)

		migrationsPath := curDir + "/../../migrations"
		err = goose.RunContext(ctx, "up", migrationConn, migrationsPath)
		if err != nil {
			return nil, err
		}
		txManager := database.NewTxManager(db)
		encryptedTxRepo := data.NewEncryptedTxRepository(db)
		decryptionDataRepo := data.NewDecryptionDataRepository(db)
		keyShareRepo := data.NewKeyShareRepository(db)
		txMapper = metrics.NewTxMapperDB(encryptedTxRepo, decryptionDataRepo, keyShareRepo, txManager)
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
