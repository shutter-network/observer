package scheduler

import (
	"context"

	"github.com/go-co-op/gocron/v2"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
)

const validatorStatusSchedulerDuration = "0 0 * * *" //midnight(12:00 am) each day

type ValidatorStatusScheduler struct {
	txMapper metrics.TxMapper
}

func NewValidatorStatusScheduler(txMapper metrics.TxMapper) *ValidatorStatusScheduler {
	return &ValidatorStatusScheduler{
		txMapper: txMapper,
	}
}

func (vs *ValidatorStatusScheduler) initValidatorStatusJob(ctx context.Context) *Job {
	definition := gocron.CronJob(
		validatorStatusSchedulerDuration,
		false,
	)

	task := gocron.NewTask(
		func(ctx context.Context) error {
			err := vs.txMapper.UpdateValidatorStatus(ctx)
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
