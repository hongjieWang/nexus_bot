package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RPCRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_rpc_requests_total",
		Help: "The total number of RPC requests made",
	})

	RPCErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_rpc_errors_total",
		Help: "The total number of RPC request errors",
	})

	TokensFoundTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bot_tokens_found_total",
		Help: "The total number of tokens identified",
	})

	ScoringDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "bot_scoring_duration_seconds",
		Help:    "Histogram of token scoring duration in seconds",
		Buckets: prometheus.DefBuckets,
	})

	TradesExecutedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bot_trades_executed_total",
		Help: "Total number of trades executed by status (success/failure)",
	}, []string{"status"})

	AlphaClustersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alpha_clusters_total",
		Help: "Current number of smart wallet clusters identified",
	})

	SybilFilteredWallets = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sybil_filtered_wallets",
		Help: "Current number of wallets filtered/flagged as sybil matrix",
	})
)

// RecordTrade 记录交易执行结果
func RecordTrade(status string) {
	TradesExecutedTotal.WithLabelValues(status).Inc()
}
