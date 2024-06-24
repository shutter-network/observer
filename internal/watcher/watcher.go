package watcher

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/common/database"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

type Watcher struct {
	config *common.Config
}

func New(config *common.Config) *Watcher {
	return &Watcher{
		config: config,
	}
}

func (w *Watcher) Start(_ context.Context, runner service.Runner) error {
	encryptedTxChannel := make(chan *EncryptedTxReceivedEvent)
	blocksChannel := make(chan *BlockReceivedEvent)
	decryptionDataChannel := make(chan *DecryptionKeysEvent)
	keyShareChannel := make(chan *KeyShareEvent)

	ethClient, err := ethclient.Dial(w.config.RpcURL)
	if err != nil {
		return err
	}

	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel, ethClient)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, encryptedTxChannel, ethClient)
	decryptionKeysWatcher := NewDecryptionKeysWatcher(w.config, blocksChannel, decryptionDataChannel)
	KeyShareWatcher := NewKeyShareWatcher(w.config, keyShareChannel)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher, decryptionKeysWatcher, KeyShareWatcher); err != nil {
		return err
	}

	txMapper, err := getTxMapperImpl(w.config)
	if err != nil {
		return err
	}
	for {
		select {

		case block := <-blocksChannel:
			slot := getSlotForBlock(block.Header)
			err := txMapper.AddBlockHash(slot, block.Header.Hash().Bytes())
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
		}
	}
}

func getTxMapperImpl(config *common.Config) (metrics.ITxMapper, error) {
	var txMapper metrics.ITxMapper

	if config.NoDB {
		txMapper = metrics.NewTxMapper()
	} else {
		err := godotenv.Load(".envrc")
		if err != nil {
			return nil, fmt.Errorf("error loading .envrc file: %w", err)
		}
		ctx := context.Background()
		dbConf := &common.DBConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     os.Getenv("DB_PORT"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DbName:   os.Getenv("DB_NAME"),
			SSLMode:  os.Getenv("DB_SSL_MODE"),
		}
		db, err := database.NewDB(ctx, dbConf)
		if err != nil {
			return nil, err
		}
		dbAddr := fmt.Sprintf("%s:%s", dbConf.Host, dbConf.Port)
		databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbConf.User, dbConf.Password, dbAddr, dbConf.DbName)

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
		transactionRepo := data.NewTransactionRepository(db)
		txMapper = metrics.NewTxMapperDB(transactionRepo, txManager)
	}
	return txMapper, nil
}
