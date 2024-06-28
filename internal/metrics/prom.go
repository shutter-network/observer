// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	encTxGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "encrypted_tx_value",
		Help: "Encypted transactions fetched from sequencer event",
	})
	decKeyGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "decryption_keys_value",
		Help: "Decryption key fetched from p2p",
	})
	keyShareGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "key_share_value",
		Help: "Key share fetched from p2p",
	})
)

func init() {
	prometheus.MustRegister(encTxGauge, decKeyGauge, keyShareGauge)
}

func SetEncTxMetrics(value float64) {
	encTxGauge.Set(value)
}

func SetDecKeyMetrics(value float64) {
	decKeyGauge.Set(value)
}

func SetKeyShareMetrics(value float64) {
	keyShareGauge.Set(value)
}
