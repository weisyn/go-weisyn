// 文件说明：
// 本文件定义内存池（mempool）组件的 Fx 模块装配入口，负责：
// 1) 通过依赖注入构造并输出 TxPool 与 CandidatePool 的实现；
// 2) 统一管理组件生命周期日志；
// 3) 装配事件集成（订阅和发布），实现"只收发事件"的边界。
//
// 设计约束：
// - 仅依赖公共接口（pkg/interfaces/*）与本组件实现；
// - 不引入网络集成（mempool 当前仅使用事件能力）。
// Package mempool provides memory pool functionality for transaction and candidate management.
package mempool

import (
	"context"
	"fmt"

	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"github.com/weisyn/v1/internal/core/mempool/candidatepool"
	candidatepooleventhandler "github.com/weisyn/v1/internal/core/mempool/candidatepool/event_handler"
	eventintegration "github.com/weisyn/v1/internal/core/mempool/integration/event"
	"github.com/weisyn/v1/internal/core/mempool/interfaces"
	"github.com/weisyn/v1/internal/core/mempool/txpool"
	txpooleventhandler "github.com/weisyn/v1/internal/core/mempool/txpool/event_handler"
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

// ModuleInput 定义内存池模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 依赖按功能分组：配置、基础设施、加密服务、合规服务、持久化存储。
type ModuleInput struct {
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

	// ========== P2-5: 持久化存储依赖（可选）==========
	PersistentStore storage.BadgerStore `optional:"true"` // BadgerDB存储（用于交易池状态持久化）

	// ========== 区块链域依赖 - 改为事件驱动 ==========
	// ChainState coreInterfaces.ChainState `optional:"false"` // 链状态服务（已移除，改用事件驱动）
}

// ModuleOutput 定义内存池模块的统一输出。
// 用于将 TxPool 与 CandidatePool 暴露给其他组件使用。

type ModuleOutput struct {
	fx.Out

	// 对外提供的标准接口服务（命名依赖）
	TxPool        mempoolIfaces.TxPool        `name:"tx_pool"`        // 交易池接口
	CandidatePool mempoolIfaces.CandidatePool `name:"candidate_pool"` // 候选区块池接口

	// 提供扩展的交易池接口，用于内部事件集成
	ExtendedTxPool txpool.ExtendedTxPool // 扩展交易池接口
}

// Module 返回统一的内存池模块。
// 负责：
// - 装配服务提供者（ProvideServices）；
// - 记录组件生命周期日志；
// - 连接事件订阅和发布（可选依赖）。
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
			// 🎯 为内存池模块添加 module 字段，日志将路由到 node-business.log
			var mempoolLogger log.Logger
			if logger != nil {
				mempoolLogger = logger.With("module", "mempool")
			}
			
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if mempoolLogger != nil {
						mempoolLogger.Info("🌊 内存池模块启动")
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if mempoolLogger != nil {
						mempoolLogger.Info("🌊 正在停止内存池服务...")
					}

					// 停止候选区块池（使用类型断言）
					if stoppable, ok := candidatePool.(interface{ Stop() error }); ok {
						if err := stoppable.Stop(); err != nil {
							if mempoolLogger != nil {
								mempoolLogger.Errorf("停止候选区块池失败: %v", err)
							}
							// 不返回错误，继续停止其他服务
						} else {
							if mempoolLogger != nil {
								mempoolLogger.Info("✅ 候选区块池已停止")
							}
						}
					}

					if mempoolLogger != nil {
						mempoolLogger.Info("🌊 内存池模块停止完成")
					}
					return nil
				},
			})
		}, fx.ParamTags(
			``,                      // fx.Lifecycle
			``,                      // log.Logger
			`name:"candidate_pool"`, // mempool.CandidatePool
		))),

		// 标准化事件集成：统一的事件订阅和处理
		fx.Invoke(fx.Annotate(
			func(
				logger log.Logger,
				eventBus event.EventBus,
				txPool mempoolIfaces.TxPool,
				candidatePool mempoolIfaces.CandidatePool,
				extendedTxPool txpool.ExtendedTxPool,
			) error {
				// 🎯 为内存池模块添加 module 字段
				var mempoolLogger log.Logger
				if logger != nil {
					mempoolLogger = logger.With("module", "mempool")
				}
				
				if eventBus == nil {
					if mempoolLogger != nil {
						mempoolLogger.Warn("EventBus未配置，跳过内存池事件集成")
					}
					return nil
				}

				// 设置事件发布下沉（出站事件）
				setupEventSinks(eventBus, mempoolLogger, extendedTxPool, candidatePool)

				// 创建事件处理器
				mempoolHandler, txPoolHandler, candidatePoolHandler := createMempoolEventHandlers(
					mempoolLogger, eventBus, txPool, candidatePool,
				)

				// 创建事件订阅注册器
				registry := eventintegration.NewEventSubscriptionRegistry(eventBus, mempoolLogger)

				// 注册所有事件订阅（入站事件）
				if err := registry.RegisterEventSubscriptions(
					mempoolHandler,
					txPoolHandler,
					candidatePoolHandler,
				); err != nil {
					if mempoolLogger != nil {
						mempoolLogger.Errorf("注册内存池事件订阅失败: %v", err)
					}
					return err
				}

				if mempoolLogger != nil {
					mempoolLogger.Info("✅ 内存池事件集成配置完成（订阅和发布）")
				}
				return nil
			},
			fx.ParamTags(
				``,                      // log.Logger
				``,                      // event.EventBus
				`name:"tx_pool"`,        // mempool.TxPool
				`name:"candidate_pool"`, // mempool.CandidatePool
				``,                      // txpool.ExtendedTxPool
			),
		)),
	)
}

// InternalServicesOutput 内部服务输出结构体
type InternalServicesOutput struct {
	fx.Out

	TxPool        interfaces.InternalTxPool        `name:"internal_tx_pool"`
	CandidatePool interfaces.InternalCandidatePool `name:"internal_candidate_pool"`
}

// mlProvideServices 是 ProvideServices 的轻量包装。
// 参数：无。
// 返回：fx.Option（提供构造函数）。
func mlProvideServices() fx.Option {
	return fx.Options(
		// 提供内部接口实例（通过 ProvideServicesInternal）
		fx.Provide(ProvideServicesInternal),
		// 绑定内部接口到公共接口（TxPool - 命名）
		fx.Provide(fx.Annotate(
			func(tx interfaces.InternalTxPool) mempoolIfaces.TxPool {
				return tx // 内部接口自动实现公共接口
			},
			fx.ParamTags(`name:"internal_tx_pool"`),
			fx.ResultTags(`name:"tx_pool"`),
		)),
		// 绑定内部接口到公共接口（CandidatePool）
		fx.Provide(fx.Annotate(
			func(cp interfaces.InternalCandidatePool) mempoolIfaces.CandidatePool {
				return cp // 内部接口自动实现公共接口
			},
			fx.ParamTags(`name:"internal_candidate_pool"`),
			fx.ResultTags(`name:"candidate_pool"`),
		)),
		// 提供 ExtendedTxPool（用于事件集成）
		fx.Provide(fx.Annotate(
			func(tx interfaces.InternalTxPool) txpool.ExtendedTxPool {
				// 类型断言为 ExtendedTxPool（内部扩展接口）
				if ext, ok := tx.(txpool.ExtendedTxPool); ok {
					return ext
				}
				return nil
			},
			fx.ParamTags(`name:"internal_tx_pool"`),
		)),
		// 提供候选区块池启动逻辑
		fx.Invoke(fx.Annotate(
			func(logger log.Logger, cp interfaces.InternalCandidatePool) error {
				// 🎯 为内存池模块添加 module 字段
				var mempoolLogger log.Logger
				if logger != nil {
					mempoolLogger = logger.With("module", "mempool")
				}
				
				// 启动候选区块池（使用类型断言调用具体实现的Start方法）
				if startable, ok := cp.(interface{ Start() error }); ok {
					if err := startable.Start(); err != nil {
						if mempoolLogger != nil {
							mempoolLogger.Errorf("启动候选区块池失败: %v", err)
						}
						return fmt.Errorf("启动候选区块池失败: %w", err)
					}
					if mempoolLogger != nil {
						mempoolLogger.Info("✅ 候选区块池已启动")
					}
				} else if mempoolLogger != nil {
					mempoolLogger.Info("候选区块池实现不支持Start方法")
				}
				return nil
			},

			fx.ParamTags(
				``,                               // log.Logger
				`name:"internal_candidate_pool"`, // mempool.CandidatePool
			),
		)),
		// 提供合规策略状态记录
		fx.Invoke(fx.Annotate(
			func(
				logger log.Logger,
				configProvider config.Provider,
				compliancePolicy complianceIfaces.Policy,
			) {
				// 🎯 为内存池模块添加 module 字段
				var mempoolLogger log.Logger
				if logger != nil {
					mempoolLogger = logger.With("module", "mempool")
				}
				
				if compliancePolicy != nil {
					complianceConfig := configProvider.GetCompliance()
					if complianceConfig != nil && complianceConfig.Enabled {
						if mempoolLogger != nil {
							mempoolLogger.Info("交易池已启用合规检查")
						}
					} else if mempoolLogger != nil {
						mempoolLogger.Info("合规策略可用但未启用")
					}
				} else if mempoolLogger != nil {
					mempoolLogger.Debug("未配置合规策略")
				}
			},
			fx.ParamTags(
				``, // log.Logger
				``, // config.Provider
				`name:"compliance_policy" optional:"true"`, // compliance.Policy（可选）
			),
		)),
	)
}

// ProvideServicesInternal 提供内部接口实例
// 参数：
// - input：ModuleInput（由 Fx 注入）。
// 返回：
// - InternalServicesOutput：包含内部接口实例的结构体
// - error：构造失败时返回错误。
func ProvideServicesInternal(input ModuleInput) (InternalServicesOutput, error) {
	// 🎯 为内存池模块添加 module 字段
	var mempoolLogger log.Logger
	if input.Logger != nil {
		mempoolLogger = input.Logger.With("module", "mempool")
	}
	
	// 验证必需的依赖
	if input.ConfigProvider == nil {
		return InternalServicesOutput{}, fmt.Errorf("配置提供者不能为空")
	}
	if input.TransactionHashServiceClient == nil {
		return InternalServicesOutput{}, fmt.Errorf("交易哈希服务不能为空")
	}
	if input.BlockHashServiceClient == nil {
		return InternalServicesOutput{}, fmt.Errorf("区块哈希服务不能为空")
	}

	// 获取配置
	txPoolOptions := input.ConfigProvider.GetTxPool()
	candidatePoolOptions := input.ConfigProvider.GetCandidatePool()
	if txPoolOptions == nil {
		return InternalServicesOutput{}, fmt.Errorf("交易池配置不能为空")
	}
	if candidatePoolOptions == nil {
		return InternalServicesOutput{}, fmt.Errorf("候选区块池配置不能为空")
	}

	// 创建交易池实例（返回内部接口类型）
	txPool, err := txpool.NewTxPoolWithCacheAndCompliance(
		txPoolOptions,
		mempoolLogger,
		input.EventBus,
		input.MemoryStore,
		input.TransactionHashServiceClient,
		nil,
		input.CompliancePolicy, // 注入合规策略
		input.PersistentStore,  // P2-5: 注入持久化存储（可选）
	)
	if err != nil {
		return InternalServicesOutput{}, fmt.Errorf("创建交易池失败: %w", err)
	}

	// 调试日志：记录 TxPool 实例指针，帮助对齐 API / Block / 共识使用的池
	if mempoolLogger != nil {
		mempoolLogger.Infof("🧩 [Fx] mempool.ProvideServicesInternal 创建 TxPool 实例: %p", txPool)
	}

	// 注册 TxPool 到内存监控系统
	if reporter, ok := txPool.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if mempoolLogger != nil {
			mempoolLogger.Info("✅ TxPool 已注册到内存监控系统")
		}
	} else if mempoolLogger != nil {
		mempoolLogger.Warn("⚠️  TxPool 未实现 MemoryReporter 接口")
	}

	// 创建候选区块池实例（返回内部接口类型）
	candidatePool, err := candidatepool.NewCandidatePoolWithCache(
		candidatePoolOptions,
		mempoolLogger,
		input.EventBus,
		input.MemoryStore,
		input.BlockHashServiceClient,
		nil,
	)
	if err != nil {
		return InternalServicesOutput{}, fmt.Errorf("创建候选区块池失败: %w", err)
	}

	return InternalServicesOutput{
		TxPool:        txPool,
		CandidatePool: candidatePool,
	}, nil
}

// ProvideServices 提供内存池服务，完成 TxPool 与 CandidatePool 的构造与返回。
// 参数：
// - input：ModuleInput（由 Fx 注入）。
// 返回：
// - ModuleOutput：包含 TxPool 与 CandidatePool；
// - error：构造失败时返回错误。
// 错误场景：
// - 缺失配置或加密服务客户端；
// - 具体实例创建失败（例如参数非法）。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 验证必需的依赖
	if input.ConfigProvider == nil {
		return ModuleOutput{}, fmt.Errorf("配置提供者不能为空")
	}
	if input.TransactionHashServiceClient == nil {
		return ModuleOutput{}, fmt.Errorf("交易哈希服务不能为空")
	}
	if input.BlockHashServiceClient == nil {
		return ModuleOutput{}, fmt.Errorf("区块哈希服务不能为空")
	}

	// 获取配置 - 配置提供者已经返回了完整的配置选项
	txPoolOptions := input.ConfigProvider.GetTxPool()
	candidatePoolOptions := input.ConfigProvider.GetCandidatePool()
	if txPoolOptions == nil {
		return ModuleOutput{}, fmt.Errorf("交易池配置不能为空")
	}
	if candidatePoolOptions == nil {
		return ModuleOutput{}, fmt.Errorf("候选区块池配置不能为空")
	}

	// 直接创建交易池实例（集成合规策略和持久化存储）
	txPool, err := txpool.NewTxPoolWithCacheAndCompliance(
		txPoolOptions,
		input.Logger,
		input.EventBus,
		input.MemoryStore,
		input.TransactionHashServiceClient,
		nil,
		input.CompliancePolicy, // 注入合规策略
		input.PersistentStore,  // P2-5: 注入持久化存储（可选）
	)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建交易池失败: %w", err)
	}

	// 记录合规策略状态
	if input.CompliancePolicy != nil {
		complianceConfig := input.ConfigProvider.GetCompliance()
		if complianceConfig != nil && complianceConfig.Enabled {
			if input.Logger != nil {
				input.Logger.Info("交易池已启用合规检查")
			}
		} else if input.Logger != nil {
			input.Logger.Info("合规策略可用但未启用")
		}
	} else if input.Logger != nil {
		input.Logger.Debug("未配置合规策略")
	}

	// 直接创建候选区块池实例（不使用链状态缓存，避免外部依赖）
	candidatePool, err := candidatepool.NewCandidatePoolWithCache(
		candidatePoolOptions,
		input.Logger,
		input.EventBus,
		input.MemoryStore,
		input.BlockHashServiceClient,
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

		if input.Logger != nil {
			input.Logger.Info("✅ 候选区块池已启动")
		}
	} else if input.Logger != nil {
		input.Logger.Info("候选区块池实现不支持Start方法")
	}

	// 将具体类型转换为接口类型
	extendedTxPool, ok := txPool.(txpool.ExtendedTxPool)
	if !ok {
		return ModuleOutput{}, fmt.Errorf("TxPool实现不符合ExtendedTxPool接口")
	}

	return ModuleOutput{
		TxPool:         txPool, // 命名依赖
		CandidatePool:  candidatePool,
		ExtendedTxPool: extendedTxPool,
	}, nil
}

// createMempoolEventHandlers 创建所有内存池事件处理器
//
// 🎯 **统一创建入口**：
// 创建并返回所有内存池相关的事件处理器实例
//
// 参数：
// - logger：日志接口
// - eventBus：事件总线接口
// - txPool：交易池接口
// - candidatePool：候选区块池接口
//
// 返回：
// - MempoolEventSubscriber：内存池通用事件处理器
// - TxPoolEventSubscriber：交易池事件处理器
// - CandidatePoolEventSubscriber：候选区块池事件处理器
func createMempoolEventHandlers(
	logger log.Logger,
	eventBus event.EventBus,
	txPool mempoolIfaces.TxPool,
	candidatePool mempoolIfaces.CandidatePool,
) (
	eventintegration.MempoolEventSubscriber,
	eventintegration.TxPoolEventSubscriber,
	eventintegration.CandidatePoolEventSubscriber,
) {
	// 创建各个事件处理器
	mempoolHandler := eventintegration.NewMempoolEventHandler(logger, eventBus, txPool, candidatePool)
	txPoolHandler := txpooleventhandler.NewTxPoolEventHandler(logger, eventBus, txPool)
	candidatePoolHandler := candidatepooleventhandler.NewCandidatePoolEventHandler(logger, eventBus, candidatePool)

	return mempoolHandler, txPoolHandler, candidatePoolHandler
}

// setupEventSinks 设置所有事件发布下沉。
// 将事件发布实现注入到 TxPool 和 CandidatePool 中，使它们能够发布事件到事件总线。
//
// 参数：
// - eventBus：事件总线接口（可选，nil 时事件发布将被禁用）
// - logger：日志接口（可选）
// - extendedTxPool：扩展的交易池接口
// - candidatePool：候选区块池接口
//
// 说明：
// - 如果 eventBus 为 nil，事件发布将被禁用（各池会使用 Noop 实现）
// - 使用类型断言确保类型安全
func setupEventSinks(
	eventBus event.EventBus,
	logger log.Logger,
	extendedTxPool txpool.ExtendedTxPool,
	candidatePool mempoolIfaces.CandidatePool,
) {
	// 设置交易池事件下沉
	txpooleventhandler.SetupTxPoolEventSink(eventBus, logger, extendedTxPool)

	// 设置候选区块池事件下沉
	candidatepooleventhandler.SetupCandidatePoolEventSink(eventBus, logger, candidatePool)
}
