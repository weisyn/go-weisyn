// Package event 提供 Block 模块的事件订阅集成
//
// 🎯 **事件订阅注册器**
//
// 本文件定义了 Block 模块的事件订阅注册器，用于统一管理事件订阅。
// 目前 Block 模块只发布事件，不订阅事件，但为了保持一致性和未来扩展性，
// 提供了事件订阅注册器的框架。
package event

import (
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ==================== 事件订阅注册器 ====================

// EventSubscriptionRegistry 事件订阅注册管理器
//
// 🎯 **统一事件订阅管理**：
// 负责管理 Block 模块的所有事件订阅，提供统一的注册和注销接口。
// 目前 Block 模块主要发布事件（BlockProcessed、ForkDetected），
// 未来可以根据需要添加事件订阅。
type EventSubscriptionRegistry struct {
	eventBus event.EventBus
	logger   log.Logger
}

// NewEventSubscriptionRegistry 创建事件订阅注册管理器
//
// 参数：
//   - eventBus: 事件总线接口
//   - logger: 日志记录器
//
// 返回：
//   - *EventSubscriptionRegistry: 事件订阅注册器实例
func NewEventSubscriptionRegistry(eventBus event.EventBus, logger log.Logger) *EventSubscriptionRegistry {
	return &EventSubscriptionRegistry{
		eventBus: eventBus,
		logger:   logger,
	}
}

// RegisterEventSubscriptions 注册所有 Block 模块事件订阅
//
// 🎯 **统一订阅注册**：
// 目前 Block 模块不订阅任何事件，仅发布事件。
// 此方法保留为未来扩展使用。
//
// 未来可能订阅的事件：
// - EventTypeConsensusResultBroadcast: 共识结果广播（影响区块验证）
// - EventTypeMempoolSizeChanged: 交易池变化（影响候选区块构建）
//
// 返回：
//   - error: 注册失败时的错误
func (r *EventSubscriptionRegistry) RegisterEventSubscriptions() error {
	if r.eventBus == nil {
		if r.logger != nil {
			r.logger.Warn("EventBus未配置，跳过Block模块事件订阅注册")
		}
		return nil
	}

	// 目前 Block 模块不订阅任何事件，仅发布事件
	// 未来可以根据需要添加事件订阅：
	//
	// 示例：订阅共识结果广播事件
	// if err := r.eventBus.Subscribe(
	//     eventconstants.EventTypeConsensusResultBroadcast,
	//     r.onConsensusResultBroadcast,
	// ); err != nil {
	//     if r.logger != nil {
	//         r.logger.Errorf("注册共识结果广播事件失败: %v", err)
	//     }
	//     return fmt.Errorf("注册共识结果广播事件失败: %w", err)
	// }

	if r.logger != nil {
		r.logger.Info("✅ Block 模块事件订阅注册完成（当前无订阅，仅发布事件）")
	}

	return nil
}

// UnregisterEventSubscriptions 注销所有事件订阅
//
// 🎯 **统一订阅注销**：
// 目前 Block 模块无订阅，此方法保留为未来扩展使用。
//
// 返回：
//   - error: 注销失败时的错误
func (r *EventSubscriptionRegistry) UnregisterEventSubscriptions() error {
	if r.logger != nil {
		r.logger.Info("✅ Block 模块事件订阅注销完成（当前无订阅）")
	}
	return nil
}

// 未来可以添加的事件处理器方法：
//
// // onConsensusResultBroadcast 处理共识结果广播事件
// func (r *EventSubscriptionRegistry) onConsensusResultBroadcast(eventData interface{}) {
//     // 处理逻辑
// }
//
// // onMempoolSizeChanged 处理交易池大小变化事件
// func (r *EventSubscriptionRegistry) onMempoolSizeChanged(eventData interface{}) {
//     // 处理逻辑
// }

