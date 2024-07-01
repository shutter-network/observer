package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
)

type SortDirection string

const (
	DESC SortDirection = "DESC"
	ASC  SortDirection = "ASC"
)

func NewDB(ctx context.Context, config *common.DBConfig) (*pgxpool.Pool, error) {
	dbpool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	if err = dbpool.Ping(ctx); err != nil {
		log.Err(err).
			Msg("Unable to ping database")
		return nil, err
	}

	return dbpool, nil
}
