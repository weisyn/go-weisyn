// Package event Chain 模块事件订阅处理器
//
// 🎯 **事件订阅集成层**
//
// 本文件定义 Chain 模块的事件订阅接口，负责处理来自其他模块的事件通知。
// Chain 模块主要关注：
// - 区块处理完成事件：自动更新链尖状态
// - 分叉检测事件：自动触发分叉处理逻辑
//
// 🏗️ **架构设计**：
// - 事件驱动：通过事件总线实现模块间解耦通信
// - 非阻塞处理：事件处理器异步执行，不阻塞发布方
// - 错误隔离：单个事件处理失败不影响其他事件
// - 统一注册：通过 RegisterEventSubscriptions 统一管理订阅
package event

import (
	"context"
	"fmt"
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"

	"github.com/weisyn/v1/internal/core/chain/interfaces"
)

// ==================== 子模块事件订阅接口 ====================

// SyncEventSubscriber sync子模块事件订阅接口
//
// 🔄 **同步模块事件处理**：
// sync子模块专门处理与区块同步相关的事件：
// - 分叉检测/处理/完成事件
// - 网络质量变化事件
//
// 由 sync/event_handler 包实现具体业务逻辑
type SyncEventSubscriber interface {
	// HandleForkDetected 处理分叉检测事件
	HandleForkDetected(eventData *types.ForkDetectedEventData) error

	// HandleForkProcessing 处理分叉处理中事件
	HandleForkProcessing(eventData *types.ForkProcessingEventData) error

	// HandleForkCompleted 处理分叉完成事件
	HandleForkCompleted(eventData *types.ForkCompletedEventData) error

	// HandleNetworkQualityChanged 处理网络质量变化事件
	HandleNetworkQualityChanged(eventData *types.NetworkQualityChangedEventData) error
}

// ==================== 事件订阅注册器 ====================

// EventSubscriptionRegistry Chain 模块事件订阅注册器
//
// 🎯 **职责**：
// - 注册 Chain 模块关心的所有事件订阅
// - 路由事件到对应的处理器
// - 管理事件处理的生命周期
//
// 📊 **订阅的事件**：
// 1. EventTypeBlockProcessed: 区块处理完成 → 记录日志（链尖已在DataWriter事务中更新）
// 2. EventTypeForkDetected: 分叉检测 → 触发分叉处理
// 3. Sync相关事件（ForkDetected/ForkProcessing/ForkCompleted/NetworkQualityChanged）→ sync服务处理
type EventSubscriptionRegistry struct {
	eventBus       event.EventBus
	logger         log.Logger
	forkHandler    interfaces.InternalForkHandler
	syncSubscriber SyncEventSubscriber // sync服务的事件订阅器（可选）
	queryService   persistence.QueryService

	// peerConnectedSyncDebouncer 用于将“短时间内大量 peer.connected”合并为一次同步触发，
	// 避免生产环境出现同步风暴/大量 goroutine。
	peerConnectedMu    sync.Mutex
	peerConnectedTimer *time.Timer
	peerConnectedLast  peer.ID
	// peerConnectedLastTriggerAt 限制 peer.connected 触发同步的最小间隔（生产保护）
	peerConnectedLastTriggerAt time.Time
}

// NewEventSubscriptionRegistry 创建事件订阅注册器
//
// 参数：
//   - eventBus: 事件总线
//   - logger: 日志服务
//   - forkHandler: 分叉处理服务（处理分叉逻辑）
//   - syncSubscriber: sync服务的事件订阅器（可选，处理同步相关事件）
//   - queryService: 查询服务（用于获取分叉区块）
func NewEventSubscriptionRegistry(
	eventBus event.EventBus,
	logger log.Logger,
	forkHandler interfaces.InternalForkHandler,
	syncSubscriber SyncEventSubscriber,
	queryService persistence.QueryService,
) *EventSubscriptionRegistry {
	return &EventSubscriptionRegistry{
		eventBus:       eventBus,
		logger:         logger,
		forkHandler:    forkHandler,
		syncSubscriber: syncSubscriber,
		queryService:   queryService,
	}
}

// RegisterEventSubscriptions 注册所有事件订阅
//
// 🔄 **注册流程**：
// 1. 订阅区块处理完成事件
// 2. 订阅分叉检测事件
// 3. 注册sync服务的事件订阅（如果syncSubscriber存在）
// 4. 记录注册结果
//
// 返回：
//   - error: 订阅失败时返回错误
func (r *EventSubscriptionRegistry) RegisterEventSubscriptions() error {
	// 1. 订阅区块处理完成事件
	// 🔧 使用异步订阅避免启动时的死锁（创世区块处理时会发布此事件）
	if err := r.eventBus.SubscribeAsync(eventconstants.EventTypeBlockProcessed, r.onBlockProcessed, false); err != nil {
		if r.logger != nil {
			r.logger.Errorf("订阅 BlockProcessed 事件失败: %v", err)
		}
		return fmt.Errorf("订阅 BlockProcessed 事件失败: %w", err)
	}

	// 1.5 订阅网络 peer 连接事件：用于触发“连接后同步检查”
	// 说明：
	// - P2P 层已发布 network.peer.connected（见 internal/core/p2p/host/network_notifiee.go）
	// - 这里在 Chain 模块接到事件后触发一次 TriggerSync（debounce 合并），修复“连上了但不触发同步”的缺陷
	if err := r.eventBus.SubscribeAsync(event.EventTypeNetworkPeerConnected, r.onNetworkPeerConnected, false); err != nil {
		if r.logger != nil {
			r.logger.Errorf("订阅 NetworkPeerConnected 事件失败: %v", err)
		}
		return fmt.Errorf("订阅 NetworkPeerConnected 事件失败: %w", err)
	}

	// 2. 订阅分叉检测事件
	// 🔧 使用异步订阅避免事件处理阻塞
	if err := r.eventBus.SubscribeAsync(eventconstants.EventTypeForkDetected, r.onForkDetected, false); err != nil {
		if r.logger != nil {
			r.logger.Errorf("订阅 ForkDetected 事件失败: %v", err)
		}
		return fmt.Errorf("订阅 ForkDetected 事件失败: %w", err)
	}

	// 3. 注册sync服务的事件订阅（如果syncSubscriber存在）
	if r.syncSubscriber != nil {
		if err := r.registerSyncEvents(); err != nil {
			if r.logger != nil {
				r.logger.Errorf("注册sync事件订阅失败: %v", err)
			}
			return fmt.Errorf("注册sync事件订阅失败: %w", err)
		}
	}

	if r.logger != nil {
		r.logger.Info("✅ Chain 模块事件订阅已注册")
	}

	return nil
}

// onNetworkPeerConnected 处理网络节点连接事件
//
// 🎯 目的：
// - 节点刚连接成功时（mDNS/DHT/Bootstrap/Dial 等），立即触发一次同步检查；
// - 采用 debounce 将多个连接事件合并，避免生产环境同步风暴；
// - 不使用 peer hint 绕过 K 桶过滤，确保只从“已进入路由表的 WES 节点”参与同步选择。
func (r *EventSubscriptionRegistry) onNetworkPeerConnected(ctx context.Context, data interface{}) error {
	peerID, ok := data.(peer.ID)
	if !ok || peerID == "" {
		return nil
	}

	// 日志：使用 Debug 避免在主网刷屏
	if r.logger != nil {
		r.logger.Debugf("[ChainEvents] 🌐 network.peer.connected: %s", peerID)
	}

	// 如果没有注入 syncSubscriber，无法触发同步
	if r.syncSubscriber == nil {
		return nil
	}

	// 通过接口方式调用 TriggerSync / CheckSync（避免强耦合）
	syncCtl, ok := r.syncSubscriber.(interface {
		TriggerSync(context.Context) error
		CheckSync(context.Context) (*types.SystemSyncStatus, error)
	})
	if !ok {
		return nil
	}

	// 生产环境：合并短时间内的多次连接事件，仅触发一次同步
	r.schedulePeerConnectedSync(peerID, syncCtl)

	return nil
}

func (r *EventSubscriptionRegistry) schedulePeerConnectedSync(peerID peer.ID, syncCtl interface {
	TriggerSync(context.Context) error
	CheckSync(context.Context) (*types.SystemSyncStatus, error)
}) {
	const debounce = 800 * time.Millisecond

	r.peerConnectedMu.Lock()
	r.peerConnectedLast = peerID
	if r.peerConnectedTimer == nil {
		// 第一次：创建定时器
		r.peerConnectedTimer = time.AfterFunc(debounce, func() {
			r.runPeerConnectedSync(syncCtl)
		})
		r.peerConnectedMu.Unlock()
		return
	}

	// 之后：重置定时器（合并多次触发）
	if !r.peerConnectedTimer.Stop() {
		// timer 可能已经触发或正在触发；不强求 drain，下一次 run 也会被 TriggerSync 内部锁保护
	}
	r.peerConnectedTimer.Reset(debounce)
	r.peerConnectedMu.Unlock()
}

func (r *EventSubscriptionRegistry) runPeerConnectedSync(syncCtl interface {
	TriggerSync(context.Context) error
	CheckSync(context.Context) (*types.SystemSyncStatus, error)
}) {
	const minInterval = 10 * time.Second

	// 生产保护：最小触发间隔（避免连接抖动导致频繁触发）
	r.peerConnectedMu.Lock()
	lastAt := r.peerConnectedLastTriggerAt
	lastPeer := r.peerConnectedLast
	if !lastAt.IsZero() && time.Since(lastAt) < minInterval {
		r.peerConnectedMu.Unlock()
		if r.logger != nil {
			r.logger.Debugf("[ChainEvents] peer-connected sync skip: reason=min-interval last_peer=%s interval=%s", lastPeer, minInterval)
		}
		return
	}
	r.peerConnectedMu.Unlock()

	// 若当前同步状态已是“无需同步/正在同步”，则跳过（减少无意义网络请求）
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
	status, _ := syncCtl.CheckSync(checkCtx)
	checkCancel()
	if status != nil {
		switch status.Status {
		case types.SyncStatusSynced, types.SyncStatusIdle, types.SyncStatusSyncing:
			if r.logger != nil {
				r.logger.Debugf("[ChainEvents] peer-connected sync skip: reason=status=%s current=%d network=%d",
					status.Status.String(), status.CurrentHeight, status.NetworkHeight)
			}
			return
		}
	}

	// 给同步一个有限超时，避免异常情况下长时间挂起
	syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 记录触发时间（仅对真正触发计数）
	r.peerConnectedMu.Lock()
	r.peerConnectedLastTriggerAt = time.Now()
	r.peerConnectedMu.Unlock()

	if err := syncCtl.TriggerSync(syncCtx); err != nil {
		if r.logger != nil {
			r.peerConnectedMu.Lock()
			last := r.peerConnectedLast
			r.peerConnectedMu.Unlock()
			r.logger.Debugf("[ChainEvents] peer-connected debounce 触发同步失败: last_peer=%s err=%v", last, err)
		}
	}
}

// registerSyncEvents 注册sync服务相关事件
//
// 🎯 **sync事件订阅**：
// 注册sync子模块关心的事件，包括：
// - ForkDetected: 分叉检测事件
// - ForkProcessing: 分叉处理中事件
// - ForkCompleted: 分叉完成事件
// - NetworkQualityChanged: 网络质量变化事件
func (r *EventSubscriptionRegistry) registerSyncEvents() error {
	// sync子模块关心的事件映射
	syncEvents := map[event.EventType]interface{}{
		// 分叉相关事件
		eventconstants.EventTypeForkDetected:   r.syncSubscriber.HandleForkDetected,
		eventconstants.EventTypeForkProcessing: r.syncSubscriber.HandleForkProcessing,
		eventconstants.EventTypeForkCompleted:  r.syncSubscriber.HandleForkCompleted,

		// 网络质量事件
		eventconstants.EventTypeNetworkQualityChanged: r.syncSubscriber.HandleNetworkQualityChanged,
	}

	for eventType, handler := range syncEvents {
		err := r.eventBus.Subscribe(eventType, handler)
		if err != nil {
			return fmt.Errorf("订阅sync事件 %s 失败: %w", eventType, err)
		}

		if r.logger != nil {
			r.logger.Infof("[ChainEvents] 📝 已订阅sync事件: %s", eventType)
		}
	}

	return nil
}

// ==================== 事件处理器 ====================

// onBlockProcessed 处理区块处理完成事件
//
// 🎯 **事件来源**：Block 模块（区块处理器）
//
// 📋 **处理逻辑**：
// 1. 提取区块高度和哈希
// 2. 调用 ChainWriter 更新链尖状态
// 3. 记录处理结果
//
// 🔒 **错误处理**：
// - 数据格式错误：记录错误日志，返回错误
// - 更新失败：记录错误日志，返回错误
//
// ⚠️ **注意事项**：
// - 本方法由事件总线异步调用，不应阻塞
// - 处理失败不影响区块处理本身（区块已处理完成）
func (r *EventSubscriptionRegistry) onBlockProcessed(ctx context.Context, data interface{}) error {
	// 1. 类型断言：提取事件数据
	eventData, ok := data.(*types.BlockProcessedEventData)
	if !ok {
		err := fmt.Errorf("BlockProcessed 事件数据类型错误: %T", data)
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	// 2. 验证事件数据
	// 注意：Height 为 0 是合法的（创世区块），所以不需要检查 Height == 0
	// 只需要检查 Hash 是否为空，因为每个区块都应该有哈希
	if eventData.Hash == "" {
		err := fmt.Errorf("BlockProcessed 事件数据不完整: Hash 为空")
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	blockHeight := eventData.Height
	blockHash := []byte(eventData.Hash)

	if r.logger != nil {
		if len(blockHash) >= 8 {
			r.logger.Debugf("📥 收到 BlockProcessed 事件: 高度=%d, 哈希=%x", blockHeight, blockHash[:8])
		} else {
			r.logger.Debugf("📥 收到 BlockProcessed 事件: 高度=%d", blockHeight)
		}
	}

	// 3. 不再更新链尖状态（DataWriter.WriteBlock()已经在事务中更新过了）
	// ❌ 移除：链尖更新（DataWriter已经更新过了）
	// 根据架构原则，链尖状态必须在DataWriter.WriteBlock()的事务中更新，
	// 事件处理器只负责其他业务逻辑（如日志、通知等），不应修改核心链状态。

	if r.logger != nil {
		r.logger.Infof("✅ BlockProcessed 事件处理完成: 区块高度=%d（链尖已在DataWriter事务中更新）", blockHeight)
	}

	return nil
}

// onForkDetected 处理分叉检测事件
//
// 🎯 **事件来源**：Block 模块（分叉检测器）或 Sync 模块
//
// 📋 **处理逻辑**：
// 1. 提取分叉区块信息
// 2. 调用 ForkHandler 处理分叉
// 3. 记录处理结果
//
// 🔒 **错误处理**：
// - 数据格式错误：记录错误日志，返回错误
// - 分叉处理失败：记录错误日志，返回错误
//
// ⚠️ **注意事项**：
// - 分叉处理可能涉及链重组，耗时较长
// - 如需避免阻塞，可考虑异步处理或排队机制
func (r *EventSubscriptionRegistry) onForkDetected(ctx context.Context, data interface{}) error {
	// 1. 类型断言：提取事件数据
	eventData, ok := data.(*types.ForkDetectedEventData)
	if !ok {
		err := fmt.Errorf("ForkDetected 事件数据类型错误: %T", data)
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	// 2. 验证事件数据
	if eventData.Height == 0 {
		err := fmt.Errorf("ForkDetected 事件数据不完整: Height 为空")
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	forkHeight := eventData.Height
	forkBlockHash := []byte(eventData.ForkBlockHash)

	if r.logger != nil {
		if len(forkBlockHash) >= 8 {
			r.logger.Warnf("📥 收到 ForkDetected 事件: 分叉高度=%d, 分叉区块哈希=%x",
				forkHeight, forkBlockHash[:8])
		} else {
			r.logger.Warnf("📥 收到 ForkDetected 事件: 分叉高度=%d",
				forkHeight)
		}
	}

	// 3. 通过查询服务获取分叉区块
	if r.queryService == nil {
		err := fmt.Errorf("QueryService 未注入，无法获取分叉区块")
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	// 尝试通过区块哈希获取区块
	forkBlock, err := r.queryService.GetBlockByHash(ctx, forkBlockHash)
	if err != nil {
		// 如果通过哈希获取失败，尝试通过高度获取
		if r.logger != nil {
			r.logger.Debugf("通过哈希获取分叉区块失败，尝试通过高度获取: %v", err)
		}
		forkBlock, err = r.queryService.GetBlockByHeight(ctx, forkHeight)
		if err != nil {
			err := fmt.Errorf("获取分叉区块失败 (高度=%d, 哈希=%x): %w", forkHeight, forkBlockHash[:min(8, len(forkBlockHash))], err)
			if r.logger != nil {
				r.logger.Errorf("❌ %v", err)
			}
			return err
		}
	}

	if forkBlock == nil {
		err := fmt.Errorf("分叉区块不存在 (高度=%d)", forkHeight)
		if r.logger != nil {
			r.logger.Errorf("❌ %v", err)
		}
		return err
	}

	// 4. 调用 ForkHandler 处理分叉
	if err := r.forkHandler.HandleFork(ctx, forkBlock); err != nil {
		if r.logger != nil {
			r.logger.Errorf("❌ 处理 ForkDetected 事件失败（分叉处理失败）: %v", err)
		}
		return fmt.Errorf("分叉处理失败: %w", err)
	}

	if r.logger != nil {
		r.logger.Infof("✅ ForkDetected 事件处理完成: 分叉已处理")
	}

	return nil
}

// ==================== 辅助函数 ====================

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
