// Package sync 提供区块链同步服务的具体实现
//
// 🎯 **薄管理器实现**
//
// 本文件实现 InternalSystemSyncService 接口，严格遵循薄管理器原则：
// - 只负责接口方法的委托，不包含复杂业务逻辑
// - 将具体实现委托给专门的处理器组件
// - 保持Manager类的简洁性和单一职责
package sync

import (
	"context"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	// 类型定义
	"github.com/weisyn/v1/pkg/types"

	// 公共接口依赖
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"

	// 内部接口依赖
	"github.com/weisyn/v1/internal/core/chain/interfaces"
	"github.com/weisyn/v1/internal/core/chain/recovery"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"

	// 业务模块
	"github.com/weisyn/v1/internal/core/chain/sync/event_handler"
	"github.com/weisyn/v1/internal/core/chain/sync/network_handler"

	// libp2p依赖
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (m *Manager) ModuleName() string {
	return "chain"
}

// CollectMemoryStats 收集链模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 已加载到内存的链高度节点数 / 索引项数（同步状态中的活跃任务和已同步节点）
// - ApproxBytes: 链索引缓存 bytes（同步状态缓存的内存估算）
// - CacheItems: height→hash 等索引缓存条目（已同步节点缓存条目）
// - QueueLength: 待处理链操作队列长度（当前活跃同步任务数，0或1）
func (m *Manager) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 统计已同步节点数量（从 sync_state.go 的全局变量）
	syncedPeersMutex.RLock()
	syncedPeerCount := len(syncedPeersCache)

	// 估算 syncedPeersCache 的内存占用（趋势用，不追求绝对精确）：
	// - 使用真实 peerID 字符串长度 + record struct 大小进行估算
	// - 避免“每条记录固定 X KB”的拍脑袋常数
	var approxBytes int64
	for pid, rec := range syncedPeersCache {
		// map key：peer.ID（底层为 string），估算 string header + payload
		approxBytes += int64(unsafe.Sizeof(pid)) + int64(len(pid))
		// map value：指针 + 指向的 record 对象
		approxBytes += int64(unsafe.Sizeof(rec))
		if rec != nil {
			// record struct 自身大小（其中 PeerID 的底层 bytes 与 key 共享，不重复计 payload）
			approxBytes += int64(unsafe.Sizeof(*rec))
		}
	}
	syncedPeersMutex.RUnlock()

	// 检查是否有活跃同步任务
	activeSyncMutex.RLock()
	hasActiveSync := activeSyncTask != nil
	activeSyncMutex.RUnlock()

	objects := int64(syncedPeerCount)
	if hasActiveSync {
		objects++ // 活跃同步任务也算一个对象
	}

	// 缓存条目：已同步节点缓存条目数
	cacheItems := int64(syncedPeerCount)

	// 队列长度：活跃同步任务数（0 或 1，因为同时只能有一个同步任务）
	queueLength := int64(0)
	if hasActiveSync {
		queueLength = 1
	}

	return metricsiface.ModuleMemoryStats{
		Module:      "chain",
		Layer:       "L4-CoreBusiness",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: queueLength,
	}
}

// ShrinkCache 主动裁剪链同步相关缓存（供 MemoryDoctor 调用）
//
// 当前主要针对 syncedPeersCache：
// - 在高压场景下清空“最近已同步节点”缓存，允许稍后按需重新同步
// - 不影响链状态和区块数据的一致性
func (m *Manager) ShrinkCache(targetSize int) {
	// 目前 chain.sync 主要缓存为全局 syncedPeersCache，这里不依赖 targetSize 精细收缩，
	// 而是直接清空缓存以快速释放内存。
	syncedPeersMutex.Lock()
	defer syncedPeersMutex.Unlock()

	if len(syncedPeersCache) == 0 {
		return
	}

	if m.logger != nil {
		m.logger.Warnf("MemoryDoctor 触发 Chain Sync 缓存收缩: 清空 syncedPeersCache, current=%d",
			len(syncedPeersCache))
	}

	for peerID := range syncedPeersCache {
		delete(syncedPeersCache, peerID)
	}
}

// ============================================================================
//                              薄管理器实现
// ============================================================================

// Manager 同步服务薄管理器
//
// 🎯 **薄实现原则**：
// - 只包含接口方法的委托实现
// - 具体业务逻辑委托给专门的处理器
// - 保持Manager类的简洁性
//
// 委托组织：
// - NetworkHandler: 处理网络协议（HandleKBucketSync, HandleRangePaginated）
// - EventHandler: 处理事件订阅（HandleFork*, HandleNetwork*）
// - 同步控制和状态查询暂时内置，后续可进一步分离
//
// 依赖原则：
// - 严格使用pkg/interfaces中的公共接口，避免依赖具体实现
// - 支持完整的依赖注入，便于测试和模块替换
// - 遵循项目的接口标准和架构规范
type Manager struct {
	// ========== 基础设施依赖 ==========
	chainQuery      persistence.ChainQuery         // 链状态查询（读操作）
	blockValidator  block.BlockValidator           // 区块验证（读操作）
	blockProcessor  block.BlockProcessor           // 区块处理（写操作）
	queryService    persistence.QueryService       // 统一查询服务（读操作，替代RepositoryManager）
	networkService  network.Network                // 网络服务（P2P通信）
	kBucketManager  kademlia.RoutingTableManager   // K桶管理器（路由表管理）
	p2pService      p2pi.Service                   // P2P服务（获取节点ID、验证节点）
	configProvider  config.Provider                // 配置提供者（标准接口）
	tempStore       storage.TempStore              // ✅ P1修复：临时存储服务（用于存储待处理区块）
	runtimeState    p2pi.RuntimeState              // 节点运行时状态（用于更新同步状态）
	blockHashClient core.BlockHashServiceClient    // 区块哈希客户端（用于构造 locator / 校验 hash）
	forkHandler     interfaces.InternalForkHandler // 分叉处理器（用于 fork-aware 自动 reorg）
	logger          log.Logger                     // 日志记录器
	eventBus        eventiface.EventBus            // 可选：用于发布corruption事件
	recoveryMgr     *recovery.DerivedDataRecoveryManager // 派生数据恢复管理器（用于Tip不一致修复）

	// ========== 业务子组件实例 ==========
	networkHandler    *network_handler.SyncNetworkHandler // 网络协议处理服务
	eventHandler      *event_handler.SyncEventHandler     // 事件处理服务
	periodicScheduler *PeriodicSyncScheduler              // 定时同步调度器
}

// NewManager 创建同步服务薄管理器
//
// 🏗️ **构造函数**：
// 创建Manager实例，注入必要的依赖，并初始化所有子组件。
// 严格使用pkg/interfaces中的公共接口，遵循依赖注入原则。
//
// 🎯 **适配新的依赖注入架构**：
// - chainQuery: 使用persistence.ChainQuery替代ChainService（读操作）
// - blockValidator: 使用block.BlockValidator替代BlockService.ValidateBlock
// - blockProcessor: 使用block.BlockProcessor替代BlockService.ProcessBlock
// - queryService: 使用persistence.QueryService替代RepositoryManager（读操作）
// - tempStore: ✅ P1修复：临时存储服务（用于存储待处理区块）
//
// 参数：
//   - chainQuery: 链状态查询服务（读操作）
//   - blockValidator: 区块验证服务（读操作）
//   - blockProcessor: 区块处理服务（写操作）
//   - queryService: 统一查询服务（读操作，替代RepositoryManager）
//   - networkService: 网络服务（P2P通信）
//   - kBucketManager: K桶管理器（路由表管理）
//   - p2pService: P2P服务（获取节点ID、验证节点）
//   - configProvider: 配置提供者（标准接口）
//   - tempStore: ✅ P1修复：临时存储服务（用于存储待处理区块）
//   - runtimeState: 节点运行时状态（用于更新同步状态）
//   - logger: 日志记录器
//
// 返回：
//   - interfaces.InternalSyncService: 内部同步服务接口
func NewManager(
	chainQuery persistence.ChainQuery,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	queryService persistence.QueryService,
	networkService network.Network,
	kBucketManager kademlia.RoutingTableManager,
	p2pService p2pi.Service,
	configProvider config.Provider,
	tempStore storage.TempStore,
	runtimeState p2pi.RuntimeState,
	blockHashClient core.BlockHashServiceClient,
	forkHandler interfaces.InternalForkHandler,
	recoveryMgr *recovery.DerivedDataRecoveryManager,
	logger log.Logger,
	eventBus eventiface.EventBus,
) interfaces.InternalSyncService {
	// 创建网络协议处理器（传入chainQuery和queryService以支持查询）
	networkHandler := network_handler.NewSyncNetworkHandler(logger, chainQuery, queryService, configProvider, blockHashClient)

	// 创建事件处理器
	eventHandler := event_handler.NewSyncEventHandler(logger)

	// 创建定时同步调度器（传入 runtimeState 以便更新同步状态）
	periodicScheduler := NewPeriodicSyncScheduler(
		chainQuery, queryService, blockValidator, blockProcessor, kBucketManager,
		networkService, p2pService, configProvider, tempStore, runtimeState, blockHashClient, forkHandler, logger, eventBus,
	)

	// 创建Manager实例
	manager := &Manager{
		// 基础设施依赖
		chainQuery:      chainQuery,
		blockValidator:  blockValidator,
		blockProcessor:  blockProcessor,
		queryService:    queryService,
		networkService:  networkService,
		kBucketManager:  kBucketManager,
		p2pService:      p2pService,
		configProvider:  configProvider,
		tempStore:       tempStore,    // ✅ P1修复：临时存储服务
		runtimeState:    runtimeState, // 节点运行时状态
		blockHashClient: blockHashClient,
		forkHandler:     forkHandler,
		logger:          logger,
		eventBus:        eventBus,
		recoveryMgr:     recoveryMgr, // 派生数据恢复管理器

		// 业务子组件
		networkHandler:    networkHandler,
		eventHandler:      eventHandler,
		periodicScheduler: periodicScheduler,
	}

	// 🔥 配置熔断器参数（从配置中读取）
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil {
			failureThreshold := bc.Sync.Advanced.CircuitBreakerFailureThreshold
			recoverySeconds := bc.Sync.Advanced.CircuitBreakerRecoverySeconds
			if failureThreshold > 0 || recoverySeconds > 0 {
				ConfigureCircuitBreaker(failureThreshold, recoverySeconds)
				if logger != nil {
					logger.Infof("🔧 熔断器已配置: failure_threshold=%d recovery_seconds=%d",
						failureThreshold, recoverySeconds)
				}
			}
		}
	}

	// 记录初始化日志
	if logger != nil {
		logger.Info("✅ 同步服务薄管理器初始化完成")
	}

	return manager
}

// GetPeriodicScheduler 获取定时同步调度器（用于生命周期管理）
func (m *Manager) GetPeriodicScheduler() *PeriodicSyncScheduler {
	return m.periodicScheduler
}

// ============================================================================
//                           公共接口实现 (SystemSyncService)
// ============================================================================

// TriggerSync 手动触发同步
//
// 🎯 **委托实现**：
// 委托给trigger.go中的具体实现处理K桶拉取同步逻辑。
func (m *Manager) TriggerSync(ctx context.Context) error {
	if m.logger != nil {
		// 触发来源可能来自：共识层自愈、启动阶段best-effort、运维接口、定时探针等。
		// 这里使用 Debug 避免在"无可用上游/孤节点"场景下被频繁触发导致刷屏。
		m.logger.Debug("收到同步触发请求")
	}

	// 🆕 等待Kademlia就绪（最多5秒）
	if err := m.waitForKademliaReady(ctx, 5*time.Second); err != nil {
		if m.logger != nil {
			m.logger.Warnf("Kademlia未就绪，同步延迟: %v", err)
		}
		// 不返回错误，允许fallback到已连接peers
	}

	// 委托给具体的同步触发实现，使用标准接口
	err := triggerSyncImpl(
		ctx,
		m.chainQuery,
		m.queryService,
		m.blockValidator,
		m.blockProcessor,
		m.kBucketManager,
		m.networkService,
		m.p2pService,
		m.configProvider,
		m.tempStore,
		m.blockHashClient,
		m.forkHandler,
		m.logger,
		m.eventBus,
		m.recoveryMgr,
	)
	if err != nil && m.logger != nil {
		m.logger.Errorf("[TriggerSync] ❌ 同步失败: %v", err)
	}
	return err
}

// CancelSync 取消当前同步
//
// 🎯 **委托实现**：
// 委托给cancel.go中的具体实现处理同步取消逻辑。
func (m *Manager) CancelSync(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Info("收到取消同步请求")
	}

	// 委托给具体的同步取消实现
	return cancelSyncImpl(ctx, m.logger)
}

// CancelSyncWithTimeout 带超时的同步取消（P2：补齐扩展接口）。
func (m *Manager) CancelSyncWithTimeout(ctx context.Context, timeout time.Duration) error {
	return CancelSyncWithTimeout(ctx, m.logger, timeout)
}

// ForceStopSync 强制停止同步（P2：补齐扩展接口）。
func (m *Manager) ForceStopSync() {
	ForceStopSync(m.logger)
}

// 🆕 waitForKademliaReady 等待Kademlia就绪
func (m *Manager) waitForKademliaReady(ctx context.Context, timeout time.Duration) error {
	if m.kBucketManager == nil {
		return fmt.Errorf("kBucketManager未注入")
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// 检查Kademlia是否就绪
			if m.kBucketManager.IsReady() {
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for Kademlia ready")
			}
		}
	}
}

// GetCancelProgress 获取取消进度快照（P2：补齐扩展接口）。
func (m *Manager) GetCancelProgress() CancelProgress {
	return GetCancelProgress()
}

// RegisterCancelCallback 注册取消完成回调（P2：补齐扩展接口）。
func (m *Manager) RegisterCancelCallback(cb func(CancelProgress)) {
	RegisterCancelCallback(cb)
}

// CheckSync 检查同步状态
//
// 🎯 **委托实现**：
// 委托给status.go中的具体实现查询当前同步状态。
func (m *Manager) CheckSync(ctx context.Context) (*types.SystemSyncStatus, error) {
	if m.logger != nil {
		m.logger.Debug("收到同步状态查询请求")
	}

	// 委托给具体的状态查询实现（传入 runtimeState 以便更新同步状态）
	return checkSyncImpl(
		ctx,
		m.chainQuery,
		m.kBucketManager,
		m.networkService,
		m.p2pService,
		m.configProvider,
		m.runtimeState,
		m.logger,
	)
}

// ============================================================================
//                         网络协议处理实现 (SyncProtocolRouter)
// ============================================================================

// HandleKBucketSync 处理K桶同步协议请求
//
// 🎯 **委托实现**：
// 委托给NetworkHandler处理K桶同步协议。
func (m *Manager) HandleKBucketSync(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("收到K桶同步请求，来源: %s, 数据大小: %d字节", from, len(reqBytes))
	}

	// 委托给NetworkHandler处理
	if m.networkHandler == nil {
		if m.logger != nil {
			m.logger.Error("网络协议处理器未初始化")
		}
		return nil, fmt.Errorf("网络协议处理器未初始化")
	}

	return m.networkHandler.HandleKBucketSync(ctx, from, reqBytes)
}

// HandleRangePaginated 处理分页区块同步协议请求
//
// 🎯 **委托实现**：
// 委托给NetworkHandler处理分页区块同步协议。
func (m *Manager) HandleRangePaginated(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("收到分页区块同步请求，来源: %s, 数据大小: %d字节", from, len(reqBytes))
	}

	// 委托给NetworkHandler处理
	if m.networkHandler == nil {
		if m.logger != nil {
			m.logger.Error("网络协议处理器未初始化")
		}
		return nil, fmt.Errorf("网络协议处理器未初始化")
	}

	return m.networkHandler.HandleRangePaginated(ctx, from, reqBytes)
}

// HandleSyncHelloV2 处理 Sync v2 握手协议请求
func (m *Manager) HandleSyncHelloV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("收到SyncHelloV2请求，来源: %s, 数据大小: %d字节", from, len(reqBytes))
	}
	if m.networkHandler == nil {
		if m.logger != nil {
			m.logger.Error("网络协议处理器未初始化")
		}
		return nil, fmt.Errorf("网络协议处理器未初始化")
	}
	return m.networkHandler.HandleSyncHelloV2(ctx, from, reqBytes)
}

// HandleSyncBlocksV2 处理 Sync v2 区块批量同步协议请求
func (m *Manager) HandleSyncBlocksV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("收到SyncBlocksV2请求，来源: %s, 数据大小: %d字节", from, len(reqBytes))
	}
	if m.networkHandler == nil {
		if m.logger != nil {
			m.logger.Error("网络协议处理器未初始化")
		}
		return nil, fmt.Errorf("网络协议处理器未初始化")
	}
	return m.networkHandler.HandleSyncBlocksV2(ctx, from, reqBytes)
}

// ============================================================================
//                         事件订阅处理实现 (SyncEventSubscriber)
// ============================================================================

// HandleForkDetected 处理分叉检测事件
//
// 🎯 **委托实现**：
// 委托给EventHandler处理分叉检测事件。
func (m *Manager) HandleForkDetected(eventData *types.ForkDetectedEventData) error {
	if m.logger != nil {
		m.logger.Info("收到分叉检测事件")
	}

	// 委托给EventHandler处理
	if m.eventHandler == nil {
		if m.logger != nil {
			m.logger.Error("事件处理器未初始化")
		}
		return fmt.Errorf("事件处理器未初始化")
	}

	return m.eventHandler.HandleForkDetected(eventData)
}

// HandleForkProcessing 处理分叉处理中事件
//
// 🎯 **委托实现**：
// 委托给EventHandler处理分叉处理事件。
func (m *Manager) HandleForkProcessing(eventData *types.ForkProcessingEventData) error {
	if m.logger != nil {
		m.logger.Info("收到分叉处理中事件")
	}

	// 委托给EventHandler处理
	if m.eventHandler == nil {
		if m.logger != nil {
			m.logger.Error("事件处理器未初始化")
		}
		return fmt.Errorf("事件处理器未初始化")
	}

	return m.eventHandler.HandleForkProcessing(eventData)
}

// HandleForkCompleted 处理分叉完成事件
//
// 🎯 **委托实现**：
// 委托给EventHandler处理分叉完成事件。
func (m *Manager) HandleForkCompleted(eventData *types.ForkCompletedEventData) error {
	if m.logger != nil {
		m.logger.Info("收到分叉完成事件")
	}

	// 委托给EventHandler处理
	if m.eventHandler == nil {
		if m.logger != nil {
			m.logger.Error("事件处理器未初始化")
		}
		return fmt.Errorf("事件处理器未初始化")
	}

	return m.eventHandler.HandleForkCompleted(eventData)
}

// HandleNetworkQualityChanged 处理网络质量变化事件
//
// 🎯 **委托实现**：
// 委托给EventHandler处理网络质量变化事件。
func (m *Manager) HandleNetworkQualityChanged(eventData *types.NetworkQualityChangedEventData) error {
	if m.logger != nil {
		m.logger.Info("收到网络质量变化事件")
	}

	// 委托给EventHandler处理
	if m.eventHandler == nil {
		if m.logger != nil {
			m.logger.Error("事件处理器未初始化")
		}
		return fmt.Errorf("事件处理器未初始化")
	}

	return m.eventHandler.HandleNetworkQualityChanged(eventData)
}

// ============================================================================
//                              编译时检查
// ============================================================================

// ============================================================================
//                           内存监控和优化方法
// ============================================================================

// MonitorMemoryUsage 监控内存使用情况并返回统计信息
func (m *Manager) MonitorMemoryUsage() map[string]interface{} {
	snapshot := GetMemorySnapshot()

	return map[string]interface{}{
		"heap_alloc_mb": snapshot.HeapAllocMB,
		"rss_mb":        snapshot.RSSMB,
		"heap_inuse_mb": snapshot.HeapInuseMB,
		"heap_sys_mb":   snapshot.HeapSysMB,
		"heap_idle_mb":  snapshot.HeapIdleMB,
		"heap_objects":  snapshot.HeapObjects,
		"num_gc":        snapshot.NumGC,
	}
}

// TriggerMemoryOptimization 触发内存优化
func (m *Manager) TriggerMemoryOptimization() {
	snapshotBefore := GetMemorySnapshot()

	// 强制垃圾回收
	runtime.GC()
	runtime.GC() // 执行两次GC确保彻底清理

	snapshotAfter := GetMemorySnapshot()

	if m.logger != nil {
		m.logger.Infof("🧹 内存优化完成: "+
			"heap_alloc=%dMB->%dMB rss=%dMB->%dMB "+
			"(heap节省=%dMB, rss节省=%dMB)",
			snapshotBefore.HeapAllocMB, snapshotAfter.HeapAllocMB,
			snapshotBefore.RSSMB, snapshotAfter.RSSMB,
			snapshotBefore.HeapAllocMB-snapshotAfter.HeapAllocMB,
			int64(snapshotBefore.RSSMB)-int64(snapshotAfter.RSSMB))
	}
}

// CheckMemoryPressure 检查内存压力并返回是否需要优化
func (m *Manager) CheckMemoryPressure() bool {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 🔧 修复：从配置系统获取内存压力阈值，移除硬编码
	var memoryPressureThreshold int64 = 500 * 1024 * 1024 // 默认500MB
	if m.configProvider != nil {
		syncOpts := m.configProvider.GetSync()
		if syncOpts != nil {
			// SyncOptions 包含 MemoryPressureThreshold 字段
			if syncOpts.MemoryPressureThreshold > 0 {
				memoryPressureThreshold = syncOpts.MemoryPressureThreshold
			}
		}
	}

	return memStats.Alloc > uint64(memoryPressureThreshold)
}

// 编译时检查接口实现
var _ interfaces.InternalSyncService = (*Manager)(nil)
