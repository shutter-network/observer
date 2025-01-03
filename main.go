package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/cmd/cli"
)

func main() {
	status := 0
	if err := cli.Cmd().Execute(); err != nil {
		log.Info().Err(err).Msg("failed running server")
		status = 1
	}
	os.Exit(status)
}
