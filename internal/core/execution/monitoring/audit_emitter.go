// Package monitoring 提供极简的审计事件处理
//
// MVP原则：
// 1. 仅提供标准日志输出，无持久化存储
// 2. 无后台任务，无内存缓存
// 3. 零配置，即用即丢
package monitoring

import (
	"fmt"

	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// BasicAuditEmitter 基础审计事件发射器
//
// # MVP原则：仅通过日志输出审计事件，不进行任何持久化或后台处理
//
// 设计目标：
// - 最小化实现：仅依赖标准日志接口，无额外依赖
// - 零存储：不保存审计历史，事件即发即丢
// - 零后台任务：无goroutine、无队列、无批处理
// - 高可靠：基于成熟的日志框架，稳定可靠
//
// 适用场景：
// - 自运行区块链节点的基础审计需求
// - 开发和测试环境的事件跟踪
// - 不需要审计持久化的简单部署
//
// 扩展方向：
// - 如需持久化，可在应用层添加日志收集工具（如filebeat、fluentd）
// - 如需结构化存储，可实现自定义AuditEventEmitter接口
type BasicAuditEmitter struct {
	// logger 日志记录器
	// 用于输出审计事件到标准日志流
	// 可为nil，此时审计事件将被静默丢弃（无错误）
	logger log.Logger
}

// NewBasicAuditEmitter 创建基础审计事件发射器
//
// 参数：
//   - logger: 日志记录器接口，可为nil（此时审计事件将被静默丢弃）
//
// 返回值：
//   - interfaces.AuditEventEmitter: 审计事件发射器接口实现
//
// 使用示例：
//
//	emitter := NewBasicAuditEmitter(logger)
//	emitter.EmitSecurityEvent(securityEvent)
//
// 注意事项：
// - logger为nil时不会报错，但审计事件不会输出
// - 该实现是线程安全的，底层日志框架负责并发控制
// - 无任何缓存或队列，事件立即处理
func NewBasicAuditEmitter(logger log.Logger) interfaces.AuditEventEmitter {
	return &BasicAuditEmitter{
		logger: logger,
	}
}

// EmitSecurityEvent 发射安全审计事件
//
// 将安全相关的审计事件格式化后输出到日志，用于安全监控和事后分析
//
// 参数：
//   - event: 安全审计事件，包含事件类型、严重程度、调用者、操作、结果等信息
//
// 日志级别映射：
//   - critical/high: ERROR级别，用于严重安全威胁
//   - medium: WARN级别，用于中等安全风险
//   - low/其他: INFO级别，用于一般安全信息
//
// 输出格式：
//
//	SECURITY_AUDIT: {EventType} [{Severity}] caller={Caller} action={Action} result={Result}
//
// 使用场景：
//   - 权限验证失败
//   - 异常访问模式检测
//   - 安全策略违规
//   - 恶意行为识别
//
// 性能特性：
//   - 同步处理，无缓存队列
//   - logger为nil时直接返回，无性能开销
//   - 格式化开销最小，仅包含关键字段
func (e *BasicAuditEmitter) EmitSecurityEvent(event interfaces.SecurityAuditEvent) {
	if e.logger != nil {
		// 格式化安全审计消息，包含关键安全信息
		message := fmt.Sprintf("SECURITY_AUDIT: %s [%s] caller=%s action=%s result=%s",
			event.EventType, event.Severity, event.Caller, event.Action, event.Result)

		// 根据严重程度选择合适的日志级别，便于日志过滤和告警
		switch event.Severity {
		case "critical", "high":
			e.logger.Error(message) // 严重安全事件使用ERROR级别
		case "medium":
			e.logger.Warn(message) // 中等安全风险使用WARN级别
		default:
			e.logger.Info(message) // 一般安全信息使用INFO级别
		}
	}
	// logger为nil时静默丢弃，不影响execution主路径性能
}

// EmitPerformanceEvent 发射性能审计事件
//
// 将执行性能相关的审计事件格式化后输出到日志，用于性能监控和优化分析
//
// 参数：
//   - event: 性能审计事件，包含执行时长、资源消耗、内存使用、引擎类型等信息
//
// 输出格式：
//
//	PERFORMANCE_AUDIT: {EventType} duration={Duration} 资源={ResourceConsumed} memory={MemoryUsed} engine={EngineType}
//
// 记录的性能指标：
//   - duration: 执行耗时（如100ms、1.5s等）
//   - 资源: 资源消耗量（整数）
//   - memory: 内存使用量（字节）
//   - engine: 执行引擎类型（WASM、ONNX等）
//
// 使用场景：
//   - 执行性能异常检测
//   - 资源使用模式分析
//   - 性能基准数据收集
//   - 引擎性能对比分析
//
// 日志级别：
//   - 固定使用INFO级别，性能事件通常不是错误
//   - 如需根据性能指标调整级别，可在应用层实现自定义逻辑
//
// 性能特性：
//   - 格式化开销极小，仅包含数值和字符串拼接
//   - 无额外计算或数据转换
//   - logger为nil时零开销
func (e *BasicAuditEmitter) EmitPerformanceEvent(event interfaces.PerformanceAuditEvent) {
	if e.logger != nil {
		// 格式化性能审计消息，包含关键性能指标
		message := fmt.Sprintf("PERFORMANCE_AUDIT: %s duration=%v 资源=%d memory=%d engine=%s",
			event.EventType, event.Duration, event.ResourceConsumed, event.MemoryUsed, event.EngineType)
		// 性能事件使用INFO级别，便于区分正常性能记录和错误事件
		e.logger.Info(message)
	}
	// logger为nil时静默丢弃，不影响execution主路径性能
}

// EmitErrorEvent 发射错误审计事件
//
// 将执行过程中发生的错误事件格式化后输出到错误日志，用于错误跟踪和问题诊断
//
// 参数：
//   - event: 错误审计事件，包含事件类型、错误类型、错误消息等信息
//
// 输出格式：
//
//	ERROR_AUDIT: {EventType} [{ErrorType}] {Message}
//
// 记录的错误信息：
//   - EventType: 事件类型（如execution_error、validation_error等）
//   - ErrorType: 具体错误类型（如engine_execution、contract_not_found等）
//   - Message: 详细错误消息
//
// 使用场景：
//   - 合约执行错误
//   - 引擎运行时错误
//   - 参数验证错误
//   - 系统内部错误
//
// 日志级别：
//   - 固定使用ERROR级别，错误事件需要重点关注
//   - 便于日志系统进行错误告警和过滤
//
// 与MetricsCollector的关系：
//   - 错误统计由MetricsCollector的RecordError处理
//   - 本方法专注于错误详情的日志记录
//   - 避免重复统计，保持模块职责清晰
//
// 性能特性：
//   - 字符串拼接开销最小
//   - 无复杂格式化或数据转换
//   - 错误路径通常对性能要求不高
func (e *BasicAuditEmitter) EmitErrorEvent(event interfaces.ErrorAuditEvent) {
	if e.logger != nil {
		// 格式化错误审计消息，包含关键错误信息
		message := fmt.Sprintf("ERROR_AUDIT: %s [%s] %s",
			event.EventType, event.ErrorType, event.Message)
		// 错误事件固定使用ERROR级别，便于错误监控和告警
		e.logger.Error(message)
	}
	// logger为nil时静默丢弃，但错误路径通常不会为nil
}
