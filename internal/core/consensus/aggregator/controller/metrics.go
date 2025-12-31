package controller

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus 指标：观测 getParentBlockHash 调用频率、耗时与错误率
var (
	consensusAggregatorParentHashRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "consensus_aggregator_get_parent_block_hash_requests_total",
		Help: "Total number of getParentBlockHash calls.",
	})
	consensusAggregatorParentHashErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "consensus_aggregator_get_parent_block_hash_errors_total",
		Help: "Total number of getParentBlockHash errors.",
	})
	consensusAggregatorParentHashDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "consensus_aggregator_get_parent_block_hash_duration_seconds",
		Help:    "Duration of getParentBlockHash calls.",
		Buckets: prometheus.DefBuckets,
	})
	
	// 弃权指标：按原因类型统计弃权次数
	consensusAggregatorWaiverTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "consensus_aggregator_waiver_total",
			Help: "Total number of aggregator waivers by reason type.",
		},
		[]string{"reason"}, // 标签：弃权原因 (height_too_far_ahead, aggregation_in_progress, read_only_mode)
	)
)

func init() {
	prometheus.MustRegister(
		consensusAggregatorParentHashRequests,
		consensusAggregatorParentHashErrors,
		consensusAggregatorParentHashDuration,
		consensusAggregatorWaiverTotal,
	)
}

// observeParentHashDuration 记录一次调用的耗时
func observeParentHashDuration(start time.Time) {
	consensusAggregatorParentHashDuration.Observe(time.Since(start).Seconds())
}

// recordWaiver 记录弃权事件
// reason: 弃权原因标签，如 "height_too_far_ahead", "aggregation_in_progress", "read_only_mode"
func recordWaiver(reason string) {
	consensusAggregatorWaiverTotal.WithLabelValues(reason).Inc()
}


