// Package consensus 提供WES系统的共识模块实现
//
// 📋 **共识核心模块 (Consensus Core Module)**
//
// 本包是WES区块链系统的共识实现模块，负责协调和管理所有共识相关的业务逻辑。
// 通过fx依赖注入框架，将矿工和聚合节点服务组织为统一的服务层，对外提供完整的共识功能。
//
// 🎯 **模块职责**：
// - 实现pkg/interfaces/consensus中定义的所有公共接口
// - 协调miner、aggregator等子模块
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
// - miner/      - 矿工管理和挖矿服务
// - aggregator/ - 聚合节点管理和区块聚合服务
//
// 🔗 **依赖关系**：
// - 基础设施：依赖crypto、storage、log、event等基础组件
// - 数据层：依赖repository和blockchain提供数据访问能力
// - 服务层：各子模块通过内部接口协调，对外统一暴露公共接口
// Package consensus provides consensus mechanism functionality for blockchain operations.
package consensus

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	// 配置
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"

	// 内部接口
	blockInternalIf "github.com/weisyn/v1/internal/core/block/interfaces"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/ures"

	// protobuf
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 管理器实现
	"github.com/weisyn/v1/internal/core/consensus/aggregator"
	aggregatorValidator "github.com/weisyn/v1/internal/core/consensus/aggregator/validator"
	"github.com/weisyn/v1/internal/core/consensus/miner"
	"github.com/weisyn/v1/internal/core/consensus/miner/incentive"
	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"

	// 内部接口
	"github.com/weisyn/v1/internal/core/consensus/interfaces"

	// integration集成组件
	eventIntegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	networkIntegration "github.com/weisyn/v1/internal/core/consensus/integration/network"

	// tx层依赖（通过公共接口）
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== 模块输入依赖 ====================

// ModuleInput 定义模块的输入依赖
//
// 🎯 **依赖注入配置说明**：
// 本结构体定义了consensus模块运行所需的所有外部依赖。
// 通过fx.In标签，fx框架会自动注入这些依赖到模块构造函数中。
//
// 🔧 **依赖等级说明**：
// - optional:"false" - 必需依赖，模块无法在缺失时启动，fx会报错
// - optional:"true"  - 可选依赖，允许为nil，模块内需要nil检查
type ModuleInput struct {
	fx.In

	// 基础设施组件
	ConfigProvider config.Provider `optional:"false"`
	Logger         log.Logger      `optional:"true"`
	EventBus       event.EventBus  `optional:"true"`

	// 存储组件
	BadgerStore     storage.BadgerStore `optional:"false"`
	MemoryStore     storage.MemoryStore `optional:"true"`
	StorageProvider storage.Provider    `optional:"false"`
	TempStore       storage.TempStore   `optional:"true"` // ✅ P1修复：临时存储服务（通过 storage 模块提供）

	// 密码学组件
	HashManager       crypto.HashManager       `optional:"false"`
	SignatureManager  crypto.SignatureManager  `optional:"true"`
	KeyManager        crypto.KeyManager        `optional:"true"`
	AddressManager    crypto.AddressManager    `optional:"true"`
	MerkleTreeManager crypto.MerkleTreeManager `optional:"false"`
	POWEngine         crypto.POWEngine         `optional:"false"`

	// 数据层（已迁移到新接口）
	EUTXOQuery persistence.UTXOQuery `optional:"false" name:"utxo_query"`
	URESCAS    ures.CASStorage       `optional:"false" name:"cas_storage"`

	// 区块链层（已迁移到新接口）
	BlockBuilder      block.BlockBuilder       `optional:"true"`
	BlockProcessor    block.BlockProcessor     `optional:"true" name:"block_processor"`
	BlockValidator    block.BlockValidator     `optional:"true" name:"block_validator"`
	ChainQuery        persistence.QueryService `optional:"true" name:"query_service"`
	ForkHandler       chain.ForkHandler        `optional:"true"`
	SystemSyncService chain.SystemSyncService  `optional:"false" name:"sync_service"` // ✅ P1修复：同步服务（通过 chain 模块提供，必需）

	// 网络组件
	P2PService     p2pi.Service     `name:"p2p_service" optional:"true"` // P2P 服务（用于获取本地节点 ID）
	NetworkService netiface.Network `name:"network_service" optional:"true"`

	// 配置相关（可选扩展配置）

	// 哈希相关服务
	TxHashClient    transaction_pb.TransactionHashServiceClient `optional:"true"`
	BlockHashClient core.BlockHashServiceClient                 `optional:"true"`

	// 内存池服务
	CandidatePool mempool.CandidatePool `optional:"true" name:"candidate_pool"` // 候选区块池（可选依赖）

	// Kademlia网络组件
	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager" optional:"true"`
	DistanceCalculator  kademlia.DistanceCalculator  `name:"distance_calculator" optional:"true"`

	// 缓存存储
	CacheStore storage.MemoryStore `optional:"true"`

	// 合规服务（可选）
	CompliancePolicy complianceIfaces.Policy `optional:"true"`

	// tx层服务（通过公共接口）
	FeeManager         txiface.FeeManager         `optional:"false"`
	IncentiveTxBuilder txiface.IncentiveTxBuilder `optional:"false"`
	Signer             txiface.Signer             `optional:"true"`

	// 节点运行时状态（通过 P2P 模块提供）
	NodeRuntimeState p2pi.RuntimeState `optional:"false" name:"node_runtime_state"`
}

// ==================== 模块输出服务 ====================

// ModuleOutput 定义模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体包装了模块内部创建的公共服务接口。
// 这些服务可以被其他模块通过fx依赖注入系统使用。
//
// 🔧 **设计原则**：
// - 只导出公共接口，不暴露内部实现细节
// - 通过fx.Out标签，让fx自动注册这些服务
// - 内部接口仅供模块内部使用，不对外暴露
type ModuleOutput struct {
	fx.Out

	// 注意：EventPublisher 现在由 eventIntegration.Module() 直接提供
	// 事件订阅功能直接使用标准的 event.EventBus 接口，不需要自定义协调器
}

// ==================== 辅助函数 ====================

// normalizeNetworkType 标准化网络类型字符串（用于配置验证）
func normalizeNetworkType(networkType string) string {
	switch networkType {
	case "mainnet", "production", "prod":
		return "production"
	case "testnet", "testing", "test":
		return "testnet"
	case "devnet", "development", "dev":
		return "development"
	default:
		// 默认视为生产环境（安全优先）
		return "production"
	}
}

// ==================== 模块构建函数 ====================

// Module 创建并配置共识核心模块
//
// 🎯 **模块构建器**：
// 本函数是共识核心模块的主要入口点，负责构建完整的fx模块配置。
// 通过fx.Module组织所有子模块的依赖注入配置，确保服务的正确创建和生命周期管理。
//
// 🏗️ **构建流程**：
// 1. 创建聚合器服务：优先创建，提供AggregatorController接口
// 2. 创建矿工服务：依赖聚合器控制器，用于区块提交
// 3. 配置依赖注入：每个管理器使用fx.Annotate进行接口绑定
// 4. 聚合输出服务：将所有服务包装为ModuleOutput统一导出
// 5. 注册网络协议和事件订阅
// 6. 注册初始化回调：模块加载完成后的日志记录
//
// 📋 **服务创建顺序**：
// - AggregatorService: 聚合节点管理器，处理区块聚合（优先创建，供矿工依赖）
// - MinerService: 矿工管理器，处理挖矿业务（依赖聚合器控制器接口）
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    consensus.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保以下依赖模块已正确加载：
// - crypto模块：提供哈希和签名服务
// - storage模块：提供数据存储服务
// - repository模块：提供数据访问接口
// - network模块：提供网络通信能力
// - mempool模块：提供候选区块池服务
//
// 🔗 **内部依赖关系**：
// - Miner依赖AggregatorController：矿工通过此接口提交挖出的区块
// - 聚合器优先创建：确保矿工创建时可以注入聚合器依赖
func Module() fx.Option {
	return fx.Module("consensus",
		// ⚠️ **重要语法说明**：
		// 由于ModuleInput包含fx.In标签，不能与fx.Annotate一起使用。
		// 在Go 1.19+和fx v1.20+中，fx.In结构体与fx.ParamTags存在冲突。
		// 解决方案：移除fx.Annotate包装，直接使用函数定义。

		fx.Provide(
			// ========== 激励组件（共识层激励机制） ==========
			// 注意：FeeManager 和 IncentiveTxBuilder 由 tx 模块提供，通过 ModuleInput 注入

			// IncentiveCollector - 矿工侧激励收集器
			fx.Annotate(
				func(
					input ModuleInput,
				) (interfaces.IncentiveCollector, error) {
					// 正确的设计：业务参数（minerAddr）不在系统启动时注入
					// minerAddr 在挖矿启动时通过 StartMining -> SetMinerAddress 提供
					return incentive.NewCollector(
						input.IncentiveTxBuilder,
						input.ConfigProvider,
					)
				},
				fx.ResultTags(`name:"consensus_incentive_collector"`),
			),

			// IncentiveValidator - 聚合器侧激励验证器
			fx.Annotate(
				func(
					input ModuleInput,
				) interfaces.IncentiveValidator {
					return aggregatorValidator.NewIncentiveValidator(
						input.FeeManager,
						input.ConfigProvider,
						input.EUTXOQuery,
					)
				},
				fx.ResultTags(`name:"consensus_incentive_validator"`),
			),

			// ========== V2：挖矿门闸检查器（供 API/运维查询） ==========
			//
			// 说明：
			// - 门闸实现仍属于 miner 子组件（internal/core/consensus/miner/quorum）；
			// - 这里仅将 Checker 以命名对象形式导出，方便 JSON-RPC 查询当前门闸状态；
			// - 不引入新的一级组件。
			fx.Annotate(
				func(input ModuleInput) quorum.Checker {
					// 从配置提供者获取共识配置
					var consensusOptions *consensusconfig.ConsensusOptions
					if input.ConfigProvider != nil {
						consensusOptions = input.ConfigProvider.GetConsensus()
					}
					if consensusOptions == nil {
						consensusOptions = consensusconfig.New(nil).GetOptions()
					}

					var consensusLogger log.Logger
					if input.Logger != nil {
						consensusLogger = input.Logger.With("module", "consensus")
					}

					return quorum.NewChecker(
						input.ConfigProvider,
						&consensusOptions.Miner,
						input.ChainQuery,
						input.ChainQuery, // QueryService（ModuleInput.ChainQuery 实际是 QueryService）
						input.RoutingTableManager,
						input.P2PService,
						input.NetworkService,
						consensusLogger,
					)
				},
				fx.ResultTags(`name:"mining_quorum_checker"`),
			),

			// ========== 共识服务 ==========

			// 聚合节点服务管理器（先创建，供矿工依赖）
			fx.Annotate(
				func(input ModuleInput) interfaces.InternalAggregatorService {
					// 从配置提供者获取共识配置
					var consensusOptions *consensusconfig.ConsensusOptions
					if input.ConfigProvider != nil {
						consensusOptions = input.ConfigProvider.GetConsensus()
					}

					// 如果没有配置，使用默认配置
					if consensusOptions == nil {
						consensusOptions = consensusconfig.New(nil).GetOptions()
					}

					// 直接返回接口类型，NewManager 已经返回 interfaces.InternalAggregatorService
					// ✅ P1修复：添加可选参数 SystemSyncService, TempStore, BlockHashClient
					// 🎯 为共识模块添加 module 字段，日志将路由到 node-system.log
					var consensusLogger log.Logger
					if input.Logger != nil {
						consensusLogger = input.Logger.With("module", "consensus")
					}
					return aggregator.NewManager(
						consensusLogger,
						input.EventBus,
						input.CandidatePool,
						input.HashManager,
						input.SignatureManager,
						input.KeyManager,
						input.POWEngine,
						input.P2PService,
						input.NetworkService,
						input.ChainQuery,
						input.DistanceCalculator,
						consensusOptions,
						input.ForkHandler,
						input.RoutingTableManager,
						input.BlockValidator,
						input.BlockProcessor,
						input.SystemSyncService, // ✅ P1修复：SystemSyncService（通过依赖注入获取）
						input.TempStore,         // ✅ P1修复：TempStore（通过依赖注入获取）
						input.BlockHashClient,   // ✅ P1修复：BlockHashServiceClient（从 ModuleInput 获取）
						input.ConfigProvider,    // 配置提供者
						input.NodeRuntimeState,  // ✅ 新增：节点运行时状态（状态机模型）
					)
				},
				fx.ResultTags(`name:"internal_aggregator_service"`),
			),

			// 矿工服务管理器（依赖聚合器控制器）
			// 使用分解的参数避免fx.In与fx.ParamTags冲突
			fx.Annotate(
				func(
					configProvider config.Provider,
					logger log.Logger,
					eventBus event.EventBus,
					blockBuilder blockInternalIf.InternalBlockBuilder, // 🔧 使用内部接口以访问缓存方法
					blockProcessor block.BlockProcessor,
					chainQuery persistence.ChainQuery,
					queryService persistence.QueryService,
					systemSyncService chain.SystemSyncService,
					memoryStore storage.MemoryStore,
					networkService netiface.Network,
					p2pService p2pi.Service,
					routingManager kademlia.RoutingTableManager,
					powEngine crypto.POWEngine,
					hashManager crypto.HashManager,
					merkleManager crypto.MerkleTreeManager,
					txHashClient transaction_pb.TransactionHashServiceClient,
					aggregatorService interfaces.InternalAggregatorService,
					incentiveCollector interfaces.IncentiveCollector, // 🔥 激励收集器
					compliancePolicy complianceIfaces.Policy,
				) consensus.MinerService {
					// 从配置提供者获取共识配置
					var consensusOptions *consensusconfig.ConsensusOptions
					if configProvider != nil {
						// 从配置提供者获取共识配置
						consensusOptions = configProvider.GetConsensus()
					}

					// 如果没有配置，使用默认配置
					if consensusOptions == nil {
						consensusOptions = consensusconfig.New(nil).GetOptions()
					}

					// 薄管理器模式：只传递必要依赖
					// 🎯 为共识模块添加 module 字段，日志将路由到 node-system.log
					var consensusLogger log.Logger
					if logger != nil {
						consensusLogger = logger.With("module", "consensus")
					}
					return miner.NewManager(
						// ========== 基础依赖 ==========
						consensusLogger,  // 日志记录器（带 module 字段）
						eventBus,         // 事件总线
						consensusOptions, // 共识配置

						// ========== 业务服务依赖（传递给子模块） ==========
						blockBuilder,      // 区块构建服务
						blockProcessor,    // 区块处理服务
						chainQuery,        // 链查询服务
						queryService,      // 统一查询服务（用于 v2 时间戳/MTP 规则）
						systemSyncService, // 系统同步服务
						memoryStore,       // 内存缓存
						networkService,    // 网络服务
						p2pService,        // P2P service（用于 v2 挖矿门闸网络确认）
						routingManager,    // Routing manager（用于 v2 挖矿门闸发现口径）

						// ========== 加密服务依赖（传递给子模块） ==========
						powEngine,     // PoW引擎
						hashManager,   // 哈希管理器
						merkleManager, // 默克尔树管理器
						txHashClient,  // 交易哈希服务客户端（统一哈希计算）

						// ========== 聚合器依赖（用于区块提交） ==========
						aggregatorService, // 聚合器控制器接口

						// ========== 激励依赖（用于创建候选区块） ==========
						incentiveCollector, // 激励收集器

						// ========== 合规依赖（可选） ==========
						compliancePolicy, // 合规策略服务

						// ========== 配置提供者（v2 共识规则） ==========
						configProvider,
					)
				},
				fx.As(new(consensus.MinerService)),
				fx.ParamTags(
					``,                                     // config.Provider
					``,                                     // log.Logger
					``,                                     // event.EventBus
					`name:"block_builder"`,                 // block.BlockBuilder (从 block 模块导出)
					`name:"block_processor"`,               // block.BlockProcessor (从 block 模块导出)
					`name:"chain_query"`,                   // persistence.ChainQuery (从 persistence 模块导出)
					`name:"query_service"`,                 // persistence.QueryService（用于读取区块时间戳/MTP）
					`name:"sync_service"`,                  // chain.SystemSyncService (从 chain 模块导出)
					``,                                     // storage.MemoryStore
					`name:"network_service"`,               // network.Network
					`name:"p2p_service"`,                   // p2pi.Service
					`name:"routing_table_manager"`,         // kademlia.RoutingTableManager
					``,                                     // crypto.POWEngine
					``,                                     // crypto.HashManager
					``,                                     // crypto.MerkleTreeManager
					``,                                     // transaction.TransactionHashServiceClient
					`name:"internal_aggregator_service"`,   // interfaces.InternalAggregatorService
					`name:"consensus_incentive_collector"`, // interfaces.IncentiveCollector
					`optional:"true"`,                      // compliance.Policy（可选）
				),
				fx.ResultTags(`name:"consensus_miner_service"`),
			),

			// 事件协调器由 eventIntegration.Module() 直接提供，避免重复提供
			// 模块输出已移除，所有服务直接由fx.Provide提供
		),

		// 延迟注入聚合器服务到矿工服务中（解决循环依赖）
		fx.Invoke(
			fx.Annotate(
				func(
					minerService consensus.MinerService,
					aggregatorService interfaces.InternalAggregatorService,
					logger log.Logger,
				) {
					// 将聚合器服务注入到矿工服务中
					if minerManager, ok := minerService.(interface {
						SetAggregatorService(interfaces.InternalAggregatorService)
					}); ok {
						minerManager.SetAggregatorService(aggregatorService)
						if logger != nil {
							// 🎯 为共识模块添加 module 字段
							consensusLogger := logger.With("module", "consensus")
							consensusLogger.Info("🔗 聚合器服务已注入到矿工服务")
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ 矿工服务不支持聚合器服务注入")
						}
					}

					// 注意：内存监控注册已移除，因为接口类型无法直接注册
					// 如果需要内存监控，应该在具体实现类型上实现 MemoryReporter 接口
				},
				fx.ParamTags(`name:"consensus_miner_service"`, `name:"internal_aggregator_service"`, ``),
			),
		),

		// 模块初始化回调：添加配置验证和警告
		fx.Invoke(func(
			logger log.Logger,
			configProvider config.Provider,
		) {
			if logger != nil {
				// 🎯 为共识模块添加 module 字段，日志将路由到 node-system.log
				consensusLogger := logger.With("module", "consensus")
				consensusLogger.Info("🚀 共识核心模块初始化完成")
			}

			// 🎯 **配置验证与警告**：检查共识模式配置是否符合环境要求
			if configProvider != nil {
				consensusOpts := configProvider.GetConsensus()
				if consensusOpts != nil {
					// 从 Provider 中获取显式的 environment 和 chain_mode
					environment := configProvider.GetEnvironment()
					chainMode := configProvider.GetChainMode()

					// 使用新的环境 + 链模式感知配置验证逻辑
					// 这里仅复用校验逻辑本身，不改变已经解析好的 consensusOpts
					cfg := consensusconfig.New(nil)
					cfg.GetOptions().Aggregator = consensusOpts.Aggregator
					if err := cfg.ValidateForEnvironment(environment, chainMode); err != nil {
						if logger != nil {
							logger.Errorf("========================================")
							logger.Errorf("❌ 共识配置验证失败")
							logger.Errorf("%s", err.Error())
							logger.Errorf("========================================")
						}
						// 注意：fx.Invoke 中的 panic 会导致应用启动失败
						panic(fmt.Sprintf("共识配置验证失败: %s", err.Error()))
					}

					// 单节点模式提示（仅在未触发致命错误时）
					if !consensusOpts.Aggregator.EnableAggregator {
						if logger != nil {
							logger.Warn("========================================")
							logger.Warn("⚠️  单节点模式已启用")
							logger.Warn("⚠️  共识模式: 单节点（无分布式共识）")
							logger.Warn("⚠️  区块确认: 立即本地确认")
							logger.Warn("⚠️  安全保障: 无拜占庭容错能力")
							logger.Warn("⚠️  适用场景: 开发 / 测试 / 小规模私链")
							logger.Warn("⚠️  不建议用于: 高价值生产公链 / 联盟链")
							logger.Warn("========================================")
						}
					} else {
						if logger != nil {
							logger.Info("✅ 分布式聚合器共识模式已启用")
							logger.Infof("   最小节点阈值: %d", consensusOpts.Aggregator.MinPeerThreshold)
						}
					}
				}
			}
		}),

		// 注册网络协议（迁移自aggregator/manager.go）
		fx.Invoke(
			fx.Annotate(
				func(
					networkService netiface.Network,
					aggregatorRouter interfaces.InternalAggregatorService,
					logger log.Logger,
				) {
					if networkService != nil && aggregatorRouter != nil {
						// 使用集成层统一注册共识流式协议
						if err := networkIntegration.RegisterStreamHandlers(networkService, aggregatorRouter, logger); err != nil {
							if logger != nil {
								logger.Infof("注册共识流式协议失败: %v", err)
							}
						} else if logger != nil {
							logger.Info("✅ 共识流式协议注册成功")
						}

						// 注册共识订阅协议处理器
						if err := networkIntegration.RegisterSubscribeHandlers(networkService, aggregatorRouter, logger); err != nil {
							if logger != nil {
								logger.Infof("注册共识订阅协议失败: %v", err)
							}
						} else if logger != nil {
							logger.Info("✅ 共识订阅协议注册成功")
						}
					}
				},
				fx.ParamTags(`name:"network_service"`, `name:"internal_aggregator_service"`, ``),
			),
		),

		// 注册共识事件订阅（迁移自eventIntegration模块）
		fx.Invoke(
			fx.Annotate(
				func(
					eventBus event.EventBus,
					aggregatorService interfaces.InternalAggregatorService,
					minerService consensus.MinerService,
					logger log.Logger,
				) {
					if eventBus != nil {
						// 类型断言检查矿工服务是否实现了事件处理接口
						var minerEventHandler eventIntegration.MinerEventSubscriber
						if meh, ok := minerService.(eventIntegration.MinerEventSubscriber); ok {
							minerEventHandler = meh
						} else {
							if logger != nil {
								logger.Warn("⚠️ 矿工服务未实现事件处理接口，跳过事件订阅注册")
							}
							return
						}

						// 使用集成层统一注册共识事件订阅
						if err := eventIntegration.RegisterEventSubscriptions(
							eventBus,
							aggregatorService, // aggregator实现了AggregatorEventSubscriber
							minerEventHandler, // miner通过类型断言获取事件处理接口
							logger,
						); err != nil {
							if logger != nil {
								logger.Infof("注册共识事件订阅失败: %v", err)
							}
						} else if logger != nil {
							logger.Info("✅ 共识事件订阅注册成功")
						}
					}
				},
				fx.ParamTags(``, `name:"internal_aggregator_service"`, `name:"consensus_miner_service"`, ``),
			),
		),

		// 🔧 修复：添加共识服务生命周期管理，确保矿工服务正确启动和停止
		fx.Invoke(
			fx.Annotate(
				func(
					lc fx.Lifecycle,
					logger log.Logger,
					configProvider config.Provider,
					addressManager crypto.AddressManager,
					minerService consensus.MinerService,
					aggregatorService interfaces.InternalAggregatorService,
				) {
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🔨 启动共识服务...")
							}

							// 单节点模式自动启动挖矿（保持原有行为）
							if configProvider != nil {
								consensusOpts := configProvider.GetConsensus()
								if consensusOpts != nil && !consensusOpts.Aggregator.EnableAggregator {
									genesisConfig := configProvider.GetUnifiedGenesisConfig()
									if genesisConfig != nil && len(genesisConfig.GenesisAccounts) > 0 {
										firstAccount := genesisConfig.GenesisAccounts[0]
										addressStr := firstAccount.Address
										if logger != nil {
											logger.Info("⚠️  单节点模式：自动启动挖矿服务")
											logger.Infof("   矿工地址: %s", addressStr)
										}

										var minerAddressBytes []byte
										if addressManager != nil {
											// 使用 AddressManager 解码地址
											if addr, err := addressManager.StringToAddress(addressStr); err == nil {
												if b, err := addressManager.AddressToBytes(addr); err == nil {
													minerAddressBytes = b
												}
											}
										}

										if len(minerAddressBytes) == 20 {
											if err := minerService.StartMining(context.Background(), minerAddressBytes); err != nil {
												if logger != nil {
													logger.Errorf("单节点模式自动启动挖矿失败: %v", err)
												}
											}
										} else if logger != nil {
											logger.Warn("⚠️  矿工地址解码失败或长度不正确，挖矿未自动启动")
											logger.Warnf("   地址: %s, 解码后长度: %d (期望: 20)", addressStr, len(minerAddressBytes))
											logger.Warn("   请手动调用 wes_startMining 启动挖矿")
										}
									}
								}
							}

							if logger != nil {
								logger.Info("✅ 共识服务启动成功")
							}
							return nil
						},
						OnStop: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🔨 停止共识服务...")
							}

							if stopMining, ok := minerService.(interface{ StopMining(context.Context) error }); ok {
								_ = stopMining.StopMining(ctx)
							}

							if stoppable, ok := aggregatorService.(interface{ Stop(context.Context) error }); ok {
								_ = stoppable.Stop(ctx)
							}

							if logger != nil {
								logger.Info("✅ 共识服务停止成功")
							}
							return nil
						},
					})
				},
				fx.ParamTags(``, ``, `optional:"true"`, `optional:"true"`, `name:"consensus_miner_service"`, `name:"internal_aggregator_service"`),
			),
		),
	)
}
