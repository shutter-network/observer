package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/shutter-network/gnosh-metrics/common/utils"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
)

const propserDutiesSchedulerDuration = "* * * * *" //every minute

type ProposerDutiesScheduler struct {
	txMapper metrics.TxMapper
}

func NewProposerDutiesScheduler(txMapper metrics.TxMapper) *ProposerDutiesScheduler {
	return &ProposerDutiesScheduler{
		txMapper: txMapper,
	}
}

func (ps *ProposerDutiesScheduler) initProposerDutiesJob(ctx context.Context) *Job {
	definition := gocron.CronJob(
		propserDutiesSchedulerDuration,
		false,
	)
	task := gocron.NewTask(
		func(ctx context.Context) error {
			currentTimestamp := time.Now().Unix()
			currentSlot := utils.GetSlotNumber(uint64(currentTimestamp), GenesisTimestamp, SlotDuration)
			currentEpoch := utils.GetEpochNumber(currentSlot, SlotsPerEpoch)
			nextEpoch := currentEpoch + 1
			err := ps.txMapper.AddProposerDuties(ctx, nextEpoch)
			if err != nil {
				return err
			}
			return nil
		},
		ctx,
	)
	return &Job{
		Definition: definition,
		Task:       task,
		Options:    []gocron.JobOption{},
	}
}
