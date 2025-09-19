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

	// 类型定义
	"github.com/weisyn/v1/pkg/types"

	// 接口依赖
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	// 配置

	// 业务模块
	"github.com/weisyn/v1/internal/core/blockchain/sync/event_handler"
	"github.com/weisyn/v1/internal/core/blockchain/sync/network_handler"

	// 集成层
	eventIntegration "github.com/weisyn/v1/internal/core/blockchain/integration/event"

	// libp2p依赖
	peer "github.com/libp2p/go-libp2p/core/peer"
)

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
	chainService      interfaces.InternalChainService // 链状态服务（内部接口）
	blockService      blockchain.BlockService         // 区块服务（验证和处理区块）
	repositoryManager repository.RepositoryManager    // 数据存储管理器（区块查询，只读访问）
	networkService    network.Network                 // 网络服务（P2P通信）
	kBucketManager    kademlia.RoutingTableManager    // K桶管理器（路由表管理）
	host              node.Host                       // 节点主机服务（获取节点ID、验证节点）
	configProvider    config.Provider                 // 配置提供者（标准接口）
	logger            log.Logger                      // 日志记录器

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
// 参数：
//   - chainService: 链状态服务（内部接口）
//   - blockService: 区块服务（验证和处理区块）
//   - repositoryManager: 数据存储管理器（区块查询）
//   - networkService: 网络服务（P2P通信）
//   - kBucketManager: K桶管理器（路由表管理）
//   - host: 节点主机服务（获取节点ID、验证节点）
//   - configProvider: 配置提供者（标准接口）
//   - logger: 日志记录器
//
// 返回：
//   - interfaces.InternalSystemSyncService: 内部同步服务接口
func NewManager(
	chainService interfaces.InternalChainService,
	blockService blockchain.BlockService,
	repositoryManager repository.RepositoryManager,
	networkService network.Network,
	kBucketManager kademlia.RoutingTableManager,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) interfaces.InternalSystemSyncService {
	// 创建网络协议处理器（传入repositoryManager以支持区块查询）
	networkHandler := network_handler.NewSyncNetworkHandler(logger, chainService, repositoryManager, configProvider)

	// 创建事件处理器
	eventHandler := event_handler.NewSyncEventHandler(logger)

	// 创建定时同步调度器
	periodicScheduler := NewPeriodicSyncScheduler(
		chainService, blockService, kBucketManager,
		networkService, host, configProvider, logger,
	)

	// 创建Manager实例
	manager := &Manager{
		// 基础设施依赖
		chainService:      chainService,
		blockService:      blockService,
		repositoryManager: repositoryManager,
		networkService:    networkService,
		kBucketManager:    kBucketManager,
		host:              host,
		configProvider:    configProvider,
		logger:            logger,

		// 业务子组件
		networkHandler:    networkHandler,
		eventHandler:      eventHandler,
		periodicScheduler: periodicScheduler,
	}

	// 记录初始化日志
	if logger != nil {
		logger.Info("✅ 同步服务薄管理器初始化完成 - 已匹配module.go期望的构造函数签名")
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
		m.logger.Info("收到手动触发同步请求")
	}

	// 委托给具体的同步触发实现，使用标准接口
	return triggerSyncImpl(
		ctx,
		m.chainService,
		m.blockService,
		m.kBucketManager,
		m.networkService,
		m.host,
		m.configProvider,
		m.logger,
	)
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

// CheckSync 检查同步状态
//
// 🎯 **委托实现**：
// 委托给status.go中的具体实现查询当前同步状态。
func (m *Manager) CheckSync(ctx context.Context) (*types.SystemSyncStatus, error) {
	if m.logger != nil {
		m.logger.Debug("收到同步状态查询请求")
	}

	// 委托给具体的状态查询实现
	return checkSyncImpl(
		ctx,
		m.chainService,
		m.kBucketManager,
		m.networkService,
		m.host,
		m.configProvider,
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

// 确保Manager正确实现了所有接口
var _ interfaces.InternalSystemSyncService = (*Manager)(nil)
var _ blockchain.SystemSyncService = (*Manager)(nil)
var _ eventIntegration.SyncEventSubscriber = (*Manager)(nil) // 确保Manager实现了事件订阅接口
