// Package blockchain 提供WES区块链系统的核心业务模块实现
//
// 📋 **区块链核心模块 (Blockchain Core Module)**
//
// 本包是WES区块链系统的核心业务实现模块，负责协调和管理所有区块链相关的业务逻辑。
// 通过fx依赖注入框架，将各个子模块组织为统一的服务层，对外提供完整的区块链功能。
//
// 🎯 **模块职责**：
// - 实现pkg/interfaces/blockchain中定义的所有公共接口
// - 协调account、block、chain、resource、transaction等子模块
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
// - account/  - 账户管理和余额查询服务
// - block/    - 区块构建、验证和处理服务
// - chain/    - 链状态查询和监控服务
// - resource/ - （已重构到transaction模块的子模块中）
// - transaction/ - 交易构建、签名和提交服务
//
// 🔗 **依赖关系**：
// - 基础设施：依赖crypto、storage、log、event等基础组件
// - 数据层：依赖repository和mempool提供数据访问能力
// - 服务层：各子模块通过内部接口协调，对外统一暴露公共接口
//
// 详细使用说明请参考：internal/core/blockchain/README.md
package blockchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/fx"

	// 公共接口
	blockchain "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 内部配置
	configpkg "github.com/weisyn/v1/internal/config"

	// libp2p

	// 配置
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"

	// 管理器实现
	"github.com/weisyn/v1/internal/core/blockchain/account"
	"github.com/weisyn/v1/internal/core/blockchain/block"
	"github.com/weisyn/v1/internal/core/blockchain/chain"
	"github.com/weisyn/v1/internal/core/blockchain/fork"
	coreifaces "github.com/weisyn/v1/internal/core/blockchain/interfaces"
	syncsvc "github.com/weisyn/v1/internal/core/blockchain/sync"
	"github.com/weisyn/v1/internal/core/blockchain/transaction"

	// 类型定义
	"github.com/weisyn/v1/pkg/types"

	// gRPC服务客户端
	core "github.com/weisyn/v1/pb/blockchain/block"
	transactionpb "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 🔗 集成层依赖
	eventIntegration "github.com/weisyn/v1/internal/core/blockchain/integration/event"
	networkIntegration "github.com/weisyn/v1/internal/core/blockchain/integration/network"
	txEventHandler "github.com/weisyn/v1/internal/core/blockchain/transaction/event_handler"
)

// minerServiceProxy 矿工服务代理，用于解决循环依赖
type minerServiceProxy struct {
	actualService consensus.MinerService
	logger        log.Logger
}

func (p *minerServiceProxy) StartMining(ctx context.Context, minerAddress []byte) error {
	if p.actualService != nil {
		return p.actualService.StartMining(ctx, minerAddress)
	}
	return fmt.Errorf("矿工服务尚未初始化")
}

func (p *minerServiceProxy) StopMining(ctx context.Context) error {
	if p.actualService != nil {
		return p.actualService.StopMining(ctx)
	}
	return fmt.Errorf("矿工服务尚未初始化")
}

func (p *minerServiceProxy) GetMiningStatus(ctx context.Context) (isRunning bool, minerAddress []byte, err error) {
	if p.actualService != nil {
		return p.actualService.GetMiningStatus(ctx)
	}
	return false, nil, fmt.Errorf("矿工服务尚未初始化")
}

// SetActualService 设置真正的矿工服务（延迟注入）
func (p *minerServiceProxy) SetActualService(service consensus.MinerService) {
	p.actualService = service
	if p.logger != nil {
		p.logger.Info("🔗 矿工服务代理已连接到真正的矿工服务")
	}
}

// ModuleInput 定义区块链核心模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 依赖按功能分组：基础设施、存储、密码学、数据层、交易池、gRPC服务、配置。
//
// 📋 **依赖分类**：
// - 基础设施：Logger、EventBus、ConfigProvider等通用组件
// - 存储组件：BadgerStore、MemoryStore等持久化和缓存服务
// - 密码学组件：HashManager、SignatureManager等安全服务
// - 数据层：RepositoryManager、UTXOManager等数据访问服务
// - 外部服务：TxPool、哈希服务客户端等外部协作组件
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

	// 事件总线统一改为未命名基础接口

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
	ResourceManager   repository.ResourceManager   `name:"public_resource_manager" optional:"false"` // 资源管理器
	// 移除构造期对 MinerService 的依赖，避免与共识模块形成环路

	// 🎯 执行层依赖（来自execution模块）
	EngineManager          execution.EngineManager          `name:"execution_engine_manager" optional:"false"` // 执行引擎管理器
	HostCapabilityRegistry execution.HostCapabilityRegistry `name:"execution_host_registry" optional:"false"`  // 宿主能力注册器
	ExecutionCoordinator   execution.ExecutionCoordinator   `name:"execution_coordinator" optional:"false"`    // 执行协调器

	// 交易池层
	TxPool mempool.TxPool `name:"tx_pool" optional:"false"`

	// 网络组件
	NodeHost       nodeiface.Host   `name:"node_host" optional:"true"`       // P2P节点主机（可选）
	NetworkService netiface.Network `name:"network_service" optional:"true"` // 完整网络服务（可选）

	// Kademlia DHT路由表管理器（用于节点发现和管理）
	KBucketManager kademlia.RoutingTableManager `name:"routing_table_manager" optional:"true"` // 路由表管理器

	// 哈希服务客户端（来自crypto模块，避免循环依赖）
	TransactionHashServiceClient transactionpb.TransactionHashServiceClient `optional:"false"`
	BlockHashServiceClient       core.BlockHashServiceClient                `optional:"false"`

	// 配置选项
	BlockchainConfig *blockchainconfig.BlockchainOptions `optional:"true"`
}

// ModuleOutput 定义区块链核心模块的输出服务
//
// 🎯 **服务导出**：
// 本结构体使用fx.Out标签，将各子模块创建的服务统一导出，供其他模块使用。
// 每个服务都有唯一的名称标识，便于在复杂的依赖图中精确定位。
//
// 📋 **导出服务**：
// - ChainService: 链状态查询服务，提供区块链基础信息和状态检查
// - BlockService: 区块管理服务，支持矿工挖矿和节点同步
// - TransactionService: 交易处理服务，管理交易完整生命周期（包含资源管理）
// - AccountService: 账户管理服务，提供用户友好的账户抽象
// - SystemSyncService: 系统同步服务，管理区块链同步状态
//
// 🔗 **服务协作**：
// 导出的服务可被其他模块（如API、矿工、监控等）注入使用，
// 形成完整的区块链应用生态系统。
type ModuleOutput struct {
	fx.Out

	// 核心区块链服务
	ChainService       blockchain.ChainService       `name:"chain_service"`
	BlockService       blockchain.BlockService       `name:"block_service"`
	TransactionService blockchain.TransactionService `name:"transaction_service"`
	AccountService     blockchain.AccountService     `name:"blockchain_account_service"`
	SystemSyncService  blockchain.SystemSyncService  `name:"sync_service"`

	// 🆕 新增：智能合约和AI模型服务（由transaction manager实现）
	ContractService blockchain.ContractService `name:"contract_service"`
	AIModelService  blockchain.AIModelService  `name:"ai_model_service"`
}

// Module 构建并返回区块链核心模块的fx配置
//
// 🎯 **模块构建器**：
// 本函数是区块链核心模块的主要入口点，负责构建完整的fx模块配置。
// 通过fx.Module组织所有子模块的依赖注入配置，确保服务的正确创建和生命周期管理。
//
// 🏗️ **构建流程**：
// 1. 创建各子模块管理器：account、block、chain、resource、transaction
// 2. 配置依赖注入：每个管理器使用fx.Annotate进行接口绑定
// 3. 聚合输出服务：将所有服务包装为ModuleOutput统一导出
// 4. 注册初始化回调：模块加载完成后的日志记录
//
// 📋 **服务创建顺序**：
// - ChainService: 链状态管理器，依赖最少，优先创建
// - BlockService: 区块管理器，依赖链状态和交易池
// - TransactionService: 交易管理器，依赖密码学和存储服务
// - AccountService: 账户管理器，依赖数据存储服务
// - ResourceService: 资源管理器，依赖数据存储服务
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    blockchain.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保以下依赖模块已正确加载：
// - crypto模块：提供哈希和签名服务
// - storage模块：提供数据存储服务
// - repository模块：提供数据访问接口
// - mempool模块：提供交易池服务
func Module() fx.Option {
	return fx.Module("blockchain",
		// 不提供矿工服务，完全依赖共识模块

		fx.Provide(
			// 链状态管理器（导出公共与内部接口）
			fx.Annotate(
				func(input ModuleInput, blockService coreifaces.InternalBlockService, txService coreifaces.InternalTransactionService) (coreifaces.InternalChainService, error) {
					return chain.NewManager(
						input.Logger,
						input.RepositoryManager,
						blockService,
						txService,
					)
				},
				fx.As(new(blockchain.ChainService)),
				fx.As(new(coreifaces.InternalChainService)),
			),

			// 分叉处理服务
			fx.Annotate(
				func(input ModuleInput, chainService coreifaces.InternalChainService, txService coreifaces.InternalTransactionService) coreifaces.InternalForkService {
					return fork.NewManager(
						chainService,
						nil, // BlockService will be injected later via circular dependency resolution
						input.RepositoryManager,
						input.EventBus,
						input.Logger,
					)
				},
				fx.As(new(coreifaces.InternalForkService)),
			),

			// 同步服务管理器
			fx.Annotate(
				func(
					configProvider config.Provider,
					logger log.Logger,
					chainService coreifaces.InternalChainService,
					blockService coreifaces.InternalBlockService,
					repositoryManager repository.RepositoryManager,
					networkService netiface.Network,
					kbucketManager kademlia.RoutingTableManager,
					host nodeiface.Host,
				) coreifaces.InternalSystemSyncService {
					return syncsvc.NewManager(
						chainService,
						blockService,
						repositoryManager,
						networkService,
						kbucketManager,
						host,
						configProvider,
						logger,
					)
				},
				fx.As(new(blockchain.SystemSyncService)),
				fx.As(new(coreifaces.InternalSystemSyncService)),
				fx.ParamTags(``, ``, ``, ``, ``, `name:"network_service"`, `name:"routing_table_manager"`, `name:"node_host"`),
			),

			// 交易管理器（导出公共与内部接口）
			fx.Annotate(
				func(input ModuleInput) coreifaces.InternalTransactionService {
					// 使用矿工服务代理以避免构造期环依赖，真实服务在 fx.Invoke 阶段注入
					minerProxy := &minerServiceProxy{logger: input.Logger}

					manager := transaction.NewManager(
						input.RepositoryManager,
						input.TxPool,
						input.UTXOManager,
						input.ResourceManager,
						minerProxy,
						input.ConfigProvider,
						input.TransactionHashServiceClient,
						input.HashManager,
						input.SignatureManager,
						input.KeyManager,
						input.AddressManager,
						input.MemoryStore,
						input.NetworkService, // ✅ 添加网络服务依赖
						// execution接口依赖
						input.EngineManager,
						input.HostCapabilityRegistry,
						input.ExecutionCoordinator,
						// 网络基础设施依赖
						input.NodeHost,
						input.KBucketManager,
						input.Logger,
					)

					return manager
				},
				fx.As(new(blockchain.TransactionService)),
				fx.As(new(blockchain.TransactionManager)), // 🆕 新增：导出为TransactionManager
				fx.As(new(blockchain.ContractService)),    // 🆕 新增：导出为ContractService
				fx.As(new(blockchain.AIModelService)),     // 🆕 新增：导出为AIModelService
				fx.As(new(coreifaces.InternalTransactionService)),
			),

			// 区块管理器（依赖内部服务与分叉处理、链状态管理）
			fx.Annotate(
				func(input ModuleInput, txService coreifaces.InternalTransactionService) coreifaces.InternalBlockService {
					return block.NewManager(
						input.RepositoryManager,
						input.TxPool,
						input.UTXOManager,
						&minerServiceProxy{logger: input.Logger},
						txService,
						input.NetworkService,
						input.EventBus,
						input.BlockHashServiceClient,
						input.TransactionHashServiceClient,
						input.MerkleTreeManager,
						input.HashManager,
						input.AddressManager,
						input.POWEngine,
						input.MemoryStore,
						input.ConfigProvider,
						input.Logger,
					)
				},
				fx.As(new(blockchain.BlockService)),
				fx.As(new(coreifaces.InternalBlockService)),
			),

			// 账户管理器
			fx.Annotate(
				func(input ModuleInput) (blockchain.AccountService, error) {
					return account.NewManager(
						input.Logger,
						input.RepositoryManager,
						input.UTXOManager,
						input.TxPool,
						input.TransactionHashServiceClient,
					)
				},
				fx.As(new(blockchain.AccountService)),
			),

			// 模块输出聚合
			func(
				chainService blockchain.ChainService,
				blockService blockchain.BlockService,
				transactionService blockchain.TransactionService,
				accountService blockchain.AccountService,
				syncService blockchain.SystemSyncService,
			) ModuleOutput {
				return ModuleOutput{
					ChainService:       chainService,
					BlockService:       blockService,
					TransactionService: transactionService,
					AccountService:     accountService,
					SystemSyncService:  syncService,
				}
			},
		),

		// ====================================================================
		//                           事件集成和协议注册
		// ====================================================================

		// 🎯 区块链事件订阅注册（参考consensus模块的简化模式）
		fx.Invoke(
			func(
				input ModuleInput,
				logger log.Logger,
				syncService coreifaces.InternalSystemSyncService,
			) error {
				if input.EventBus == nil {
					if logger != nil {
						logger.Info("EventBus不可用，跳过区块链事件订阅注册")
					}
					return nil
				}

				// 使用Manager作为同步事件订阅者
				txSubscriber := txEventHandler.NewTransactionEventHandler(logger, input.EventBus)

				// 创建事件订阅注册中心（使用Manager聚合同步事件）
				registry := eventIntegration.NewEventSubscriptionRegistry(
					input.EventBus,
					logger,
					syncService, // 使用Manager而非直接的事件处理器
					txSubscriber,
				)

				// 注册所有事件订阅
				if err := registry.RegisterEventSubscriptions(); err != nil {
					if logger != nil {
						logger.Errorf("区块链事件订阅注册失败: %v", err)
					}
					return err
				}

				if logger != nil {
					logger.Info("✅ 区块链事件订阅注册完成（对齐consensus简化模式）")
				}
				return nil
			},
		),

		// ====================================================================
		//                           网络协议注册和生命周期管理
		// ====================================================================

		// 🔗 延迟注入矿工服务到区块与交易管理器（解决与共识模块的循环依赖）
		fx.Invoke(
			fx.Annotate(
				func(
					minerService consensus.MinerService,
					transactionService blockchain.TransactionService,
					blockService blockchain.BlockService,
					logger log.Logger,
				) {
					// 注入到交易管理器
					if txMgr, ok := transactionService.(interface{ SetMinerService(consensus.MinerService) }); ok {
						txMgr.SetMinerService(minerService)
						if logger != nil {
							logger.Info("🔗 矿工服务已注入到交易管理器")
						}
					} else if logger != nil {
						logger.Warn("⚠️ 交易服务不支持矿工服务注入")
					}

					// 注入到区块管理器
					if blkMgr, ok := blockService.(interface{ SetMinerService(consensus.MinerService) }); ok {
						blkMgr.SetMinerService(minerService)
						if logger != nil {
							logger.Info("🔗 矿工服务已注入到区块管理器")
						}
					} else if logger != nil {
						logger.Warn("⚠️ 区块服务不支持矿工服务注入")
					}
				},
				fx.ParamTags(`name:"consensus_miner_service"`, `name:"transaction_service"`, `name:"block_service"`, ``),
			),
		),

		// 🔗 注册网络集成协议处理器（仅装配领域路由，无业务实现）
		//
		// ⚠️ **重要语法说明**：
		// 当函数参数包含fx.In结构体时，不能使用fx.Annotate包装，
		// 因为fx.In结构体与fx.ParamTags存在冲突。
		// 错误示例：fx.Annotate(func(input ModuleInput, ...) {}, fx.ParamTags(...))
		// 正确示例：直接使用func(input ModuleInput, ...) {}
		fx.Invoke(
			func(
				input ModuleInput,
				logger log.Logger,
				// 领域路由：交易公告（由transaction域实现）与区块公告（由sync域实现）
				txService coreifaces.InternalTransactionService,
				syncService coreifaces.InternalSystemSyncService,
			) error {
				if input.NetworkService == nil || logger == nil {
					return nil
				}

				// 交易公告路由器：通过GetNetworkHandler获取transaction域的网络处理器实现
				var txRouter networkIntegration.TxAnnounceRouter
				if handler, ok := txService.(interface {
					GetNetworkHandler() networkIntegration.TxAnnounceRouter
				}); ok {
					txRouter = handler.GetNetworkHandler()
				}

				// 注意：区块公告处理已迁移到其他模块，此处仅处理交易网络集成

				// 注册流式协议处理器
				// 1. 注册同步流式协议
				err := networkIntegration.RegisterSyncStreamHandlers(
					input.NetworkService,
					syncService, // InternalSystemSyncService 继承了 SyncProtocolRouter
					logger,
				)
				if err != nil {
					logger.Errorf("注册同步流式协议失败: %v", err)
					return err
				}

				// 2. 注册交易流式协议（双重保障传播的备份路径）
				if txProtocolRouter, ok := txRouter.(networkIntegration.TxProtocolRouter); ok {
					err = networkIntegration.RegisterTxStreamHandlers(
						input.NetworkService,
						txProtocolRouter, // transaction的network handler实现了TxProtocolRouter
						logger,
					)
					if err != nil {
						logger.Errorf("注册交易流式协议失败: %v", err)
						return err
					}
				} else {
					logger.Warn("交易路由器未实现TxProtocolRouter接口，跳过交易流式协议注册")
				}

				// 注册订阅处理器（仅注册交易公告处理器）
				err = networkIntegration.RegisterSubscribeHandlers(
					input.NetworkService,
					txRouter, // 只处理交易公告
					logger,
				)
				if err != nil {
					logger.Errorf("注册订阅协议失败: %v", err)
					return err
				}

				logger.Info("✅ 区块链网络集成协议注册完成（流式+订阅）")
				return nil
			},
		),

		// ====================================================================
		//                           创世区块启动检查
		// ====================================================================

		// 创世区块初始化检查（在所有服务加载完成后执行）
		fx.Invoke(
			func(
				input ModuleInput,
				chainService coreifaces.InternalChainService,
				blockService coreifaces.InternalBlockService,
				transactionService coreifaces.InternalTransactionService,
			) error {
				if input.Logger != nil {
					input.Logger.Info("开始创世区块初始化检查...")
				}

				// ✅ 服务依赖已通过构造函数直接注入，无需SetServices调用
				if input.Logger != nil {
					input.Logger.Info("✅ 链管理器服务依赖已通过构造函数注入完成")
				}

				// 获取创世区块配置
				var genesisConfig *types.GenesisConfig

				// 尝试从配置提供者获取区块链配置
				if input.ConfigProvider != nil {
					// 使用配置提供者的内部实现，避免创建新的Config实例
					if provider, ok := input.ConfigProvider.(*configpkg.Provider); ok {
						// 直接调用provider内部的区块链配置获取方法
						if blockchainConfig := provider.GetBlockchain(); blockchainConfig != nil {
							// 从配置选项中获取创世配置（包含完整的账户信息）
							var genesisAccounts []types.GenesisAccount
							for _, account := range blockchainConfig.GenesisConfig.Accounts {
								genesisAccounts = append(genesisAccounts, types.GenesisAccount{
									PublicKey:      account.PublicKey,
									InitialBalance: fmt.Sprintf("%d", account.Amount),
								})
							}

							genesisConfig = &types.GenesisConfig{
								NetworkID:       blockchainConfig.NetworkType,
								ChainID:         blockchainConfig.ChainID,
								Timestamp:       blockchainConfig.GenesisTimestamp,
								GenesisAccounts: genesisAccounts,
							}

							if input.Logger != nil {
								input.Logger.Infof("使用配置加载的创世配置，网络: %s，链ID: %d，账户数: %d",
									genesisConfig.NetworkID, genesisConfig.ChainID, len(genesisConfig.GenesisAccounts))
								if len(genesisAccounts) > 0 {
									input.Logger.Debugf("genesis_first_account_amount: %s", genesisAccounts[0].InitialBalance)
								}
							}
						} else {
							if input.Logger != nil {
								input.Logger.Info("配置提供者中无区块链配置，使用默认创世配置")
							}
							genesisConfig = createDefaultGenesisConfig()
						}
					} else {
						if input.Logger != nil {
							input.Logger.Info("配置提供者类型不匹配，使用默认创世配置")
						}
						genesisConfig = createDefaultGenesisConfig()
					}
				} else {
					if input.Logger != nil {
						input.Logger.Info("配置提供者为空，使用默认创世配置")
					}
					genesisConfig = createDefaultGenesisConfig()
				}

				// 通过链服务检查是否需要创世区块
				if chainManager, ok := chainService.(*chain.Manager); ok {
					ctx := context.Background()

					// 检查并初始化创世区块
					created, err := chainManager.InitializeGenesisIfNeeded(ctx, genesisConfig)
					if err != nil {
						if input.Logger != nil {
							input.Logger.Errorf("创世区块初始化失败: %v", err)
						}
						return fmt.Errorf("创世区块初始化失败: %w", err)
					}

					if created {
						if input.Logger != nil {
							input.Logger.Info("✅ 创世区块初始化完成")
						}
					} else {
						if input.Logger != nil {
							input.Logger.Info("✅ 链已初始化，跳过创世区块创建")
						}
					}
				} else {
					if input.Logger != nil {
						input.Logger.Warn("⚠️ 无法获取链管理器，跳过创世区块检查")
					}
				}

				return nil
			},
		),

		// ====================================================================
		//                           同步服务事件订阅
		// ====================================================================

		// 🔄 注册对等节点连接事件的同步触发逻辑（事件驱动同步，支持去抖和限流）
		fx.Invoke(
			func(
				input ModuleInput,
				syncService coreifaces.InternalSystemSyncService,
				chainService coreifaces.InternalChainService,
			) error {
				if input.Logger != nil {
					input.Logger.Info("注册对等节点连接事件驱动的自动同步...")
				}

				// 去抖与限流状态管理
				var debounceStateMutex sync.RWMutex
				peerLastTriggered := make(map[peer.ID]time.Time)
				var globalLastTriggered time.Time

				// 只有当事件总线可用时才设置事件驱动同步
				if input.EventBus != nil {
					// 订阅对等节点连接事件，触发自动同步
					peerConnectedHandler := func(ctx context.Context, data interface{}) error {
						if peerID, ok := data.(peer.ID); ok {
							if input.Logger != nil {
								input.Logger.Infof("🔗 对等节点连接事件：%s，触发自动同步检查...", peerID.String()[:12]+"...")
							}

							// 异步执行同步检查，避免阻塞事件处理
							go func() {
								// 获取去抖和限流配置
								var peerDebounceMs int = 1000   // 默认1000ms
								var globalIntervalMs int = 2000 // 默认2000ms
								if input.ConfigProvider != nil {
									if blockchainConfig := input.ConfigProvider.GetBlockchain(); blockchainConfig != nil {
										if blockchainConfig.Sync.Advanced.PeerEventDebounceMs > 0 {
											peerDebounceMs = blockchainConfig.Sync.Advanced.PeerEventDebounceMs
										}
										if blockchainConfig.Sync.Advanced.GlobalMinTriggerIntervalMs > 0 {
											globalIntervalMs = blockchainConfig.Sync.Advanced.GlobalMinTriggerIntervalMs
										}
									}
								}

								now := time.Now()
								skipReason := ""

								// 检查去抖和限流条件
								debounceStateMutex.Lock()

								// 检查同一节点去抖间隔
								if lastTime, exists := peerLastTriggered[peerID]; exists {
									peerInterval := now.Sub(lastTime)
									if peerInterval < time.Duration(peerDebounceMs)*time.Millisecond {
										skipReason = fmt.Sprintf("peer debounce (Δt=%dms < %dms)", peerInterval.Milliseconds(), peerDebounceMs)
									}
								}

								// 检查全局最小触发间隔
								if skipReason == "" {
									globalInterval := now.Sub(globalLastTriggered)
									if globalInterval < time.Duration(globalIntervalMs)*time.Millisecond {
										skipReason = fmt.Sprintf("global rate-limit (Δt=%dms < %dms)", globalInterval.Milliseconds(), globalIntervalMs)
									}
								}

								if skipReason != "" {
									debounceStateMutex.Unlock()
									if input.Logger != nil {
										input.Logger.Infof("⏩ skip: %s, peer=%s", skipReason, peerID.String()[:12]+"...")
									}
									return
								}

								// 更新触发时间记录
								peerLastTriggered[peerID] = now
								globalLastTriggered = now
								debounceStateMutex.Unlock()
								// 检查系统就绪状态
								ready, err := chainService.IsReady(context.Background())
								if err != nil {
									if input.Logger != nil {
										input.Logger.Debugf("事件驱动同步-系统就绪检查失败: %v", err)
									}
									return
								}

								if !ready {
									if input.Logger != nil {
										input.Logger.Debug("事件驱动同步-系统尚未就绪，跳过自动同步")
									}
									return
								}

								// 暂时跳过高度探测（待接口完善后启用）
								// 高度探测接口未启用
								if input.Logger != nil {
									input.Logger.Debugf("🔍 对等节点连接: %s，准备触发同步", peerID.String()[:12]+"...")
								}

								// 触发网络同步（对等节点连接后）
								if err := syncService.TriggerSync(context.Background()); err != nil {
									if input.Logger != nil {
										input.Logger.Debugf("事件驱动同步失败: %v", err)
									}
								} else {
									if input.Logger != nil {
										input.Logger.Info("✅ 对等节点连接后自动同步已触发")
									}
								}
							}()
						}
						return nil
					}

					// 订阅network.peer.connected事件
					if err := input.EventBus.Subscribe(event.EventTypeNetworkPeerConnected, peerConnectedHandler); err != nil {
						if input.Logger != nil {
							input.Logger.Warnf("订阅对等节点连接事件失败: %v", err)
						}
					} else {
						if input.Logger != nil {
							input.Logger.Info("✅ 已订阅对等节点连接事件，将在节点连接后自动触发同步")
						}
					}
				} else {
					if input.Logger != nil {
						input.Logger.Warn("⚠️ EventBus不可用，无法设置事件驱动自动同步")
					}
				}

				return nil
			},
		),

		// ====================================================================
		//                           生命周期管理
		// ====================================================================

		// 🔄 区块链系统生命周期管理（集成定时同步调度器）
		fx.Invoke(
			func(
				lc fx.Lifecycle,
				input ModuleInput,
				syncService coreifaces.InternalSystemSyncService,
			) {
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if input.Logger != nil {
							input.Logger.Info("🚀 区块链核心系统启动")
						}

						// 启动定时同步调度器
						if syncManager, ok := syncService.(*syncsvc.Manager); ok {
							if periodicScheduler := syncManager.GetPeriodicScheduler(); periodicScheduler != nil {
								if err := periodicScheduler.Start(ctx); err != nil {
									if input.Logger != nil {
										input.Logger.Warnf("启动定时同步调度器失败: %v", err)
									}
								} else {
									if input.Logger != nil {
										input.Logger.Info("✅ 定时同步调度器已启动")
									}
								}
							}
						}

						return nil
					},
					OnStop: func(ctx context.Context) error {
						// 停止定时同步调度器
						if syncManager, ok := syncService.(*syncsvc.Manager); ok {
							if periodicScheduler := syncManager.GetPeriodicScheduler(); periodicScheduler != nil {
								periodicScheduler.Stop()
								if input.Logger != nil {
									input.Logger.Info("🛑 定时同步调度器已停止")
								}
							}
						}

						// 停止时取消所有正在进行的同步
						if err := syncService.CancelSync(ctx); err != nil {
							if input.Logger != nil {
								input.Logger.Warnf("停止同步服务时出错: %v", err)
							}
						}

						if input.Logger != nil {
							input.Logger.Info("🛑 区块链核心系统已停止")
						}
						return nil
					},
				})
			},
		),

		fx.Invoke(
			func(logger log.Logger) {
				if logger != nil {
					logger.Info("区块链核心模块已加载")
				}
			},
		),

		// 监听peer连接事件的逻辑已迁移至 integration/event 层，模块保持纯装配
	)
}

// ============================================================================
// 🔧 创世区块配置辅助函数
// ============================================================================

// createDefaultGenesisConfig 创建默认创世配置
//
// 🎯 **默认创世配置生成器**
//
// 当系统没有提供创世配置时，创建一个最小化的默认配置：
// 1. 设置基本的网络参数
// 2. 使用当前时间戳
// 3. 不包含预设账户（纯净的创世状态）
//
// 返回值：
//
//	*types.GenesisConfig: 默认创世配置对象
func createDefaultGenesisConfig() *types.GenesisConfig {
	return &types.GenesisConfig{
		ChainID:         1,                        // 默认链ID
		NetworkID:       "weisyn_default",         // 默认网络ID
		Timestamp:       time.Now().Unix(),        // 当前时间戳
		GenesisAccounts: []types.GenesisAccount{}, // 空账户列表
	}
}
