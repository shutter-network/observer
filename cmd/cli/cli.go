package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/mitchellh/mapstructure"
	metricsCommon "github.com/shutter-network/observer/common"
	"github.com/shutter-network/observer/common/database"
	"github.com/shutter-network/observer/internal/metrics"
	"github.com/shutter-network/observer/internal/scheduler"
	"github.com/shutter-network/observer/internal/watcher"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/address"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/env"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/keys"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/service"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
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
			config.P2P = &p2p.Config{}
			config.P2P.P2PKey = p2pKey
			bootstrapAddressesStringified := os.Getenv("P2P_BOOTSTRAP_ADDRESSES")
			if bootstrapAddressesStringified == "" {
				return fmt.Errorf("bootstrap addresses not provided in the env")
			}
			bootstrapAddresses := strings.Split(bootstrapAddressesStringified, ",")

			bootstrapP2PAddresses := make([]*address.P2PAddress, len(bootstrapAddresses))

			for i, addr := range bootstrapAddresses {
				bootstrapP2PAddresses[i] = address.MustP2PAddress(addr)
			}
			config.P2P.CustomBootstrapAddresses = bootstrapP2PAddresses
			config.P2P.ListenAddresses = []*address.P2PAddress{
				address.MustP2PAddress("/ip4/0.0.0.0/tcp/23003"),
				address.MustP2PAddress("/ip4/0.0.0.0/udp/23003/quic-v1"),
				address.MustP2PAddress("/ip4/0.0.0.0/udp/23003/quic-v1/webtransport"),
				address.MustP2PAddress("/ip6/::/tcp/23003"),
				address.MustP2PAddress("/ip6/::/udp/23003/quic-v1"),
				address.MustP2PAddress("/ip6/::/udp/23003/quic-v1/webtransport"),
			}

			p2pEnviroment, err := strconv.ParseInt(os.Getenv("P2P_ENVIRONMENT"), 10, 0)
			if err != nil {
				return err
			}
			config.P2P.Environment = env.Environment(p2pEnviroment)
			config.P2P.DiscoveryNamespace = os.Getenv("P2P_DISCOVERY_NAMESPACE")
			config.InclusionDelay, err = strconv.ParseInt(os.Getenv("INCLUSION_DELAY"), 10, 0)
			if err != nil {
				return err
			}
			return Start()
		},
	}

	cmd.PersistentFlags().StringVar(
		&config.RpcURL,
		"rpc-url",
		"",
		"gnosis websocket rpc url",
	)

	cmd.MarkPersistentFlagRequired("rpc-url")

	cmd.PersistentFlags().StringVar(
		&config.BeaconAPIURL,
		"beacon-api-url",
		"",
		"gnosis beacon api url",
	)

	cmd.MarkPersistentFlagRequired("beacon-api-url")

	cmd.PersistentFlags().StringVarP(
		&config.SequencerContractAddress,
		"sequencer-contract-address",
		"s",
		"",
		"sequencer contract address",
	)
	cmd.MarkPersistentFlagRequired("sequencer-contract-address")

	cmd.PersistentFlags().StringVarP(
		&config.ValidatorRegistryContractAddress,
		"validator-registry-contract-address",
		"v",
		"",
		"validator registry contract address",
	)
	cmd.MarkPersistentFlagRequired("validator-registry-contract-address")

	cmd.PersistentFlags().String("p2pkey", "", "P2P key value (base64 encoded)")
	viper.BindPFlag("p2pkey", cmd.PersistentFlags().Lookup("p2pkey"))
	cmd.MarkPersistentFlagRequired("p2pkey")

	cmd.PersistentFlags().Int64Var(
		&config.InclusionDelay,
		"inclusion-delay",
		0,
		"delay to send transaction onchain for testing purposes",
	)

	return cmd
}

func Start() error {
	log.Info().Msg("Starting Observer")
	// start services here
	ctx := context.Background()

	services := []service.Service{}
	metrics.EnableMetrics()
	// TODO: make a decision to add host and port via cli args for metrics
	metricsServer := metrics.NewMetricsServer(&metricsCommon.MetricsServerConfig{
		Host: "0.0.0.0",
		Port: 4000,
	})
	services = append(services, metricsServer)

	db, err := database.NewDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	// Ensure db is always closed
	defer func() {
		if db != nil {
			log.Info().Msg("Closing database connection")
			db.Close()
		}
	}()

	err = database.PerformMigration(ctx)
	if err != nil {
		return fmt.Errorf("failed to perform database migration: %w", err)
	}

	watcher := watcher.New(&config, db)
	scheduler := scheduler.New(&config, db)
	services = append(services, watcher, scheduler)

	// RunWithSighandler will handle SIGINT/SIGTERM
	if err := service.RunWithSighandler(ctx, services...); err != nil {
		return fmt.Errorf("service error: %w", err)
	}

	return nil
}
