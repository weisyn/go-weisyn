// Package eutxo 提供 EUTXO 模块的 fx 配置
//
// 🎯 **模块职责**：
// - 提供 UTXOWriter 服务
// - 提供 UTXOSnapshot 服务
// - 管理生命周期
// - 处理延迟依赖注入
//
// 📋 **导出服务**：
// - eutxo.UTXOWriter (公共接口)
// - eutxo.UTXOSnapshot (公共接口)
// - interfaces.InternalUTXOWriter (内部接口)
// - interfaces.InternalUTXOSnapshot (内部接口)
package eutxo

import (
	"context"

	"go.uber.org/fx"

	// 公共接口
	eutxoif "github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	chainif "github.com/weisyn/v1/pkg/interfaces/persistence"
	core "github.com/weisyn/v1/pb/blockchain/block"

	// 内部实现
	"github.com/weisyn/v1/internal/core/eutxo/health"
	"github.com/weisyn/v1/internal/core/eutxo/interfaces"
	eutxoquery "github.com/weisyn/v1/internal/core/eutxo/query"
	"github.com/weisyn/v1/internal/core/eutxo/snapshot"
	"github.com/weisyn/v1/internal/core/eutxo/writer"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 eutxo 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger log.Logger `optional:"true"` // 日志记录器

	// ========== 存储组件 ==========
	BadgerStore storage.BadgerStore `optional:"false"` // BadgerDB存储

	// ========== 密码学组件 ==========
	HashManager crypto.HashManager `optional:"false"` // 哈希管理器

	// ========== 哈希服务客户端 ==========
	BlockHashClient core.BlockHashServiceClient `optional:"false"` // 区块哈希服务客户端

	// ========== 事件总线 ==========
	EventBus event.EventBus `optional:"true"` // 事件总线（可选）

	// ========== 链查询服务 ==========
	ChainQuery chainif.ChainQuery `optional:"true"` // 链查询服务（用于健康检查）
}

// ============================================================================
//                              模块输出服务
// ============================================================================

// ModuleOutput 定义 eutxo 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	UTXOWriter        eutxoif.UTXOWriter        `name:"utxo_writer"`        // UTXO写入器
	UTXOSnapshot      eutxoif.UTXOSnapshot      `name:"utxo_snapshot"`      // UTXO快照服务
	ResourceUTXOQuery eutxoif.ResourceUTXOQuery `name:"resource_utxo_query"` // 资源UTXO查询服务（公共接口）

	// 内部接口导出（命名，供延迟注入使用）
	InternalUTXOWriter        interfaces.InternalUTXOWriter        `name:"utxo_writer"`        // 内部UTXO写入器（命名，供延迟注入使用）
	InternalUTXOSnapshot      interfaces.InternalUTXOSnapshot      `name:"utxo_snapshot"`      // 内部UTXO快照服务（命名，供延迟注入使用）
	InternalUTXOQuery         interfaces.InternalUTXOQuery         `name:"utxo_query"`         // 内部UTXO查询服务（命名，供延迟注入使用）
	InternalResourceUTXOQuery interfaces.InternalResourceUTXOQuery `name:"resource_utxo_query"` // 内部资源UTXO查询服务（命名，供 ResourceViewService 使用）
}

// ============================================================================
//                              模块定义
// ============================================================================

// ProvideServices 提供 eutxo 模块的所有服务
//
// 🎯 **服务创建**：
// 本函数负责创建 eutxo 模块的所有服务实例，并通过 ModuleOutput 统一导出。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 🎯 为 EUTXO 模块添加 module 字段，日志将路由到 node-business.log
	var eutxoLogger log.Logger
	if input.Logger != nil {
		eutxoLogger = input.Logger.With("module", "eutxo")
	}
	
	// 创建 UTXOWriter 服务
	utxoWriter, err := writer.NewService(input.BadgerStore, input.HashManager, input.EventBus, eutxoLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 创建 UTXOSnapshot 服务
	utxoSnapshot, err := snapshot.NewService(input.BadgerStore, input.HashManager, input.BlockHashClient, eutxoLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 创建 UTXOQuery 服务（内部使用）
	utxoQuery, err := eutxoquery.NewService(input.BadgerStore, eutxoLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 创建 ResourceUTXOQuery 服务（新增）
	resourceUTXOQuery, err := eutxoquery.NewResourceService(input.BadgerStore, eutxoLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// ✅ 启动时健康检查与自动修复
	if input.ChainQuery != nil {
		healthChecker := health.NewHealthChecker(
			input.BadgerStore,
			input.ChainQuery,
			eutxoLogger,
		)

		// 执行健康检查（自动修复模式）
		if eutxoLogger != nil {
			eutxoLogger.Info("🔍 开始UTXO集健康检查...")
		}

		report, err := healthChecker.PerformCheck(context.Background(), true)
		if err != nil {
			if eutxoLogger != nil {
				eutxoLogger.Errorf("健康检查失败: %v", err)
			}
			// 不阻断启动，仅记录错误
		} else {
			if eutxoLogger != nil {
				eutxoLogger.Infof("✅ 健康检查完成: 总=%d, 损坏=%d, 已修复=%d, 无法修复=%d",
					report.TotalUTXOs, report.CorruptUTXOs, report.RepairedUTXOs, report.UnrepairableUTXOs)
			}

			if report.UnrepairableUTXOs > 0 && eutxoLogger != nil {
				eutxoLogger.Warnf("⚠️ 存在 %d 个无法自动修复的UTXO，建议人工检查", report.UnrepairableUTXOs)
			}

			// 更新监控指标（如果已注册）
			if report.CorruptUTXOs > 0 {
				UpdateMetrics(report)
			}
		}
	} else if eutxoLogger != nil {
		eutxoLogger.Warn("⚠️ ChainQuery未提供，跳过UTXO集健康检查")
	}

	// 类型断言为公共接口
	var publicUTXOWriter eutxoif.UTXOWriter = utxoWriter
	var publicUTXOSnapshot eutxoif.UTXOSnapshot = utxoSnapshot
	var publicResourceUTXOQuery eutxoif.ResourceUTXOQuery = resourceUTXOQuery

	// 注册 EUTXO UTXOWriter 到内存监控系统
	if reporter, ok := utxoWriter.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if eutxoLogger != nil {
			eutxoLogger.Info("✅ EUTXO UTXOWriter 已注册到内存监控系统")
		}
	}

	return ModuleOutput{
		UTXOWriter:              publicUTXOWriter,
		UTXOSnapshot:            publicUTXOSnapshot,
		ResourceUTXOQuery:       publicResourceUTXOQuery,
		InternalUTXOWriter:      utxoWriter,
		InternalUTXOSnapshot:    utxoSnapshot,
		InternalUTXOQuery:       utxoQuery,
		InternalResourceUTXOQuery: resourceUTXOQuery,
	}, nil
}

// Module 返回 eutxo 模块的 fx 配置
//
// 🎯 **模块职责**：
// - 提供 UTXOWriter 服务 ✅
// - 提供 UTXOSnapshot 服务 ✅
// - 管理生命周期 ✅
// - 处理延迟依赖注入 ✅
//
// 🔗 **依赖关系**：
// - 输入：Storage, HashManager, EventBus（可选）, Logger
// - 输出：UTXOWriter, UTXOSnapshot
//
// 📋 **导出服务**：
// - eutxoif.UTXOWriter (name: "utxo_writer") ✅
// - eutxoif.UTXOSnapshot (name: "utxo_snapshot") ✅
// - interfaces.InternalUTXOWriter (内部使用) ✅
// - interfaces.InternalUTXOSnapshot (内部使用) ✅
func Module() fx.Option {
	return fx.Module("eutxo",
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
		//                        延迟依赖注入
		// ====================================================================

		// 🔥 注入 Writer 和 Query 到 Snapshot
		// ⚠️ **架构修复**：移除了 BlockQuery 依赖，EUTXO 模块不应依赖 persistence 模块
		fx.Invoke(
			fx.Annotate(
				func(
					utxoSnapshot interfaces.InternalUTXOSnapshot,
					utxoWriter interfaces.InternalUTXOWriter,
					utxoQuery interfaces.InternalUTXOQuery,
					logger log.Logger,
				) {
					// 🎯 为 EUTXO 模块添加 module 字段
					var eutxoLogger log.Logger
					if logger != nil {
						eutxoLogger = logger.With("module", "eutxo")
					}
					
					// 注入 Writer 到 Snapshot（用于快照恢复）
					utxoSnapshot.SetWriter(utxoWriter)

					// 注入 Query 到 Snapshot（用于快照创建）
					utxoSnapshot.SetQuery(utxoQuery)

					if eutxoLogger != nil {
						eutxoLogger.Info("🔗 UTXOWriter 已注入到 UTXOSnapshot")
						eutxoLogger.Info("🔗 UTXOQuery 已注入到 UTXOSnapshot")
					}
				},
				// ✅ 修复：使用参数标签指定依赖来源
				fx.ParamTags(
					`name:"utxo_snapshot"`, // InternalUTXOSnapshot（从本模块提供）
					`name:"utxo_writer"`,  // InternalUTXOWriter（从本模块提供）
					`name:"utxo_query"`,   // InternalUTXOQuery（从本模块提供）
					``,                    // logger（可选）
				),
			),
		),

		// ====================================================================
		//                         生命周期管理
		// ====================================================================

		fx.Invoke(
			func(lc fx.Lifecycle, logger log.Logger) {
				// 🎯 为 EUTXO 模块添加 module 字段
				var eutxoLogger log.Logger
				if logger != nil {
					eutxoLogger = logger.With("module", "eutxo")
				}
				
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if eutxoLogger != nil {
							eutxoLogger.Info("🚀 EUTXO 模块启动")
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if eutxoLogger != nil {
							eutxoLogger.Info("🛑 EUTXO 模块停止")
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
					// 🎯 为 EUTXO 模块添加 module 字段
					eutxoLogger := logger.With("module", "eutxo")
					eutxoLogger.Info("✅ EUTXO 模块已加载 (Writer, Snapshot, Query)")
				}
			},
		),
	)
}
