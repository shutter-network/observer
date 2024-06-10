package common

import (
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/address"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/medley/encodeable/env"
	"github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"
)

type Config struct {
	RpcURL          string
	ContractAddress string
	P2P             *p2p.Config
}

func (config *Config) BuildDefaultP2PConfig() {
	config.P2P = &p2p.Config{}
	config.P2P.CustomBootstrapAddresses = []*address.P2PAddress{address.MustP2PAddress("/dns4/207.154.243.191/tcp/23000/p2p/12D3KooWPjX9v7FWmPvAUSTMpG7j2jXWxNnUxyDZrXPqmK29QNEd")}
	config.P2P.ListenAddresses = []*address.P2PAddress{
		address.MustP2PAddress("/ip4/0.0.0.0/tcp/0"),
		address.MustP2PAddress("/ip4/0.0.0.0/udp/0/quic-v1"),
		address.MustP2PAddress("/ip4/0.0.0.0/udp/0/quic-v1/webtransport"),
		address.MustP2PAddress("/ip6/::/tcp/0"),
		address.MustP2PAddress("/ip6/::/udp/0/quic-v1"),
		address.MustP2PAddress("/ip6/::/udp/0/quic-v1/webtransport"),
	}

	config.P2P.Environment = env.EnvironmentLocal
	config.P2P.DiscoveryNamespace = "shutter-60"
}
