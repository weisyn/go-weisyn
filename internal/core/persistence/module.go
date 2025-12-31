// Package persistence 提供统一数据持久化服务的 fx 模块配置
//
// 📦 **Persistence 模块 (Persistence Module)**
//
// 本包提供 WES 系统的统一数据持久化服务（QueryService + DataWriter）的依赖注入配置。
//
// 🎯 **模块职责**：
// - 提供 QueryService（统一查询入口）
// - 提供 DataWriter（统一写入入口）
// - 协调所有数据读写操作
//
// 💡 **设计原则**：
// - CQRS 架构：读写分离，QueryService 和 DataWriter 在同一组件中
// - 统一入口：QueryService 是唯一查询入口，DataWriter 是唯一写入入口
// - 避免循环依赖：DataWriter 直接读存储，不依赖 QueryService
//
// 🏗️ **架构规范**：
// ```
// 公共接口（pkg/interfaces/persistence）
//
//	↑ fx.As() 绑定
//
// 内部接口（internal/core/persistence/interfaces）
//
//	↑ 实现
//
// 具体服务（internal/core/persistence/*/service.go）
// ```
package persistence

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/core/persistence/query/account"
	"github.com/weisyn/v1/internal/core/persistence/query/aggregator"
	"github.com/weisyn/v1/internal/core/persistence/query/block"
	"github.com/weisyn/v1/internal/core/persistence/query/chain"
	"github.com/weisyn/v1/internal/core/persistence/query/eutxo"
	queryinterfaces "github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/internal/core/persistence/query/pricing"
	"github.com/weisyn/v1/internal/core/persistence/query/resource"
	"github.com/weisyn/v1/internal/core/persistence/query/tx"
	persistencerepair "github.com/weisyn/v1/internal/core/persistence/repair"
	"github.com/weisyn/v1/internal/core/persistence/writer"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 persistence 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 依赖按功能分组：基础设施、存储、密码学、哈希服务客户端。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger         log.Logger      `optional:"true"` // 日志记录器
	EventBus       event.EventBus  `optional:"true"` // 事件总线（用于corruption事件发布）
	ConfigProvider config.Provider `optional:"true"` // 配置提供者（用于repair参数）

	// ========== 存储组件 ==========
	BadgerStore storage.BadgerStore `optional:"false"` // BadgerDB存储
	FileStore   storage.FileStore   `optional:"false"` // 文件存储

	// ========== 密码学组件 ==========
	HashManager crypto.HashManager `optional:"false"` // 哈希管理器（用于UTXOQuery状态根计算）

	// ========== 哈希服务客户端 ==========
	BlockHashClient       core.BlockHashServiceClient              `optional:"false"` // 区块哈希服务客户端
	TransactionHashClient transaction.TransactionHashServiceClient `optional:"false"` // 交易哈希服务客户端
}

// ============================================================================
//                              模块输出服务
// ============================================================================

// ModuleOutput 定义 persistence 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	QueryService persistence.QueryService `name:"query_service"` // 统一查询服务
	DataWriter   persistence.DataWriter   `name:"data_writer"`   // 统一写入服务

	// 子查询服务导出（命名依赖）
	ChainQuery    persistence.ChainQuery    `name:"chain_query"`    // 链状态查询
	BlockQuery    persistence.BlockQuery    `name:"block_query"`    // 区块查询
	TxQuery       persistence.TxQuery       `name:"tx_query"`       // 交易查询
	UTXOQuery     persistence.UTXOQuery     `name:"utxo_query"`     // UTXO查询
	ResourceQuery persistence.ResourceQuery `name:"resource_query"` // 资源查询
	AccountQuery  persistence.AccountQuery  `name:"account_query"`  // 账户查询
	PricingQuery  persistence.PricingQuery  `name:"pricing_query"`  // 定价查询（Phase 2）

	// 内部接口导出（未命名，供内部使用）
	InternalChainQuery    queryinterfaces.InternalChainQuery    // 内部链状态查询
	InternalBlockQuery    queryinterfaces.InternalBlockQuery    // 内部区块查询
	InternalTxQuery       queryinterfaces.InternalTxQuery       // 内部交易查询
	InternalUTXOQuery     queryinterfaces.InternalUTXOQuery     // 内部UTXO查询
	InternalResourceQuery queryinterfaces.InternalResourceQuery // 内部资源查询
	InternalAccountQuery  queryinterfaces.InternalAccountQuery  // 内部账户查询
	InternalPricingQuery  queryinterfaces.InternalPricingQuery  // 内部定价查询（Phase 2）
}

// ============================================================================
//                              模块定义
// ============================================================================

// ProvideServices 提供 persistence 模块的所有服务
//
// 🎯 **服务创建**：
// 本函数负责创建 persistence 模块的所有服务实例，并通过 ModuleOutput 统一导出。
// 注意：子查询服务之间有依赖关系，需要按顺序创建。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 🎯 为持久化模块添加 module 字段，日志将路由到 node-system.log
	var persistenceLogger log.Logger
	if input.Logger != nil {
		persistenceLogger = input.Logger.With("module", "persistence")
	}

	// ✅ 自愈子组件（属于 persistence 内部能力，不作为 core 一级组件）
	// - 不引入新的 fx module
	// - 仅在存在 EventBus 时订阅 corruption.detected
	if input.EventBus != nil {
		opts := persistencerepair.Options{}
		if input.ConfigProvider != nil && input.ConfigProvider.GetBlockchain() != nil {
			adv := input.ConfigProvider.GetBlockchain().Sync.Advanced
			opts.Enabled = adv.RepairEnabled
			opts.MaxConcurrency = adv.RepairMaxConcurrency
			opts.ThrottleSeconds = adv.RepairThrottleSeconds
			opts.HashIndexWindow = adv.RepairHashIndexWindow
		}
		// 默认启用（若未提供 ConfigProvider，则使用内部默认值）
		if input.ConfigProvider == nil {
			opts.Enabled = true
		}
		if mgr, err := persistencerepair.NewManager(input.BadgerStore, input.FileStore, input.BlockHashClient, input.TransactionHashClient, input.EventBus, persistenceLogger, opts); err == nil {
			mgr.RegisterSubscriptions(context.Background())
			if persistenceLogger != nil {
				persistenceLogger.Info("🩹 Persistence RepairManager 已启用（订阅 corruption.detected）")
			}
		} else if persistenceLogger != nil {
			persistenceLogger.Warnf("Persistence RepairManager 初始化失败（已降级为禁用）: %v", err)
		}
	}

	// 1. 创建基础查询服务（注意顺序：BlockQuery 需要在 ChainQuery 之前创建）
	txQuery, err := tx.NewService(input.BadgerStore, input.FileStore, input.TransactionHashClient, input.EventBus, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 TxQuery 失败: %w", err)
	}

	// 2. 创建 BlockQuery（区块数据从 blocks/ 文件读取，Badger 存索引）
	blockQuery, err := block.NewService(input.BadgerStore, input.FileStore, input.ConfigProvider, input.EventBus, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 BlockQuery 失败: %w", err)
	}

	// 3. 创建 ChainQuery（依赖 BlockQuery 用于链尖修复）
	chainQuery, err := chain.NewService(input.BadgerStore, persistenceLogger, blockQuery)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 ChainQuery 失败: %w", err)
	}

	// 3.1 启动时验证并修复链尖数据（关键：防止链尖数据损坏导致系统无法启动）
	if chainQueryService, ok := chainQuery.(*chain.Service); ok {
		if err := chainQueryService.ValidateAndRepairOnStartup(context.Background()); err != nil {
			// 链尖修复失败是严重错误，应该阻止系统启动
			return ModuleOutput{}, fmt.Errorf("启动时链尖验证失败: %w", err)
		}
	}

	// 4. 创建 UTXOQuery（依赖 BadgerStore, HashManager）
	utxoQuery, err := eutxo.NewService(input.BadgerStore, input.HashManager, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 UTXOQuery 失败: %w", err)
	}

	// 5. 创建 ResourceQuery（依赖 BadgerStore, FileStore, TxQuery）
	resourceQuery, err := resource.NewService(input.BadgerStore, input.FileStore, txQuery, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 ResourceQuery 失败: %w", err)
	}

	// 6. 创建 AccountQuery（依赖 BadgerStore, UTXOQuery）
	accountQuery, err := account.NewService(input.BadgerStore, utxoQuery, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 AccountQuery 失败: %w", err)
	}

	// 7. 创建 PricingQuery（依赖 BadgerStore, TxQuery, ResourceQuery）（Phase 2）
	pricingQuery, err := pricing.NewService(input.BadgerStore, txQuery, resourceQuery, persistenceLogger)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 PricingQuery 失败: %w", err)
	}

	// 8. 创建 QueryService（聚合所有子查询服务）
	queryService, err := aggregator.NewService(
		chainQuery,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		persistenceLogger,
	)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 QueryService 失败: %w", err)
	}

	// 9. 创建 DataWriter（依赖 BadgerStore, FileStore, BlockHashClient, TransactionHashClient）
	dataWriter := writer.NewService(
		input.BadgerStore,
		input.FileStore,
		input.BlockHashClient,
		input.TransactionHashClient,
		persistenceLogger,
	)

	// 类型断言为公共接口
	var publicChainQuery persistence.ChainQuery = chainQuery
	var publicBlockQuery persistence.BlockQuery = blockQuery
	var publicTxQuery persistence.TxQuery = txQuery
	var publicUTXOQuery persistence.UTXOQuery = utxoQuery
	var publicResourceQuery persistence.ResourceQuery = resourceQuery
	var publicAccountQuery persistence.AccountQuery = accountQuery
	var publicPricingQuery persistence.PricingQuery = pricingQuery
	var publicQueryService persistence.QueryService = queryService
	var publicDataWriter persistence.DataWriter = dataWriter

	return ModuleOutput{
		QueryService:          publicQueryService,
		DataWriter:            publicDataWriter,
		ChainQuery:            publicChainQuery,
		BlockQuery:            publicBlockQuery,
		TxQuery:               publicTxQuery,
		UTXOQuery:             publicUTXOQuery,
		ResourceQuery:         publicResourceQuery,
		AccountQuery:          publicAccountQuery,
		PricingQuery:          publicPricingQuery,
		InternalChainQuery:    chainQuery,
		InternalBlockQuery:    blockQuery,
		InternalTxQuery:       txQuery,
		InternalUTXOQuery:     utxoQuery,
		InternalResourceQuery: resourceQuery,
		InternalAccountQuery:  accountQuery,
		InternalPricingQuery:  pricingQuery,
	}, nil
}

// Module fx模块定义
//
// 🎯 **模块职责**：
// 提供统一数据持久化服务（QueryService + DataWriter）的依赖注入配置。
//
// 💡 **设计原则**：
// - 分层提供：各子服务通过内部接口绑定到公共接口
// - 统一聚合：QueryService 聚合所有子查询服务
// - 统一写入：DataWriter 提供统一写入入口
// - 接口隔离：调用方只依赖公共接口
//
// ⚠️ **注意事项**：
// - QueryService 和 DataWriter 在同一组件中，但职责分离
// - DataWriter 不依赖 QueryService，避免循环依赖
// - 所有服务通过 fx 依赖注入提供
func Module() fx.Option {
	return fx.Module(
		"persistence",
		// ====================================================================
		//                           服务提供
		// ====================================================================

		fx.Provide(
			// 提供所有服务（通过 ModuleOutput 统一导出）
			// fx 会自动展开 ModuleOutput 结构体（因为它有 fx.Out）
			// 所有带 name tag 的字段会注册为命名依赖
			// 所有未命名的字段会注册为未命名依赖
			// 注意：统一使用命名依赖，确保一致性
			ProvideServices,
		),

		// ====================================================================
		//                           生命周期管理
		// ====================================================================

		fx.Invoke(
			fx.Annotate(
				func(
					queryService persistence.QueryService,
					dataWriter persistence.DataWriter,
					logger log.Logger,
					lc fx.Lifecycle,
				) {
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🚀 Persistence 模块已启动（已聚合查询和写入服务）")
							}
							// 确保 DataWriter 和 QueryService 实例不为 nil
							if queryService == nil {
								return fmt.Errorf("QueryService 实例未成功创建")
							}
							if dataWriter == nil {
								return fmt.Errorf("DataWriter 实例未成功创建")
							}
							return nil
						},
						OnStop: func(ctx context.Context) error {
							if logger != nil {
								logger.Info("🛑 Persistence 模块已停止")
							}
							return nil
						},
					})
				},
				// 使用命名依赖注入（QueryService 和 DataWriter 通过命名依赖提供）
				fx.ParamTags(
					`name:"query_service"`, // persistence.QueryService
					`name:"data_writer"`,   // persistence.DataWriter
					``,                     // log.Logger
					``,                     // fx.Lifecycle
				),
			),
		),

		// 资产 UTXO 自动健康检查与修复控制器
		fx.Invoke(
			StartAutoAssetUTXOHealthController,
		),
	)
}
