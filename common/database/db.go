package database

import (
	"context"
	"fmt"

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
	dataSourceName := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s password=%s",
		config.Host,
		config.Port,
		config.User,
		config.DbName,
		config.SSLMode,
		config.Password,
	)

	dbpool, err := pgxpool.New(ctx, dataSourceName)
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
