// Package health 提供链健康检查的监控指标
package health

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ============================================================================
//                              Prometheus 指标
// ============================================================================

var (
	// healthCheckTotal 健康检查总次数
	healthCheckTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "check_total",
		Help:      "Total number of health checks",
	}, []string{"check_type", "status"}) // check_type: quick/deep, status: success/failed

	// healthCheckDuration 健康检查耗时
	healthCheckDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "check_duration_seconds",
		Help:      "Duration of health checks",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s ~ 51.2s
	}, []string{"check_type"})

	// issuesDetected 检测到的问题数量
	issuesDetected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "issues_detected_total",
		Help:      "Total number of issues detected",
	}, []string{"issue_type", "severity"})

	// repairAttempts 修复尝试次数
	repairAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "repair_attempts_total",
		Help:      "Total number of repair attempts",
	}, []string{"issue_type", "result"}) // result: success/failed

	// repairDuration 修复操作耗时
	repairDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "repair_duration_seconds",
		Help:      "Duration of repair operations",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s ~ 512s
	}, []string{"issue_type", "level"}) // level: selective/regional/full

	// readonlyModeActive 只读模式状态
	readonlyModeActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "chain",
		Name:      "readonly_mode_active",
		Help:      "Whether the chain is in read-only mode (1=yes, 0=no)",
	})

	// readonlyExitAttempts 退出只读模式尝试次数
	readonlyExitAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "wes",
		Subsystem: "chain",
		Name:      "readonly_exit_attempts_total",
		Help:      "Total number of attempts to exit read-only mode",
	}, []string{"result"}) // result: success/failed

	// lastHealthCheckTimestamp 上次健康检查时间戳
	lastHealthCheckTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "last_check_timestamp_seconds",
		Help:      "Timestamp of the last health check",
	}, []string{"check_type"})

	// unrepa irableIssuesCount 不可修复问题数量
	unrepairableIssuesCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "chain_health",
		Name:      "unrepairable_issues_count",
		Help:      "Number of detected issues that cannot be automatically repaired",
	})
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(
		healthCheckTotal,
		healthCheckDuration,
		issuesDetected,
		repairAttempts,
		repairDuration,
		readonlyModeActive,
		readonlyExitAttempts,
		lastHealthCheckTimestamp,
		unrepairableIssuesCount,
	)
}

// ============================================================================
//                              指标更新函数
// ============================================================================

// RecordHealthCheck 记录健康检查
func RecordHealthCheck(checkType string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "failed"
	}

	healthCheckTotal.WithLabelValues(checkType, status).Inc()
	healthCheckDuration.WithLabelValues(checkType).Observe(duration)
	lastHealthCheckTimestamp.WithLabelValues(checkType).SetToCurrentTime()
}

// RecordIssueDetected 记录检测到的问题
func RecordIssueDetected(issueType, severity string) {
	issuesDetected.WithLabelValues(issueType, severity).Inc()
}

// RecordRepairAttempt 记录修复尝试
func RecordRepairAttempt(issueType string, success bool, duration float64, level string) {
	result := "success"
	if !success {
		result = "failed"
	}

	repairAttempts.WithLabelValues(issueType, result).Inc()
	repairDuration.WithLabelValues(issueType, level).Observe(duration)
}

// SetReadOnlyMode 设置只读模式状态
func SetReadOnlyMode(active bool) {
	if active {
		readonlyModeActive.Set(1)
	} else {
		readonlyModeActive.Set(0)
	}
}

// RecordReadOnlyExit 记录退出只读模式尝试
func RecordReadOnlyExit(success bool) {
	result := "success"
	if !success {
		result = "failed"
	}

	readonlyExitAttempts.WithLabelValues(result).Inc()
}

// SetUnrepairableIssuesCount 设置不可修复问题数量
func SetUnrepairableIssuesCount(count int) {
	unrepairableIssuesCount.Set(float64(count))
}

