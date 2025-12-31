// Package chain 提供链状态管理的核心实现
//
// 🔗 **Chain 模块 (Chain Module)**
//
// 本包实现了链状态管理的核心功能，包括：
// - 分叉处理（ForkHandler）
// - 同步服务（SystemSyncService）
// - 事件集成（Event Integration）✅
// - 生命周期管理
//
// 🏗️ **模块架构**：
// - 使用 fx 依赖注入框架
// - 遵循 CQRS 架构原则（只读）
// - 支持事件驱动通信
// - 提供完整的生命周期管理
//
// ⚠️ **CHAIN模块完全只读**：
// - 链状态查询通过 persistence.QueryService
// - 同步状态实时计算，不持久化
// - 区块写入通过 BLOCK 模块的单一入口
//
// 📦 **导出服务**：
// - chain.ForkHandler: 分叉处理接口 ✅
// - chain.SystemSyncService: 同步服务接口 ✅
package chain

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/fx"

	// 公共接口
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	blockif "github.com/weisyn/v1/pkg/interfaces/block"
	chainif "github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	mempoolif "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"

	// 内部实现
	configimpl "github.com/weisyn/v1/internal/config"
	confignode "github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/internal/core/chain/fork"
	"github.com/weisyn/v1/internal/core/chain/gc"
	eventIntegration "github.com/weisyn/v1/internal/core/chain/integration/event"
	networkIntegration "github.com/weisyn/v1/internal/core/chain/integration/network"
	"github.com/weisyn/v1/internal/core/chain/interfaces"
	"github.com/weisyn/v1/internal/core/chain/recovery"
	"github.com/weisyn/v1/internal/core/chain/startup"
	"github.com/weisyn/v1/internal/core/chain/sync"
	"github.com/weisyn/v1/internal/core/diagnostics"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/persistence/repair"
	"github.com/weisyn/v1/pkg/interfaces/config"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 chain 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 依赖按功能分组：基础设施、存储、密码学、数据层、外部服务。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger         log.Logger      `optional:"true"`  // 日志记录器
	ConfigProvider config.Provider `optional:"false"` // 配置提供者

	// ========== 存储组件 ==========
	TempStore   storage.TempStore   `optional:"true"` // 临时存储服务
	BadgerStore storage.BadgerStore `optional:"true"` // ✅ 用于 fork/reorg 的状态清理（可选但强烈建议注入）
	FileStore   storage.FileStore   `optional:"true"` // 文件存储服务（用于 BlockFileGC）

	// ========== 密码学组件 ==========
	HashManager crypto.HashManager `optional:"false"` // 哈希管理器

	// ========== 哈希服务客户端 ==========
	BlockHashClient core.BlockHashServiceClient `optional:"false"` // 区块哈希服务客户端
	TxHashClient    txpb.TransactionHashServiceClient `optional:"false"` // 交易哈希服务客户端（用于交易索引、回滚清理）

	// ========== 数据层依赖 ==========
	QueryService persistence.QueryService `optional:"false" name:"query_service"` // 统一查询服务

	// ========== 区块链域依赖 ==========
	BlockValidator blockif.BlockValidator `optional:"false" name:"block_validator"` // 区块验证器
	BlockProcessor blockif.BlockProcessor `optional:"false" name:"block_processor"` // 区块处理器

	// ========== 网络组件 ==========
	NetworkService      network.Network              `optional:"true" name:"network_service"`       // 网络服务
	RoutingTableManager kademlia.RoutingTableManager `optional:"true" name:"routing_table_manager"` // 路由表管理器
	P2PService          p2pi.Service                 `optional:"true" name:"p2p_service"`           // P2P服务

	// ========== EUTXO 域依赖 ==========
	UTXOSnapshot eutxo.UTXOSnapshot `optional:"true" name:"utxo_snapshot"` // UTXO快照服务（可选）

	// ========== 数据写入服务 ==========
	DataWriter persistence.DataWriter `optional:"true" name:"data_writer"` // 数据写入服务（可选）

	// ========== 事件总线 ==========
	EventBus event.EventBus `optional:"true"` // 事件总线（可选）

	// ========== 节点运行时状态 ==========
	// NodeRuntimeState 从 P2P 模块获取（由 P2P 模块管理）
	NodeRuntimeState p2pi.RuntimeState `optional:"false" name:"node_runtime_state"`
}

// ============================================================================
//                              模块输出服务
// ============================================================================

// ModuleOutput 定义 chain 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	ForkHandler       chainif.ForkHandler       `name:"fork_handler"` // 分叉处理器
	SystemSyncService chainif.SystemSyncService `name:"sync_service"` // 系统同步服务

	// 内部接口导出（命名，供延迟注入使用）
	InternalForkHandler       interfaces.InternalForkHandler `name:"fork_handler"` // 内部分叉处理器（命名，供延迟注入使用）
	InternalSystemSyncService interfaces.InternalSyncService `name:"sync_service"` // 内部系统同步服务（命名，供延迟注入使用）

	// 注意：NodeRuntimeState 不再由 chain 模块导出，而是从 P2P 模块获取
}

// startRuntimeMonitors 启动一个简单的运行时监控协程，周期性输出内存与 goroutine 数量。
// 说明：
// - 仅用于运行时观测和现场排障，不参与任何共识逻辑；
// - 日志频率默认为 30 秒一次，开销很小；
// - 当 goroutine 数量超过阈值（默认 1000）时，输出 WARN 级别日志；
// - 当 ctx 结束时自动退出。
func startRuntimeMonitors(ctx context.Context, logger log.Logger) {
	if logger == nil {
		return
	}

	const goroutineWarningThreshold = 1000 // goroutine 警告阈值
	const goroutineGrowthThreshold = 100   // 🆕 goroutine 增长警告阈值（30秒内）

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// 🆕 跟踪上一次的goroutine数量，用于检测增长趋势
		lastNumGoroutines := runtime.NumGoroutine()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				numG := runtime.NumGoroutine()

				// 🆕 计算goroutine增长量
				growth := numG - lastNumGoroutines

				// 🆕 检测快速增长（可能表明goroutine泄漏）
				if growth > goroutineGrowthThreshold {
					logger.Warnf("[RuntimeMonitor] ⚠️ Goroutine快速增长检测: "+
						"增长=%d, 当前=%d, 上次=%d, heap_alloc=%dMB heap_objects=%d",
						growth, numG, lastNumGoroutines, m.Alloc/1024/1024, m.HeapObjects)
				}

				// 🔧 扩展监控：提供更详细的内存分析
				heapAllocMB := m.Alloc / 1024 / 1024           // 当前堆分配（实际使用）
				heapSysMB := m.HeapSys / 1024 / 1024           // 从OS获取的堆内存
				heapIdleMB := m.HeapIdle / 1024 / 1024         // 空闲但未释放的堆内存
				heapInuseMB := m.HeapInuse / 1024 / 1024       // 正在使用的堆内存
				totalAllocMB := m.TotalAlloc / 1024 / 1024     // 累计分配（仅供参考）
				sysMB := m.Sys / 1024 / 1024                   // 从OS获取的总内存
				rssBytes := getRSSBytesForRuntimeMonitor()
				rssMB := rssBytes / 1024 / 1024

				// 如果 goroutine 数量超过阈值，输出 WARN 级别日志
				if numG > goroutineWarningThreshold {
					logger.Warnf("[RuntimeMonitor] ⚠️  Goroutine 数量异常: "+
						"heap_alloc=%dMB heap_sys=%dMB heap_idle=%dMB heap_inuse=%dMB rss=%dMB "+
						"total_alloc=%dMB sys=%dMB heap_objects=%d goroutines=%d gc_count=%d (阈值=%d)",
						heapAllocMB, heapSysMB, heapIdleMB, heapInuseMB,
						rssMB, totalAllocMB, sysMB, m.HeapObjects, numG, m.NumGC, goroutineWarningThreshold)
				} else {
					logger.Infof("[RuntimeMonitor] "+
						"heap_alloc=%dMB heap_sys=%dMB heap_idle=%dMB rss=%dMB "+
						"sys=%dMB heap_objects=%d goroutines=%d gc_count=%d",
						heapAllocMB, heapSysMB, heapIdleMB,
						rssMB, sysMB, m.HeapObjects, numG, m.NumGC)
				}
				
				// 🚨 诊断：基于 RSS（物理内存）判断内存压力，而非 heap_alloc（虚拟内存）
				//
				// 🆕 2025-12-18 修复：
				// - heap_alloc 包含了 BadgerDB mmap 的虚拟地址空间（可达 100GB+），不应作为告警依据
				// - BadgerDB 使用 mmap 将 value log 文件映射到虚拟地址空间，但物理内存（RSS）只在实际访问时才分配
				// - 因此，只关注 RSS（物理内存）才能准确反映真实内存压力
				//
				// 告警规则：
				// - RSS > 4GB: ERROR 级别，表示物理内存压力大，需要立即排查
				// - RSS > 2GB: WARN 级别，表示物理内存偏高，建议关注
				// - heap_alloc vs RSS 比例过大（>50x）: DEBUG 级别，仅记录（正常现象，BadgerDB mmap 导致）
				if rssMB > 4096 {
					// 物理内存 > 4GB，严重告警
					logger.Errorf("[RuntimeMonitor] 🔴 高内存压力警告(RSS): "+
						"rss=%dMB (>4GB) heap_alloc=%dMB heap_sys=%dMB heap_idle=%dMB heap_inuse=%dMB "+
						"建议: 立即抓取 /debug/pprof/heap 并分析，或先尝试 /debug/memory/force-gc",
						rssMB, heapAllocMB, heapSysMB, heapIdleMB, heapInuseMB)
				} else if rssMB > 2048 {
					// 物理内存 > 2GB，警告
					logger.Warnf("[RuntimeMonitor] 🟠 内存压力偏高(RSS): "+
						"rss=%dMB (>2GB) heap_alloc=%dMB heap_sys=%dMB heap_idle=%dMB heap_inuse=%dMB",
						rssMB, heapAllocMB, heapSysMB, heapIdleMB, heapInuseMB)
				} else if heapAllocMB > rssMB*50 {
					// heap_alloc 虚高但 RSS 正常：BadgerDB mmap 导致，仅 DEBUG 记录
					logger.Debugf("[RuntimeMonitor] ℹ️  检测到 mmap 虚拟地址占用（BadgerDB value log）: "+
						"heap_alloc=%dMB RSS=%dMB 比例=%dx（正常现象，无需告警）",
						heapAllocMB, rssMB, heapAllocMB/rssMB)
				}

				// 🆕 更新上次goroutine计数
				lastNumGoroutines = numG
			}
		}
	}()
}

// getRSSBytesForRuntimeMonitor 获取进程 RSS（bytes）。
//
// - darwin: 使用 syscall.Getrusage 获取 ru_maxrss（实现为"峰值 RSS"，可用于粗略观测）
//   ⚠️ 注意：ru_maxrss 返回的是峰值 RSS（进程运行期间的最大值），不是当前 RSS
//   这意味着即使内存已释放，Maxrss 也不会减少，只会增加
//   因此日志中的 RSS 值可能高于 ps aux 显示的当前 RSS
// - linux: 读取 /proc/self/status 的 VmRSS（KB，当前RSS）
// - 其他平台: 返回 0
func getRSSBytesForRuntimeMonitor() uint64 {
	switch runtime.GOOS {
	case "darwin":
		var r syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
			return 0
		}
		// macOS 的 ru_maxrss 单位是字节，返回峰值 RSS
		return uint64(r.Maxrss)
	case "linux":
		f, err := os.Open("/proc/self/status")
		if err != nil {
			return 0
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, perr := strconv.ParseUint(fields[1], 10, 64)
					if perr != nil {
						return 0
					}
					return kb * 1024
				}
			}
		}
		return 0
	default:
		return 0
	}
}

// ============================================================================
//                         MemoryGuard 配置和适配器
// ============================================================================

// getMemoryGuardConfig 从配置提供者获取 MemoryGuard 配置
func getMemoryGuardConfig(configProvider config.Provider) *diagnostics.MemoryGuardConfig {
	cfg := diagnostics.DefaultMemoryGuardConfig()

	if configProvider == nil {
		return cfg
	}

	// 使用 GetMemoryMonitoring() 获取内存监控配置
	memoryMonitoring := configProvider.GetMemoryMonitoring()
	if memoryMonitoring == nil || memoryMonitoring.MemoryGuard == nil {
		return cfg
	}

	guardCfg := memoryMonitoring.MemoryGuard

	if guardCfg.Enabled != nil {
		cfg.Enabled = *guardCfg.Enabled
	}
	if guardCfg.SoftLimitMB != nil {
		cfg.SoftLimitMB = *guardCfg.SoftLimitMB
	}
	if guardCfg.HardLimitMB != nil {
		cfg.HardLimitMB = *guardCfg.HardLimitMB
	}
	if guardCfg.AutoProfile != nil {
		cfg.AutoProfile = *guardCfg.AutoProfile
	}
	if guardCfg.ProfileOutputDir != nil {
		cfg.ProfileOutputDir = *guardCfg.ProfileOutputDir
	}
	if guardCfg.CheckIntervalSeconds != nil && *guardCfg.CheckIntervalSeconds > 0 {
		cfg.CheckInterval = time.Duration(*guardCfg.CheckIntervalSeconds) * time.Second
	}

	return cfg
}

// memoryGuardLoggerAdapter 日志适配器
type memoryGuardLoggerAdapter struct {
	logger log.Logger
}

func (a *memoryGuardLoggerAdapter) Debugf(format string, args ...interface{}) {
	if a.logger != nil {
		a.logger.Debugf(format, args...)
	}
}

func (a *memoryGuardLoggerAdapter) Infof(format string, args ...interface{}) {
	if a.logger != nil {
		a.logger.Infof(format, args...)
	}
}

func (a *memoryGuardLoggerAdapter) Warnf(format string, args ...interface{}) {
	if a.logger != nil {
		a.logger.Warnf(format, args...)
	}
}

func (a *memoryGuardLoggerAdapter) Errorf(format string, args ...interface{}) {
	if a.logger != nil {
		a.logger.Errorf(format, args...)
	}
}

// ============================================================================
//                              模块定义
// ============================================================================

// ProvideServices 提供 chain 模块的所有服务
//
// 🎯 **服务创建**：
// 本函数负责创建 chain 模块的所有服务实例，并通过 ModuleOutput 统一导出。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 🎯 为链模块添加 module 字段，日志将路由到 node-system.log
	var chainLogger log.Logger
	if input.Logger != nil {
		chainLogger = input.Logger.With("module", "chain")
	}

	// 创建 ForkHandler 服务
	forkHandler, err := fork.NewService(
		input.QueryService,
		input.HashManager,
		input.BlockHashClient,
		input.TxHashClient,
		input.BadgerStore,
		input.ConfigProvider,
		input.EventBus,
		chainLogger,
	)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("创建 ForkHandler 失败: %w", err)
	}

	// 创建 SystemSyncService 服务（传入 NodeRuntimeState 以便更新同步状态）
	syncService := sync.NewManager(
		input.QueryService, // 作为ChainQuery使用
		input.BlockValidator,
		input.BlockProcessor,
		input.QueryService, // 作为QueryService使用
		input.NetworkService,
		input.RoutingTableManager,
		input.P2PService,
		input.ConfigProvider,
		input.TempStore,
		input.NodeRuntimeState, // 节点运行时状态（用于更新同步状态）
		input.BlockHashClient,
		forkHandler,
		nil, // recoveryMgr - 待fx集成后替换，详见PENDING_FX_INTEGRATION.md
		chainLogger,
		input.EventBus,
	)

	// ================================
	// UTXO 自愈（CHAIN 内部子能力）
	// ================================
	if input.EventBus != nil && input.UTXOSnapshot != nil && input.QueryService != nil && input.BlockProcessor != nil {
		recoveryMgr := recovery.NewUTXORecoveryManager(input.QueryService, input.BlockProcessor, input.UTXOSnapshot, input.EventBus, chainLogger)
		recoveryMgr.RegisterSubscriptions(context.Background())
		if chainLogger != nil {
			chainLogger.Info("🩹 UTXORecoveryManager 已启用（订阅 corruption.detected: utxo_inconsistent）")
		}
	}

	// 类型断言为公共接口
	var publicForkHandler chainif.ForkHandler = forkHandler
	var publicSyncService chainif.SystemSyncService = syncService

	// 注册 Chain SyncService 到内存监控系统
	if reporter, ok := syncService.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if chainLogger != nil {
			chainLogger.Info("✅ Chain SyncService 已注册到内存监控系统")
		}
	}

	// 注意：NodeRuntimeState 从 P2P 模块获取，不再由 chain 模块创建
	return ModuleOutput{
		ForkHandler:               publicForkHandler,
		SystemSyncService:         publicSyncService,
		InternalForkHandler:       forkHandler,
		InternalSystemSyncService: syncService,
	}, nil
}

// Module 返回 chain 模块的 fx 配置
//
// 🎯 **模块职责**：
// - 提供 ForkHandler 服务 ✅
// - 提供 SystemSyncService 服务 ✅
// - 注册事件订阅 ✅
// - 注册网络协议 ✅
// - 管理生命周期 ✅
//
// 🔗 **依赖关系**：
// - 输入：Logger, EventBus（可选）, Network（可选）, QueryService, BlockValidator, BlockProcessor
// - 输出：ForkHandler, SystemSyncService
//
// 📋 **导出服务**：
// - chainif.ForkHandler (name: "fork_handler") ✅
// - chainif.SystemSyncService (name: "sync_service") ✅
// - interfaces.InternalForkHandler (未命名，内部使用) ✅
// - interfaces.InternalSyncService (未命名，内部使用) ✅
func Module() fx.Option {
	return fx.Module("chain",
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
		//                           事件集成
		// ====================================================================

		// 事件订阅注册
		fx.Invoke(
			fx.Annotate(
				func(
					eventBus event.EventBus,
					logger log.Logger,
					forkHandler interfaces.InternalForkHandler,
					syncService interfaces.InternalSyncService,
					queryService persistence.QueryService,
				) error {
					if eventBus == nil {
						if logger != nil {
							logger.Warn("EventBus不可用，跳过chain模块事件订阅")
						}
						return nil
					}

					// 创建事件订阅注册器（包含sync服务的事件订阅）
					registry := eventIntegration.NewEventSubscriptionRegistry(
						eventBus,
						logger,
						forkHandler,
						syncService, // syncService实现了SyncEventSubscriber接口
						queryService,
					)

					// 注册所有事件订阅（ForkHandler和Sync服务的事件）
					if err := registry.RegisterEventSubscriptions(); err != nil {
						if logger != nil {
							logger.Errorf("chain模块事件订阅注册失败: %v", err)
						}
						return err
					}

					if logger != nil {
						logger.Info("✅ chain模块事件订阅已注册（包括sync服务事件）")
					}

					return nil
				},
				fx.ParamTags(
					``,                     // event.EventBus
					``,                     // log.Logger
					`name:"fork_handler"`,  // interfaces.InternalForkHandler
					`name:"sync_service"`,  // interfaces.InternalSyncService
					`name:"query_service"`, // persistence.QueryService
				),
			),
		),

		// ====================================================================
		//                           网络集成
		// ====================================================================

		// 注册同步网络协议处理器
		fx.Invoke(
			fx.Annotate(
				func(
					networkService network.Network,
					syncService interfaces.InternalSyncService,
					logger log.Logger,
				) error {
					if networkService == nil {
						if logger != nil {
							logger.Warn("Network不可用，跳过chain模块网络协议注册")
						}
						return nil
					}

					// 注册同步协议处理器（SyncProtocolRouter接口）
					if err := networkIntegration.RegisterSyncStreamHandlers(
						networkService,
						syncService, // syncService实现了SyncProtocolRouter接口
						logger,
					); err != nil {
						if logger != nil {
							logger.Errorf("chain模块网络协议注册失败: %v", err)
						}
						return err
					}

					if logger != nil {
						logger.Info("✅ chain模块网络协议已注册（同步协议）")
					}

					return nil
				},
				fx.ParamTags(`name:"network_service"`, `name:"sync_service"`, ``),
			),
		),

		// ====================================================================
		//                           延迟依赖注入
		// ====================================================================

		// ✅ BlockProcessor 到 ForkHandler 的延迟注入（关键：否则生产环境无法执行 reorg）
		fx.Invoke(
			fx.Annotate(
				func(
					forkHandler interfaces.InternalForkHandler,
					blockProcessor blockif.BlockProcessor,
					logger log.Logger,
				) {
					if forkService, ok := forkHandler.(*fork.Service); ok {
						if blockProcessor != nil {
							forkService.SetBlockProcessor(blockProcessor)
							if logger != nil {
								logger.Info("🔗 BlockProcessor 已注入到 ForkHandler")
							}
						} else {
							if logger != nil {
								logger.Warn("⚠️ BlockProcessor 未注入到 ForkHandler（reorg 将无法执行）")
							}
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ ForkHandler 类型断言失败（BlockProcessor 注入）")
						}
					}
				},
				fx.ParamTags(
					`name:"fork_handler"`,    // chain.ForkHandler
					`name:"block_processor"`, // block.BlockProcessor
					``,                       // log.Logger
				),
			),
		),

		// UTXOSnapshot 到 ForkHandler 的延迟注入（P3-1：完整实现）
		fx.Invoke(
			fx.Annotate(
				func(
					forkHandler interfaces.InternalForkHandler,
					utxoSnapshot eutxo.UTXOSnapshot,
					logger log.Logger,
				) {
					// 类型断言并注入 UTXOSnapshot
					if forkService, ok := forkHandler.(*fork.Service); ok {
						if utxoSnapshot != nil {
							forkService.SetUTXOSnapshot(utxoSnapshot)
							if logger != nil {
								logger.Info("🔗 UTXOSnapshot 已注入到 ForkHandler")
							}
						} else {
							if logger != nil {
								logger.Warn("⚠️ UTXOSnapshot 未注入")
							}
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ ForkHandler 类型断言失败")
						}
					}
				},
				fx.ParamTags(
					`name:"fork_handler"`,  // chain.ForkHandler
					`name:"utxo_snapshot"`, // eutxo.UTXOSnapshot
					``,                     // log.Logger
				),
			),
		),

		// ✅ 修复 P0-3：DataWriter 到 ForkHandler 的延迟注入
		fx.Invoke(
			fx.Annotate(
				func(
					forkHandler interfaces.InternalForkHandler,
					dataWriter persistence.DataWriter,
					logger log.Logger,
				) {
					// 类型断言并注入 DataWriter
					if forkService, ok := forkHandler.(*fork.Service); ok {
						if dataWriter != nil {
							forkService.SetDataWriter(dataWriter)
							if logger != nil {
								logger.Info("🔗 DataWriter 已注入到 ForkHandler")
							}
						} else {
							if logger != nil {
								logger.Warn("⚠️ DataWriter 未注入")
							}
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ ForkHandler 类型断言失败")
						}
					}
				},
				fx.ParamTags(
					`name:"fork_handler"`, // chain.ForkHandler
					`name:"data_writer"`,  // persistence.DataWriter
					``,                    // log.Logger
				),
			),
		),

		// ✅ TxPool 到 ForkHandler 的延迟注入（用于 reorg tx-recovery）
		fx.Invoke(
			fx.Annotate(
				func(
					forkHandler interfaces.InternalForkHandler,
					txPool mempoolif.TxPool,
					logger log.Logger,
				) {
					if forkService, ok := forkHandler.(*fork.Service); ok {
						if txPool != nil {
							forkService.SetTxPool(txPool)
							if logger != nil {
								logger.Info("🔗 TxPool 已注入到 ForkHandler")
							}
						} else {
							if logger != nil {
								logger.Warn("⚠️ TxPool 未注入到 ForkHandler（reorg tx-recovery 将跳过）")
							}
						}
					} else {
						if logger != nil {
							logger.Warn("⚠️ ForkHandler 类型断言失败（TxPool 注入）")
						}
					}
				},
				fx.ParamTags(
					`name:"fork_handler"`, // chain.ForkHandler
					`name:"tx_pool"`,      // mempool.TxPool
					``,                    // log.Logger
				),
			),
		),

		// ====================================================================
		//                           启动流程初始化
		// ====================================================================

		// 创世区块初始化检查和启动时同步触发（在所有服务加载完成后执行）
		fx.Invoke(
			fx.Annotate(
				func(
					queryService persistence.QueryService,
					blockProcessor blockif.BlockProcessor,
					genesisBuilder blockif.GenesisBlockBuilder,
					addressManager crypto.AddressManager,
					powEngine crypto.POWEngine,
					routingManager kademlia.RoutingTableManager,
					syncService chainif.SystemSyncService,
					badgerStore storage.BadgerStore,
					fileStore storage.FileStore,
					blockHashClient core.BlockHashServiceClient,
					configProvider config.Provider,
					logger log.Logger,
				) error {
					if logger != nil {
						logger.Info("🚀 开始区块链启动流程初始化...")
					}

					ctx := context.Background()

					// ============================================================
					// 阶段1: 创世区块检查
					// ============================================================
					if logger != nil {
						logger.Info("📍 阶段1: 创世区块检查")
					}

					// 加载创世配置（必须从配置中获取，不允许使用默认值）
					var genesisConfig *types.GenesisConfig
					if configProvider != nil {
						// 尝试从ConfigProvider获取统一创世配置
						genesisConfig = configProvider.GetUnifiedGenesisConfig()
						if genesisConfig != nil && logger != nil {
							logger.Infof("✅ 使用统一创世配置，网络: %s，链ID: %d，时间戳: %d，账户数: %d",
								genesisConfig.NetworkID, genesisConfig.ChainID, genesisConfig.Timestamp, len(genesisConfig.GenesisAccounts))
						}
					}

					// 🔧 验证必填配置项（启动时验证）
					if configProvider != nil {
						// 获取统一创世配置用于验证
						unifiedGenesis := configProvider.GetUnifiedGenesisConfig()

						// 获取appConfig用于验证
						appConfig := configProvider.GetAppConfig()

						// 执行配置验证
						if err := configimpl.ValidateMandatoryConfig(appConfig, unifiedGenesis); err != nil {
							errMsg := fmt.Sprintf("❌ 配置验证失败\n%s\n"+
								"   请检查配置文件，确保以下必填项已正确配置：\n"+
								"   - network.chain_id: 链ID（不能为0）\n"+
								"   - network.network_name: 网络名称（不能为空）\n"+
								"   - genesis.timestamp: 创世时间戳（不能为0）\n"+
								"   - genesis.accounts: 创世账户（至少一个）",
								err.Error())

							if logger != nil {
								logger.Errorf("========================================")
								logger.Errorf("%s", errMsg)
								logger.Errorf("========================================")
							}
							return fmt.Errorf("配置验证失败: %w", err)
						}

						if logger != nil {
							logger.Info("✅ 配置验证通过：所有必填项已正确配置")
						}
					}

					// 验证创世配置必须存在且时间戳必须已配置
					if genesisConfig == nil {
						return fmt.Errorf("启动失败：未找到创世配置，必须在配置文件中指定 genesis 配置")
					}
					if genesisConfig.Timestamp == 0 {
						return fmt.Errorf("启动失败：创世配置时间戳不能为空或0，必须在配置文件中显式指定 genesis.timestamp")
					}

					// 验证持久化的 genesis_hash（如果存在）
					if badgerStore != nil {
						if err := startup.ValidatePersistedGenesisHash(ctx, badgerStore, genesisConfig); err != nil {
							if logger != nil {
								logger.Errorf("========================================")
								logger.Errorf("❌ 链身份验证失败: %v", err)
								logger.Errorf("========================================")
							}
							return fmt.Errorf("链身份验证失败: %w", err)
						}
						if logger != nil {
							logger.Info("✅ 链身份验证通过：历史记录的 genesis_hash 与当前配置一致")
						}
					}

					// 打印启动日志：boot.chain_identity 和 boot.node_policy
					if logger != nil && configProvider != nil {
						appCfg := configProvider.GetAppConfig()
						if appCfg != nil {
							// 计算并打印链身份
							genesisHash, err := confignode.CalculateGenesisHash(genesisConfig)
							if err == nil {
								localChainIdentity := confignode.BuildLocalChainIdentity(appCfg, genesisHash)
								logger.Infof("boot.chain_identity: chain_id=%s network_namespace=%s chain_mode=%s genesis_hash=%s (前8位: %s)",
									localChainIdentity.ChainID, localChainIdentity.NetworkNamespace,
									localChainIdentity.ChainMode, localChainIdentity.GenesisHash,
									getHashPrefix(localChainIdentity.GenesisHash, 8))
							}

							// ✅ Phase 5.3：已移除节点角色策略打印
							// 现在使用状态机模型，不再使用 NodeRole/策略矩阵
							// 节点运行时状态将在 NodeRuntimeState 初始化后通过 API 查询
						}
					}

					// 直接调用启动函数（带存储版本，用于持久化 genesis_hash 和自动修复索引）
					created, err := startup.InitializeGenesisIfNeededWithStore(
						ctx,
						queryService,
						blockProcessor,
						genesisBuilder,
						addressManager,
						powEngine,
						genesisConfig,
						badgerStore,
						fileStore,
						blockHashClient,
						logger,
					)
					if err != nil {
						if logger != nil {
							logger.Errorf("创世区块初始化失败: %v", err)
						}
						return fmt.Errorf("创世区块初始化失败: %w", err)
					}

					if created {
						if logger != nil {
							logger.Info("✅ 创世区块初始化完成")
						}
					} else {
						if logger != nil {
							logger.Info("✅ 链已初始化，跳过创世区块创建")
						}
					}

					// ============================================================
					// 阶段2: 同步策略与启动同步（破坏性重构）
					// ============================================================
					if logger != nil {
						logger.Info("📍 阶段2: 同步策略与启动同步")
					}

					// 查询当前链信息
					chainInfo, err := queryService.GetChainInfo(ctx)
					if err != nil {
						if logger != nil {
							logger.Errorf("获取链信息失败: %v", err)
						}
						return fmt.Errorf("获取链信息失败: %w", err)
					}

					localHeight := chainInfo.Height
					if logger != nil {
						logger.Infof("当前本地链高度: %d", localHeight)
					}

					// ============================================================
					// 阶段2.0: 存储一致性门闸（blocks/ + Badger 索引）
					// ============================================================
					// 目标：
					// - 区块原始数据必须落盘在 blocks/；
					// - Badger 仅存链尖与索引；
					// - 由于跨存储无法强原子提交，本门闸做 fail-fast 检测，避免在“tip 指向缺失块文件”状态下继续运行。
					if badgerStore != nil && fileStore != nil {
						if ierr := repair.CheckBlocksAndBadgerTip(ctx, badgerStore, fileStore, logger); ierr != nil {
							if logger != nil {
								logger.Errorf("❌ 存储一致性门闸失败（blocks/+Badger）: %v", ierr)
							}
							return fmt.Errorf("存储一致性检查失败: %w", ierr)
						}
					}

					// ✅ Phase 5.3：读取 sync.startup_mode，决定启动同步策略
					// 不再使用 node.role，因为现在使用状态机模型
					appCfg := configProvider.GetAppConfig()
					startupMode := ""

					if appCfg != nil {
						if appCfg.Sync != nil && appCfg.Sync.StartupMode != nil {
							startupMode = strings.ToLower(strings.TrimSpace(*appCfg.Sync.StartupMode))
						}
					}

					// 未显式配置时，按环境推导默认模式：dev → from_genesis，其它 → from_network
					if startupMode == "" {
						env := strings.ToLower(configProvider.GetEnvironment())
						if env == "dev" {
							startupMode = "from_genesis"
						} else {
							startupMode = "from_network"
						}
					}

					if logger != nil {
						logger.Infof("启动同步策略: startup_mode=%s, local_height=%d", startupMode, localHeight)
					}

					// 🎯 根据 startup_mode 决定启动阶段是否“优先尝试从网络补齐历史”
					// - from_network:
					//   - 如果本地高度=0，优先尝试从网络同步已有区块高度；
					//   - 如果网络中不存在任何上游WES节点，则退化为单节点 Bootstrapping，
					//     创世区块仍由本地根据配置构建，不再使用“禁止本地创世”的语义。
					// - from_genesis: 直接从本地创世高度开始运行（典型单节点 / 私有链场景）
					// - snapshot: 从快照导入后再追同步（预留）
					switch startupMode {
					case "from_network":
						if localHeight == 0 {
							if logger != nil {
								logger.Info("🌐 from_network 模式且本地高度为0：后台尝试从网络同步已有区块，如无上游节点则进入单节点 Bootstrapping 模式")
							}

							// 🎯 架构原则：module.go 只做依赖注入和启动编排，不做业务决策
							// - 不区分 dev/test/prod 环境，所有环境统一行为
							// - 不等待 P2P 节点，不阻塞启动流程
							// - 只做后台 best-effort 同步尝试，让节点尽快对外提供 API
							// - 真正的"能否挖矿"决策由共识层通过 CheckSync + RuntimeState 统一判断
							if syncService != nil {
								go func() {
									if err := syncService.TriggerSync(context.Background()); err != nil && logger != nil {
										logger.Debugf("启动时后台同步触发失败（真正的同步错误）: %v", err)
									} else if logger != nil {
										logger.Info("✅ 启动时后台同步流程已执行（可能已完成同步，或当前无上游节点）")
									}
								}()
							} else if logger != nil {
								logger.Warn("syncService 未初始化，无法在启动阶段触发后台同步")
							}
							// 不阻塞，直接继续启动流程，后续由共识层的 CheckSync + 单节点 Bootstrapping 特判决定是否允许挖矿

						} else {
							// 本地高度 > 0，执行常规同步检查
							if logger != nil {
								logger.Infof("本地已有区块（高度: %d），执行启动同步检查", localHeight)
							}
							if syncService != nil {
								go func() {
									if err := syncService.TriggerSync(context.Background()); err != nil {
										if logger != nil {
											logger.Debugf("启动时同步触发: %v", err)
										}
									}
								}()
							}
						}
					case "from_genesis":
						// from_genesis 模式：允许从创世开始，不强制同步
						if logger != nil {
							logger.Info("✅ from_genesis 模式：允许从创世开始，不强制同步")
						}
						// 可选：如果本地高度=0 且有可用节点，仍可触发一次同步检查（非阻塞）
						if localHeight == 0 && syncService != nil {
							go func() {
								time.Sleep(5 * time.Second) // 延迟触发，避免阻塞启动
								if err := syncService.TriggerSync(context.Background()); err != nil {
									if logger != nil {
										logger.Debugf("from_genesis 模式可选同步触发: %v", err)
									}
								}
							}()
						}
					default:
						// snapshot 模式（预留）
						if logger != nil {
							logger.Infof("snapshot 模式（预留）: local_height=%d", localHeight)
						}
						// 暂不实现，后续可扩展
					}

					if logger != nil {
						logger.Info("✅ 区块链启动流程初始化完成")
					}

					return nil
				},
				fx.ParamTags(
					`name:"query_service"`,         // persistence.QueryService
					`name:"block_processor"`,       // block.BlockProcessor
					`name:"genesis_builder"`,       // block.GenesisBlockBuilder
					``,                             // crypto.AddressManager
					``,                             // crypto.POWEngine
					`name:"routing_table_manager"`, // kademlia.RoutingTableManager
					`name:"sync_service"`,          // chain.SystemSyncService
					``,                             // storage.BadgerStore (无需命名标签，通过类型匹配)
					``,                             // storage.FileStore (无需命名标签，通过类型匹配)
					``,                             // config.Provider
					``,                             // log.Logger
				),
			),
		),

		// ====================================================================
		//                           生命周期管理
		// ====================================================================

		fx.Invoke(
			func(lc fx.Lifecycle, logger log.Logger, configProvider config.Provider, hashManager crypto.HashManager) {
				// ✅ 创建独立的、长生命周期的context用于RuntimeMonitor
				// 修复原因：OnStart的ctx在函数返回后会被取消，导致RuntimeMonitor仅运行7ms就停止
				ctx, cancel := context.WithCancel(context.Background())

				// 🆕 创建 MemoryGuard 实例
				var memoryGuard *diagnostics.MemoryGuard
				memoryGuardConfig := getMemoryGuardConfig(configProvider)
				if memoryGuardConfig.Enabled {
					// 创建适配器以适配日志接口
					var guardLogger diagnostics.MemoryGuardLogger
					if logger != nil {
						guardLogger = &memoryGuardLoggerAdapter{logger}
					}
					memoryGuard = diagnostics.NewMemoryGuard(memoryGuardConfig, guardLogger)

					// 🔧 注册 HashService 到 MemoryGuard（修复内存泄漏）
					if hashService, ok := hashManager.(*hash.HashService); ok {
						memoryGuard.RegisterCacheCleaner(hashService)
						if logger != nil {
							logger.Info("✅ HashService 已注册到 MemoryGuard（LRU缓存自动清理）")
						}
					}
				}

				lc.Append(fx.Hook{
					OnStart: func(_ context.Context) error {
						if logger != nil {
							logger.Info("🚀 Chain 模块启动")
						}
						// 启动运行时监控协程（内存与 goroutine 数量），用于现场排障
						// 使用独立的长生命周期ctx，而非OnStart的短生命周期参数ctx
						startRuntimeMonitors(ctx, logger)

						// 🆕 启动 MemoryGuard（内存保护守护程序）
						if memoryGuard != nil {
							if err := memoryGuard.Start(ctx); err != nil {
								if logger != nil {
									logger.Warnf("MemoryGuard 启动失败: %v", err)
								}
							}
						}
						return nil
					},
					OnStop: func(_ context.Context) error {
						if logger != nil {
							logger.Info("🛑 Chain 模块停止")
						}
						// 🆕 停止 MemoryGuard
						if memoryGuard != nil {
							if err := memoryGuard.Stop(); err != nil {
								if logger != nil {
									logger.Warnf("MemoryGuard 停止失败: %v", err)
								}
							}
						}
						// ✅ 显式取消context，停止RuntimeMonitor
						cancel()
						return nil
					},
				})
			},
		),

		// ====================================================================
		//                           生命周期管理（Sync服务）
		// ====================================================================

		// Sync服务的生命周期管理（启动定时同步调度器）
		fx.Invoke(
			fx.Annotate(
				func(
					lc fx.Lifecycle,
					syncService interfaces.InternalSyncService,
					logger log.Logger,
				) {
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							// 启动定时同步调度器
							if syncManager, ok := syncService.(*sync.Manager); ok {
								periodicScheduler := syncManager.GetPeriodicScheduler()
								if periodicScheduler != nil {
									if err := periodicScheduler.Start(ctx); err != nil {
										if logger != nil {
											logger.Errorf("启动定时同步调度器失败: %v", err)
										}
										return err
									}
									if logger != nil {
										logger.Info("✅ 定时同步调度器已启动")
									}
								}
							}

							// ✅ 在服务启动时执行一次 CheckSync，初始化 RuntimeState 的同步状态快照
							// 说明：
							// - 之前 RuntimeState.isFullySynced 默认 false，且只有在显式调用 CheckSync 时才会被更新
							// - 如果节点在未调用任何 API（wes_getSyncStatus / 挖矿前置检查）之前就依赖 RuntimeState，
							//   可能会读到一个"永远为 false"且与真实高度不一致的值
							// - 这里在启动阶段触发一次 CheckSync，既能更新 RuntimeState，又不会阻塞整体启动流程
							go func() {
								// 使用一个有限超时的上下文，避免在启动阶段被卡死
								checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
								defer cancel()

								if _, err := syncService.CheckSync(checkCtx); err != nil {
									if logger != nil {
										logger.Debugf("启动阶段初始同步状态检查失败（可忽略）: %v", err)
									}
								} else {
									if logger != nil {
										logger.Info("✅ 启动阶段已完成一次同步状态检查，RuntimeState 已初始化")
									}
								}
							}()

							// 🔥 启动节点健康度监控（定期输出熔断状态）
							go func() {
								ticker := time.NewTicker(5 * time.Minute)
								defer ticker.Stop()

								for {
									select {
									case <-ctx.Done():
										return
									case <-ticker.C:
										metrics := sync.GetPeerHealthMetrics()
										if logger != nil {
											logger.Infof("📊 节点健康度: 总计=%d, 健康=%d, 熔断=%d",
												metrics["total_tracked_peers"],
												metrics["healthy_peers"],
												metrics["circuit_broken_peers"])
										}
									}
								}
							}()

							return nil
						},
						OnStop: func(ctx context.Context) error {
							// 停止定时同步调度器
							if syncManager, ok := syncService.(*sync.Manager); ok {
								periodicScheduler := syncManager.GetPeriodicScheduler()
								if periodicScheduler != nil {
									periodicScheduler.Stop()
									if logger != nil {
										logger.Info("🛑 定时同步调度器已停止")
									}
								}
							}
							return nil
						},
					})
				},
				fx.ParamTags(
					``,                    // fx.Lifecycle
					`name:"sync_service"`, // chain.SystemSyncService
					``,                    // log.Logger
				),
			),
		),

		// ====================================================================
		//                    BlockFileGC 服务（可选维护服务）
		// ====================================================================

		// 提供 BlockFileGC 服务
		fx.Provide(
			func(
				configProvider config.Provider,
				logger log.Logger,
				badgerStore storage.BadgerStore,
				fileStore storage.FileStore,
			) *gc.BlockFileGC {
				// 从配置中获取 GC 配置
				var gcConfig *gc.BlockFileGCConfig
				if configProvider != nil {
					if blockchainOpts := configProvider.GetBlockchain(); blockchainOpts != nil {
						if blockchainOpts.BlockFileGC != nil {
							// 转换配置
							gcConfig = &gc.BlockFileGCConfig{
								Enabled:                 blockchainOpts.BlockFileGC.Enabled,
								DryRun:                  blockchainOpts.BlockFileGC.DryRun,
								IntervalSeconds:         blockchainOpts.BlockFileGC.IntervalSeconds,
								RateLimitFilesPerSecond: blockchainOpts.BlockFileGC.RateLimitFilesPerSecond,
								ProtectRecentHeight:     blockchainOpts.BlockFileGC.ProtectRecentHeight,
								BatchSize:               50, // 默认批量大小
							}
						}
					}
				}

				// 如果没有配置或未启用，返回 nil
				if gcConfig == nil || !gcConfig.Enabled {
					if logger != nil {
						logger.Info("🗑️  BlockFileGC 未启用")
					}
					return nil
				}

				// 检查依赖
				if badgerStore == nil || fileStore == nil {
					if logger != nil {
						logger.Warn("⚠️  BlockFileGC 无法启动：缺少 BadgerStore 或 FileStore")
					}
					return nil
				}

				// 创建 GC 服务
				gcService := gc.NewBlockFileGC(gcConfig, logger, badgerStore, fileStore)
				if logger != nil {
					logger.Infof("🗑️  BlockFileGC 服务已创建（enabled=%v, dry_run=%v, interval=%ds）",
						gcConfig.Enabled, gcConfig.DryRun, gcConfig.IntervalSeconds)
				}
				return gcService
			},
		),

		// BlockFileGC 生命周期管理
		fx.Invoke(
			func(
				lifecycle fx.Lifecycle,
				gcService *gc.BlockFileGC,
				logger log.Logger,
			) {
				if gcService == nil {
					// GC 未启用，跳过
					return
				}

				lifecycle.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if err := gcService.Start(ctx); err != nil {
							if logger != nil {
								logger.Errorf("启动 BlockFileGC 失败: %v", err)
							}
							// GC 启动失败不影响主流程
							return nil
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if err := gcService.Stop(ctx); err != nil {
							if logger != nil {
								logger.Errorf("停止 BlockFileGC 失败: %v", err)
							}
						}
						return nil
					},
				})
			},
		),

		// 模块加载日志
		fx.Invoke(func(logger log.Logger) {
			if logger != nil {
				logger.Info("✅ Chain 模块已加载 (ForkHandler, SystemSyncService, BlockFileGC, 事件集成, 网络集成已启用)")
			}
		}),
	)
}

// ============================================================================
//                              模块信息
// ============================================================================

// Version 模块版本
const Version = "1.0.0"

// Name 模块名称
const Name = "chain"

// Description 模块描述
const Description = "链状态管理模块，提供链尖更新和分叉处理能力"

// ============================================================================
//                           启动流程辅助函数
// ============================================================================
// 注意：不再提供 createDefaultGenesisConfig 函数
// 创世配置必须从配置文件中显式指定，不允许使用默认值
// 这确保所有节点创建相同的创世区块，符合区块链一致性要求

// getHashPrefix 获取哈希字符串的前缀（安全版本）
func getHashPrefix(hash string, length int) string {
	if len(hash) < length {
		return hash
	}
	return hash[:length]
}
