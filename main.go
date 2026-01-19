package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/shutter-network/observer/cmd/cli"
	"github.com/shutter-network/observer/common"
)

func main() {
	common.SetupLogging()
	status := 0
	if err := cli.Cmd().Execute(); err != nil {
		log.Info().Err(err).Msg("failed running server")
		status = 1
	}
	os.Exit(status)
}
