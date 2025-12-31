// Package execution 提供WES系统的ISPC执行模块实现
//
// 📋 **ISPC执行核心模块 (Execution Core Module)**
//
// 本包是WES区块链系统的ISPC(本征自证计算)执行实现模块，负责协调和管理所有执行相关的业务逻辑。
// 通过fx依赖注入框架，将执行协调器、交易构建器、ZK证明生成器、执行上下文管理器等组织为统一的服务层，
// 对外提供完整的执行即构建功能。
//
// 🎯 **模块职责**：
// - 实现pkg/interfaces/execution中定义的所有公共接口
// - 协调coordinator、transaction、context、zkproof等子模块
// - 管理依赖注入和服务生命周期
// - 提供统一的配置和错误处理机制
//
// 🏗️ **架构特点**：
// - fx依赖注入：使用fx框架管理组件生命周期和依赖关系
// - 模块化设计：每个子模块专注特定业务领域，低耦合高内聚
// - 接口导向：通过接口而非具体类型进行依赖，便于测试和扩展
// - 配置驱动：支持灵活的配置管理和环境适配
//
// 📦 **子模块组织**：
// - coordinator/ - ISPC执行协调器，统筹整个执行即构建流程
// - transaction/ - 动态交易构建器，专注交易的预处理构建和动态填充
// - context/     - 执行上下文管理器，管理执行环境和状态
// - zkproof/     - 零知识证明生成器，为执行结果提供可验证性
//
// 🔗 **依赖关系**：
// - 基础设施：依赖crypto、storage、log、event等基础组件
// - 区块链服务：依赖blockchain.TransactionManager等公共服务
// - 引擎层：内部创建 WASM 和 ONNX 引擎，不再依赖外部接口
// - 数据层：依赖repository和mempool提供数据访问能力
//
// 详细使用说明请参考：internal/core/execution/README.md
package execution

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/fx"

	// 公共接口

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	execution "github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/ures"

	// 管理器实现
	infraClockImpl "github.com/weisyn/v1/internal/core/infrastructure/clock"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	ctxmgr "github.com/weisyn/v1/internal/core/ispc/context"
	"github.com/weisyn/v1/internal/core/ispc/coordinator"
	ispcEngines "github.com/weisyn/v1/internal/core/ispc/engines"
	ispcEnginesONNX "github.com/weisyn/v1/internal/core/ispc/engines/onnx"
	ispcEnginesWASM "github.com/weisyn/v1/internal/core/ispc/engines/wasm"
	"github.com/weisyn/v1/internal/core/ispc/hostabi"
	"github.com/weisyn/v1/internal/core/ispc/hostabi/adapter"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	"github.com/weisyn/v1/internal/core/tx/selector"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	onnxdeps "github.com/weisyn/v1/pkg/build/deps/onnx"
)

// ==================== 模块输入依赖 ====================

// ModuleInput 定义ISPC执行模块的输入依赖
//
// 🎯 **依赖注入配置说明**：
// 本结构体定义了execution模块运行所需的所有外部依赖。
// 通过fx.In标签，fx框架会自动注入这些依赖到模块构造函数中。
//
// 📋 **依赖分类**：
// - 基础设施：Logger、EventBus、ConfigProvider等通用组件
// - 存储组件：BadgerStore、MemoryStore等持久化和缓存服务
// - 密码学组件：HashManager、SignatureManager等安全服务
// - 区块链服务：TransactionManager等公共服务
// - 引擎服务：WASMEngine、ONNXEngine等执行引擎
// - 数据层：RepositoryManager等数据访问服务
//
// ⚠️ **可选性控制**：
// - optional:"false" - 必需依赖，缺失时启动失败
// - optional:"true"  - 可选依赖，允许为nil，模块内需要nil检查
type ModuleInput struct {
	fx.In

	// 基础设施组件
	ConfigProvider config.Provider `optional:"false"`
	Logger         log.Logger      `optional:"true"`
	EventBus       event.EventBus  `optional:"true"`

	// 存储组件
	BadgerStore       storage.BadgerStore `optional:"false"`
	MemoryStore       storage.MemoryStore `optional:"true"`
	StorageProvider   storage.Provider    `optional:"false"`
	FileStoreRootPath string              `name:"file_store_root_path" optional:"false"` // 文件存储根路径（从storage模块注入）

	// 密码学组件
	HashManager      crypto.HashManager      `optional:"false"`
	SignatureManager crypto.SignatureManager `optional:"false"` // 改为必需，替代CryptoService
	KeyManager       crypto.KeyManager       `optional:"true"`
	AddressManager   crypto.AddressManager   `optional:"true"`
	// CryptoService    crypto.CryptoService    `optional:"false"` // TODO: 暂时注释，使用SignatureManager替代

	// ⚠️ 架构修正：彻底移除对TX层的依赖，打破循环依赖
	//
	// ✅ 正确的依赖方向：tx → ispc (单向)
	// ❌ 错误的依赖方向：ispc → tx (已移除)
	//
	// ISPC层专注执行，返回执行产物(ExecutionResult)
	// TX层负责交易生命周期(构建/签名/提交)
	//
	// TransactionManager 已从ISPC依赖中移除

	// M2重构：添加UnifiedTransactionFacade依赖（仅用于SDKAdapter）
	// SDKAdapter需要调用TX L3 Facade.Compose阶段创建草稿
	UnifiedTransactionFacade adapter.UnifiedTransactionFacade `optional:"true"` // 可选，仅Host模式需要

	// ⚠️ 引擎服务（执行引擎）- 已移除构造时依赖，改为运行时注入
	// 原因：避免循环依赖（ispc → engines → ispc）
	// WASMEngine 和 ONNXEngine 将通过 fx.Invoke 运行时注入到 Coordinator
	// WASMEngine engines.WASMEngine `name:"wasm_engine" optional:"false"`
	// ONNXEngine engines.ONNXEngine `name:"onnx_engine" optional:"false"`

	// ⚠️ **架构变更**：引擎已完全内部化，不再依赖外部接口
	// WASM 和 ONNX 引擎现在直接在 ISPC 内部创建和管理

	// ABI 服务（从ISPC内部engines模块获取，可选）
	// ABIService ispcInterfaces.ABIService `name:"execution_abi_service" optional:"true"` // 暂时注释，如需可启用

	// ✅ HostFunctionProvider 不再从外部注入
	// 改为 ISPC 内部创建（自给自足），见下方 module.go 中的 Provide

	// ⚠️ 不能在这里注入 ChainService/BlockService！
	// 原因：会导致循环依赖（blockchain → tx → ispc → blockchain）
	// 解决方案：通过 fx.Invoke 在所有模块初始化后注入

	// 数据层
	EUTXOQuery persistence.UTXOQuery `optional:"false" name:"utxo_query"`
	URESCAS    ures.CASStorage       `optional:"false" name:"cas_storage"`

	// TX 层服务（用于 HostABI）
	TransactionDraftService tx.TransactionDraftService `optional:"false"` // HostFunctionProvider 需要
}

// ==================== 模块输出服务 ====================

// ModuleOutput 定义ISPC执行模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体包装了模块内部创建的公共服务接口。
// 这些服务可以被其他模块通过fx依赖注入系统使用。
//
// 📋 **导出服务**：
// - ISPCCoordinator: ISPC执行协调器，提供统一的执行入口
// - HostFunctionProvider: 宿主函数提供者，供 WASM/ONNX 引擎使用
//
// 🔧 **设计原则**：
// - 只导出公共接口，不暴露内部实现细节
// - 通过fx.Out标签，让fx自动注册这些服务
// - 内部接口仅供模块内部使用，不对外暴露
//
// ✅ **自给自足**：
// - ISPC 模块内部创建 HostFunctionProvider，不依赖外部注入
// - 保证 ISPC 的完整性和独立性
type ModuleOutput struct {
	fx.Out

	// 核心执行服务
	ISPCCoordinator execution.ISPCCoordinator `name:"execution_coordinator"`

	// ⚠️ HostFunctionProvider 不通过输出聚合提供，直接在 fx.Provide 中提供
	// 原因：避免循环依赖（输出聚合 → engines → HostFunctionProvider → 输出聚合）
}

// ==================== 模块构建器 ====================

// Module 构建并返回ISPC执行模块的fx配置
//
// 🎯 **模块构建器**：
// 本函数是ISPC执行模块的主要入口点，负责构建完整的fx模块配置。
// 通过fx.Module组织所有子模块的依赖注入配置，确保服务的正确创建和生命周期管理。
//
// 🏗️ **构建流程**：
// 1. 创建各子模块管理器：coordinator、transaction、context、zkproof
// 2. 配置依赖注入：每个管理器使用fx.Annotate进行接口绑定
// 3. 聚合输出服务：将所有服务包装为ModuleOutput统一导出
// 4. 注册初始化回调：模块加载完成后的日志记录
//
// 📋 **服务创建顺序**：
// - Context: 执行上下文管理器，基础服务，优先创建
// - ZKProof: 零知识证明生成器，依赖密码学服务
// - Transaction: 交易构建器，依赖区块链公共服务
// - Coordinator: 执行协调器，依赖所有其他服务，最后创建
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    execution.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保以下依赖模块已正确加载：
// - crypto模块：提供密码学服务
// - storage模块：提供数据存储服务
// - blockchain模块：提供区块链公共服务
// - engines模块：提供WASM和ONNX执行引擎
func Module() fx.Option {
	return fx.Module("execution",
		fx.Provide(
			// 执行上下文管理器（基础服务，优先创建）
			fx.Annotate(
				func(input ModuleInput) *ctxmgr.Manager {
					// 按配置选择时钟实现
					var clockService infraClock.Clock
					switch input.ConfigProvider.GetClock().Type {
					case "ntp":
						c, err := infraClockImpl.NewNTPClock(input.ConfigProvider.GetClock().NTPServer, input.ConfigProvider.GetClock().SyncInterval)
						if err != nil {
							clockService = infraClockImpl.NewSystemClock()
						} else {
							clockService = c
						}
					case "roughtime":
						clockService = infraClockImpl.NewRoughtimeClock()
					case "deterministic":
						base := time.Unix(input.ConfigProvider.GetClock().DeterministicBaseUnix, 0)
						clockService = infraClockImpl.NewDeterministicClock(base)
					default:
						clockService = infraClockImpl.NewSystemClock()
					}

					// 注册时钟指标（仅对具有Health方法的实现）
					if ntp, ok := clockService.(*infraClockImpl.NTPClock); ok {
						_ = infraClockImpl.RegisterClockMetrics(ntp.Health)
					}
					// 🎯 为 Executor 模块添加 module 字段，日志将路由到 node-business.log
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					return ctxmgr.NewManager(
						executorLogger,
						input.ConfigProvider,
						clockService,
					)
				},
				// 暂不导出公共接口，仅供内部使用
			),

			// 零知识证明生成器
			fx.Annotate(
				func(input ModuleInput) *zkproof.Manager {
					// 🎯 为 Executor 模块添加 module 字段
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					return zkproof.NewManager(
						input.HashManager,
						input.SignatureManager,
						executorLogger,
						input.ConfigProvider,
					)
				},
				// 暂不导出公共接口，仅供内部使用
			),

		// ✅ 宿主函数提供者（ISPC 自给自足，内部创建）
		// ⚠️ 不使用 ModuleInput，避免依赖 ABIService（来自 engines 输出聚合）
		// 🔧 同时提供具体类型和接口类型
		fx.Annotate(
			func(
				logger log.Logger,
				eutxoQuery persistence.UTXOQuery,
				uresCAS ures.CASStorage,
				draftSvc tx.TransactionDraftService,
				txHashClient transaction.TransactionHashServiceClient,
				addrMgr crypto.AddressManager,
			) (*hostabi.HostFunctionProvider, ispcInterfaces.HostFunctionProvider) {
				// 🎯 为 Executor 模块添加 module 字段
				var executorLogger log.Logger
				if logger != nil {
					executorLogger = logger.With("module", "executor")
				}
				// 创建 HostFunctionProvider
				// chainQuery、txQuery、resourceQuery、txAdapter 通过 fx.Invoke 稍后注入（避免循环依赖）
				provider := hostabi.NewHostFunctionProvider(
					executorLogger,
					eutxoQuery,
					uresCAS,
					draftSvc,
					nil,          // txAdapter 将通过 fx.Invoke 注入
					txHashClient, // 交易哈希服务客户端
					addrMgr,      // addressManager 用于 Base58Check 编码
				)
				// 🔧 同时返回具体类型和接口类型，让 fx 可以注入两种类型
				return provider, provider
			},
			fx.ParamTags(
				``,                   // log.Logger
				`name:"utxo_query"`,  // persistence.UTXOQuery
				`name:"cas_storage"`, // ures.CASStorage
				``,                   // tx.TransactionDraftService
				``,                   // transaction.TransactionHashServiceClient
				``,                   // crypto.AddressManager
			),
		),

			// ✅ WASM Engine（ISPC内部引擎，直接创建）
			// 🎯 架构变更：不再从旧engines模块接收，直接在ISPC内部创建
			fx.Annotate(
				func(
					input ModuleInput,
					hostProvider ispcInterfaces.HostFunctionProvider, // 以接口形式接收 HostFunctionProvider
				) (ispcInterfaces.InternalWASMEngine, error) {
					// 🎯 为 Executor 模块添加 module 字段
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					// 直接创建内部WASM引擎
					// fileStoreRootPath 从 ModuleInput 的命名依赖注入
					// uresCAS 从 ModuleInput 的命名依赖注入
					return ispcEnginesWASM.NewEngine(
						executorLogger,
						input.URESCAS,
						input.StorageProvider,
						input.FileStoreRootPath,
						hostProvider,
					)
				},
			),

			// ✅ ONNX Engine（ISPC内部引擎，根据平台支持情况创建）
			// 🎯 平台感知：仅在支持的平台上创建 ONNX 引擎
			// 🎯 架构变更：不再从旧engines模块接收，直接在ISPC内部创建
			fx.Annotate(
				func(
					input ModuleInput,
				) (ispcInterfaces.InternalONNXEngine, error) {
					// 🎯 为 Executor 模块添加 module 字段
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					
					// 检查平台是否支持 ONNX Runtime
					if !onnxdeps.IsPlatformSupported() {
						info := onnxdeps.GetPlatformSupportInfo()
						if executorLogger != nil {
							executorLogger.Warnf("⚠️ 当前平台 (%s) 不支持 ONNX Runtime: %s", info.Platform, info.Reason)
							executorLogger.Info("ℹ️ ONNX AI 推理功能将不可用，但区块链核心功能（WASM、交易、共识等）正常工作")
						}
						// 返回 nil，表示 ONNX 引擎不可用
						// 引擎管理器会处理 nil 的情况
						return nil, nil
					}
					
					// 平台支持，创建 ONNX 引擎
					// uresCAS 从 ModuleInput 的命名依赖注入
					return ispcEnginesONNX.NewEngine(
						executorLogger,
						input.URESCAS,
					)
				},
			),

			// ✅ engines.Manager（ISPC内部引擎统一管理器）
			// 🎯 架构变更：直接使用内部引擎，不再需要适配器
			fx.Annotate(
				func(
					logger log.Logger,
					wasmEngine ispcInterfaces.InternalWASMEngine,
					onnxEngine ispcInterfaces.InternalONNXEngine,
				) (ispcInterfaces.InternalEngineManager, error) {
					// 直接使用内部引擎创建管理器
					// ⚠️ 关闭执行结果缓存，确保 BalanceOf 等只读查询实时返回
					return ispcEngines.NewManagerWithCache(logger, wasmEngine, onnxEngine, false, 0, 0)
				},
			),

			// ISPC执行协调器（核心服务，依赖所有其他服务）
			// ✅ 架构修正：通过engineManager访问引擎，符合单一入口约束
			fx.Annotate(
				func(
					input ModuleInput,
					contextMgr *ctxmgr.Manager,
					zkproofMgr *zkproof.Manager,
					hostProvider *hostabi.HostFunctionProvider, // 接收HostFunctionProvider实例
					engineManager ispcInterfaces.InternalEngineManager, // 引擎统一管理器
				) *coordinator.Manager {
					// hostProvider已经是*hostabi.HostFunctionProvider类型，直接使用
					// 🎯 为 Executor 模块添加 module 字段
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					return coordinator.NewManager(
						engineManager, // ✅ 通过engines.Manager统一访问
						contextMgr,
						zkproofMgr,
						hostProvider,
						executorLogger,
						input.ConfigProvider,
					)
				},
				fx.As(new(execution.ISPCCoordinator)), // 导出为执行协调器
			),

			// M2重构：添加SDKAdapter（Host模式适配器）
			// 📋 职责：连接合约SDK到TX Facade，仅调用Compose阶段
			// 🎯 归属：ISPC域（ispc/hostabi/adapter）
			// 🔧 依赖：UnifiedTransactionFacade（可选，仅Host模式需要）
			fx.Annotate(
				func(input ModuleInput) *adapter.SDKAdapter {
					// 🎯 为 Executor 模块添加 module 字段
					var executorLogger log.Logger
					if input.Logger != nil {
						executorLogger = input.Logger.With("module", "executor")
					}
					
					// 如果没有注入Facade（非Host模式），返回nil适配器
					if input.UnifiedTransactionFacade == nil {
						if executorLogger != nil {
							executorLogger.Info("⚠️ UnifiedTransactionFacade未注入，SDKAdapter创建为nil（非Host模式）")
						}
						return nil
					}

					return adapter.NewSDKAdapter(input.UnifiedTransactionFacade)
				},
			),

			// 模块输出聚合（只输出 ISPCCoordinator，HostFunctionProvider 已直接提供）
			func(executionCoordinator execution.ISPCCoordinator) ModuleOutput {
				return ModuleOutput{
					ISPCCoordinator: executionCoordinator,
				}
			},
		),

		// ⚠️ 运行时依赖注入：在所有模块初始化后，注入engines/blockchain/repository/tx服务
		// 🎯 **断环设计**：避免构造期循环依赖（ispc → engines → ispc, ispc → blockchain → tx → ispc）
		// 📋 **机制**：通过fx.Invoke在所有Provider完成后调用SetRuntimeDependencies
		fx.Invoke(fx.Annotate(
			func(
				executionCoordinator execution.ISPCCoordinator,
				hostProvider *hostabi.HostFunctionProvider, // 接收HostFunctionProvider实例
				queryService persistence.QueryService, // 统一查询服务（包含ChainQuery、TxQuery、ResourceQuery）
				eutxoQuery persistence.UTXOQuery,
				uresCAS ures.CASStorage,
				draftService tx.TransactionDraftService,
				txVerifier tx.TxVerifier,       // TX验证器（用于创建txAdapter）
				selectorService *selector.Service, // UTXO选择器（用于创建txAdapter）
				hashManager crypto.HashManager, // 哈希管理器（用于计算区块哈希）
				logger log.Logger,
			) error {
				logger.Info("🔧 开始注入ISPC运行时依赖...")

				// 1. 注入 engines/blockchain/repository/tx 服务到 Coordinator
				mgr, ok := executionCoordinator.(*coordinator.Manager)
				if !ok {
					err := fmt.Errorf("ISPCCoordinator 不是 *coordinator.Manager 的实现，无法注入运行时依赖")
					logger.Errorf("%v", err)
					return err
				}

				// ✅ 架构变更：不再需要SetEngines
				// engineManager已在构造时注入coordinator
				// 这里只需要确保运行时依赖已注入到hostProvider即可

				// 注册 ISPC Coordinator 到内存监控系统
				if reporter, ok := executionCoordinator.(metricsiface.MemoryReporter); ok {
					metricsutil.RegisterMemoryReporter(reporter)
					if logger != nil {
						executorLogger := logger.With("module", "executor")
						executorLogger.Info("✅ ISPC Coordinator 已注册到内存监控系统")
					}
				}

				// 注入其他运行时依赖（修复：传递 queryService 而不是 eutxoQuery）
				if err := mgr.SetRuntimeDependencies(queryService, uresCAS, draftService, hashManager); err != nil {
					logger.Errorf("注入ISPC Coordinator运行时依赖失败: %v", err)
					return fmt.Errorf("failed to inject runtime dependencies: %w", err)
				}
				logger.Debug("✅ Coordinator.SetRuntimeDependencies 完成")

				// 2. 注入查询服务和创建txAdapter到HostFunctionProvider
				// ✅ 架构说明：HostFunctionProvider使用适配器模式（adapter.WASMAdapter）
				// 适配器负责构建宿主函数映射，provider只负责协调和依赖管理
				// hostProvider已经是*hostabi.HostFunctionProvider类型，直接使用

				// 注入查询服务（QueryService包含所有查询接口）
				// 这些依赖将在GetWASMHostFunctions时传递给WASMAdapter
				hostProvider.SetChainQuery(queryService)
				hostProvider.SetBlockQuery(queryService) // QueryService实现了BlockQuery接口
				hostProvider.SetTxQuery(queryService)
				hostProvider.SetResourceQuery(queryService)
				logger.Debug("✅ HostFunctionProvider查询服务注入完成")

				// 注入HashManager（用于计算区块哈希，WASM宿主函数get_block_hash需要）
				hostProvider.SetHashManager(hashManager)
				logger.Debug("✅ HostFunctionProvider.HashManager注入完成")

				// 创建并注入txAdapter（用于WASM宿主函数host_build_transaction）
				// txAdapter将通过适配函数传递给WASMAdapter.buildTxFromDraft
				txAdapter := hostabi.NewTxAdapter(draftService, txVerifier, selectorService)
				hostProvider.SetTxAdapter(txAdapter)
				logger.Debug("✅ HostFunctionProvider.txAdapter注入完成")

				logger.Info("✅ ISPC执行模块已加载完成，运行时依赖注入成功")
				return nil
			},
				fx.ParamTags(
				``,                     // execution.ISPCCoordinator
				``,                     // *hostabi.HostFunctionProvider
				`name:"query_service"`, // persistence.QueryService
				`name:"utxo_query"`,    // persistence.UTXOQuery
				`name:"cas_storage"`,   // ures.CASStorage
				``,                     // tx.TransactionDraftService
				``,                     // tx.TxVerifier
				``,                     // *selector.Service
				``,                     // crypto.HashManager
				``,                     // log.Logger
			),
		)),

		// P0: 异步功能初始化 - 根据配置启用异步ZK证明和异步轨迹记录
		// 🎯 **配置驱动集成**：根据配置文件启用异步优化功能
		fx.Invoke(fx.Annotate(
			func(
				executionCoordinator execution.ISPCCoordinator,
				contextMgr *ctxmgr.Manager,
				configProvider config.Provider,
				logger log.Logger,
			) error {
				// 获取ISPC配置
				blockchainConfig := configProvider.GetBlockchain()
				if blockchainConfig == nil || blockchainConfig.Execution.ISPC == nil {
					logger.Debug("ISPC配置不存在，使用默认配置（异步功能禁用）")
					return nil
				}

				ispcConfig := blockchainConfig.Execution.ISPC
				coordinatorMgr, ok := executionCoordinator.(*coordinator.Manager)
				if !ok {
					logger.Warn("ISPCCoordinator不是*coordinator.Manager类型，无法启用异步功能")
					return nil
				}

				// 启用异步ZK证明生成（如果配置启用）
				if ispcConfig.AsyncZKProof != nil && ispcConfig.AsyncZKProof.Enabled {
					workers := ispcConfig.AsyncZKProof.Workers
					if workers <= 0 {
						workers = 2 // 默认值
					}
					minWorkers := ispcConfig.AsyncZKProof.MinWorkers
					if minWorkers <= 0 {
						minWorkers = 1 // 默认值
					}
					maxWorkers := ispcConfig.AsyncZKProof.MaxWorkers
					if maxWorkers <= 0 {
						maxWorkers = 10 // 默认值
					}

					if err := coordinatorMgr.EnableAsyncZKProofGeneration(workers, minWorkers, maxWorkers); err != nil {
						logger.Errorf("❌ 启用异步ZK证明生成失败: %v", err)
						// 不返回错误，继续初始化其他功能
					} else {
						logger.Infof("✅ 异步ZK证明生成已启用: workers=%d, minWorkers=%d, maxWorkers=%d", workers, minWorkers, maxWorkers)
					}
				} else {
					logger.Debug("异步ZK证明生成未启用（配置禁用或未配置）")
				}

				// 启用异步轨迹记录（如果配置启用）
				if ispcConfig.AsyncTrace != nil && ispcConfig.AsyncTrace.Enabled {
					workers := ispcConfig.AsyncTrace.Workers
					if workers <= 0 {
						workers = 2 // 默认值
					}
					batchSize := ispcConfig.AsyncTrace.BatchSize
					if batchSize <= 0 {
						batchSize = 100 // 默认值
					}
					batchTimeout := ispcConfig.AsyncTrace.BatchTimeout
					if batchTimeout <= 0 {
						batchTimeout = 100 * time.Millisecond // 默认值
					}
					maxRetries := ispcConfig.AsyncTrace.MaxRetries
					if maxRetries <= 0 {
						maxRetries = 3 // 默认值
					}
					retryDelay := ispcConfig.AsyncTrace.RetryDelay
					if retryDelay <= 0 {
						retryDelay = 10 * time.Millisecond // 默认值
					}

					if err := contextMgr.EnableAsyncTraceRecording(workers, batchSize, batchTimeout, maxRetries, retryDelay); err != nil {
						logger.Errorf("❌ 启用异步轨迹记录失败: %v", err)
						// 不返回错误，继续初始化其他功能
					} else {
						logger.Infof("✅ 异步轨迹记录已启用: workers=%d, batchSize=%d, batchTimeout=%v, maxRetries=%d, retryDelay=%v",
							workers, batchSize, batchTimeout, maxRetries, retryDelay)
					}
				} else {
					logger.Debug("异步轨迹记录未启用（配置禁用或未配置）")
				}

				return nil
			},
		)),

		// P0: 引擎生命周期管理 - 优雅关闭
		// 🎯 **优雅关闭**：在应用停止时关闭引擎管理器，释放所有资源
		// ⚠️ **重要说明**：引擎关闭只在应用级别的 `OnStop` 时发生，不会在运行时关闭
		fx.Invoke(fx.Annotate(
			func(
				lc fx.Lifecycle,
				engineManager ispcInterfaces.InternalEngineManager,
				executionCoordinator execution.ISPCCoordinator,
				contextMgr *ctxmgr.Manager,
				logger log.Logger,
			) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						logger.Info("✅ ISPC引擎管理器已启动")
						return nil
					},
					OnStop: func(ctx context.Context) error {
						logger.Info("🔄 开始关闭ISPC引擎管理器...")

						// 禁用异步功能（优雅关闭）
						coordinatorMgr, ok := executionCoordinator.(*coordinator.Manager)
						if ok {
							if coordinatorMgr.IsAsyncZKProofGenerationEnabled() {
								if err := coordinatorMgr.DisableAsyncZKProofGeneration(); err != nil {
									logger.Warnf("禁用异步ZK证明生成失败: %v", err)
								} else {
									logger.Info("✅ 异步ZK证明生成已禁用")
								}
							}
						}

						if contextMgr.IsAsyncTraceRecordingEnabled() {
							if err := contextMgr.DisableAsyncTraceRecording(); err != nil {
								logger.Warnf("禁用异步轨迹记录失败: %v", err)
							} else {
								logger.Info("✅ 异步轨迹记录已禁用")
							}
						}

						// 设置关闭超时（30秒）
						shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
						defer cancel()

						// 关闭引擎管理器
						if err := engineManager.Shutdown(shutdownCtx); err != nil {
							logger.Errorf("❌ 关闭ISPC引擎管理器失败: %v", err)
							// 不返回错误，继续关闭其他服务
							return nil
						}

						logger.Info("✅ ISPC引擎管理器已成功关闭")
						return nil
					},
				})
			},
		)),
	)
}
