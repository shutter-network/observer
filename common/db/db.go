package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/common"
)

func NewDB(ctx context.Context, config *common.DBConfig) (*pgxpool.Pool, error) {
	dataSourceName := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable password=%s",
		config.Host,
		config.Port,
		config.User,
		config.Dbname,
		config.Password,
	)
	dbpool, err := pgxpool.Connect(ctx, dataSourceName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}
	defer dbpool.Close()

	if err = dbpool.Ping(context.Background()); err != nil {
		log.Err(err).
			Msg("Unable to ping database")
		return nil, err
	}

	return dbpool, nil
}
