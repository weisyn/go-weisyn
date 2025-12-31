package coordinator

import (
	"fmt"
	"sync"

	// 公共接口依赖
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/ures"
	"github.com/weisyn/v1/pkg/interfaces/tx"

	// 内部模块依赖
	ctxmgr "github.com/weisyn/v1/internal/core/ispc/context"
	"github.com/weisyn/v1/internal/core/ispc/billing"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	"github.com/weisyn/v1/internal/core/ispc/hostabi"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
)

// Manager ISPC（Intrinsic Self-Proving Computing）执行协调器管理器
//
// 🎯 **设计理念**：实现本征自证计算范式
//
// 本管理器实现 pkg/interfaces/ispc.ISPCCoordinator 公共接口，
// 通过依赖注入框架组织所有必要的公共服务和内部组件，
// 为 ISPC 本征自证执行功能提供统一的协调入口。
//
// 🏗️ **架构特点**：
// - 实现 ISPC 规范：遵循 _docs/specs/ispc/ 中定义的本征自证计算范式
// - 依赖公共接口：复用成熟的区块链公共服务
// - 协调内部组件：统筹 context、zkproof 等子模块
// - 执行即证明：WASM/ONNX 执行与 ZK 证明一体化
type Manager struct {
	// ==================== 执行引擎服务 ====================
	// ✅ 通过engines.Manager统一访问，符合架构约束：单一入口、引擎内部化
	engineManager ispcInterfaces.InternalEngineManager // 引擎统一管理器

	// ==================== 内部子模块 ====================
	contextManager *ctxmgr.Manager      // 执行上下文管理器
	zkproofManager *zkproof.Manager     // 零知识证明管理器
	hostProvider   ispcInterfaces.HostFunctionProvider // 宿主函数提供者（通过内部接口暴露）
	computeMeter   ComputeMeter         // 算力计量器（Phase 1 新增）
	billingOrchestrator billing.BillingOrchestrator // 计费编排器（Phase 3 新增）

	// ==================== 基础设施服务 ====================
	logger         log.Logger      // 日志服务
	configProvider config.Provider // 配置提供者
	hashManager    crypto.HashManager // 哈希管理器（P1: 用于确定性哈希计算）

	// ==================== 运行时依赖（断环设计）====================
	// 🎯 **设计说明**：避免构造期循环依赖，通过运行时注入实现
	// 这些依赖不在构造函数中接收，而是在app层启动后通过SetRuntimeDependencies注入
	eutxoQuery   persistence.QueryService       // 查询服务（运行时注入，实现了UTXOQuery/TxQuery/ResourceQuery/ChainQuery）
	uresCAS      ures.CASStorage                // URES存储（运行时注入）
	draftService tx.TransactionDraftService     // 交易草稿服务（运行时注入）
	runtimeMutex sync.RWMutex                   // 运行时依赖访问锁
	
	// P0: 异步ZK证明生成（异步ZK证明生成优化）
	asyncZKProofEnabled bool                    // 是否启用异步ZK证明生成（默认false，保持向后兼容）
	zkProofTaskQueue    *zkproof.ZKProofTaskQueue // ZK证明任务队列
	zkProofWorkerPool   *zkproof.ZKProofWorkerPool // ZK证明工作线程池
	zkProofTaskStore    map[string]*zkproof.ZKProofTask // 任务存储（taskID -> task）
	zkProofTaskMutex    sync.RWMutex            // 任务存储访问锁
}

// NewManager 创建 ISPC（Intrinsic Self-Proving Computing）执行协调器
//
// 🎯 **依赖注入构造器**：
// 本构造器实现 ISPC 本征自证计算范式，接收所有必要的执行引擎和证明组件。
//
// 📋 **参数说明**：
//   - engineManager: 引擎统一管理器（协调WASM/ONNX引擎）
//   - contextManager: ISPC 执行上下文管理器（确定性时钟、上下文隔离）
//   - zkproofManager: 零知识证明管理器（本征自证的核心）
//   - hostProvider: 宿主函数提供者（WASM/ONNX 与区块链交互桥梁）
//   - logger: 日志服务
//   - configProvider: 配置提供者
//
// 🔧 **返回值**：
//   - *Manager: 完整初始化的 ISPC 协调器实例
//
// 📚 **相关规范**：
//   - _docs/specs/ispc/INTRINSIC_SELF_PROVING_COMPUTING_SPECIFICATION.md
//   - docs/system/standards/principles/code-organization.md
func NewManager(
	engineManager ispcInterfaces.InternalEngineManager,
	contextManager *ctxmgr.Manager,
	zkproofManager *zkproof.Manager,
	hostProvider ispcInterfaces.HostFunctionProvider,
	logger log.Logger,
	configProvider config.Provider,
) *Manager {
	return &Manager{
		engineManager:  engineManager,
		contextManager: contextManager,
		zkproofManager: zkproofManager,
		hostProvider:   hostProvider,
		logger:         logger,
		configProvider: configProvider,
		// Phase 1: 算力计量器（默认实现）
		computeMeter: NewDefaultComputeMeter(logger),
		// P0: 异步ZK证明生成（默认禁用，保持向后兼容）
		asyncZKProofEnabled: false,
		zkProofTaskQueue:    nil,
		zkProofWorkerPool:   nil,
		zkProofTaskStore:    make(map[string]*zkproof.ZKProofTask),
	}
}

// ==================== 接口实现说明 ====================
//
// Manager 实现了两套接口:
//
// 1. 旧接口 (ispc.ISPCCoordinator):
//    - CallFunctionPre / CallFunctionPost / GetCurrentTime
//    - 实现在 legacy_pre_post.go (已标记为deprecated)
//    - 为了向后兼容保留,但不推荐使用
//
// 2. 新接口 (interfaces.ISPCCoordinator):
//    - ExecuteContract / ExecuteAIModel
//    - 实现在 execute_contract.go (推荐使用)
//    - 返回 ExecutionResult,不依赖TX层
//
// 🎯 架构原则: tx → ispc (单向依赖)

// ==================== 运行时依赖注入（断环关键）====================

// SetRuntimeDependencies 运行时注入persistence/tx依赖
//
// 🎯 **设计说明**：
// - 避免构造期循环依赖（ispc → persistence → tx → ispc）
// - 在app层启动完成后，通过此方法注入运行时依赖
// - 这些依赖仅在执行期使用，不进入Provider依赖图
//
// 📋 **参数**：
//   - queryService: 查询服务（实现了UTXOQuery、TxQuery、ResourceQuery、ChainQuery）
//   - uresCAS: URES存储服务（用于合约资源访问）
//   - draftSvc: 交易草稿服务（用于合约构建交易）
//
// 🔧 **返回值**：
//   - error: 注入失败时的错误信息
//
// 🔒 **并发安全**：使用写锁保护
// ⚠️ **调用时机**：必须在ExecuteWASMContract之前调用

func (m *Manager) SetRuntimeDependencies(
	queryService persistence.QueryService, // 修复：应该传入 QueryService 而不是 UTXOQuery
	uresCAS ures.CASStorage,
	draftSvc tx.TransactionDraftService,
	hashMgr crypto.HashManager, // P1: 哈希管理器（用于确定性哈希计算）
) error {
	m.runtimeMutex.Lock()
	defer m.runtimeMutex.Unlock()

	if queryService == nil {
		return fmt.Errorf("queryService cannot be nil")
	}
	if uresCAS == nil {
		return fmt.Errorf("uresCAS cannot be nil")
	}
	if draftSvc == nil {
		return fmt.Errorf("draftService cannot be nil")
	}
	if m.hostProvider == nil {
		return fmt.Errorf("hostProvider cannot be nil")
	}
	if hashMgr == nil {
		return fmt.Errorf("hashManager cannot be nil")
	}

	m.eutxoQuery = queryService // QueryService 实现了 UTXOQuery 接口
	m.uresCAS = uresCAS
	m.draftService = draftSvc
	m.hashManager = hashMgr // P1: 注入哈希管理器

	// ✅ 将运行时依赖注入到hostProvider，使其能够创建HostABI
	// 符合架构约束：hostabi统一提供宿主函数，coordinator只注入依赖
	// 注意：eutxoQuery、uresCAS、draftService已在NewHostFunctionProvider时注入
	// 这里只需注入其他查询服务（chainQuery、blockQuery、txQuery、resourceQuery）
	if hp, ok := m.hostProvider.(*hostabi.HostFunctionProvider); ok && hp != nil {
		hp.SetChainQuery(queryService)
		hp.SetBlockQuery(queryService)
		hp.SetTxQuery(queryService)
		hp.SetResourceQuery(queryService)
	} else if m.logger != nil {
		m.logger.Warn("HostFunctionProvider 实例不是 *hostabi.HostFunctionProvider，跳过运行时查询服务注入")
	}

	// Phase 3: 初始化计费编排器（需要 PricingQuery）
	m.billingOrchestrator = billing.NewDefaultBillingOrchestrator(queryService)

	m.logger.Info("✅ ISPC Coordinator运行时依赖注入完成（包括hostProvider、hashManager和billingOrchestrator）")
	return nil
}

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (m *Manager) ModuleName() string {
	return "ispc"
}

// CollectMemoryStats 收集 ISPC 模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 当前活跃执行上下文 / session 数
// - ApproxBytes: 执行上下文 / 证明缓存估算大小
// - CacheItems: 模型/合约代码缓存条数（已加载的 WASM/ONNX）
// - QueueLength: 待执行任务队列长度（ZK 证明任务队列）
func (m *Manager) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 统计活跃执行上下文数量
	contextCount := int64(0)
	if m.contextManager != nil {
		// 使用 ContextManager 提供的实时统计，而不是拍脑袋估算
		contextCount = m.contextManager.ActiveContextCount()
	}

	// 统计 ZK 证明任务数量
	zkTaskCount := int64(0)
	m.zkProofTaskMutex.RLock()
	if m.zkProofTaskStore != nil {
		zkTaskCount = int64(len(m.zkProofTaskStore))
	}
	m.zkProofTaskMutex.RUnlock()

	objects := contextCount + zkTaskCount

	// 📌 暂不对 ISPC 执行上下文 / 证明任务做 bytes 级别估算，以避免拍脑袋的固定常数。
	// 实际内存占用请结合：
	// - runtime.MemStats
	// - objects（活跃上下文 + ZK 任务数量）
	approxBytes := int64(0)

	// 缓存条目：模型/合约代码缓存（估算，实际应该从 engineManager 获取）
	cacheItems := int64(0) // 简化估算

	// 队列长度：ZK 证明任务队列长度
	queueLength := zkTaskCount
	if m.zkProofTaskQueue != nil {
		// 如果任务队列有 Size() 方法，应该使用实际值
		// 这里使用 zkTaskCount 作为估算
	}

	return metricsiface.ModuleMemoryStats{
		Module:      "ispc",
		Layer:       "L4-CoreBusiness",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: queueLength,
	}
}

// ShrinkCache 供 MemoryDoctor 调用，用于在高压场景下收缩 ISPC 相关缓存。
//
// 当前 Coordinator 自身不直接维护大规模缓存，主要缓存存在于 engines.Manager 的 executionCache。
// 这里暂时只记录日志；实际缓存收缩由 engines.Manager 的 ClearCache 负责。
func (m *Manager) ShrinkCache(targetSize int) {
	if m.logger != nil {
		m.logger.Warnf("MemoryDoctor 请求收缩 ISPC Coordinator 缓存，但当前组件未维护本地大缓存，targetSize=%d",
			targetSize)
	}
}
