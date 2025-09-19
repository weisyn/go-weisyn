package coordinator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"

	"github.com/weisyn/v1/internal/core/execution/env"
	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/internal/core/execution/manager"

	// "github.com/weisyn/v1/internal/core/execution/monitoring" // MVP: 已移除
	"github.com/weisyn/v1/internal/core/execution/security"
)

// ResourceExecutionCoordinator 负责协调执行资源（合约/模型）的主要组件
//
// 职责：
// 1. 接收标准化的执行参数并进行预处理验证
// 2. 通过宿主注册表构建宿主绑定，为执行环境提供统一的宿主能力
// 3. 通过引擎管理器按引擎类型分发执行请求到对应引擎
// 4. 处理执行结果，包括副作用提交、指标记录和审计事件发射
// 5. 提供统一的错误分类和结构化错误处理
//
// 依赖注入设计：所有依赖组件通过构造函数注入，支持 fx 依赖注入框架
type ResourceExecutionCoordinator struct {
	// 引擎管理器：负责多引擎注册、查询与分发
	engineManager execution.EngineManager

	// 执行分发器：提供熔断、限流和智能调度功能
	dispatcher *manager.Dispatcher

	// 宿主能力注册表：聚合各宿主能力提供者，构建统一的宿主接口
	hostRegistry execution.HostCapabilityRegistry

	// 指标收集器：记录执行性能指标、错误统计等可观测数据
	metricsCollector interfaces.MetricsCollector

	// 审计事件发射器：发射安全、性能、错误等结构化事件
	auditEmitter interfaces.AuditEventEmitter

	// 副作用处理器：处理执行产生的副作用（如UTXO操作、状态变更）
	sideEffectProcessor interfaces.SideEffectProcessor

	// 安全集成器：联动各引擎安全管理器进行统一安全校验
	securityIntegrator *security.SecurityIntegrator

	// 配额管理器：管理执行资源配额和限制
	quotaManager *security.QuotaManager

	// 审计追踪器：记录和管理执行审计轨迹
	// auditTracker已移除，遵循MVP极简原则

	// 环境顾问：提供基于ML的智能执行决策和资源优化建议
	envAdvisor *env.CoordinatorAdapter

	// 日志记录器：记录执行过程中的调试和错误信息
	logger log.Logger

	// 配置：协调器运行时配置，如超时阈值、重试策略等
	config *CoordinatorConfig
}

// CoordinatorConfig 协调器极简配置参数。
// 遵循自运行原则：仅保留影响资源限制的核心配置，其余使用智能默认。
type CoordinatorConfig struct {
	// 默认执行超时时间（毫秒）
	// 这是唯一需要根据硬件性能调整的配置项
	DefaultTimeoutMs int64

	// 最大资源限制 - 保护节点资源的关键配置
	MaxExecutionFeeLimit uint64

	// 最大内存限制（字节）- 保护节点内存的关键配置
	MaxMemoryLimit uint32

	// 以下功能均为智能默认启用，无需配置：
	// - 预处理验证：始终启用，确保参数安全
	// - 后处理：始终启用，确保副作用处理
	// - 审计事件：始终启用基础审计
	// - 错误重试：智能重试（3次），无需配置
	// - 安全校验：始终启用，确保执行安全
	// - 配额管理：始终启用，保护系统资源
	// - 审计追踪：已移除，遵循MVP极简原则
}

// NewResourceExecutionCoordinator 创建资源执行协调器实例
func NewResourceExecutionCoordinator(
	engineManager execution.EngineManager,
	dispatcher *manager.Dispatcher,
	hostRegistry execution.HostCapabilityRegistry,
	metricsCollector interfaces.MetricsCollector,
	auditEmitter interfaces.AuditEventEmitter,
	sideEffectProcessor interfaces.SideEffectProcessor,
	securityIntegrator *security.SecurityIntegrator,
	quotaManager *security.QuotaManager,
	// auditTracker已移除，遵循MVP极简原则,
	envAdvisor *env.CoordinatorAdapter,
	logger log.Logger,
	config *CoordinatorConfig,
) *ResourceExecutionCoordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	// 🔧 强制修复内存限制以支持大型WASM合约
	if config.MaxMemoryLimit < 268435456 {
		config.MaxMemoryLimit = 268435456 // 强制设置为256MB
	}

	return &ResourceExecutionCoordinator{
		engineManager:       engineManager,
		dispatcher:          dispatcher,
		hostRegistry:        hostRegistry,
		metricsCollector:    metricsCollector,
		auditEmitter:        auditEmitter,
		sideEffectProcessor: sideEffectProcessor,
		securityIntegrator:  securityIntegrator,
		quotaManager:        quotaManager,
		// auditTracker 已移除，遵循MVP极简原则
		envAdvisor: envAdvisor,
		logger:     logger,
		config:     config,
	}
}

// DefaultCoordinatorConfig 返回零配置的协调器配置。
// 体现自运行原则：仅保留资源限制配置，其余使用智能默认。
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		DefaultTimeoutMs:     180000,    // 🔧 修复：3分钟执行超时
		MaxExecutionFeeLimit: 1000000,   // 100万资源 - 保护节点资源
		MaxMemoryLimit:       268435456, // 256MB - 支持大型WASM合约
		// 其他所有功能均为智能默认启用：
		// - 预处理验证、后处理、审计事件、安全校验、配额管理
		// - 错误重试：智能3次重试，无需配置
	}
}

// Execute 执行资源的核心方法
func (c *ResourceExecutionCoordinator) Execute(ctx context.Context, params types.ExecutionParams) (types.ExecutionResult, error) {
	// 记录执行开始时间
	startTime := time.Now()

	// MVP极简：移除复杂的审计数据收集，仅保留基础日志

	// 从参数中提取引擎类型
	engineType, err := c.extractEngineType(params)
	if err != nil {
		return types.ExecutionResult{}, c.wrapError(ErrorTypeParameterValidation, "failed to extract engine type", err, params)
	}

	// 记录执行开始指标
	c.metricsCollector.RecordExecutionStart(engineType, params.ResourceID)

	// 🔧 调试日志：执行协调器开始
	if c.logger != nil {
		c.logger.Debugf("🔧 执行协调器开始: ResourceID=%x, Entry=%s, EngineType=%s", params.ResourceID, params.Entry, engineType)
	}

	// 阶段0：ML智能决策和参数优化
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段0: 开始ML智能决策和参数优化")
	}
	optimizedParams, mlAdvice, err := c.applyMLOptimization(ctx, params)
	if err != nil {
		// ML优化失败不影响执行，使用原始参数并记录警告
		c.recordMLOptimizationWarning(err)
		optimizedParams = params
		mlAdvice = nil
		if c.logger != nil {
			c.logger.Debugf("🔧 阶段0: ML优化失败，使用原始参数")
		}
	} else if mlAdvice != nil {
		// 记录ML优化建议应用
		c.recordMLOptimizationApplied(mlAdvice)
		if c.logger != nil {
			c.logger.Debugf("🔧 阶段0: ML优化成功应用")
		}
	}

	// 阶段1：参数预处理
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段1: 开始参数预处理验证")
	}
	// 预处理验证（自运行节点始终启用，确保参数安全）
	if true { // 原：c.config.EnablePreprocessValidation
		if err := c.preprocessParameters(optimizedParams); err != nil {
			if c.logger != nil {
				c.logger.Errorf("❌ 阶段1: 参数预处理失败: %v", err)
			}
			c.recordExecutionFailure(engineType, startTime, ErrorTypeParameterValidation, err)

			// 发射参数验证失败的安全审计事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitSecurityEvent(interfaces.SecurityAuditEvent{
					EventType: "parameter_validation_failed",
					Severity:  "high",
					Timestamp: time.Now(),
					Caller:    params.Caller,
					Action:    "parameter_validation",
					Result:    "failed",
				})
			}

			return types.ExecutionResult{}, err
		}
	}

	// 阶段1.5：安全校验
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段1.5: 开始安全校验")
	}
	// 安全校验（自运行节点始终启用，确保执行安全）
	if c.securityIntegrator != nil { // 原：c.config.EnableSecurityValidation &&
		if err := c.securityIntegrator.ValidateExecution(ctx, params); err != nil {
			if c.logger != nil {
				c.logger.Errorf("❌ 阶段1.5: 安全校验失败: %v", err)
			}
			c.recordExecutionFailure(engineType, startTime, ErrorTypeParameterValidation, err)

			// 发射安全校验失败的安全审计事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitSecurityEvent(interfaces.SecurityAuditEvent{
					EventType: "security_validation_failed",
					Severity:  "critical",
					Timestamp: time.Now(),
					Caller:    params.Caller,
					Action:    "security_validation",
					Result:    "failed",
				})
			}

			return types.ExecutionResult{}, c.wrapError(ErrorTypeParameterValidation, "security validation failed", err, params)
		}
	}

	// 阶段1.8：配额检查和分配
	var quotaAllocation *security.QuotaAllocation
	// 配额管理（自运行节点始终启用，保护系统资源）
	if c.quotaManager != nil { // 原：c.config.EnableQuotaManagement &&
		allocation, err := c.quotaManager.CheckQuota(ctx, params)
		if err != nil {
			c.recordExecutionFailure(engineType, startTime, ErrorTypeResourceLimit, err)

			// 发射配额检查失败的安全审计事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitSecurityEvent(interfaces.SecurityAuditEvent{
					EventType: "quota_check_failed",
					Severity:  "high",
					Timestamp: time.Now(),
					Caller:    params.Caller,
					Action:    "quota_check",
					Result:    "failed",
				})
			}

			return types.ExecutionResult{}, c.wrapError(ErrorTypeResourceLimit, "quota check failed", err, params)
		}
		quotaAllocation = allocation
	}

	// 确保配额在执行结束后释放
	defer func() {
		if quotaAllocation != nil {
			if releaseErr := c.quotaManager.ReleaseQuota(quotaAllocation.AllocationID); releaseErr != nil {
				// 记录配额释放失败，但不影响主流程
				// 审计事件（自运行节点始终启用基础审计）
				if true { // 原：c.config.EnableAuditEvents
					c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
						EventType: "internal_error",
						ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
						Timestamp: time.Now(),
						Message:   fmt.Sprintf("Failed to release quota: %v", releaseErr),
						// Context field removed in simplified ErrorAuditEvent
					})
				}
			}
		}
	}()

	// 阶段2：宿主绑定
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段2: 开始宿主绑定")
	}
	hostBinding := c.hostRegistry.BuildStandardInterface()

	// 阶段2.5：为引擎适配器绑定宿主接口
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段2.5: 开始引擎适配器绑定")
	}
	if err := c.bindHostToEngine(engineType, hostBinding); err != nil {
		if c.logger != nil {
			c.logger.Errorf("❌ 阶段2.5: 引擎适配器绑定失败: %v", err)
		}
		c.recordExecutionFailure(engineType, startTime, ErrorTypeHostFunction, err)

		// 发射宿主绑定失败的错误审计事件
		// 审计事件（自运行节点始终启用基础审计）
		if true { // 原：c.config.EnableAuditEvents
			c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
				EventType: "host_binding_error",
				ErrorType: types.ExecutionErrorType(ErrorTypeHostFunction),
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Host binding failed: %v", err),
				// Context field removed in simplified ErrorAuditEvent
			})
		}

		return types.ExecutionResult{}, c.wrapError(ErrorTypeHostFunction, "host binding failed", err, params)
	}

	// 阶段3：引擎分发执行
	if c.logger != nil {
		c.logger.Debugf("🔧 阶段3: 开始引擎分发执行，引擎类型=%s", engineType)
	}
	result, err := c.executeWithEngine(ctx, engineType, params, hostBinding)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("❌ 阶段3: 引擎执行失败: %v", err)
		}
		c.recordExecutionFailure(engineType, startTime, ErrorTypeEngineExecution, err)

		// 发射引擎执行失败的错误审计事件
		// 审计事件（自运行节点始终启用基础审计）
		if true { // 原：c.config.EnableAuditEvents
			c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
				EventType: "engine_execution_error",
				ErrorType: types.ExecutionErrorType(ErrorTypeEngineExecution),
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Engine execution failed: %v", err),
				// Context field removed in simplified ErrorAuditEvent
			})
		}

		return types.ExecutionResult{}, err
	}

	// 阶段4：结果后处理
	// 后处理（自运行节点始终启用，确保副作用处理）
	if true { // 原：c.config.EnablePostProcessing
		if err := c.postProcessResult(result, params); err != nil {
			c.recordExecutionFailure(engineType, startTime, ErrorTypeInternal, err)

			// 发射后处理失败的错误审计事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
					EventType: "internal_error",
					ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Post-processing failed: %v", err),
					// Context field removed in simplified ErrorAuditEvent
				})
			}

			return types.ExecutionResult{}, c.wrapError(ErrorTypeInternal, "post-processing failed", err, params)
		}
	}

	// 记录执行成功指标
	duration := time.Since(startTime)
	c.metricsCollector.RecordExecutionComplete(engineType, duration, true)
	c.metricsCollector.RecordResourceConsumption(engineType, result.Consumed)

	// 集成引擎执行画像：把当前指标快照写入结果元数据
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	if em, ok := c.engineManager.(interface {
		GetMetrics() map[types.EngineType]interface{}
	}); ok {
		result.Metadata["engine_metrics"] = em.GetMetrics()
	}

	// 发射性能审计事件（含关键统计）
	// 审计事件（自运行节点始终启用基础审计）
	if true { // 原：c.config.EnableAuditEvents {
		c.auditEmitter.EmitPerformanceEvent(interfaces.PerformanceAuditEvent{
			EventType:        "execution_complete",
			Timestamp:        time.Now(),
			Duration:         duration,
			ResourceConsumed: result.Consumed,
			MemoryUsed:       c.extractMemoryUsage(result),
			EngineType:       engineType,
			// ResourceID, Description, Metrics fields removed in simplified PerformanceAuditEvent
		})
	}

	// MVP极简：移除复杂的审计轨迹功能
	if false { // c.config.EnableAuditTracking && c.auditTracker != nil {
		// auditExecCtx.EngineType = engineType // MVP: 已移除

		// 构建审计执行结果
		// auditResult := &monitoring.AuditExecutionResult{
		// 	Success:       true,
		// 	ReturnData:    result.ReturnData,
		// 	ResourceConsumed:   result.Consumed,
		// 	ExecutionTime: duration,
		// }

		// 构建审计性能指标
		// auditMetrics := monitoring.AuditPerformanceMetrics{
		// 	TotalDuration:     duration,
		// 	ExecutionDuration: duration,
		// }

		// 转换安全事件类型
		// var auditSecEvents []monitoring.AuditSecurityEvent // MVP: 已移除
		// for _, evt := range auditSecurityEvents { // MVP: 已移除
		//	auditSecEvents = append(auditSecEvents, monitoring.AuditSecurityEvent{
		//		EventType:   evt.EventType,
		//		Severity:    evt.Severity,
		//		Timestamp:   time.Now(),
		//		Description: evt.Description,
		//	})
		// }

		// 记录审计轨迹 - 使用新的接口方法
		// trackingID := c.auditTracker.StartExecution(ctx, params) // MVP: 已移除

		// 记录执行完成
		// c.auditTracker.EndExecution(trackingID, result, nil) // MVP: 已移除

		// 审计记录成功，无需额外处理
	}

	return result, nil
}

// extractEngineType 从执行参数中提取引擎类型
func (c *ResourceExecutionCoordinator) extractEngineType(params types.ExecutionParams) (types.EngineType, error) {
	// 从执行上下文中查找引擎类型
	if engineTypeVal, exists := params.Context["engine_type"]; exists {
		if engineTypeStr, ok := engineTypeVal.(string); ok {
			return types.EngineType(engineTypeStr), nil
		}
		return "", fmt.Errorf("engine_type in context is not a string: %T", engineTypeVal)
	}

	// 如果未指定引擎类型，默认使用WASM引擎
	return types.EngineTypeWASM, nil
}

// preprocessParameters 预处理和验证执行参数
func (c *ResourceExecutionCoordinator) preprocessParameters(params types.ExecutionParams) error {
	// 验证资源ID
	if len(params.ResourceID) == 0 {
		return errors.New("resource ID cannot be empty")
	}

	// 验证资源限制
	if params.ExecutionFeeLimit == 0 {
		return errors.New("资源 limit must be greater than zero")
	}
	if params.ExecutionFeeLimit > c.config.MaxExecutionFeeLimit {
		return fmt.Errorf("资源 limit %d exceeds maximum %d", params.ExecutionFeeLimit, c.config.MaxExecutionFeeLimit)
	}

	// 验证内存限制
	if params.MemoryLimit > c.config.MaxMemoryLimit {
		return fmt.Errorf("memory limit %d exceeds maximum %d", params.MemoryLimit, c.config.MaxMemoryLimit)
	}

	// 验证超时时间
	if params.Timeout <= 0 {
		return errors.New("timeout must be greater than zero")
	}

	// 验证调用者地址格式
	if params.Caller == "" {
		return errors.New("caller address cannot be empty")
	}

	// 验证合约地址格式
	if params.ContractAddr == "" {
		return errors.New("contract address cannot be empty")
	}

	return nil
}

// executeWithEngine 通过指定引擎执行资源
func (c *ResourceExecutionCoordinator) executeWithEngine(
	ctx context.Context,
	engineType types.EngineType,
	params types.ExecutionParams,
	hostBinding execution.HostStandardInterface,
) (types.ExecutionResult, error) {
	if c.logger != nil {
		c.logger.Debugf("🔧 executeWithEngine开始: 引擎类型=%s, ResourceID=%x, Entry=%s", engineType, params.ResourceID, params.Entry)
	}

	// 设置执行超时
	timeoutDuration := time.Duration(params.Timeout) * time.Millisecond
	if c.logger != nil {
		c.logger.Debugf("🔧 设置执行超时: %v", timeoutDuration)
	}
	execCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()

	// 通过分发器执行（包含熔断/限流/智能调度）
	if c.logger != nil {
		c.logger.Debugf("🔧 开始通过分发器执行")
	}
	var result *types.ExecutionResult
	var err error

	if c.dispatcher != nil {
		// 优先使用分发器（提供熔断、限流和智能调度功能）
		if c.logger != nil {
			c.logger.Debugf("🔧 使用分发器执行: 引擎类型=%s", engineType)
		}
		result, err = c.dispatcher.Dispatch(engineType, params)
		if c.logger != nil {
			if err != nil {
				c.logger.Errorf("❌ 分发器执行失败: %v", err)
			} else {
				c.logger.Debugf("✅ 分发器执行成功")
			}
		}
	} else {
		// 回退到直接引擎管理器调用
		if c.logger != nil {
			c.logger.Debugf("🔧 使用引擎管理器直接执行: 引擎类型=%s", engineType)
		}
		result, err = c.engineManager.Execute(engineType, params)
		if c.logger != nil {
			if err != nil {
				c.logger.Errorf("❌ 引擎管理器执行失败: %v", err)
			} else {
				c.logger.Debugf("✅ 引擎管理器执行成功")
			}
		}
	}
	if err != nil {
		// 检查是否为超时错误
		if execCtx.Err() == context.DeadlineExceeded {
			// 发射超时安全审计事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitSecurityEvent(interfaces.SecurityAuditEvent{
					EventType: "execution_timeout",
					Severity:  "medium",
					Timestamp: time.Now(),
					Caller:    params.Caller,
					Action:    "execution",
					Result:    "timeout",
				})
			}
			return types.ExecutionResult{}, c.wrapError(ErrorTypeTimeout, "execution timeout", err, params)
		}
		return types.ExecutionResult{}, c.wrapError(ErrorTypeEngineExecution, "engine execution failed", err, params)
	}

	return *result, nil
}

// postProcessResult 后处理执行结果
func (c *ResourceExecutionCoordinator) postProcessResult(result types.ExecutionResult, params types.ExecutionParams) error {
	// 从执行结果的元数据中解析副作用
	sideEffects, err := c.extractSideEffects(result)
	if err != nil {
		// 发射副作用解析错误事件
		// 审计事件（自运行节点始终启用基础审计）
		if true { // 原：c.config.EnableAuditEvents
			c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
				EventType: "internal_error",
				ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("failed to extract side effects: %v", err),
				// Context field removed in simplified ErrorAuditEvent
			})
		}
		return fmt.Errorf("failed to extract side effects: %w", err)
	}

	// 处理UTXO副作用
	if len(sideEffects.UTXOEffects) > 0 {
		// 转换为interfaces包的类型
		utxoEffects := make([]interfaces.UTXOSideEffect, len(sideEffects.UTXOEffects))
		for i := range sideEffects.UTXOEffects {
			utxoEffects[i] = interfaces.UTXOSideEffect{}
		}
		if err := c.sideEffectProcessor.ProcessUTXOSideEffects(context.Background(), utxoEffects); err != nil {
			// 发射UTXO副作用处理错误事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
					EventType: "internal_error",
					ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("failed to process UTXO side effects: %v", err),
					// Context field removed in simplified ErrorAuditEvent
				})
			}
			return fmt.Errorf("failed to process UTXO side effects: %w", err)
		}
	}

	// 处理状态副作用
	if len(sideEffects.StateEffects) > 0 {
		// 转换为interfaces包的类型
		stateEffects := make([]interfaces.StateSideEffect, len(sideEffects.StateEffects))
		for i := range sideEffects.StateEffects {
			stateEffects[i] = interfaces.StateSideEffect{}
		}
		if err := c.sideEffectProcessor.ProcessStateSideEffects(context.Background(), stateEffects); err != nil {
			// 发射状态副作用处理错误事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
					EventType: "internal_error",
					ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("failed to process state side effects: %v", err),
					// Context field removed in simplified ErrorAuditEvent
				})
			}
			return fmt.Errorf("failed to process state side effects: %w", err)
		}
	}

	// 处理事件副作用
	if len(sideEffects.EventEffects) > 0 {
		// 转换为interfaces包的类型
		eventEffects := make([]interfaces.EventSideEffect, len(sideEffects.EventEffects))
		for i := range sideEffects.EventEffects {
			eventEffects[i] = interfaces.EventSideEffect{}
		}
		if err := c.sideEffectProcessor.ProcessEventSideEffects(context.Background(), eventEffects); err != nil {
			// 发射事件副作用处理错误事件
			// 审计事件（自运行节点始终启用基础审计）
			if true { // 原：c.config.EnableAuditEvents
				c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
					EventType: "internal_error",
					ErrorType: types.ExecutionErrorType(ErrorTypeInternal),
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("failed to process event side effects: %v", err),
					// Context field removed in simplified ErrorAuditEvent
				})
			}
			return fmt.Errorf("failed to process event side effects: %w", err)
		}
	}

	// 发射副作用处理成功的审计事件
	// 审计事件（自运行节点始终启用基础审计）
	if true { // 原：c.config.EnableAuditEvents && (len(sideEffects.UTXOEffects) > 0 || len(sideEffects.StateEffects) > 0 || len(sideEffects.EventEffects) > 0) {
		c.auditEmitter.EmitPerformanceEvent(interfaces.PerformanceAuditEvent{
			EventType:        "side_effects_processed",
			Timestamp:        time.Now(),
			Duration:         0,
			ResourceConsumed: 0,
			MemoryUsed:       0,
			EngineType:       types.EngineTypeWASM, // 默认值
			// ResourceID, Description, Metrics fields removed in simplified PerformanceAuditEvent
		})
	}

	return nil
}

// extractSideEffects 从执行结果中提取副作用信息
func (c *ResourceExecutionCoordinator) extractSideEffects(result types.ExecutionResult) (*interfaces.SideEffectCollection, error) {
	sideEffects := &interfaces.SideEffectCollection{
		UTXOEffects:  []interfaces.UTXOSideEffect{},
		StateEffects: []interfaces.StateSideEffect{},
		EventEffects: []interfaces.EventSideEffect{},
	}

	// 从结果元数据中提取副作用信息
	if result.Metadata == nil {
		return sideEffects, nil
	}

	// 提取UTXO副作用
	if utxoData, exists := result.Metadata["utxo_side_effects"]; exists {
		if utxoEffects, ok := utxoData.([]interfaces.UTXOSideEffect); ok {
			sideEffects.UTXOEffects = utxoEffects
		} else if utxoMap, ok := utxoData.([]map[string]interface{}); ok {
			// 处理通用map格式的UTXO副作用
			for _, effect := range utxoMap {
				if effectType, ok := effect["type"].(string); ok {
					utxoEffect := interfaces.UTXOSideEffect{
						Type: interfaces.UTXOEffectType(effectType),
					}
					if utxoID, ok := effect["utxo_id"].(string); ok {
						utxoEffect.UTXOID = utxoID
					}
					if amount, ok := effect["amount"].(uint64); ok {
						utxoEffect.Amount = amount
					}
					if owner, ok := effect["owner"].(string); ok {
						utxoEffect.Owner = owner
					}
					if tokenType, ok := effect["token_type"].(string); ok {
						utxoEffect.TokenType = tokenType
					}
					sideEffects.UTXOEffects = append(sideEffects.UTXOEffects, utxoEffect)
				}
			}
		}
	}

	// 提取状态副作用
	if stateData, exists := result.Metadata["state_side_effects"]; exists {
		if stateEffects, ok := stateData.([]interfaces.StateSideEffect); ok {
			sideEffects.StateEffects = stateEffects
		} else if stateMap, ok := stateData.([]map[string]interface{}); ok {
			// 处理通用map格式的状态副作用
			for _, effect := range stateMap {
				if key, ok := effect["key"].(string); ok {
					stateEffect := interfaces.StateSideEffect{
						Key: key,
					}
					if effectType, ok := effect["type"].(string); ok {
						stateEffect.Type = interfaces.StateEffectType(effectType)
					}
					if oldValue, ok := effect["old_value"].([]byte); ok {
						stateEffect.OldValue = oldValue
					}
					if newValue, ok := effect["new_value"].([]byte); ok {
						stateEffect.NewValue = newValue
					}
					if contract, ok := effect["contract"].(string); ok {
						stateEffect.Contract = contract
					}
					sideEffects.StateEffects = append(sideEffects.StateEffects, stateEffect)
				}
			}
		}
	}

	// 提取事件副作用
	if eventData, exists := result.Metadata["event_side_effects"]; exists {
		if eventEffects, ok := eventData.([]interfaces.EventSideEffect); ok {
			sideEffects.EventEffects = eventEffects
		} else if eventMap, ok := eventData.([]map[string]interface{}); ok {
			// 处理通用map格式的事件副作用
			for _, effect := range eventMap {
				if eventName, ok := effect["event_name"].(string); ok {
					eventEffect := interfaces.EventSideEffect{
						EventName: eventName,
					}
					if effectType, ok := effect["type"].(string); ok {
						eventEffect.Type = interfaces.EventEffectType(effectType)
					}
					if contract, ok := effect["contract"].(string); ok {
						eventEffect.Contract = contract
					}
					if data, ok := effect["data"].(map[string]interface{}); ok {
						eventEffect.Data = data
					}
					if indexed, ok := effect["indexed"].([]string); ok {
						eventEffect.Indexed = indexed
					}
					if timestamp, ok := effect["timestamp"].(int64); ok {
						eventEffect.Timestamp = timestamp
					}
					sideEffects.EventEffects = append(sideEffects.EventEffects, eventEffect)
				}
			}
		}
	}

	return sideEffects, nil
}

// recordExecutionFailure 记录执行失败的指标和事件
func (c *ResourceExecutionCoordinator) recordExecutionFailure(
	engineType types.EngineType,
	startTime time.Time,
	errorType ExecutionErrorType,
	err error,
) {
	duration := time.Since(startTime)

	// 记录失败指标
	c.metricsCollector.RecordExecutionComplete(engineType, duration, false)
	c.metricsCollector.RecordError(types.ExecutionErrorType(errorType), err.Error())
}

// wrapError 包装错误为标准化执行错误
func (c *ResourceExecutionCoordinator) wrapError(
	errorType ExecutionErrorType,
	message string,
	cause error,
	params types.ExecutionParams,
) error {
	return &ExecutionError{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now().Unix(),
	}
}

// ==================== 本地类型定义 ====================
//
// 注意：副作用相关类型已统一移至 internal/core/execution/interfaces/effects.go
// 审计事件相关类型已统一移至 internal/core/execution/interfaces/monitoring.go
// 本文件仅保留协调器特有的本地类型定义

// ExecutionErrorType 执行错误类型枚举
// 定义了执行过程中可能出现的各种错误类型，用于错误分类和处理
type ExecutionErrorType string

const (
	ErrorTypeParameterValidation ExecutionErrorType = "parameter_validation" // 参数验证错误
	ErrorTypeEngineExecution     ExecutionErrorType = "engine_execution"     // 引擎执行错误
	ErrorTypeHostFunction        ExecutionErrorType = "host_function"        // 宿主函数错误
	ErrorTypeTimeout             ExecutionErrorType = "timeout"              // 执行超时错误
	ErrorTypeResourceLimit       ExecutionErrorType = "resource_limit"       // 资源限制错误
	ErrorTypeInternal            ExecutionErrorType = "internal"             // 内部错误
)

// ExecutionError 标准化执行错误类型
// 提供了结构化的错误信息，包含错误类型、消息、原因和时间戳
type ExecutionError struct {
	Type      ExecutionErrorType `json:"type"`      // 错误类型分类
	Message   string             `json:"message"`   // 错误描述信息
	Cause     error              `json:"-"`         // 原始错误对象（不序列化）
	Timestamp int64              `json:"timestamp"` // 错误发生时间戳
}

// Error 实现error接口，提供错误信息的字符串表示
func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap 实现errors.Unwrap接口，支持错误链的向上传播
func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// bindHostToEngine 为指定引擎类型的适配器绑定宿主接口
func (c *ResourceExecutionCoordinator) bindHostToEngine(
	engineType types.EngineType,
	hostBinding execution.HostStandardInterface,
) error {
	// 从引擎管理器获取指定类型的引擎适配器
	adapter, found := c.engineManager.GetEngine(engineType)
	if !found {
		return fmt.Errorf("engine adapter not found for type %s", engineType)
	}

	// 创建宿主绑定接口（适配器期望的HostBinding类型）
	binding := &standardHostBinding{
		stdInterface: hostBinding,
	}

	// 调用适配器的BindHost方法
	if err := adapter.BindHost(binding); err != nil {
		return fmt.Errorf("failed to bind host to %s engine: %w", engineType, err)
	}

	return nil
}

// standardHostBinding 标准宿主绑定的适配器实现
// 将HostStandardInterface适配为EngineAdapter期望的HostBinding接口
type standardHostBinding struct {
	stdInterface execution.HostStandardInterface
}

// Standard 实现execution.HostBinding接口的Standard方法
// 返回标准宿主接口，供引擎适配器使用
func (b *standardHostBinding) Standard() execution.HostStandardInterface {
	return b.stdInterface
}

// applyMLOptimization 应用ML智能决策和参数优化
func (c *ResourceExecutionCoordinator) applyMLOptimization(ctx context.Context, params types.ExecutionParams) (types.ExecutionParams, *MLOptimizationAdvice, error) {
	if c.envAdvisor == nil {
		// 如果没有环境顾问，返回原始参数
		return params, nil, nil
	}

	// 创建参数副本，避免修改原始参数
	optimizedParams := params

	var mlAdvice *MLOptimizationAdvice

	// 1. 获取资源限制建议（基于合约地址和入口函数）
	if params.ContractAddr != "" && params.Entry != "" {
		if resourceAdvice, err := c.envAdvisor.AdviseResourceLimits(ctx, params.ContractAddr, params.Entry); err == nil && resourceAdvice != nil {
			// 应用资源建议（仅在建议值更优时）
			if resourceAdvice.ExecutionFeeLimit > 0 && (params.ExecutionFeeLimit == 0 || resourceAdvice.ExecutionFeeLimit < params.ExecutionFeeLimit) {
				optimizedParams.ExecutionFeeLimit = resourceAdvice.ExecutionFeeLimit
			}
			if resourceAdvice.MemoryLimit > 0 && (params.MemoryLimit == 0 || resourceAdvice.MemoryLimit < params.MemoryLimit) {
				optimizedParams.MemoryLimit = resourceAdvice.MemoryLimit
			}
			if resourceAdvice.TimeoutMs > 0 && (params.Timeout == 0 || resourceAdvice.TimeoutMs < params.Timeout) {
				optimizedParams.Timeout = resourceAdvice.TimeoutMs
			}

			if mlAdvice == nil {
				mlAdvice = &MLOptimizationAdvice{}
			}
			mlAdvice.ResourceAdvice = resourceAdvice
		}
	}

	// 2. 获取执行成本预测
	if costPrediction, err := c.envAdvisor.PredictExecutionCost(ctx, optimizedParams); err == nil && costPrediction != nil {
		// 如果预测的资源消耗显著低于当前资源限制，可以适当调整
		if costPrediction.ExpectedResource > 0 && costPrediction.ConfidencePct > 0.7 {
			// 留出20%的安全边际
			suggestedResource := uint64(float64(costPrediction.ExpectedResource) * 1.2)
			if optimizedParams.ExecutionFeeLimit == 0 || suggestedResource < optimizedParams.ExecutionFeeLimit {
				optimizedParams.ExecutionFeeLimit = suggestedResource
			}
		}

		if mlAdvice == nil {
			mlAdvice = &MLOptimizationAdvice{}
		}
		mlAdvice.CostPrediction = costPrediction
	}

	// 3. 分析历史性能（用于审计和日志）
	if params.ContractAddr != "" {
		if perfAnalysis, err := c.envAdvisor.AnalyzePerformanceHistory(ctx, params.ContractAddr); err == nil && perfAnalysis != nil {
			if mlAdvice == nil {
				mlAdvice = &MLOptimizationAdvice{}
			}
			mlAdvice.PerformanceAnalysis = perfAnalysis
		}
	}

	return optimizedParams, mlAdvice, nil
}

// recordMLOptimizationWarning 记录ML优化警告
func (c *ResourceExecutionCoordinator) recordMLOptimizationWarning(err error) {
	// 审计事件（自运行节点始终启用基础审计）
	if c.auditEmitter != nil { // 原：&& c.config.EnableAuditEvents
		c.auditEmitter.EmitErrorEvent(interfaces.ErrorAuditEvent{
			EventType: "ml_optimization_warning",
			Message:   fmt.Sprintf("ML optimization failed: %v", err),
			// Context field removed in simplified ErrorAuditEvent
			Timestamp: time.Now(),
		})
	}
}

// recordMLOptimizationApplied 记录ML优化建议应用
func (c *ResourceExecutionCoordinator) recordMLOptimizationApplied(advice *MLOptimizationAdvice) {
	// 审计事件（自运行节点始终启用基础审计）
	if c.auditEmitter != nil { // 原：&& c.config.EnableAuditEvents
		context := map[string]any{
			"optimization_applied": true,
		}

		if advice.ResourceAdvice != nil {
			context["resource_advice"] = map[string]any{
				"资源_limit":     advice.ResourceAdvice.ExecutionFeeLimit,
				"memory_limit": advice.ResourceAdvice.MemoryLimit,
				"timeout_ms":   advice.ResourceAdvice.TimeoutMs,
				"rationale":    advice.ResourceAdvice.Rationale,
			}
		}

		if advice.CostPrediction != nil {
			context["cost_prediction"] = map[string]any{
				"expected_resource": advice.CostPrediction.ExpectedResource,
				"expected_time_ms":  advice.CostPrediction.ExpectedTimeMs,
				"confidence_pct":    advice.CostPrediction.ConfidencePct,
				"model_version":     advice.CostPrediction.ModelVersion,
			}
		}

		c.auditEmitter.EmitPerformanceEvent(interfaces.PerformanceAuditEvent{
			EventType:        "ml_optimization_applied",
			Timestamp:        time.Now(),
			Duration:         0,
			ResourceConsumed: 0,
			MemoryUsed:       0,
			EngineType:       types.EngineTypeWASM, // 默认值
			// ResourceID, Description, Metrics fields removed in simplified PerformanceAuditEvent
		})
	}
}

// MLOptimizationAdvice ML优化建议集合
type MLOptimizationAdvice struct {
	ResourceAdvice      *env.CoordinatorResourceAdvice
	CostPrediction      *env.CoordinatorCostPrediction
	PerformanceAnalysis *env.CoordinatorPerformanceAnalysis
}

// extractMemoryUsage 从执行结果中提取内存使用量
func (c *ResourceExecutionCoordinator) extractMemoryUsage(result types.ExecutionResult) uint32 {
	if result.Metadata == nil {
		return 0
	}

	// 尝试从元数据中提取内存使用量
	if memUsage, exists := result.Metadata["memory_used"]; exists {
		if memUsageUint32, ok := memUsage.(uint32); ok {
			return memUsageUint32
		}
		if memUsageUint64, ok := memUsage.(uint64); ok {
			return uint32(memUsageUint64)
		}
		if memUsageInt, ok := memUsage.(int); ok {
			return uint32(memUsageInt)
		}
	}

	// 如果没有明确的内存使用量，尝试从其他指标推算
	if result.Consumed > 0 {
		// 基于资源消耗的粗略估算（1 资源 ≈ 1 字节内存）
		return uint32(result.Consumed)
	}

	return 0
}
