package metrics

import (
	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/discard"
	prometheus "github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const TxPoolSubsystem = "txpool"

type TxPoolMetrics struct {
	Size metrics.Gauge
}

var TxPool = NopTxPoolMetrics()

func PromTxPoolMetrics() *TxPoolMetrics {
	return &TxPoolMetrics{
		Size: prometheus.NewGaugeFrom(stdprometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: TxPoolSubsystem,
			Name:      "size",
			Help:      "Size of txpool.",
		}, []string{}),
	}
}

func NopTxPoolMetrics() *TxPoolMetrics {
	return &TxPoolMetrics{
		Size: discard.NewGauge(),
	}
}
