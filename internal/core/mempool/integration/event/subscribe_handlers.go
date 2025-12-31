// Package event 内存池事件集成
//
// 🎯 **内存池事件订阅接口标准化**
//
// 本文件定义内存池组件的事件订阅接口，参考consensus、blockchain、execution和repositories模块的标准模式：
// - 定义MempoolEventSubscriber等订阅接口
// - 提供统一的事件订阅注册机制
// - 支持交易池和候选区块池的事件处理
//
// 🏗️ **设计原则**：
// - 接口导向：定义清晰的订阅接口约定
// - 事件分类：按功能领域划分订阅接口
// - 类型安全：使用强类型事件常量
// - 解耦设计：事件处理与业务逻辑分离
package event

import (
	"context"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 事件订阅注册管理器 ====================

// EventSubscriptionRegistry 事件订阅注册管理器
//
// 🎯 **统一事件订阅管理**：
// 负责管理所有内存池相关的事件订阅，提供统一的注册和注销接口
type EventSubscriptionRegistry struct {
	eventBus event.EventBus
	logger   log.Logger
}

// NewEventSubscriptionRegistry 创建事件订阅注册管理器
func NewEventSubscriptionRegistry(eventBus event.EventBus, logger log.Logger) *EventSubscriptionRegistry {
	return &EventSubscriptionRegistry{
		eventBus: eventBus,
		logger:   logger,
	}
}

// RegisterEventSubscriptions 注册所有内存池事件订阅
//
// 🎯 **统一订阅注册**：
// 将各个订阅者接口的处理方法注册到事件总线
func (r *EventSubscriptionRegistry) RegisterEventSubscriptions(
	mempoolSubscriber MempoolEventSubscriber,
	txPoolSubscriber TxPoolEventSubscriber,
	candidatePoolSubscriber CandidatePoolEventSubscriber,
) error {
	if r.eventBus == nil {
		r.logger.Warn("EventBus未配置，跳过事件订阅注册")
		return nil
	}

	// 注册内存池通用事件
	if err := r.registerMempoolEvents(mempoolSubscriber); err != nil {
		return err
	}

	// 注册交易池事件
	if err := r.registerTxPoolEvents(txPoolSubscriber); err != nil {
		return err
	}

	// 注册候选区块池事件
	if err := r.registerCandidatePoolEvents(candidatePoolSubscriber); err != nil {
		return err
	}

	r.logger.Info("内存池事件订阅注册完成")
	return nil
}

// ==================== 内存池通用事件订阅接口 ====================

// MempoolEventSubscriber 内存池通用事件订阅接口
//
// 🎯 **内存池通用事件处理**：
// 处理系统级别的内存池相关事件，如系统停止、网络变化等
type MempoolEventSubscriber interface {
	// HandleSystemStopping 处理系统停止事件
	HandleSystemStopping(ctx context.Context, eventData *types.SystemStoppingEventData) error

	// HandleNetworkQualityChanged 处理网络质量变化事件
	HandleNetworkQualityChanged(ctx context.Context, eventData *types.NetworkQualityChangedEventData) error

	// HandleBlockProcessed 处理区块处理完成事件
	HandleBlockProcessed(ctx context.Context, eventData *types.BlockProcessedEventData) error

	// HandleChainReorganized 处理链重组事件
	HandleChainReorganized(ctx context.Context, eventData *types.ChainReorganizedEventData) error

	// HandleConsensusResultBroadcast 处理共识结果广播事件
	HandleConsensusResultBroadcast(ctx context.Context, eventData *types.ConsensusResultEventData) error
}

// ==================== 交易池事件订阅接口 ====================

// TxPoolEventSubscriber 交易池事件订阅接口
//
// 🎯 **交易池事件处理**：
// 处理交易池相关的事件，如交易添加、移除、确认等
type TxPoolEventSubscriber interface {
	// HandleResourceExhausted 处理资源耗尽事件
	HandleResourceExhausted(ctx context.Context, eventData *types.ResourceExhaustedEventData) error

	// HandleMemoryPressure 处理内存压力事件
	HandleMemoryPressure(ctx context.Context, eventData *types.MemoryPressureEventData) error

	// HandleTransactionReceived 处理交易接收事件
	HandleTransactionReceived(ctx context.Context, eventData *types.TransactionReceivedEventData) error

	// HandleTransactionFailed 处理交易失败事件
	HandleTransactionFailed(ctx context.Context, eventData *types.TransactionFailedEventData) error

	// HandleForkDetected 处理分叉检测事件
	HandleForkDetected(ctx context.Context, eventData *types.ForkDetectedEventData) error
}

// ==================== 候选区块池事件订阅接口 ====================

// CandidatePoolEventSubscriber 候选区块池事件订阅接口
//
// 🎯 **候选区块池事件处理**：
// 处理候选区块池相关的事件，如候选区块添加、移除、过期等
type CandidatePoolEventSubscriber interface {
	// HandleBlockProduced 处理区块生产事件
	HandleBlockProduced(ctx context.Context, eventData *types.BlockProducedEventData) error

	// HandleConsensusStateChanged 处理共识状态变化事件
	HandleConsensusStateChanged(ctx context.Context, eventData *types.ConsensusStateChangedEventData) error

	// HandleResourceExhausted 处理资源耗尽事件
	HandleResourceExhausted(ctx context.Context, eventData *types.ResourceExhaustedEventData) error

	// HandleStorageSpaceLow 处理存储空间不足事件
	HandleStorageSpaceLow(ctx context.Context, eventData *types.StorageSpaceLowEventData) error

	// HandleSystemStopping 处理系统停止事件
	HandleSystemStopping(ctx context.Context, eventData *types.SystemStoppingEventData) error
}

// ==================== 事件订阅注册实现 ====================

// registerMempoolEvents 注册内存池通用事件
func (r *EventSubscriptionRegistry) registerMempoolEvents(subscriber MempoolEventSubscriber) error {
	// 🔧 使用异步订阅避免事件处理阻塞启动流程
	// BlockProcessed 等事件在启动时就会触发，如果使用同步订阅会导致死锁
	events := map[eventconstants.EventType]interface{}{
		eventconstants.EventTypeSystemStopping:           subscriber.HandleSystemStopping,
		eventconstants.EventTypeNetworkQualityChanged:    subscriber.HandleNetworkQualityChanged,
		eventconstants.EventTypeBlockProcessed:           subscriber.HandleBlockProcessed,
		eventconstants.EventTypeChainReorganized:         subscriber.HandleChainReorganized,
		eventconstants.EventTypeConsensusResultBroadcast: subscriber.HandleConsensusResultBroadcast,
	}

	for eventType, handler := range events {
		// 使用异步订阅，transactional=false（不需要事务保证）
		err := r.eventBus.SubscribeAsync(eventType, handler, false)
		if err != nil {
			r.logger.Errorf("注册内存池事件 %s 失败: %v", eventType, err)
			return err
		}
		r.logger.Debugf("注册内存池事件 %s 成功（异步订阅）", eventType)
	}

	return nil
}

// registerTxPoolEvents 注册交易池事件
func (r *EventSubscriptionRegistry) registerTxPoolEvents(subscriber TxPoolEventSubscriber) error {
	events := map[eventconstants.EventType]interface{}{
		eventconstants.EventTypeResourceExhausted:   subscriber.HandleResourceExhausted,
		eventconstants.EventTypeMempoolPressureHigh: subscriber.HandleMemoryPressure,
		eventconstants.EventTypeTransactionReceived: subscriber.HandleTransactionReceived,
		eventconstants.EventTypeTransactionFailed:   subscriber.HandleTransactionFailed,
		eventconstants.EventTypeForkDetected:        subscriber.HandleForkDetected,
	}

	for eventType, handler := range events {
		err := r.eventBus.Subscribe(eventType, handler)
		if err != nil {
			r.logger.Errorf("注册交易池事件 %s 失败: %v", eventType, err)
			return err
		}
		r.logger.Debugf("注册交易池事件 %s 成功", eventType)
	}

	return nil
}

// registerCandidatePoolEvents 注册候选区块池事件
func (r *EventSubscriptionRegistry) registerCandidatePoolEvents(subscriber CandidatePoolEventSubscriber) error {
	events := map[eventconstants.EventType]interface{}{
		eventconstants.EventTypeBlockProduced:         subscriber.HandleBlockProduced,
		eventconstants.EventTypeConsensusStateChanged: subscriber.HandleConsensusStateChanged,
		eventconstants.EventTypeResourceExhausted:     subscriber.HandleResourceExhausted,
		eventconstants.EventTypeStorageSpaceLow:       subscriber.HandleStorageSpaceLow,
		eventconstants.EventTypeSystemStopping:        subscriber.HandleSystemStopping,
	}

	for eventType, handler := range events {
		err := r.eventBus.Subscribe(eventType, handler)
		if err != nil {
			r.logger.Errorf("注册候选区块池事件 %s 失败: %v", eventType, err)
			return err
		}
		r.logger.Debugf("注册候选区块池事件 %s 成功", eventType)
	}

	return nil
}
