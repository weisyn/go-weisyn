package kbucket

import "sync/atomic"

// KBucketMetrics K桶可观测指标
type KBucketMetrics struct {
	// 清理原因计数
	CleanupDisconnected       int64 // 断连清理
	CleanupLongTimeUnused     int64 // 长期无用清理
	CleanupLowHealth          int64 // 低健康分清理
	CleanupConnectedViolation int64 // 违反连接约束（应为0）

	// 状态分布（快照，非累计）
	ActiveCount       int64
	SuspectCount      int64
	QuarantinedCount  int64
	EvictedCount      int64

	// 事件计数
	NoClosestPeersFound int64 // FindClosestPeers失败次数

	// 维护统计
	MaintenanceRuns       int64 // 维护循环执行次数
	HealthDecayOperations int64 // 健康分衰减操作次数

	// Phase 2：探测相关指标
	ProbeAttempts         int64 // 探测尝试次数
	ProbeSuccessCount     int64 // 探测成功次数
	ProbeFailCount        int64 // 探测失败次数
	ProbePreventedCleanup int64 // 探测防止的清理次数（关键指标）
	ProbeTimeout          int64 // 探测超时次数
	CleanupProbeFailed    int64 // 探测失败导致的清理
}

// RecordCleanup 记录清理事件
func (m *KBucketMetrics) RecordCleanup(reason string, isConnected bool) {
	if isConnected {
		atomic.AddInt64(&m.CleanupConnectedViolation, 1)
	}

	switch reason {
	case "disconnected":
		atomic.AddInt64(&m.CleanupDisconnected, 1)
	case "long_time_unused":
		atomic.AddInt64(&m.CleanupLongTimeUnused, 1)
	case "low_health":
		atomic.AddInt64(&m.CleanupLowHealth, 1)
	case "probe_failed":
		atomic.AddInt64(&m.CleanupProbeFailed, 1)
	}
}

// RecordNoClosestPeers 记录FindClosestPeers失败事件
func (m *KBucketMetrics) RecordNoClosestPeers() {
	atomic.AddInt64(&m.NoClosestPeersFound, 1)
}

// RecordMaintenanceRun 记录维护循环执行
func (m *KBucketMetrics) RecordMaintenanceRun() {
	atomic.AddInt64(&m.MaintenanceRuns, 1)
}

// RecordHealthDecayOperation 记录健康分衰减操作
func (m *KBucketMetrics) RecordHealthDecayOperation() {
	atomic.AddInt64(&m.HealthDecayOperations, 1)
}

// UpdateStateDistribution 更新状态分布快照
func (m *KBucketMetrics) UpdateStateDistribution(active, suspect, quarantined, evicted int64) {
	atomic.StoreInt64(&m.ActiveCount, active)
	atomic.StoreInt64(&m.SuspectCount, suspect)
	atomic.StoreInt64(&m.QuarantinedCount, quarantined)
	atomic.StoreInt64(&m.EvictedCount, evicted)
}

// GetSnapshot 获取指标快照
func (m *KBucketMetrics) GetSnapshot() *KBucketMetrics {
	return &KBucketMetrics{
		CleanupDisconnected:       atomic.LoadInt64(&m.CleanupDisconnected),
		CleanupLongTimeUnused:     atomic.LoadInt64(&m.CleanupLongTimeUnused),
		CleanupLowHealth:          atomic.LoadInt64(&m.CleanupLowHealth),
		CleanupConnectedViolation: atomic.LoadInt64(&m.CleanupConnectedViolation),
		ActiveCount:               atomic.LoadInt64(&m.ActiveCount),
		SuspectCount:              atomic.LoadInt64(&m.SuspectCount),
		QuarantinedCount:          atomic.LoadInt64(&m.QuarantinedCount),
		EvictedCount:              atomic.LoadInt64(&m.EvictedCount),
		NoClosestPeersFound:       atomic.LoadInt64(&m.NoClosestPeersFound),
		MaintenanceRuns:           atomic.LoadInt64(&m.MaintenanceRuns),
		HealthDecayOperations:     atomic.LoadInt64(&m.HealthDecayOperations),
		ProbeAttempts:             atomic.LoadInt64(&m.ProbeAttempts),
		ProbeSuccessCount:         atomic.LoadInt64(&m.ProbeSuccessCount),
		ProbeFailCount:            atomic.LoadInt64(&m.ProbeFailCount),
		ProbePreventedCleanup:     atomic.LoadInt64(&m.ProbePreventedCleanup),
		ProbeTimeout:              atomic.LoadInt64(&m.ProbeTimeout),
		CleanupProbeFailed:        atomic.LoadInt64(&m.CleanupProbeFailed),
	}
}

