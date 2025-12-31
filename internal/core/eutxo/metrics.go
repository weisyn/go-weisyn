// Package eutxo 提供UTXO相关的监控指标
package eutxo

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weisyn/v1/internal/core/eutxo/health"
)

// ============================================================================
//                          Prometheus 监控指标
// ============================================================================

var (
	// utxoCorruptCount 损坏UTXO数量（当前值）
	utxoCorruptCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "corrupt_count",
		Help:      "Current number of UTXOs with BlockHeight=0",
	})

	// utxoRepairTotal 已修复UTXO总数（累计值）
	utxoRepairTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "repair_total",
		Help:      "Total number of repaired UTXOs since node start",
	})

	// utxoHealthCheckDuration 健康检查耗时（直方图）
	utxoHealthCheckDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "health_check_duration_seconds",
		Help:      "Duration of UTXO health checks in seconds",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s ~ 51.2s
	})

	// utxoHealthCheckTotal 健康检查总次数（按结果分类）
	utxoHealthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "wes",
			Subsystem: "utxo",
			Name:      "health_check_total",
			Help:      "Total number of UTXO health checks by result",
		},
		[]string{"result"}, // success, failed
	)

	// utxoTotalCount UTXO集总数
	utxoTotalCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "total_count",
		Help:      "Total number of UTXOs in the set",
	})

	// utxoUnrepairableCount 无法修复的UTXO数量
	utxoUnrepairableCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "unrepairable_count",
		Help:      "Number of UTXOs that cannot be automatically repaired",
	})

	// snapshotRepairTotal 快照创建期间修复的UTXO总数
	snapshotRepairTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "utxo",
		Name:      "snapshot_repair_total",
		Help:      "Total number of UTXOs repaired during snapshot creation",
	})
)

// ============================================================================
//                          指标注册
// ============================================================================

func init() {
	// 注册所有UTXO相关指标
	prometheus.MustRegister(
		utxoCorruptCount,
		utxoRepairTotal,
		utxoHealthCheckDuration,
		utxoHealthCheckTotal,
		utxoTotalCount,
		utxoUnrepairableCount,
		snapshotRepairTotal,
	)
}

// ============================================================================
//                          指标更新函数
// ============================================================================

// UpdateMetrics 更新健康检查相关指标
//
// 参数：
//   - report: 健康检查报告
func UpdateMetrics(report *health.HealthReport) {
	if report == nil {
		return
	}

	// 更新各项指标
	utxoTotalCount.Set(float64(report.TotalUTXOs))
	utxoCorruptCount.Set(float64(report.CorruptUTXOs - report.RepairedUTXOs))
	utxoRepairTotal.Add(float64(report.RepairedUTXOs))
	utxoUnrepairableCount.Set(float64(report.UnrepairableUTXOs))

	// 记录健康检查耗时
	duration := report.EndTime.Sub(report.StartTime).Seconds()
	utxoHealthCheckDuration.Observe(duration)

	// 记录健康检查结果
	if report.UnrepairableUTXOs == 0 {
		utxoHealthCheckTotal.WithLabelValues("success").Inc()
	} else {
		utxoHealthCheckTotal.WithLabelValues("failed").Inc()
	}
}

// RecordSnapshotRepair 记录快照创建期间的UTXO修复
//
// 参数：
//   - count: 修复的UTXO数量
func RecordSnapshotRepair(count int) {
	if count > 0 {
		snapshotRepairTotal.Add(float64(count))
	}
}

// ============================================================================
//                          指标重置（测试用）
// ============================================================================

// ResetMetrics 重置所有指标（仅用于测试）
func ResetMetrics() {
	utxoCorruptCount.Set(0)
	utxoTotalCount.Set(0)
	utxoUnrepairableCount.Set(0)
}

