// Package event 共识事件订阅处理器
//
// 🎯 **事件订阅集成层**
//
// 本文件定义共识模块的事件订阅接口，参考network模块的设计模式：
// - 定义Aggregator和Miner事件订阅接口
// - 提供统一的事件订阅注册函数
// - 确保事件处理的解耦与可测试性
//
// 🏗️ **设计原则**：
// - 接口继承：子模块通过继承这些接口实现具体处理
// - 统一注册：通过RegisterEventSubscriptions统一管理订阅
// - 职责分离：Aggregator处理链重组，Miner处理分叉事件
// - 依赖注入：支持测试和模块替换
package event

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 事件订阅接口定义 ====================

// AggregatorEventSubscriber 聚合器事件订阅接口
//
// 🎯 **聚合器事件处理**：
// 定义聚合器关心的事件类型处理方法，主要处理：
// - 链重组事件：影响聚合器决策和状态
// - 网络变化事件：影响聚合器连接和通信
//
// 由 aggregator/event_handler 子包实现具体业务逻辑
type AggregatorEventSubscriber interface {
	// HandleChainReorganized 处理链重组事件
	//
	// 当检测到区块链重组时触发，聚合器需要：
	// - 重新评估当前决策状态
	// - 清理可能无效的候选区块
	// - 重置聚合器内部状态
	//
	// @param ctx 上下文，支持取消和超时
	// @param eventData 链重组事件数据
	// @return error 处理过程中的错误
	HandleChainReorganized(ctx context.Context, eventData *types.ChainReorganizedEventData) error

	// HandleNetworkQualityChanged 处理网络质量变化事件
	//
	// 当网络连接质量发生重大变化时触发，聚合器需要：
	// - 调整候选区块收集策略
	// - 更新网络评分权重
	// - 适配网络条件变化
	//
	// @param ctx 上下文，支持取消和超时
	// @param eventData 网络质量变化事件数据
	// @return error 处理过程中的错误
	HandleNetworkQualityChanged(ctx context.Context, eventData *types.NetworkQualityChangedEventData) error
}

// MinerEventSubscriber 矿工事件订阅接口
//
// 🎯 **矿工事件处理**：
// 定义矿工关心的事件类型处理方法，主要处理：
// - 分叉检测事件：立即暂停挖矿避免冲突
// - 分叉处理事件：维持暂停状态等待处理完成  
// - 分叉完成事件：根据结果决定是否恢复挖矿
//
// 由 miner/event_handler 子包实现具体业务逻辑
type MinerEventSubscriber interface {
	// HandleForkDetected 处理分叉检测事件
	//
	// 当检测到区块链分叉时立即触发，矿工需要：
	// - 立即暂停当前挖矿作业
	// - 保存当前挖矿状态用于恢复
	// - 等待分叉处理完成
	//
	// @param ctx 上下文，支持取消和超时
	// @param eventData 分叉检测事件数据
	// @return error 处理过程中的错误
	HandleForkDetected(ctx context.Context, eventData *types.ForkDetectedEventData) error

	// HandleForkProcessing 处理分叉处理中事件
	//
	// 在分叉处理过程中持续触发，矿工需要：
	// - 确保挖矿保持暂停状态
	// - 监控分叉处理进度
	// - 记录处理状态用于调试
	//
	// @param ctx 上下文，支持取消和超时
	// @param eventData 分叉处理中事件数据
	// @return error 处理过程中的错误
	HandleForkProcessing(ctx context.Context, eventData *types.ForkProcessingEventData) error

	// HandleForkCompleted 处理分叉完成事件
	//
	// 当分叉处理完成时触发，矿工需要：
	// - 根据处理结果决定是否恢复挖矿
	// - 如果成功则使用保存的状态恢复挖矿
	// - 如果失败则保持暂停等待人工干预
	//
	// @param ctx 上下文，支持取消和超时
	// @param eventData 分叉完成事件数据
	// @return error 处理过程中的错误
	HandleForkCompleted(ctx context.Context, eventData *types.ForkCompletedEventData) error
}

// ==================== 事件订阅注册函数 ====================

// RegisterEventSubscriptions 注册共识事件订阅
//
// 🎯 **统一事件订阅管理**：
// 为Aggregator和Miner组件统一注册所需的事件订阅，确保：
// - 事件路由到正确的处理器
// - 错误处理和日志记录统一管理
// - 支持组件的可选性（如果某个组件为nil则跳过）
//
// 参数：
//   - eventBus: 事件总线接口，用于订阅事件
//   - aggregatorSubscriber: 聚合器事件订阅处理器（可选）
//   - minerSubscriber: 矿工事件订阅处理器（可选）
//   - logger: 日志服务接口（可选）
//
// 返回：
//   - error: 订阅过程中的错误
func RegisterEventSubscriptions(
	eventBus event.EventBus,
	aggregatorSubscriber AggregatorEventSubscriber,
	minerSubscriber MinerEventSubscriber,
	logger log.Logger,
) error {
	if eventBus == nil {
		if logger != nil {
			logger.Warn("[EventSubscription] 事件总线未提供，跳过事件订阅注册")
		}
		return nil
	}

	if logger != nil {
		logger.Info("[EventSubscription] 开始注册共识事件订阅...")
	}

	// ==================== 注册聚合器事件订阅 ====================
	if aggregatorSubscriber != nil {
		if logger != nil {
			logger.Debug("[EventSubscription] 注册聚合器事件订阅...")
		}

		// 订阅链重组事件
		if err := eventBus.Subscribe(
			event.EventTypeChainReorganized,
			func(ctx context.Context, e event.Event) error {
				// 类型转换：从通用Event接口提取具体的事件数据
				eventData, ok := e.Data().(*types.ChainReorganizedEventData)
				if !ok {
					return fmt.Errorf("无效的链重组事件数据类型")
				}
				return aggregatorSubscriber.HandleChainReorganized(ctx, eventData)
			},
		); err != nil {
			if logger != nil {
				logger.Errorf("[EventSubscription] 聚合器链重组事件订阅失败: %v", err)
			}
			return err
		}

		// 订阅网络质量变化事件
		if err := eventBus.Subscribe(
			event.EventTypeNetworkQualityChanged,
			func(ctx context.Context, e event.Event) error {
				// 类型转换：从通用Event接口提取具体的事件数据
				eventData, ok := e.Data().(*types.NetworkQualityChangedEventData)
				if !ok {
					return fmt.Errorf("无效的网络质量变化事件数据类型")
				}
				return aggregatorSubscriber.HandleNetworkQualityChanged(ctx, eventData)
			},
		); err != nil {
			if logger != nil {
				logger.Errorf("[EventSubscription] 聚合器网络质量变化事件订阅失败: %v", err)
			}
			return err
		}

		if logger != nil {
			logger.Info("[EventSubscription] ✅ 聚合器事件订阅注册完成")
		}
	} else {
		if logger != nil {
			logger.Debug("[EventSubscription] 聚合器订阅处理器未提供，跳过聚合器事件订阅")
		}
	}

	// ==================== 注册矿工事件订阅 ====================
	if minerSubscriber != nil {
		if logger != nil {
			logger.Debug("[EventSubscription] 注册矿工事件订阅...")
		}

		// 订阅分叉检测事件
		if err := eventBus.Subscribe(
			event.EventTypeForkDetected,
			func(ctx context.Context, e event.Event) error {
				// 类型转换：从通用Event接口提取具体的事件数据
				eventData, ok := e.Data().(*types.ForkDetectedEventData)
				if !ok {
					return fmt.Errorf("无效的分叉检测事件数据类型")
				}
				return minerSubscriber.HandleForkDetected(ctx, eventData)
			},
		); err != nil {
			if logger != nil {
				logger.Errorf("[EventSubscription] 矿工分叉检测事件订阅失败: %v", err)
			}
			return err
		}

		// 订阅分叉处理中事件
		if err := eventBus.Subscribe(
			event.EventTypeForkProcessing,
			func(ctx context.Context, e event.Event) error {
				// 类型转换：从通用Event接口提取具体的事件数据
				eventData, ok := e.Data().(*types.ForkProcessingEventData)
				if !ok {
					return fmt.Errorf("无效的分叉处理中事件数据类型")
				}
				return minerSubscriber.HandleForkProcessing(ctx, eventData)
			},
		); err != nil {
			if logger != nil {
				logger.Errorf("[EventSubscription] 矿工分叉处理中事件订阅失败: %v", err)
			}
			return err
		}

		// 订阅分叉完成事件
		if err := eventBus.Subscribe(
			event.EventTypeForkCompleted,
			func(ctx context.Context, e event.Event) error {
				// 类型转换：从通用Event接口提取具体的事件数据
				eventData, ok := e.Data().(*types.ForkCompletedEventData)
				if !ok {
					return fmt.Errorf("无效的分叉完成事件数据类型")
				}
				return minerSubscriber.HandleForkCompleted(ctx, eventData)
			},
		); err != nil {
			if logger != nil {
				logger.Errorf("[EventSubscription] 矿工分叉完成事件订阅失败: %v", err)
			}
			return err
		}

		if logger != nil {
			logger.Info("[EventSubscription] ✅ 矿工事件订阅注册完成")
		}
	} else {
		if logger != nil {
			logger.Debug("[EventSubscription] 矿工订阅处理器未提供，跳过矿工事件订阅")
		}
	}

	if logger != nil {
		logger.Info("[EventSubscription] 🎉 共识事件订阅注册完成")
	}

	return nil
}
