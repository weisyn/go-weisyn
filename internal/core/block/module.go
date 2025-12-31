// Package block 提供区块管理的核心实现
//
// 🔗 **Block 模块 (Block Module)**
//
// 本包实现了区块管理的核心功能，包括：
// - 区块构建（BlockBuilder）
// - 区块验证（BlockValidator）
// - 区块处理（BlockProcessor）
// - 事件集成（Event Integration）✅
// - 生命周期管理
//
// 🏗️ **模块架构**：
// - 使用 fx 依赖注入框架
// - 遵循 CQRS 架构原则
// - 支持事件驱动通信
// - 提供完整的生命周期管理
//
// 📦 **导出服务**：
// - blockutil.BlockBuilder: 区块构建接口 ✅
// - blockutil.BlockValidator: 区块验证接口 ✅
// - blockutil.BlockProcessor: 区块处理接口 ✅
package block

import (
	"context"

	"go.uber.org/fx"

	// 公共接口
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	blockif "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"

	// 内部实现
	"github.com/weisyn/v1/internal/core/block/builder"
	"github.com/weisyn/v1/internal/core/block/genesis"
	eventintegration "github.com/weisyn/v1/internal/core/block/integration/event"
	"github.com/weisyn/v1/internal/core/block/interfaces"
	blockprocessor "github.com/weisyn/v1/internal/core/block/processor"
	"github.com/weisyn/v1/internal/core/block/validator"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 block 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 依赖按功能分组：基础设施、存储、密码学、数据层、外部服务。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger log.Logger `optional:"true"` // 日志记录器
	ConfigProvider config.Provider `optional:"false"` // 配置提供者（v2 共识规则必需）

	// ========== 存储组件 ==========
	BadgerStore storage.BadgerStore `optional:"false"` // BadgerDB存储

	// ========== 密码学组件 ==========
	HashManager crypto.HashManager `optional:"false"` // 哈希管理器

	// ========== 哈希服务客户端 ==========
	BlockHashClient       core.BlockHashServiceClient              `optional:"false"` // 区块哈希服务客户端
	TransactionHashClient transaction.TransactionHashServiceClient `optional:"false"` // 交易哈希服务客户端

	// ========== 数据层依赖 ==========
	QueryService persistence.QueryService `optional:"false" name:"query_service"` // 统一查询服务
	DataWriter   persistence.DataWriter   `optional:"false" name:"data_writer"`   // 统一写入服务

	// ========== 区块链域依赖 ==========
	TxPool      mempool.TxPool `optional:"false" name:"tx_pool"`               // 交易内存池
	TxProcessor tx.TxProcessor `optional:"false"`                              // 交易处理器
	TxVerifier  tx.TxVerifier  `optional:"false" name:"tx_verifier"`           // 交易验证器
	FeeManager  tx.FeeManager  `optional:"false" name:"consensus_fee_manager"` // 费用管理器

	// ========== EUTXO 域依赖 ==========
	UTXOWriter eutxo.UTXOWriter `optional:"true"` // UTXO写入器（可选）

	// ========== ISPC 域依赖 ==========
	ZKProofService ispc.ZKProofService `optional:"true"` // ZK证明服务（可选，用于验证StateOutput的ZK证明）

	// ========== 写控制 ==========
	WriteGate wgif.WriteGate `optional:"true"` // 全局写门闸（可选，用于只读模式和 REORG 写控制）

	// ========== 事件总线 ==========
	EventBus event.EventBus `optional:"true"` // 事件总线（可选）
}

// ============================================================================
//                              模块输出服务
// ============================================================================

// ModuleOutput 定义 block 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	BlockBuilder   blockif.BlockBuilder        `name:"block_builder"`   // 区块构建器
	BlockValidator blockif.BlockValidator      `name:"block_validator"` // 区块验证器
	BlockProcessor blockif.BlockProcessor      `name:"block_processor"` // 区块处理器
	GenesisBuilder blockif.GenesisBlockBuilder `name:"genesis_builder"` // 创世区块构建器

	// 内部接口导出（命名依赖，供其他模块使用）
	InternalBlockBuilder   interfaces.InternalBlockBuilder        `name:"block_builder"`   // 内部区块构建器（命名版本）
	InternalBlockValidator interfaces.InternalBlockValidator      `name:"block_validator"` // 内部区块验证器（命名版本）
	InternalBlockProcessor interfaces.InternalBlockProcessor      `name:"block_processor"` // 内部区块处理器（命名版本）
	InternalGenesisBuilder interfaces.InternalGenesisBlockBuilder `name:"genesis_builder"` // 内部创世区块构建器（命名版本）
}

// ============================================================================
//                              模块定义
// ============================================================================

// Module 返回 block 模块的 fx 配置
//
// 🎯 **模块职责**：
// - 提供 BlockBuilder 服务 ✅
// - 提供 BlockValidator 服务 ✅
// - 提供 BlockProcessor 服务 ✅
// - 注册事件发布和订阅 ✅
// - 管理生命周期 ✅
//
// 🔗 **依赖关系**：
// - 输入：Storage, Mempool, TxProcessor, HashManager, QueryService, Consensus（可选）, UTXOWriter, DataWriter, EventBus（可选）, Logger
// - 输出：BlockBuilder, BlockValidator, BlockProcessor
//
// 📋 **导出服务**：
// - blockif.BlockBuilder (name: "block_builder") ✅
// - blockif.BlockValidator (name: "block_validator") ✅
// - blockif.BlockProcessor (name: "block_processor") ✅
// - interfaces.InternalBlockBuilder (未命名，内部使用) ✅
// - interfaces.InternalBlockValidator (未命名，内部使用) ✅
// - interfaces.InternalBlockProcessor (未命名，内部使用) ✅
// ProvideServices 提供 block 模块的所有服务
//
// 🎯 **服务创建**：
// 本函数负责创建 block 模块的所有服务实例，并通过 ModuleOutput 统一导出。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 🎯 为区块模块添加 module 字段，日志将路由到 node-system.log
	var blockLogger log.Logger
	if input.Logger != nil {
		blockLogger = input.Logger.With("module", "block")
	}
	
	// 从 QueryService 获取 UTXOQuery、BlockQuery 和 ChainQuery
	var utxoQuery persistence.UTXOQuery
	var blockQuery persistence.BlockQuery
	var chainQuery persistence.ChainQuery
	if input.QueryService != nil {
		utxoQuery = input.QueryService  // QueryService 本身实现了 UTXOQuery
		blockQuery = input.QueryService // QueryService 本身实现了 BlockQuery
		chainQuery = input.QueryService // QueryService 本身实现了 ChainQuery
	}

	// 创建 BlockBuilder 服务
	blockBuilder, err := builder.NewService(
		input.BadgerStore,
		input.TxPool,
		input.TxProcessor,
		input.HashManager,
		input.BlockHashClient,
		input.TransactionHashClient,
		utxoQuery,
		blockQuery,
		chainQuery,
		input.FeeManager,
		input.ConfigProvider,
		blockLogger,
	)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 创建 BlockValidator 服务
	blockValidator, err := validator.NewService(
		input.QueryService,
		input.HashManager,
		input.BlockHashClient,
		input.TransactionHashClient,
		input.TxVerifier,
		input.ConfigProvider,
		input.EventBus,
		blockLogger,
	)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 从 QueryService 获取 UTXOQuery（用于 BlockProcessor）
	var processorUTXOQuery persistence.UTXOQuery
	if input.QueryService != nil {
		processorUTXOQuery = input.QueryService
	}

	// 创建 BlockProcessor 服务
	blockProcessor, err := blockprocessor.NewService(
		input.DataWriter,
		input.TxProcessor,
		input.UTXOWriter,
		processorUTXOQuery,
		input.TxPool,
		input.HashManager,
		input.BlockHashClient,
		input.TransactionHashClient,
		input.ZKProofService, // ZK证明服务（可选，用于验证StateOutput的ZK证明）
		input.EventBus,
		blockLogger,
		input.WriteGate, // 全局写门闸（可选，用于只读模式和 REORG 写控制）
	)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 从 QueryService 获取 UTXOQuery（用于 GenesisBlockBuilder）
	var genesisUTXOQuery persistence.UTXOQuery
	if input.QueryService != nil {
		genesisUTXOQuery = input.QueryService
	}

	// 创建 GenesisBlockBuilder 服务
	genesisBuilder, err := genesis.NewService(
		input.TransactionHashClient,
		input.HashManager,
		genesisUTXOQuery,
		blockLogger,
	)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 类型断言为公共接口
	var publicBlockBuilder blockif.BlockBuilder = blockBuilder
	var publicBlockValidator blockif.BlockValidator = blockValidator
	var publicBlockProcessor blockif.BlockProcessor = blockProcessor
	var publicGenesisBuilder blockif.GenesisBlockBuilder = genesisBuilder

	// 注册 BlockBuilder 到内存监控系统
	if reporter, ok := blockBuilder.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if blockLogger != nil {
			blockLogger.Info("✅ BlockBuilder 已注册到内存监控系统")
		}
	}

	return ModuleOutput{
		BlockBuilder:           publicBlockBuilder,
		BlockValidator:         publicBlockValidator,
		BlockProcessor:         publicBlockProcessor,
		GenesisBuilder:         publicGenesisBuilder,
		InternalBlockBuilder:   blockBuilder,
		InternalBlockValidator: blockValidator,
		InternalBlockProcessor: blockProcessor,
		InternalGenesisBuilder: genesisBuilder,
	}, nil
}

func Module() fx.Option {
	return fx.Module("block",
		// ====================================================================
		//                           服务提供
		// ====================================================================

		fx.Provide(
			// 提供所有服务（通过 ModuleOutput 统一导出）
			// fx 会自动展开 ModuleOutput 结构体（因为它有 fx.Out）
			// 所有带 name tag 的字段会注册为命名依赖
			// 所有未命名的字段会注册为未命名依赖
			ProvideServices,
		),

		// ====================================================================
		//                           延迟依赖注入
		// ====================================================================

		// 🔥 注入 Validator 到 Processor（避免循环依赖）
		fx.Invoke(
			fx.Annotate(
				func(
					processor interfaces.InternalBlockProcessor,
					validator interfaces.InternalBlockValidator,
					logger log.Logger,
				) {
					// 🎯 为区块模块添加 module 字段
					var blockLogger log.Logger
					if logger != nil {
						blockLogger = logger.With("module", "block")
					}
					
					// 类型断言获取 blockprocessor.Service
					if procService, ok := processor.(*blockprocessor.Service); ok {
						procService.SetValidator(validator)
						if blockLogger != nil {
							blockLogger.Info("🔗 Validator 已注入到 Processor")
						}
					}
				},
				fx.ParamTags(
					`name:"block_processor"`, // interfaces.InternalBlockProcessor
					`name:"block_validator"`, // interfaces.InternalBlockValidator
					``,                       // log.Logger
				),
			),
		),

		// ====================================================================
		//                           事件集成
		// ====================================================================

		// 注册事件发布和订阅（可选）
		fx.Invoke(
			fx.Annotate(
				func(
					eventBus event.EventBus,
					logger log.Logger,
					processor interfaces.InternalBlockProcessor,
				) error {
				// 🎯 为区块模块添加 module 字段
				var blockLogger log.Logger
				if logger != nil {
					blockLogger = logger.With("module", "block")
				}
				
				if eventBus == nil {
					if blockLogger != nil {
						blockLogger.Warn("EventBus不可用，跳过block模块事件订阅")
					}
					return nil
				}

				// P3-2: 创建事件订阅注册器
				registry := eventintegration.NewEventSubscriptionRegistry(eventBus, blockLogger)

				// 注册所有事件订阅（目前Block模块不订阅任何事件，仅发布事件）
				if err := registry.RegisterEventSubscriptions(); err != nil {
					if blockLogger != nil {
						blockLogger.Errorf("注册Block模块事件订阅失败: %v", err)
					}
					return err
				}

				if blockLogger != nil {
					blockLogger.Info("✅ block模块事件集成已配置")
				}

				return nil
			},
				fx.ParamTags(
					``,                     // event.EventBus
					``,                     // log.Logger
					`name:"block_processor"`, // interfaces.InternalBlockProcessor
				),
			),
		),

		// ====================================================================
		//                           生命周期管理
		// ====================================================================

		fx.Invoke(
			func(lc fx.Lifecycle, logger log.Logger) {
				// 🎯 为区块模块添加 module 字段
				var blockLogger log.Logger
				if logger != nil {
					blockLogger = logger.With("module", "block")
				}
				
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if blockLogger != nil {
							blockLogger.Info("🚀 Block 模块启动")
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if blockLogger != nil {
							blockLogger.Info("🛑 Block 模块停止")
						}
						return nil
					},
				})
			},
		),

		// 模块加载日志
		fx.Invoke(
			func(logger log.Logger) {
				if logger != nil {
					// 🎯 为区块模块添加 module 字段
					blockLogger := logger.With("module", "block")
					blockLogger.Info("✅ Block 模块已加载 (Builder, Validator, Processor 可用)")
				}
			},
		),
	)
}

// ============================================================================
//                              模块元信息
// ============================================================================

// Version 模块版本
const Version = "1.0.0"

// Name 模块名称
const Name = "block"

// Description 模块描述
const Description = "区块管理模块，提供区块构建、验证和处理能力"
