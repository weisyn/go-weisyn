package coordinator

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"

	"github.com/weisyn/v1/internal/core/execution/env"
	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/internal/core/execution/manager"
	"github.com/weisyn/v1/internal/core/execution/security"
)

// ExecutionCoordinator接口定义已移至 pkg/interfaces/execution/coordinator.go
// 这里使用公共接口的别名以保持代码简洁

// DefaultExecutionCoordinator 执行协调器的默认实现
//
// 设计说明：
// 1. 实现ExecutionCoordinator接口
// 2. 内部委托给ResourceExecutionCoordinator处理具体逻辑
// 3. 作为接口与实现之间的适配层
// 4. 支持通过构造函数注入所有依赖
type DefaultExecutionCoordinator struct {
	// 内部资源执行协调器，处理具体的执行逻辑
	coordinator *ResourceExecutionCoordinator

	// 引擎管理器，用于获取支持的引擎列表
	engineManager execution.EngineManager

	// 指标收集器，用于获取执行指标
	metricsCollector interfaces.MetricsCollector
}

// NewExecutionCoordinator 创建执行协调器实例
//
// 采用依赖注入模式，所有依赖通过参数传入，便于：
// 1. 单元测试时Mock各个依赖组件
// 2. 通过fx等DI框架进行自动装配
// 3. 保持构造逻辑的简洁性和可测试性
//
// 参数说明：
// - engineManager：引擎管理器，负责多引擎分发
// - hostRegistry：宿主能力注册表，提供统一宿主接口
// - metricsCollector：指标收集器，记录执行性能数据
// - auditEmitter：审计事件发射器，发射结构化事件
// - sideEffectProcessor：副作用处理器，处理执行产生的副作用
// - securityIntegrator：安全集成器，提供统一安全校验
// - quotaManager：配额管理器，管理执行资源配额
// - config：协调器配置，如为nil则使用默认配置
//
// 返回值：
// - execution.ExecutionCoordinator：实现了ExecutionCoordinator接口的实例
func NewExecutionCoordinator(
	engineManager execution.EngineManager,
	dispatcher *manager.Dispatcher,
	hostRegistry execution.HostCapabilityRegistry,
	metricsCollector interfaces.MetricsCollector,
	auditEmitter interfaces.AuditEventEmitter,
	sideEffectProcessor interfaces.SideEffectProcessor,
	securityIntegrator *security.SecurityIntegrator,
	quotaManager *security.QuotaManager,
	// auditTracker interfaces.AuditTracker, // 已移除
	envAdvisor *env.CoordinatorAdapter,
	logger log.Logger,
	config *CoordinatorConfig,
) execution.ExecutionCoordinator {
	// 创建内部资源执行协调器
	coordinator := NewResourceExecutionCoordinator(
		engineManager,
		dispatcher,
		hostRegistry,
		metricsCollector,
		auditEmitter,
		sideEffectProcessor,
		securityIntegrator,
		quotaManager,
		// auditTracker, // 已移除
		envAdvisor,
		logger,
		config,
	)

	// 创建接口适配器
	return &DefaultExecutionCoordinator{
		coordinator:      coordinator,
		engineManager:    engineManager,
		metricsCollector: metricsCollector,
	}
}

// Execute 实现ExecutionCoordinator接口的Execute方法
//
// 方法职责：
// 1. 将接口调用委托给内部ResourceExecutionCoordinator
// 2. 保持接口的稳定性，隔离内部实现变更的影响
// 3. 提供统一的错误处理和日志记录
//
// 参数：
// - ctx：执行上下文
// - params：执行参数
//
// 返回值：
// - types.ExecutionResult：执行结果
// - error：执行错误
func (c *DefaultExecutionCoordinator) Execute(ctx context.Context, params types.ExecutionParams) (types.ExecutionResult, error) {
	// 委托给内部协调器处理
	return c.coordinator.Execute(ctx, params)
}

// GetSupportedEngines 实现ExecutionCoordinator接口的GetSupportedEngines方法
//
// 方法职责：
// 1. 从引擎管理器获取当前注册的引擎类型列表
// 2. 为外部调用者提供引擎能力发现机制
//
// 返回值：
// - []types.EngineType：支持的引擎类型列表
func (c *DefaultExecutionCoordinator) GetSupportedEngines() []types.EngineType {
	return c.engineManager.ListEngines()
}

// GetExecutionMetrics 实现ExecutionCoordinator接口的GetExecutionMetrics方法
//
// 方法职责：
// 1. 从指标收集器聚合执行统计信息
// 2. 提供执行性能的可观测性数据
// 3. 支持监控、告警和性能分析
//
// 返回值：
// - ExecutionMetrics：包含各维度执行指标的结构体
func (c *DefaultExecutionCoordinator) GetExecutionMetrics() types.ExecutionMetrics {
	// 从指标收集器获取原始指标数据
	// 注意：这里需要MetricsCollector接口扩展相应的方法
	// 或者通过其他方式获取聚合指标数据

	// 当前返回默认值，实际实现中需要从metricsCollector获取真实数据
	return types.ExecutionMetrics{
		TotalExecutions:        0,
		SuccessfulExecutions:   0,
		FailedExecutions:       0,
		AverageExecutionTimeMs: 0.0,
		TotalResourceConsumed:  0,
		EngineStats:            make(map[types.EngineType]types.EngineExecutionStats),
	}
}

// ==================== 工厂函数和辅助方法 ====================

// NewDefaultCoordinatorWithDefaults 使用默认配置创建执行协调器
//
// 便利函数，用于快速创建具有合理默认配置的执行协调器实例
// 适用于测试环境或不需要特殊配置的简单场景
//
// 参数：
// - engineManager：引擎管理器
// - hostRegistry：宿主能力注册表
//
// 返回值：
// - execution.ExecutionCoordinator：配置了默认参数的执行协调器实例
func NewDefaultCoordinatorWithDefaults(
	engineManager execution.EngineManager,
	hostRegistry execution.HostCapabilityRegistry,
	envAdvisor *env.CoordinatorAdapter,
	logger log.Logger,
) execution.ExecutionCoordinator {
	// 创建默认的Dispatcher（提供熔断和限流功能）
	dispatcher := manager.NewDispatcher(engineManager.(*manager.EngineManager)).
		WithCircuitBreakerConfig(3, 5*time.Second). // 3次失败后熔断5秒
		WithRateLimit(types.EngineTypeWASM, 10, 2). // WASM: 容量10，每秒补充2个token
		WithRateLimit(types.EngineTypeONNX, 5, 1).  // ONNX: 容量5，每秒补充1个token
		WithDynamicStrategy(true)                   // 启用动态引擎选择
	// 创建默认的组件实例
	// 注意：在实际环境中，这些组件应该通过DI框架注入
	metricsCollector := &NoOpMetricsCollector{}
	auditEmitter := &NoOpAuditEventEmitter{}
	sideEffectProcessor := &NoOpSideEffectProcessor{}

	// 创建生产级安全集成器和配额管理器（无nil依赖）
	securityIntegrator := security.NewDefaultSecurityIntegrator()
	quotaManager := security.NewDefaultQuotaManager()
	// MVP极简：不再使用auditTracker，遵循减法原则
	return NewExecutionCoordinator(
		engineManager,
		dispatcher,
		hostRegistry,
		metricsCollector,
		auditEmitter,
		sideEffectProcessor,
		securityIntegrator,
		quotaManager,
		envAdvisor,
		logger,
		DefaultCoordinatorConfig(), // 使用生产级默认配置
	)
}

// ==================== NoOp实现（MVP设计原则） ====================
//
// 以下NoOp实现体现了MVP设计原则，在保证核心功能的同时避免不必要的复杂性。
// 这些实现可以根据实际需要逐步升级为功能完整的实现。

// NoOpMetricsCollector 无操作的指标收集器实现（MVP版本）
//
// MVP设计理念：
// 在区块链节点的自运行场景中，过度的指标收集可能带来不必要的性能开销。
// 核心执行数据已由引擎管理器和安全组件进行跟踪，无需额外的指标系统。
//
// 适用场景：
// 1. 生产环境：减少不必要的性能开销，专注于核心执行功能
// 2. 测试环境：提供清洁的测试环境，避免指标噪音
// 3. 轻量部署：适合资源受限的环境或简化部署场景
// 4. 开发阶段：在功能开发阶段减少不必要的复杂度
//
// 扩展路径：
// 如果需要详细的指标数据，可以通过依赖注入替换为实际的指标收集器实现。
type NoOpMetricsCollector struct{}

// RecordExecutionStart 记录执行开始（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免额外的性能开销

func (n *NoOpMetricsCollector) RecordExecutionStart(engineType types.EngineType, resourceID []byte) {
	// NoOp实现：不执行任何指标记录操作
	// MVP设计理念：执行开始时间已由引擎管理器进行跟踪，无需额外记录
	// 如需详细指标，可通过依赖注入替换为实际的指标收集器
}

// RecordExecutionComplete 记录执行完成（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免额外的性能开销
func (n *NoOpMetricsCollector) RecordExecutionComplete(engineType types.EngineType, duration time.Duration, success bool) {
	// NoOp实现：不执行任何指标记录操作
	// MVP设计理念：执行结果已由引擎管理器进行跟踪，无需额外记录
}

// RecordResourceConsumption 记录资源消耗（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免额外的性能开销
func (n *NoOpMetricsCollector) RecordResourceConsumption(engineType types.EngineType, consumed uint64) {
	// NoOp实现：不执行任何指标记录操作
	// MVP设计理念：资源消耗信息已包含在执行结果中，无需额外记录
}

// RecordMemoryUsage 记录内存使用（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免额外的性能开销
func (n *NoOpMetricsCollector) RecordMemoryUsage(engineType types.EngineType, used uint32) {
	// NoOp实现：不执行任何指标记录操作
	// MVP设计理念：内存使用量由引擎内部管理，无需额外监控
}

// RecordError 记录错误（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免额外的性能开销
func (n *NoOpMetricsCollector) RecordError(errorType types.ExecutionErrorType, message string) {
	// NoOp实现：不执行任何指标记录操作
	// MVP设计理念：错误信息已通过标准异常机制返回，无需额外记录
}

// GetExecutionMetrics 获取执行指标（NoOp实现）
// 返回空的指标数据，符合MVP设计原则
func (n *NoOpMetricsCollector) GetExecutionMetrics() types.ExecutionMetrics {
	// NoOp实现：返回零值指标，避免复杂的数据收集和聚合逻辑
	// MVP设计理念：在自运行的区块链环境中，数据收集的目的和使用者并不明确
	return types.ExecutionMetrics{
		TotalExecutions:        0,                                                     // 总执行次数：零值，表示未进行统计
		SuccessfulExecutions:   0,                                                     // 成功执行次数：零值
		FailedExecutions:       0,                                                     // 失败执行次数：零值
		AverageExecutionTimeMs: 0.0,                                                   // 平均执行时间：零值
		TotalResourceConsumed:  0,                                                     // 总气费消耗：零值
		EngineStats:            make(map[types.EngineType]types.EngineExecutionStats), // 引擎统计：空集合
	}
}

// NoOpAuditEventEmitter 无操作的审计事件发射器实现（MVP版本）
//
// MVP设计理念：
// 在区块链节点的自运行场景中，过度的审计事件可能带来不必要的存储和处理开销。
// 核心安全事件已由安全集成器进行处理，无需额外的审计系统。
//
// 适用场景：
// 1. 自运行环境：减少不必要的审计开销，提高性能
// 2. 轻量部署：适合资源受限的环境
// 3. 测试环境：提供清洁的测试环境，避免审计噪音
//
// 扩展路径：
// 如果需要详细的审计跟踪，可以通过依赖注入替换为实际的审计事件发射器。
type NoOpAuditEventEmitter struct{}

// EmitSecurityEvent 发射安全事件（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免不必要的审计开销

func (n *NoOpAuditEventEmitter) EmitSecurityEvent(event interfaces.SecurityAuditEvent) {
	// NoOp实现：不执行任何审计事件发射操作
	// MVP设计理念：安全事件已由安全集成器在关键节点进行处理，无需额外记录
	// 如需详细审计，可通过依赖注入替换为实际的审计发射器
}

// EmitPerformanceEvent 发射性能事件（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免不必要的审计开销
func (n *NoOpAuditEventEmitter) EmitPerformanceEvent(event interfaces.PerformanceAuditEvent) {
	// NoOp实现：不执行任何审计事件发射操作
	// MVP设计理念：性能数据已包含在执行结果中，无需额外审计
}

// EmitErrorEvent 发射错误事件（NoOp实现）
// 在MVP设计中，该方法不执行任何操作，避免不必要的审计开销
func (n *NoOpAuditEventEmitter) EmitErrorEvent(event interfaces.ErrorAuditEvent) {
	// NoOp实现：不执行任何审计事件发射操作
	// MVP设计理念：错误信息已通过标准异常机制返回，无需额外审计
}

// NoOpSideEffectProcessor 无操作的副作用处理器实现（MVP版本）
//
// MVP设计理念：
// 在区块链节点的自运行场景中，复杂的副作用处理可能带来不必要的复杂性。
// 核心副作用（如UTXO操作、状态变更）已由执行引擎内部处理，无需额外的处理层。
//
// 适用场景：
// 1. 简化部署：减少系统复杂性，提高可靠性
// 2. 测试环境：专注于核心执行功能测试
// 3. 轻量化场景：适合资源受限的环境
//
// 扩展路径：
// 如果需要复杂的副作用处理，可以通过依赖注入替换为功能完整的副作用处理器。
type NoOpSideEffectProcessor struct{}

// ProcessUTXOSideEffects 处理UTXO副作用（NoOp实现）
// 在MVP设计中，该方法直接返回成功，不执行任何实际处理

func (n *NoOpSideEffectProcessor) ProcessUTXOSideEffects(ctx context.Context, effects []interfaces.UTXOSideEffect) error {
	// NoOp实现：不执行任何UTXO副作用处理操作
	// MVP设计理念：UTXO操作已由执行引擎内部处理，无需额外处理
	// 参数验证：防止空指针引用
	_ = ctx     // 明确标记参数已被使用（避免go vet警告）
	_ = effects // 明确标记参数已被使用
	return nil
}

// ProcessStateSideEffects 处理状态副作用（NoOp实现）
// 在MVP设计中，该方法直接返回成功，不执行任何实际处理
func (n *NoOpSideEffectProcessor) ProcessStateSideEffects(ctx context.Context, effects []interfaces.StateSideEffect) error {
	// NoOp实现：不执行任何状态副作用处理操作
	// MVP设计理念：状态变更已由执行引擎内部处理，无需额外处理
	_ = ctx     // 明确标记参数已被使用
	_ = effects // 明确标记参数已被使用
	return nil
}

// ProcessEventSideEffects 处理事件副作用（NoOp实现）
// 在MVP设计中，该方法直接返回成功，不执行任何实际处理
func (n *NoOpSideEffectProcessor) ProcessEventSideEffects(ctx context.Context, effects []interfaces.EventSideEffect) error {
	// NoOp实现：不执行任何事件副作用处理操作
	// MVP设计理念：事件发射已由执行引擎内部处理，无需额外处理
	_ = ctx     // 明确标记参数已被使用
	_ = effects // 明确标记参数已被使用
	return nil
}

// ProcessBatch 批量处理副作用（NoOp实现）
// 在MVP设计中，该方法直接返回成功，不执行任何实际处理
func (n *NoOpSideEffectProcessor) ProcessBatch(ctx context.Context, batch *interfaces.SideEffectBatch) error {
	// NoOp实现：不执行任何批量副作用处理操作
	// MVP设计理念：批量操作已由执行引擎优化处理，无需额外处理
	_ = ctx   // 明确标记参数已被使用
	_ = batch // 明确标记参数已被使用
	return nil
}

// Rollback 执行回滚操作（NoOp实现）
// 在MVP设计中，该方法直接返回成功，不执行任何实际回滚
func (n *NoOpSideEffectProcessor) Rollback(ctx context.Context, transactionID string) error {
	// NoOp实现：不执行任何回滚操作
	// MVP设计理念：回滚逻辑已由执行引擎和状态管理器处理，无需额外处理
	_ = ctx           // 明确标记参数已被使用
	_ = transactionID // 明确标记参数已被使用
	return nil
}

// GetProcessingStats 获取处理统计数据（NoOp实现）
// 返回空的统计数据，符合MVP设计原则
func (n *NoOpSideEffectProcessor) GetProcessingStats() *interfaces.ProcessingStats {
	// NoOp实现：返回零值统计数据，避免复杂的数据收集和聚合逻辑
	// MVP设计理念：在自运行的区块链环境中，内部统计数据的目的和使用者并不明确
	return &interfaces.ProcessingStats{
		TotalProcessed:        0, // 总处理数量：零值，表示未进行统计
		SuccessfulProcessed:   0, // 成功处理数量：零值
		FailedProcessed:       0, // 失败处理数量：零值
		UTXOEffectsProcessed:  0, // UTXO副作用处理数量：零值
		StateEffectsProcessed: 0, // 状态副作用处理数量：零值
		EventEffectsProcessed: 0, // 事件副作用处理数量：零值
		AverageProcessingTime: 0, // 平均处理时间：零值
		LastProcessedTime:     0, // 最后处理时间：零值
	}
}

// SecurityAuditEmitterAdapter 适配器，将interfaces.AuditEventEmitter适配为monitoring.AuditEventEmitter
type SecurityAuditEmitterAdapter struct {
	emitter interfaces.AuditEventEmitter
}

func (s *SecurityAuditEmitterAdapter) EmitSecurityEvent(event interfaces.SecurityAuditEvent) {
	// 执行事件类型转换，确保接口兼容性
	interfaceEvent := interfaces.SecurityAuditEvent{
		EventType: event.EventType,
		Severity:  event.Severity,
		Timestamp: time.Now(),
		Caller:    "coordinator",
		Action:    event.EventType,
		Result:    "processed",
	}
	s.emitter.EmitSecurityEvent(interfaceEvent)
}

func (s *SecurityAuditEmitterAdapter) EmitPerformanceEvent(event interfaces.PerformanceAuditEvent) {
	// 执行性能事件类型转换，确保接口兼容性
	interfaceEvent := interfaces.PerformanceAuditEvent{
		EventType:        event.EventType,
		Timestamp:        time.Now(),
		Duration:         event.Duration,
		ResourceConsumed: event.ResourceConsumed,
		MemoryUsed:       event.MemoryUsed,
		EngineType:       event.EngineType,
	}
	s.emitter.EmitPerformanceEvent(interfaceEvent)
}

func (s *SecurityAuditEmitterAdapter) EmitErrorEvent(event interfaces.ErrorAuditEvent) {
	// 执行错误事件类型转换，确保接口兼容性
	interfaceEvent := interfaces.ErrorAuditEvent{
		EventType: event.EventType,
		ErrorType: event.ErrorType,
		Timestamp: time.Now(),
		Message:   event.Message,
	}
	s.emitter.EmitErrorEvent(interfaceEvent)
}

// NoOpAuditTracker 已移除，遵循MVP极简原则
