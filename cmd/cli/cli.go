package cli

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/runner"
	"github.com/shutter-network/gnosh-metrics/internal/watcher"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/keys"
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
			if err := viper.UnmarshalKey("p2pKey", &p2pKey, viper.DecodeHook(mapstructure.TextUnmarshallerHookFunc())); err != nil {
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
		"",
		"sequencer contract address",
	)

	cmd.PersistentFlags().String("p2pkey", "", "P2P key value (base64 encoded)")
	viper.BindPFlag("p2pkey", cmd.PersistentFlags().Lookup("p2pkey"))

	config.BuildDefaultP2PConfig()
	return cmd
}

func Start() error {

	// start watchers here
	ctx := context.Background()

	watcher := watcher.New(&config)

	runner := runner.NewRunner(ctx)
	watcher.Start(ctx, runner)

	return nil
}
