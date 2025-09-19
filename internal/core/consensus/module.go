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
package consensus

import (
	"context"

	"go.uber.org/fx"

	// 配置
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// protobuf
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 管理器实现
	"github.com/weisyn/v1/internal/core/consensus/aggregator"
	"github.com/weisyn/v1/internal/core/consensus/miner"

	// 内部接口
	"github.com/weisyn/v1/internal/core/consensus/interfaces"

	// integration集成组件
	eventIntegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	networkIntegration "github.com/weisyn/v1/internal/core/consensus/integration/network"
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

	// 密码学组件
	HashManager       crypto.HashManager       `optional:"false"`
	SignatureManager  crypto.SignatureManager  `optional:"true"`
	KeyManager        crypto.KeyManager        `optional:"true"`
	AddressManager    crypto.AddressManager    `optional:"true"`
	MerkleTreeManager crypto.MerkleTreeManager `optional:"false"`
	POWEngine         crypto.POWEngine         `optional:"false"`

	// 数据层
	RepositoryManager repository.RepositoryManager `optional:"false"`
	UTXOManager       repository.UTXOManager       `optional:"false"`

	// 区块链层（恢复必要的业务依赖）
	ChainService       blockchain.ChainService       `optional:"true"`
	BlockService       blockchain.BlockService       `optional:"true"`
	TransactionService blockchain.TransactionService `optional:"true"`
	SystemSyncService  blockchain.SystemSyncService  `optional:"true"`

	// 网络组件
	NodeHost       nodeiface.Host   `name:"node_host" optional:"true"`
	NetworkService netiface.Network `name:"network_service" optional:"true"`
	P2PService     nodeiface.Host   `name:"node_host" optional:"true"`

	// 配置相关（可选扩展配置）

	// 哈希相关服务
	TxHashClient    transaction.TransactionHashServiceClient `optional:"true"`
	BlockHashClient core.BlockHashServiceClient              `optional:"true"`

	// 内存池服务
	CandidatePool mempool.CandidatePool `optional:"true" name:"candidate_pool"` // 候选区块池（可选依赖）

	// Kademlia网络组件
	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager" optional:"true"`
	DistanceCalculator  kademlia.DistanceCalculator  `name:"distance_calculator" optional:"true"`

	// 缓存存储
	CacheStore storage.MemoryStore `optional:"true"`

	// 合规服务（可选）
	CompliancePolicy complianceIfaces.Policy `optional:"true"`
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
					return aggregator.NewManager(
						input.Logger,
						input.EventBus,
						input.CandidatePool,
						// 修复：添加缺失的依赖参数
						input.HashManager,
						input.SignatureManager,
						input.KeyManager, // 添加密钥管理器
						input.POWEngine,  // 添加POW引擎
						input.NodeHost,
						input.NetworkService,
						input.ChainService,
						input.DistanceCalculator,  // 使用正确的 DistanceCalculator
						consensusOptions,          // 添加配置参数
						input.SystemSyncService,   // 添加同步服务参数
						input.RoutingTableManager, // 添加路由表管理器参数
						input.BlockService,        // 添加区块服务依赖用于处理共识结果
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
					blockService blockchain.BlockService,
					chainService blockchain.ChainService,
					systemSyncService blockchain.SystemSyncService,
					memoryStore storage.MemoryStore,
					networkService netiface.Network,
					powEngine crypto.POWEngine,
					hashManager crypto.HashManager,
					merkleManager crypto.MerkleTreeManager,
					aggregatorService interfaces.InternalAggregatorService,
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
					return miner.NewManager(
						// ========== 基础依赖 ==========
						logger,           // 日志记录器
						eventBus,         // 事件总线
						consensusOptions, // 共识配置

						// ========== 业务服务依赖（传递给子模块） ==========
						blockService,      // 区块服务
						chainService,      // 链服务
						systemSyncService, // 同步服务
						memoryStore,       // 内存缓存
						networkService,    // 网络服务

						// ========== 加密服务依赖（传递给子模块） ==========
						powEngine,     // PoW引擎
						hashManager,   // 哈希管理器
						merkleManager, // 默克尔树管理器

						// ========== 聚合器依赖（用于区块提交） ==========
						aggregatorService, // 聚合器控制器接口

						// ========== 合规依赖（可选） ==========
						compliancePolicy, // 合规策略服务
					)
				},
				fx.As(new(consensus.MinerService)),
				fx.ParamTags(``, ``, ``, ``, ``, ``, ``, `name:"network_service"`, ``, ``, ``, `name:"internal_aggregator_service"`, `optional:"true"`),
				fx.ResultTags(`name:"consensus_miner_service"`),
			),

			// 事件协调器由 eventIntegration.Module() 统一提供，避免重复提供
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
							logger.Info("🔗 聚合器服务已注入到矿工服务")
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ 矿工服务不支持聚合器服务注入")
						}
					}
				},
				fx.ParamTags(`name:"consensus_miner_service"`, `name:"internal_aggregator_service"`, ``),
			),
		),

		// 模块初始化回调
		fx.Invoke(func(
			logger log.Logger,
		) {
			if logger != nil {
				logger.Info("🚀 共识核心模块初始化完成")
			}
		}),

		// 注意：已移除全局 setter 调用，哈希客户端现在通过 Provider 注入

		// 延迟注入区块服务到矿工管理器（解决循环依赖）
		fx.Invoke(
			fx.Annotate(
				func(
					minerService consensus.MinerService,
					blockService blockchain.BlockService,
					logger log.Logger,
				) {
					// 将区块服务注入到矿工管理器
					if minerManager, ok := minerService.(interface{ SetBlockService(blockchain.BlockService) }); ok {
						minerManager.SetBlockService(blockService)
						if logger != nil {
							logger.Info("🔗 区块服务已注入到矿工管理器")
						}
					} else if logger != nil {
						logger.Warn("⚠️ 矿工管理器不支持区块服务注入")
					}
				},
				fx.ParamTags(`name:"consensus_miner_service"`, `name:"block_service"`, ``),
			),
		),

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
					minerService consensus.MinerService,
					aggregatorService interfaces.InternalAggregatorService,
				) {
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🔨 启动共识服务...")
							}

							// 启动矿工服务（如果需要）
							// 注意：矿工服务通常是按需启动的，不是自动启动

							if logger != nil {
								logger.Info("✅ 共识服务启动成功")
							}
							return nil
						},
						OnStop: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🔨 停止共识服务...")
							}

							// 停止矿工服务（如果正在运行）
							if stopMining, ok := minerService.(interface{ StopMining(context.Context) error }); ok {
								if err := stopMining.StopMining(ctx); err != nil {
									if logger != nil {
										logger.Errorf("停止矿工服务失败: %v", err)
									}
									// 不返回错误，继续停止其他服务
								}
							}

							// 停止聚合器服务（如果有停止方法）
							if stoppable, ok := aggregatorService.(interface{ Stop(context.Context) error }); ok {
								if err := stoppable.Stop(ctx); err != nil {
									if logger != nil {
										logger.Errorf("停止聚合器服务失败: %v", err)
									}
								}
							}

							if logger != nil {
								logger.Info("✅ 共识服务停止成功")
							}
							return nil
						},
					})
				},
				fx.ParamTags(``, ``, `name:"consensus_miner_service"`, `name:"internal_aggregator_service"`),
			),
		),

		// 注释：聚合器初始化已移除，因为聚合器是被动激活的内部服务
	)
}
