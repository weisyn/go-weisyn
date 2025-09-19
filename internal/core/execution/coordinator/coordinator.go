// Package coordinator 执行协调器模块
//
// 本模块实现了执行层的核心协调功能，负责统一管理和调度智能合约及AI模型的执行。
// 采用MVP设计原则，专注于核心协调功能，避免过度设计。
//
// 核心职责：
// 1. 提供统一的执行协调接口，封装复杂的多引擎执行逻辑
// 2. 集成安全管理、资源管理、环境顾问等支撑组件
// 3. 提供灵活的工厂函数，支持不同场景的协调器创建
// 4. 通过NoOp实现提供测试友好的轻量化选项
//
// 设计原则：
// - 依赖注入：所有组件通过fx框架进行依赖注入
// - 接口优先：面向接口编程，降低组件间耦合
// - MVP极简：默认使用NoOp实现，避免不必要的复杂性
// - 零配置：提供合理的默认配置，支持开箱即用
package coordinator

import (
	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	"github.com/weisyn/v1/internal/core/execution/env"
	"github.com/weisyn/v1/internal/core/execution/manager"
	"github.com/weisyn/v1/internal/core/execution/security"
)

// NewExecutionCoordinatorSimple 创建执行协调器的简化工厂函数
//
// 功能说明：
// 本函数是从module.go迁移而来的工厂函数，专门用于fx依赖注入框架。
// 根据提供的dispatcher参数智能选择创建策略：
// - 如果提供了dispatcher：直接使用，获得完整的熔断/限流/智能调度功能
// - 如果未提供dispatcher：自动创建默认配置的完整协调器
//
// 设计优势：
// 1. 灵活适配：支持手动提供dispatcher或自动创建
// 2. 零配置：未提供dispatcher时使用合理默认配置
// 3. 一致性：无论哪种方式都获得生产级的安全配置
// 4. 可测试：通过NoOp组件保持测试友好性
//
// 参数说明：
// - engineManager：引擎管理器，提供多引擎注册、查询与分发能力
// - hostRegistry：宿主能力注册表，聚合各宿主能力提供者，构建统一的宿主接口
// - envAdvisor：环境顾问，提供基于ML的智能执行决策和资源优化建议
// - dispatcher：执行分发器，可选参数，提供熔断、限流和智能调度功能
// - logger：日志记录器，用于记录协调器创建和运行日志
//
// 返回值：
// - execution.ExecutionCoordinator：实现了ExecutionCoordinator接口的协调器实例
func NewExecutionCoordinatorSimple(
	// 引擎管理器：提供多引擎注册、查询与分发能力
	engineManager execution.EngineManager,
	// 宿主能力注册表：聚合各宿主能力提供者，构建统一的宿主接口
	hostRegistry execution.HostCapabilityRegistry,
	// 环境顾问：提供基于ML的智能执行决策和资源优化建议
	envAdvisor *env.CoordinatorAdapter,
	// 执行分发器：可选参数，提供熔断、限流和智能调度功能
	dispatcher *manager.Dispatcher,
	// 日志记录器：用于记录协调器创建和运行日志
	logger log.Logger,
) execution.ExecutionCoordinator {
	// 策略1：优先使用提供的dispatcher（手动配置模式）
	if dispatcher != nil {
		if logger != nil {
			logger.Info("使用提供的执行分发器（熔断/限流/智能调度）")
		}
		// 直接使用提供的dispatcher创建协调器
		// 注意：这里使用NoOp组件以符合MVP设计原则，实际业务逻辑由dispatcher和安全组件处理
		return NewExecutionCoordinator(
			engineManager,
			dispatcher,
			hostRegistry,
			&NoOpMetricsCollector{},                 // MVP：使用NoOp指标收集器
			&NoOpAuditEventEmitter{},                // MVP：使用NoOp审计发射器
			&NoOpSideEffectProcessor{},              // MVP：使用NoOp副作用处理器
			security.NewDefaultSecurityIntegrator(), // 生产级：安全集成器（包含基础安全检查）
			security.NewDefaultQuotaManager(),       // 生产级：配额管理器（包含资源限制）
			envAdvisor,                              // 智能决策：环境顾问（ML优化建议）
			logger,                                  // 日志记录器
			DefaultCoordinatorConfig(),              // 生产级：默认配置（合理的超时和资源限制）
		)
	} else {
		// 策略2：使用默认配置创建完整的执行协调器（自动配置模式）
		// 这种模式会自动创建包含熔断、限流功能的dispatcher
		return NewDefaultCoordinatorWithDefaults(
			engineManager,
			hostRegistry,
			envAdvisor,
			logger,
		)
	}
}

// ==================== 模块设计说明 ====================
//
// 本coordinator模块的设计遵循以下原则：
//
// 1. **MVP优先**：
//    - 默认使用NoOp实现，避免不必要的复杂性
//    - 核心安全和资源管理功能保持生产级
//    - 通过接口隔离，支持后续功能扩展
//
// 2. **依赖注入友好**：
//    - 所有工厂函数支持fx框架的依赖注入
//    - 通过接口参数降低组件间的直接依赖
//    - 支持测试时的Mock替换
//
// 3. **配置简化**：
//    - 提供合理的默认配置，支持零配置启动
//    - 仅保留影响性能和安全的核心配置项
//    - 避免过多的可配置选项增加复杂性
//
// 4. **扩展友好**：
//    - 通过接口设计支持组件的后续扩展
//    - NoOp实现可以根据需要逐步替换为功能完整的实现
//    - 模块间通过标准接口通信，降低耦合度
//
// 5. **智能化增强**：
//    - 集成环境顾问提供ML驱动的执行优化
//    - 支持基于历史数据的资源预测和调整
//    - 通过智能决策提升执行效率
