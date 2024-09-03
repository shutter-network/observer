package watcher

import (
	"context"
	"errors"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	sequencerBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/sequencer"
	validatorRegistryBindings "github.com/shutter-network/gnosh-contracts/gnoshcontracts/validatorregistry"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/data"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
)

const (
	//chiado network
	ChiadoChainID                                = 10200
	ChiadoGenesisTimestamp                       = 1665396300
	ChiadoSlotDuration                           = 5
	ChiadoValidatorRegistryDeploymentBlockNumber = 9884076

	//mainnet network
	GnosisMainnetChainID                         = 100
	GnosisMainnetGenesisTimestamp                = 1638993340
	GnosisMainnetSlotDuration                    = 5
	GnosisValidatorRegistryDeploymentBlockNumber = 34627171
)

var (
	GenesisTimestamp                       uint64
	SlotDuration                           uint64
	ValidatorRegistryDeploymentBlockNumber uint64
)

type Watcher struct {
	config *common.Config
	db     *pgxpool.Pool
}

func New(
	config *common.Config,
	db *pgxpool.Pool,
) *Watcher {
	return &Watcher{
		config: config,
		db:     db,
	}
}

func (w *Watcher) Start(ctx context.Context, runner service.Runner) error {
	txSubmittedEventChannel := make(chan *sequencerBindings.SequencerTransactionSubmitted)
	validatorRegistryChannel := make(chan *validatorRegistryBindings.ValidatorregistryUpdated)

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
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return err
	}
	err = setNetworkConfig(ctx, chainID)
	if err != nil {
		return err
	}
	beaconAPIClient, err := beaconapiclient.New(w.config.BeaconAPIURL)
	if err != nil {
		return err
	}

	txMapper := metrics.NewTxMapperDB(
		ctx,
		w.db,
		w.config,
		ethClient,
		beaconAPIClient,
		chainID.Int64(),
	)

	blocksWatcher := NewBlocksWatcher(w.config, blocksChannel, ethClient)
	encryptionTxWatcher := NewEncryptedTxWatcher(w.config, txSubmittedEventChannel, ethClient)

	blockNumber, err := txMapper.QueryBlockNumberFromValidatorRegistryEventsSyncedUntil(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			blockNumber = int64(ValidatorRegistryDeploymentBlockNumber)
		} else {
			return err
		}
	}

	validatorRegisterWatcher := NewValidatorRegistryWatcher(w.config, validatorRegistryChannel, ethClient, blockNumber)

	p2pMsgsWatcher := NewP2PMsgsWatcherWatcher(w.config, blocksChannel, decryptionDataChannel, keyShareChannel, txMapper)
	if err := runner.StartService(blocksWatcher, encryptionTxWatcher, p2pMsgsWatcher, validatorRegisterWatcher); err != nil {
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
				keys, identites := getDecryptionKeysAndIdentities(dd.Keys)
				err := txMapper.AddDecryptionKeysAndMessages(
					ctx,
					&metrics.DecKeysAndMessages{
						Eon:        dd.Eon,
						Keys:       keys,
						Identities: identites,
						Slot:       dd.Slot,
						InstanceID: dd.InstanceID,
						TxPointer:  dd.TxPointer,
					},
				)
				if err != nil {
					log.Err(err).Msg("err adding decryption data")
					return err
				}
				log.Info().
					Int("total decryption keys", len(dd.Keys)).
					Int64("slot", dd.Slot).
					Msg("new decryption keys received")

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
			case vr := <-validatorRegistryChannel:
				err = txMapper.AddValidatorRegistryEvent(ctx, vr)
				if err != nil {
					log.Err(err).Msg("err adding validator registry")
					return err
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	return nil
}

func setNetworkConfig(ctx context.Context, chainID *big.Int) error {
	switch chainID.Int64() {
	case ChiadoChainID:
		GenesisTimestamp = ChiadoGenesisTimestamp
		SlotDuration = ChiadoSlotDuration
		ValidatorRegistryDeploymentBlockNumber = ChiadoValidatorRegistryDeploymentBlockNumber
		return nil
	case GnosisMainnetChainID:
		GenesisTimestamp = GnosisMainnetGenesisTimestamp
		SlotDuration = GnosisMainnetSlotDuration
		ValidatorRegistryDeploymentBlockNumber = GnosisValidatorRegistryDeploymentBlockNumber
		return nil
	default:
		return errors.New("encountered unsupported chain id")
	}
}
