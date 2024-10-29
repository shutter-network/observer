package scheduler

import (
	"context"
	"errors"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-co-op/gocron/v2"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/beaconapiclient"
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
	GenesisTimestamp uint64
	SlotDuration     uint64
	SlotsPerEpoch    uint64 = 16
)

type Job struct {
	Definition gocron.JobDefinition
	Task       gocron.Task
	Options    []gocron.JobOption
}

type Scheduler struct {
	config *common.Config
	db     *pgxpool.Pool
	jobs   []*Job
}

func New(
	config *common.Config,
	db *pgxpool.Pool,
) *Scheduler {
	return &Scheduler{
		config: config,
		db:     db,
		jobs:   []*Job{},
	}
}

func (s *Scheduler) AddJob(job *Job) {
	s.jobs = append(s.jobs, job)
}

func (s *Scheduler) Start(ctx context.Context, runner service.Runner) error {
	dialer := rpc.WithWebsocketDialer(websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		NetDial: (&net.Dialer{
			Timeout:   45 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
	})
	client, err := rpc.DialOptions(ctx, s.config.RpcURL, dialer)
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
	beaconAPIClient, err := beaconapiclient.New(s.config.BeaconAPIURL)
	if err != nil {
		return err
	}

	txMapper := metrics.NewTxMapperDB(
		ctx,
		s.db,
		s.config,
		ethClient,
		beaconAPIClient,
		chainID.Int64(),
		GenesisTimestamp,
		SlotDuration,
	)

	validatorStatusScheduler := NewValidatorStatusScheduler(txMapper)
	validatorStatusJob := validatorStatusScheduler.initValidatorStatusJob(ctx)
	s.AddJob(validatorStatusJob)

	sch, err := gocron.NewScheduler()
	if err != nil {
		return err
	}
	for _, job := range s.jobs {
		j, err := sch.NewJob(job.Definition, job.Task, job.Options...)
		if err != nil {
			return err
		}

		log.Debug().Str("id", j.ID().String()).Msg("scheduler job")
	}

	sch.Start()

	runner.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				err = sch.Shutdown()
				if err != nil {
					return err
				}
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
		return nil
	case GnosisMainnetChainID:
		GenesisTimestamp = GnosisMainnetGenesisTimestamp
		SlotDuration = GnosisMainnetSlotDuration
		return nil
	default:
		return errors.New("encountered unsupported chain id")
	}
}
