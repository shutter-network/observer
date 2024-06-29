package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/metrics"
	"github.com/shutter-network/gnosh-metrics/internal/watcher"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/keys"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var config metricsCommon.Config

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts fetching recent encrypted transactions and their associated decryption keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			var p2pKey *keys.Libp2pPrivate
			if err := viper.UnmarshalKey("p2pkey", &p2pKey, viper.DecodeHook(mapstructure.TextUnmarshallerHookFunc())); err != nil {
				fmt.Println("Error unmarshalling P2PKey:", err)
				return err
			}
			config.P2P.P2PKey = p2pKey
			return Start()
		},
	}

	cmd.PersistentFlags().StringVarP(
		&config.RpcURL,
		"rpc-url",
		"",
		"wss://rpc.chiadochain.net/wss",
		"gnosis testnet rpc url",
	)

	cmd.PersistentFlags().StringVarP(
		&config.ContractAddress,
		"contract-address",
		"",
		"0xd073BD5A717Dce1832890f2Fdd9F4fBC4555e41A",
		"sequencer contract address",
	)

	cmd.PersistentFlags().BoolVar(
		&config.NoDB,
		"no-db",
		false,
		"use memory storage instead of database",
	)

	cmd.PersistentFlags().String("p2pkey", "", "P2P key value (base64 encoded)")
	viper.BindPFlag("p2pkey", cmd.PersistentFlags().Lookup("p2pkey"))

	config.BuildDefaultP2PConfig()
	return cmd
}

func Start() error {
	// start watchers here
	ctx := context.Background()

	go runPromMetrics(&config)
	watcher := watcher.New(&config)
	return service.Run(ctx, watcher)
}

func runPromMetrics(config *metricsCommon.Config) {
	if !config.NoDB {
		metrics.EnableMetrics()

		http.Handle("/metrics", promhttp.Handler())
		log.Info().Msg("Starting metrics server at :3000")
		if err := http.ListenAndServe(":3000", nil); err != nil {
			log.Err(err).Msg("error starting server at port 2112")
		}
	}
}
