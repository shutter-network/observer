// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsEncTxReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "encrypted_tx_received_total",
		Help: "Total encrypted transactions fetched from sequencer event",
	})
	metricsDecKeyReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "decryption_keys_received_total",
		Help: "Total decryption key fetched from p2p",
	})
	metricsKeyShareReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "key_share_received_total",
		Help: "Total key share fetched from p2p",
	})
	metricsShutterTxIncludedInBlock = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "shutter_tx_should_be_included_in_block_total",
		Help: "Total shutterized txs included in the block",
	})
)

func EnableMetrics() {
	prometheus.MustRegister(
		metricsEncTxReceived,
		metricsDecKeyReceived,
		metricsKeyShareReceived,
		metricsShutterTxIncludedInBlock,
	)
}
