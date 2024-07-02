package cli

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
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
	// start services here
	ctx := context.Background()

	watcher := watcher.New(&config)
	services := []service.Service{watcher}
	if !config.NoDB {
		metrics.EnableMetrics()
		//TODO: make a decision to add host and port via cli args for metrics
		metricsServer := metrics.NewMetricsServer(&metricsCommon.MetricsServerConfig{
			Host: "localhost",
			Port: 8080,
		})
		services = append(services, metricsServer)
	}
	return service.Run(ctx, services...)
}
