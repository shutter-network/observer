package common

import (
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type Config struct {
	RpcURL                           string
	BeaconAPIURL                     string
	SequencerContractAddress         string
	ValidatorRegistryContractAddress string
	P2P                              *p2p.Config
	InclusionDelay                   int64
}

type MetricsServerConfig struct {
	Host string
	Port uint16
}
