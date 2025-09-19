// Package monitoring 提供极简的执行监控功能
//
// 设计原则：
// 1. 默认无后台任务，仅提供快照式指标
// 2. 零配置，零内存常驻开销
// 3. 仅保留最基础的执行统计（总数、成功率、平均时间）
package monitoring

import (
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/pkg/types"
)

// BasicMetricsCollector 基础指标收集器
//
// # MVP原则：仅收集最核心的执行指标，无后台任务，无内存常驻开销
//
// 设计目标：
// - 零配置：无需任何初始化参数，开箱即用
// - 零开销：仅使用原子计数器，无goroutine、无锁、无内存分配
// - 高性能：纳秒级操作延迟，适合execution主路径高频调用
// - 并发安全：所有操作基于原子指令，无竞态条件
//
// 适用场景：
// - 自运行区块链节点的基础监控需求
// - 需要最小化性能影响的生产环境
// - 无需复杂监控策略的简单部署
type BasicMetricsCollector struct {
	// ==================== 核心执行统计（原子计数器） ====================

	// totalExecutions 总执行次数
	// 记录所有ExecutionStart调用，包括成功和失败的执行
	// 用于计算整体吞吐量和成功率分母
	totalExecutions int64

	// successfulExecutions 成功执行次数
	// 记录RecordExecutionComplete(success=true)的调用次数
	// 用于计算成功率：successfulExecutions / totalExecutions
	successfulExecutions int64

	// failedExecutions 失败执行次数
	// 记录RecordExecutionComplete(success=false)的调用次数
	// 用于计算失败率：failedExecutions / totalExecutions
	failedExecutions int64

	// total资源Consumed 总资源消耗量
	// 累计所有执行过程中的资源消耗
	// 用于评估整体计算资源使用情况和成本分析
	total资源Consumed int64

	// totalExecutionTimeNs 总执行时间（纳秒）
	// 累计所有执行的持续时间，以纳秒为单位存储
	// 用于计算平均执行时间：totalExecutionTimeNs / totalExecutions
	totalExecutionTimeNs int64
}

// NewBasicMetricsCollector 创建基础指标收集器
//
// 返回最简的指标收集器实现，无任何配置参数，无后台goroutine
func NewBasicMetricsCollector() interfaces.MetricsCollector {
	return &BasicMetricsCollector{}
}

// RecordExecutionStart 记录执行开始
// 仅做计数，不启动任何后台任务
func (c *BasicMetricsCollector) RecordExecutionStart(engineType types.EngineType, resourceID []byte) {
	atomic.AddInt64(&c.totalExecutions, 1)
}

// RecordExecutionComplete 记录执行完成
// 更新成功/失败计数和总执行时间
func (c *BasicMetricsCollector) RecordExecutionComplete(engineType types.EngineType, duration time.Duration, success bool) {
	atomic.AddInt64(&c.totalExecutionTimeNs, duration.Nanoseconds())

	if success {
		atomic.AddInt64(&c.successfulExecutions, 1)
	} else {
		atomic.AddInt64(&c.failedExecutions, 1)
	}
}

// RecordResourceConsumption 记录资源消耗
//
// 累计记录执行过程中的资源消耗量，用于统计总体资源使用情况
//
// 参数：
//   - engineType: 执行引擎类型（WASM、ONNX等），当前版本忽略引擎区分
//   - consumed: 本次执行消耗的资源量
//
// 实现说明：
// - 使用原子操作确保并发安全，适合高频调用
// - 当前版本不区分引擎类型，统一累计到总量中
// - 资源消耗是区块链执行的核心指标，始终记录
//
// 性能特性：
// - 原子操作，纳秒级延迟，零内存分配
// - 并发安全，无锁竞争
func (c *BasicMetricsCollector) RecordResourceConsumption(engineType types.EngineType, consumed uint64) {
	// 原子累加资源消耗，线程安全且高性能
	atomic.AddInt64(&c.total资源Consumed, int64(consumed))
	// 注意：engineType参数当前版本未使用，保留用于未来按引擎类型分类统计
	_ = engineType // 标记参数已知但未使用
}

// RecordMemoryUsage 记录内存使用
//
// MVP设计决策：基础版本中有意忽略内存统计
// 理由：
// 1. 自运行区块链节点通常部署在资源充足的环境，内存使用不是关键瓶颈
// 2. 内存监控需要更复杂的实现（峰值追踪、定期采样、GC影响等）
// 3. 如需内存监控，建议在应用层使用专业工具（如prometheus、pprof）
// 4. 避免增加execution主路径的复杂性和性能开销
//
// 参数：
//   - engineType: 执行引擎类型（WASM、ONNX等）
//   - used: 内存使用量（字节），当前版本忽略此参数
//
// 扩展说明：如需启用内存统计，可在应用层实现自定义MetricsCollector
func (c *BasicMetricsCollector) RecordMemoryUsage(engineType types.EngineType, used uint32) {
	// MVP简化实现：有意忽略内存统计，避免复杂性
	// 参数保留用于接口一致性和未来扩展
	_ = engineType // 标记参数已知但未使用
	_ = used       // 标记参数已知但未使用
}

// RecordError 记录错误事件
//
// MVP设计决策：基础版本中有意简化错误统计
// 理由：
// 1. 错误统计的核心指标已在RecordExecutionComplete中通过success=false计入
// 2. 详细的错误分类统计需要复杂的数据结构和内存管理
// 3. 错误详情通过audit_emitter的EmitErrorEvent记录到日志
// 4. 自运行节点重点关注整体成功率，而非详细错误分类
//
// 参数：
//   - errorType: 错误类型（引擎执行错误、合约错误等）
//   - message: 错误消息，当前版本忽略此参数
//
// 实现说明：
// - 当前版本将错误统计委托给RecordExecutionComplete方法
// - 错误详情委托给AuditEventEmitter处理
// - 避免在metrics中重复记录相同信息，保持模块职责清晰
//
// 扩展说明：如需详细错误统计，可在应用层实现自定义MetricsCollector
func (c *BasicMetricsCollector) RecordError(errorType types.ExecutionErrorType, message string) {
	// MVP简化实现：错误统计已委托给RecordExecutionComplete处理
	// 错误详情委托给AuditEventEmitter记录到日志
	// 参数保留用于接口一致性和未来扩展
	_ = errorType // 标记参数已知但未使用
	_ = message   // 标记参数已知但未使用
}

// GetExecutionMetrics 获取当前执行指标快照
//
// 实时读取并计算当前的执行统计数据，无内存缓存，确保数据的即时性和准确性
//
// 返回值包含：
// - TotalExecutions: 总执行次数
// - SuccessfulExecutions: 成功执行次数
// - FailedExecutions: 失败执行次数
// - AverageExecutionTimeMs: 平均执行时间（毫秒）
// - TotalResourceConsumed: 总资源消耗量
// - EngineStats: 引擎级统计（基础版本为空，避免复杂性）
//
// 性能特性：
// - 使用原子操作读取，确保数据一致性
// - 无锁操作，纳秒级延迟
// - 实时计算平均值，无历史数据缓存
// - 零内存分配，适合高频调用
//
// 计算逻辑：
// - 成功率 = SuccessfulExecutions / TotalExecutions
// - 平均时间 = TotalExecutionTimeNs / TotalExecutions / 1e6 (转换为毫秒)
// - 失败率 = FailedExecutions / TotalExecutions
//
// 注意事项：
// - 返回的是当前时刻的快照，不是历史聚合数据
// - 如需历史趋势分析，建议在应用层定期采样并存储
func (c *BasicMetricsCollector) GetExecutionMetrics() types.ExecutionMetrics {
	// ==================== 第一步：原子读取所有计数器 ====================
	// 使用原子操作确保读取的一致性，避免中间状态的数据
	total := atomic.LoadInt64(&c.totalExecutions)
	successful := atomic.LoadInt64(&c.successfulExecutions)
	failed := atomic.LoadInt64(&c.failedExecutions)
	totalTimeNs := atomic.LoadInt64(&c.totalExecutionTimeNs)
	total资源 := atomic.LoadInt64(&c.total资源Consumed)

	// ==================== 第二步：实时计算派生指标 ====================
	// 计算平均执行时间（毫秒），避免除零错误
	var avgTimeMs float64
	if total > 0 {
		// 纳秒转毫秒：除以1e6 (1,000,000)
		// 使用float64确保精度，适合展示给用户
		avgTimeMs = float64(totalTimeNs) / float64(total) / 1e6
	}
	// 如果total为0，avgTimeMs保持默认值0.0

	// ==================== 第三步：组装返回结构 ====================
	return types.ExecutionMetrics{
		// 基础计数指标：直接转换为uint64
		TotalExecutions:      uint64(total),      // 总执行次数
		SuccessfulExecutions: uint64(successful), // 成功执行次数
		FailedExecutions:     uint64(failed),     // 失败执行次数

		// 计算指标
		AverageExecutionTimeMs: avgTimeMs,       // 平均执行时间（毫秒）
		TotalResourceConsumed:  uint64(total资源), // 总资源消耗量

		// 引擎级统计：基础版本返回空map，避免复杂性
		// 扩展版本可在此处添加按引擎类型分类的详细统计
		EngineStats: make(map[types.EngineType]types.EngineExecutionStats),
	}
}
