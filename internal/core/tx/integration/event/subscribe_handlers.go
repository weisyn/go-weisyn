// Package event 区块链事件订阅处理器
//
// 🎯 **事件订阅集成层**
//
// 本文件定义区块链模块的事件订阅接口，参考consensus模块的设计模式。
// 区块链模块按照子模块职责分工处理事件：
// - sync子模块：处理分叉、同步、网络质量相关事件
// - transaction子模块：处理交易生命周期、UTXO状态相关事件
//
// 🏗️ **正确的架构设计**：
// - 子模块专责：sync和transaction各自处理相关事件
// - 接口清晰：每个子模块有独立的事件处理器
// - 统一注册：通过RegisterEventSubscriptions注册所有订阅
// - 依赖注入：支持测试和模块替换
package event

import (
	"fmt"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 子模块事件订阅接口 ====================

// TransactionEventSubscriber transaction子模块事件订阅接口
//
// 💰 **交易模块事件处理**：
// transaction子模块专门处理与交易生命周期相关的事件：
// - 交易接收/验证/执行/确认/失败事件（基于 pkg/constants/events/system_events.go）
// - UTXO状态变化事件
// - 内存池交易相关事件
//
// 由 transaction/event_handler 包实现具体业务逻辑
type TransactionEventSubscriber interface {
	// 交易生命周期事件处理（基于 pkg/constants/events/system_events.go 定义）

	// HandleTransactionReceived 处理交易接收事件
	// 对应事件类型：EventTypeTransactionReceived = "blockchain.transaction.received"
	HandleTransactionReceived(eventData *types.TransactionReceivedEventData) error

	// HandleTransactionValidated 处理交易验证事件
	// 对应事件类型：EventTypeTransactionValidated = "blockchain.transaction.validated"
	HandleTransactionValidated(eventData *types.TransactionValidatedEventData) error

	// HandleTransactionExecuted 处理交易执行事件
	// 对应事件类型：EventTypeTransactionExecuted = "blockchain.transaction.executed"
	HandleTransactionExecuted(eventData *types.TransactionExecutedEventData) error

	// HandleTransactionFailed 处理交易失败事件
	// 对应事件类型：EventTypeTransactionFailed = "blockchain.transaction.failed"
	HandleTransactionFailed(eventData *types.TransactionFailedEventData) error

	// HandleTransactionConfirmed 处理交易确认事件
	// 对应事件类型：EventTypeTransactionConfirmed = "blockchain.transaction.confirmed"
	HandleTransactionConfirmed(eventData *types.TransactionConfirmedEventData) error

	// 内存池相关事件处理

	// HandleMempoolTransactionAdded 处理交易添加到池事件
	// 对应事件类型：EventTypeTxAdded = "mempool.tx.added"
	// 注意：使用 TransactionReceivedEventData 作为添加事件的数据结构
	HandleMempoolTransactionAdded(eventData *types.TransactionReceivedEventData) error

	// HandleMempoolTransactionRemoved 处理交易从池移除事件
	// 对应事件类型：EventTypeTxRemoved = "mempool.tx.removed"
	HandleMempoolTransactionRemoved(eventData *types.TransactionRemovedEventData) error
}

// SyncEventSubscriber sync子模块事件订阅接口
//
// 🔄 **同步模块事件处理**：
// sync子模块专门处理与区块同步相关的事件：
// - 分叉检测/处理/完成事件
// - 网络质量变化事件
// - 共识结果对同步策略的影响
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

// EventSubscriptionRegistry 区块链事件订阅注册器
//
// 🎯 **统一事件订阅管理**：
// 负责管理blockchain模块内所有子模块的事件订阅：
// - transaction子模块的交易生命周期和内存池事件订阅
// - sync子模块的分叉和网络事件订阅
// - 统一的订阅注册和取消管理
type EventSubscriptionRegistry struct {
	eventBus              event.EventBus
	logger                log.Logger
	transactionSubscriber TransactionEventSubscriber
	syncSubscriber        SyncEventSubscriber
	subscriptionIDs       []types.SubscriptionID // 订阅ID列表，用于取消订阅
}

// NewEventSubscriptionRegistry 创建事件订阅注册器
func NewEventSubscriptionRegistry(
	eventBus event.EventBus,
	logger log.Logger,
	transactionSubscriber TransactionEventSubscriber,
	syncSubscriber SyncEventSubscriber,
) *EventSubscriptionRegistry {
	return &EventSubscriptionRegistry{
		eventBus:              eventBus,
		logger:                logger,
		transactionSubscriber: transactionSubscriber,
		syncSubscriber:        syncSubscriber,
	}
}

// RegisterEventSubscriptions 注册所有事件订阅
//
// 🔧 **统一订阅注册**：
// 按子模块注册相关事件订阅：
// 1. 注册transaction子模块相关事件
// 2. 注册sync子模块相关事件
// 3. 记录订阅ID以便后续管理
//
// @return error 注册过程中的错误
func (r *EventSubscriptionRegistry) RegisterEventSubscriptions() error {
	// 注册transaction子模块事件
	if r.transactionSubscriber != nil {
		if err := r.registerTransactionEvents(); err != nil {
			// 注册失败时清理已注册的订阅
			r.UnregisterEventSubscriptions()
			return fmt.Errorf("transaction事件注册失败: %w", err)
		}
	}

	// 注册sync子模块事件
	if r.syncSubscriber != nil {
		if err := r.registerSyncEvents(); err != nil {
			// 注册失败时清理已注册的订阅
			r.UnregisterEventSubscriptions()
			return fmt.Errorf("sync事件注册失败: %w", err)
		}
	}

	if r.logger != nil {
		r.logger.Infof("[TxEvents] ✅ 交易事件订阅注册完成，共 %d 个订阅", len(r.subscriptionIDs))
	}

	return nil
}

// registerTransactionEvents 注册transaction子模块相关事件
func (r *EventSubscriptionRegistry) registerTransactionEvents() error {
	// transaction子模块关心的事件映射（基于 pkg/constants/events/system_events.go）
	transactionEvents := map[event.EventType]interface{}{
		// 交易生命周期事件
		eventconstants.EventTypeTransactionReceived:  r.transactionSubscriber.HandleTransactionReceived,
		eventconstants.EventTypeTransactionValidated: r.transactionSubscriber.HandleTransactionValidated,
		eventconstants.EventTypeTransactionExecuted:  r.transactionSubscriber.HandleTransactionExecuted,
		eventconstants.EventTypeTransactionFailed:    r.transactionSubscriber.HandleTransactionFailed,
		eventconstants.EventTypeTransactionConfirmed: r.transactionSubscriber.HandleTransactionConfirmed,

		// 内存池交易事件
		eventconstants.EventTypeTxAdded:   r.transactionSubscriber.HandleMempoolTransactionAdded,
		eventconstants.EventTypeTxRemoved: r.transactionSubscriber.HandleMempoolTransactionRemoved,
	}

	for eventType, handler := range transactionEvents {
		err := r.eventBus.Subscribe(eventType, handler)
		if err != nil {
			return fmt.Errorf("订阅transaction事件 %s 失败: %w", eventType, err)
		}

		if r.logger != nil {
			r.logger.Infof("[TxEvents] 📝 已订阅transaction事件: %s", eventType)
		}
	}

	return nil
}

// registerSyncEvents 注册sync子模块相关事件
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
			r.logger.Infof("[BlockchainEvents] 📝 已订阅sync事件: %s", eventType)
		}
	}

	return nil
}

// UnregisterEventSubscriptions 取消所有事件订阅
//
// 🔧 **清理订阅**：
// 取消blockchain模块的所有事件订阅，通常在模块关闭时调用
//
// @return error 取消订阅过程中的错误
func (r *EventSubscriptionRegistry) UnregisterEventSubscriptions() error {
	var allErrors []error

	// 逐个取消订阅
	for _, subscriptionID := range r.subscriptionIDs {
		if err := r.eventBus.UnsubscribeByID(subscriptionID); err != nil {
			allErrors = append(allErrors, fmt.Errorf("取消订阅 %s 失败: %w", subscriptionID, err))
		}
	}

	// 清空订阅ID列表
	r.subscriptionIDs = nil

	if len(allErrors) > 0 {
		return fmt.Errorf("取消区块链事件订阅失败: %v", allErrors)
	}

	if r.logger != nil {
		r.logger.Infof("[BlockchainEvents] 🧹 区块链事件订阅已全部取消")
	}

	return nil
}
