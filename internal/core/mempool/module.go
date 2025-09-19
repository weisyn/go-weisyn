// 文件说明：
// 本文件定义内存池（mempool）组件的 Fx 模块装配入口，负责：
// 1) 通过依赖注入构造并输出 TxPool 与 CandidatePool 的实现；
// 2) 统一管理组件生命周期日志；
// 3) 装配事件集成（incoming/outgoing），实现“只收发事件”的边界。
//
// 设计约束：
// - 仅依赖公共接口（pkg/interfaces/*）与本组件实现；
// - 不引入网络集成（mempool 当前仅使用事件能力）。
package mempool

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/mempool/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/event_handler"
	eventintegration "github.com/weisyn/v1/internal/core/mempool/integration/event"
	"github.com/weisyn/v1/internal/core/mempool/txpool"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"go.uber.org/fx"
)

// ModuleParams 定义内存池模块的统一依赖参数。
// 参数说明：
// - ConfigProvider：配置提供者，负责提供 TxPool/CandidatePool 的配置；
// - Logger：日志接口，可选；
// - EventBus：事件总线接口，可选；
// - MemoryStore：内存存储接口，可选；
// - TransactionHashServiceClient：交易哈希 gRPC 客户端；
// - BlockHashServiceClient：区块哈希 gRPC 客户端。
// 备注：不引入链状态或网络依赖，遵循“仅事件”的组件边界。
// 返回值：无（由 Fx 负责解包注入）。
// 错误：无（Fx 构造阶段如需校验在 ProvideServices 内完成）。
//
// ModuleOutput 定义模块对外输出的接口聚合。
// 字段说明：
// - TxPool：交易内存池接口实例；
// - CandidatePool：候选区块内存池接口实例。
//
// Module 返回 mempool 组件的 Fx 装配入口。
// 参数：无。
// 返回：fx.Option 用于被上层应用集成。
// 副作用：注册生命周期日志与事件接线（incoming/outgoing）。

type ModuleParams struct {
	fx.In

	// ========== 配置依赖 ==========
	ConfigProvider config.Provider `optional:"false"` // 配置提供者

	// ========== 基础设施依赖 ==========
	Logger      log.Logger          `optional:"true"` // 日志记录器
	EventBus    event.EventBus      `optional:"true"` // 事件总线
	MemoryStore storage.MemoryStore `optional:"true"` // 内存存储

	// ========== 加密服务依赖 ==========
	TransactionHashServiceClient transaction.TransactionHashServiceClient `optional:"false"` // 交易哈希服务
	BlockHashServiceClient       core.BlockHashServiceClient              `optional:"false"` // 区块哈希服务

	// ========== 合规服务依赖（可选）==========
	CompliancePolicy complianceIfaces.Policy `name:"compliance_policy" optional:"true"` // 合规策略服务

	// ========== 区块链域依赖 - 改为事件驱动 ==========
	// ChainState coreInterfaces.ChainState `optional:"false"` // 链状态服务（已移除，改用事件驱动）
}

// ModuleOutput 定义内存池模块的统一输出。
// 用于将 TxPool 与 CandidatePool 暴露给其他组件使用。

type ModuleOutput struct {
	fx.Out

	// 对外提供的标准接口服务
	TxPool        mempoolIfaces.TxPool        `name:"tx_pool"`        // 交易池接口
	CandidatePool mempoolIfaces.CandidatePool `name:"candidate_pool"` // 候选区块池接口

	// 提供扩展的交易池接口，用于内部事件集成
	ExtendedTxPool txpool.ExtendedTxPool // 扩展交易池接口
}

// Module 返回统一的内存池模块。
// 负责：
// - 装配服务提供者（ProvideServices）；
// - 记录组件生命周期日志；
// - 连接事件 incoming/outgoing（可选依赖）。
func Module() fx.Option {
	return fx.Module("mempool",
		// 提供统一的内存池服务
		mlProvideServices(),

		// 添加候选区块池的停止生命周期管理
		fx.Invoke(fx.Annotate(func(
			lc fx.Lifecycle,
			logger log.Logger,
			candidatePool mempoolIfaces.CandidatePool,
		) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					logger.Info("🌊 内存池模块启动")
					return nil
				},
				OnStop: func(ctx context.Context) error {
					logger.Info("🌊 正在停止内存池服务...")

					// 停止候选区块池（使用类型断言）
					if stoppable, ok := candidatePool.(interface{ Stop() error }); ok {
						if err := stoppable.Stop(); err != nil {
							logger.Errorf("停止候选区块池失败: %v", err)
							// 不返回错误，继续停止其他服务
						} else {
							logger.Info("✅ 候选区块池已停止")
						}
					}

					logger.Info("🌊 内存池模块停止完成")
					return nil
				},
			})
		}, fx.ParamTags(``, ``, `name:"candidate_pool"`))),

		// 标准化事件集成：统一的事件订阅和处理
		fx.Invoke(fx.Annotate(func(
			logger log.Logger,
			eventBus event.EventBus,
			txPool mempoolIfaces.TxPool,
			candidatePool mempoolIfaces.CandidatePool,
		) error {
			if eventBus == nil {
				logger.Warn("EventBus未配置，跳过内存池事件集成")
				return nil
			}

			// 创建事件处理器
			mempoolHandler, txPoolHandler, candidatePoolHandler := event_handler.CreateMempoolEventHandlers(
				logger, eventBus, txPool, candidatePool,
			)

			// 创建事件订阅注册器
			registry := eventintegration.NewEventSubscriptionRegistry(eventBus, logger)

			// 注册所有事件订阅
			if err := registry.RegisterEventSubscriptions(
				mempoolHandler,
				txPoolHandler,
				candidatePoolHandler,
			); err != nil {
				logger.Errorf("注册内存池事件订阅失败: %v", err)
				return err
			}

			logger.Info("内存池事件集成配置完成")
			return nil
		}, fx.ParamTags(``, ``, `name:"tx_pool"`, `name:"candidate_pool"`))),
	)
}

// mlProvideServices 是 ProvideServices 的轻量包装。
// 参数：无。
// 返回：fx.Option（提供构造函数）。
func mlProvideServices() fx.Option {
	return fx.Provide(ProvideServices)
}

// ProvideServices 提供内存池服务，完成 TxPool 与 CandidatePool 的构造与返回。
// 参数：
// - params：ModuleParams（由 Fx 注入）。
// 返回：
// - ModuleOutput：包含 TxPool 与 CandidatePool；
// - error：构造失败时返回错误。
// 错误场景：
// - 缺失配置或加密服务客户端；
// - 具体实例创建失败（例如参数非法）。
func ProvideServices(params ModuleParams) (ModuleOutput, error) {
	// 验证必需的依赖
	if params.ConfigProvider == nil {
		return ModuleOutput{}, fmt.Errorf("配置提供者不能为空")
	}
	if params.TransactionHashServiceClient == nil {
		return ModuleOutput{}, fmt.Errorf("交易哈希服务不能为空")
	}
	if params.BlockHashServiceClient == nil {
		return ModuleOutput{}, fmt.Errorf("区块哈希服务不能为空")
	}

	// 获取配置 - 配置提供者已经返回了完整的配置选项
	txPoolOptions := params.ConfigProvider.GetTxPool()
	candidatePoolOptions := params.ConfigProvider.GetCandidatePool()
	if txPoolOptions == nil {
		return ModuleOutput{}, fmt.Errorf("交易池配置不能为空")
	}
	if candidatePoolOptions == nil {
		return ModuleOutput{}, fmt.Errorf("候选区块池配置不能为空")
	}

	// 直接创建交易池实例（集成合规策略）
	txPool, err := txpool.NewTxPoolWithCacheAndCompliance(
		txPoolOptions,
		params.Logger,
		params.EventBus,
		params.MemoryStore,
		params.TransactionHashServiceClient,
		nil,
		params.CompliancePolicy, // 注入合规策略
	)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建交易池失败: %w", err)
	}

	// 记录合规策略状态
	if params.CompliancePolicy != nil {
		complianceConfig := params.ConfigProvider.GetCompliance()
		if complianceConfig != nil && complianceConfig.Enabled {
			if params.Logger != nil {
				params.Logger.Info("交易池已启用合规检查")
			}
		} else if params.Logger != nil {
			params.Logger.Info("合规策略可用但未启用")
		}
	} else if params.Logger != nil {
		params.Logger.Debug("未配置合规策略")
	}

	// 直接创建候选区块池实例（不使用链状态缓存，避免外部依赖）
	candidatePool, err := candidatepool.NewCandidatePoolWithCache(
		candidatePoolOptions,
		params.Logger,
		params.EventBus,
		params.MemoryStore,
		params.BlockHashServiceClient,
		nil,
	)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建候选区块池失败: %w", err)
	}

	// 🔧 修复：启动候选区块池（使用类型断言调用具体实现的Start方法）
	if startable, ok := candidatePool.(interface{ Start() error }); ok {
		if err := startable.Start(); err != nil {
			return ModuleOutput{}, fmt.Errorf("启动候选区块池失败: %w", err)
		}

		if params.Logger != nil {
			params.Logger.Info("✅ 候选区块池已启动")
		}
	} else if params.Logger != nil {
		params.Logger.Info("候选区块池实现不支持Start方法")
	}

	// 将具体类型转换为接口类型
	extendedTxPool, ok := txPool.(txpool.ExtendedTxPool)
	if !ok {
		return ModuleOutput{}, fmt.Errorf("TxPool实现不符合ExtendedTxPool接口")
	}

	return ModuleOutput{
		TxPool:         txPool,
		CandidatePool:  candidatePool,
		ExtendedTxPool: extendedTxPool,
	}, nil
}
