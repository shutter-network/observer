package common

import "github.com/shutter-network/rolling-shutter/rolling-shutter/p2p"

type Config struct {
	RpcURL          string
	ContractAddress string
	P2P             *p2p.Config
}
