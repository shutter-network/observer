package main

import (
	"context"
	"fmt"
	"os"

	metricsCommon "github.com/shutter-network/gnosh-metrics/common"
	"github.com/shutter-network/gnosh-metrics/internal/runner"
	"github.com/shutter-network/gnosh-metrics/internal/watcher"
	"github.com/spf13/cobra"
)

var config metricsCommon.Config

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts fetching recent encrypted transactions and their associated decryption keys",
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}

func Start() error {

	// start watchers here

	fmt.Println("cli started")
	ctx := context.Background()

	watcher := watcher.New(&config)

	runner := runner.NewRunner(ctx)
	watcher.Start(ctx, runner)

	return nil
}

func main() {
	status := 0
	if err := Cmd().Execute(); err != nil {
		fmt.Println("failed running server")
		status = 1
	}
	os.Exit(status)
}
