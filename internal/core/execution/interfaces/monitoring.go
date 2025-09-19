package interfaces

import (
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// ==================== 监控审计内部接口 ====================
// MVP原则：仅保留确需跨子模块的最小接口

// MetricsCollector 指标收集器接口
// 仅提供基础的执行指标收集功能
type MetricsCollector interface {
	// 记录执行开始
	RecordExecutionStart(engineType types.EngineType, resourceID []byte)

	// 记录执行完成
	RecordExecutionComplete(engineType types.EngineType, duration time.Duration, success bool)

	// 记录资源消耗
	RecordResourceConsumption(engineType types.EngineType, consumed uint64)

	// 记录内存使用
	RecordMemoryUsage(engineType types.EngineType, used uint32)

	// 记录错误
	RecordError(errorType types.ExecutionErrorType, message string)

	// 获取执行指标快照
	GetExecutionMetrics() types.ExecutionMetrics
}

// AuditEventEmitter 审计事件发射器接口
// 仅提供基础的事件发射功能，默认通过日志输出
type AuditEventEmitter interface {
	// 发射安全事件
	EmitSecurityEvent(event SecurityAuditEvent)

	// 发射性能事件
	EmitPerformanceEvent(event PerformanceAuditEvent)

	// 发射错误事件
	EmitErrorEvent(event ErrorAuditEvent)
}

// ==================== 数据结构定义 ====================
// MVP原则：仅保留最基础的审计事件结构

// SecurityAuditEvent 安全审计事件（简化版）
type SecurityAuditEvent struct {
	EventType string    `json:"event_type"`
	Severity  string    `json:"severity"` // critical, high, medium, low
	Timestamp time.Time `json:"timestamp"`
	Caller    string    `json:"caller"`
	Action    string    `json:"action"`
	Result    string    `json:"result"` // success, denied, error
}

// PerformanceAuditEvent 性能审计事件（简化版）
type PerformanceAuditEvent struct {
	EventType        string           `json:"event_type"`
	Timestamp        time.Time        `json:"timestamp"`
	Duration         time.Duration    `json:"duration"`
	ResourceConsumed uint64           `json:"resource_consumed"`
	MemoryUsed       uint32           `json:"memory_used"`
	EngineType       types.EngineType `json:"engine_type"`
}

// ErrorAuditEvent 错误审计事件（简化版）
type ErrorAuditEvent struct {
	EventType string                   `json:"event_type"`
	ErrorType types.ExecutionErrorType `json:"error_type"`
	Timestamp time.Time                `json:"timestamp"`
	Message   string                   `json:"message"`
}
