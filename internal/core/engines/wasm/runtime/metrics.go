package runtime

import (
	"sync/atomic"
	"time"
)

// ExecutionMetrics WASM运行时基础统计
//
// 🔧 **修复说明**：
// 清理了过度监控的字段，符合项目"接口不暴露指标"偏好：
// - 移除了详细性能指标（minDuration、maxDuration、平均值计算等）
// - 移除了内存使用统计（peakMemoryUsed、averageMemoryUsed等）
// - 移除了编译指标（compilationTime、cacheHitRate等）
// - 移除了实例池指标（poolHitRate、poolMissCount等）
// - 移除了错误统计和聚类（errorCounts、errorClusters等）
// - 移除了实时窗口和观察者模式（recentExecutions、observers等）
// - 移除了审计事件系统（auditEvents、auditObservers等）
//
// 🎯 **保留原则**：
// 仅保留最必要的计数与时间指标：
// - 执行次数统计：用于基础运行状态确认
// - 总执行时间：用于粗略性能评估
// - 总资源消耗：区块链核心资源指标
//
// 📋 **内部使用**：
// 不通过公共接口暴露，仅供WASM引擎内部诊断使用
type ExecutionMetrics struct {
	// 基础执行统计（原子计数器）
	totalExecutions      int64 // 总执行次数
	successfulExecutions int64 // 成功执行次数
	failedExecutions     int64 // 失败执行次数
	totalExecutionTimeNs int64 // 总执行时间（纳秒）
	totalResourceUsed    int64 // 总资源消耗
}

// ❌ **已删除大量复杂监控结构体和方法**
//
// 🚨 **清理内容**：
// 1. **ExecutionRecord/MetricsSnapshot** - 详细执行记录和复杂指标快照
// 2. **MetricsObserver/AuditObserver** - 观察者模式和审计通知机制
// 3. **AuditEvent/ErrorCluster** - 复杂审计系统和错误聚类分析
// 4. **PerformanceAlert/AlertThresholds** - 性能告警系统和阈值配置
// 5. **所有复杂计算方法** - 平均值计算、窗口维护、观察者通知等
//
// 🎯 **删除理由**：
// - 违反项目"接口不暴露指标"偏好
// - 没有明确的消费者和使用场景
// - 增加系统复杂度而无实际价值
// - 在自治系统中，组件应该专注于自身功能

// NewExecutionMetrics 创建简化的执行统计收集器
//
// 符合项目"接口不暴露指标"偏好，仅提供内部基础统计
func NewExecutionMetrics() *ExecutionMetrics {
	return &ExecutionMetrics{}
}

// RecordExecutionStart 记录执行开始
// 仅做基础计数，不记录详细信息
func (em *ExecutionMetrics) RecordExecutionStart() {
	atomic.AddInt64(&em.totalExecutions, 1)
}

// RecordExecutionComplete 记录执行完成
// 更新成功/失败计数和执行时间
func (em *ExecutionMetrics) RecordExecutionComplete(duration time.Duration, success bool) {
	atomic.AddInt64(&em.totalExecutionTimeNs, duration.Nanoseconds())

	if success {
		atomic.AddInt64(&em.successfulExecutions, 1)
	} else {
		atomic.AddInt64(&em.failedExecutions, 1)
	}
}

// RecordResourceConsumption 记录资源消耗
// 累计记录资源使用量（区块链核心指标）
func (em *ExecutionMetrics) RecordResourceConsumption(resourceUsed uint64) {
	atomic.AddInt64(&em.totalResourceUsed, int64(resourceUsed))
}

// GetBasicStats 获取基础统计信息（内部使用）
//
// 🎯 **内部诊断专用**：
// 仅供WASM引擎内部诊断使用，不通过公共接口暴露
// 返回最基础的计数信息，避免复杂计算
func (em *ExecutionMetrics) GetBasicStats() (executions, successes, failures int64, totalTimeNs, total资源 int64) {
	return atomic.LoadInt64(&em.totalExecutions),
		atomic.LoadInt64(&em.successfulExecutions),
		atomic.LoadInt64(&em.failedExecutions),
		atomic.LoadInt64(&em.totalExecutionTimeNs),
		atomic.LoadInt64(&em.totalResourceUsed)
}
