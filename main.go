package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/shutter-network/gnosh-metrics/cmd/cli"

	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Info().Any("pprof", http.ListenAndServe("0.0.0.0:6060", nil)).Msg("started pprof")
	}()
	status := 0
	if err := cli.Cmd().Execute(); err != nil {
		log.Info().Err(err).Msg("failed running server")
		status = 1
	}
	os.Exit(status)
}
