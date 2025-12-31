// Package event_handler 实现聚合器事件处理服务
//
// 🎯 **聚合器事件处理服务模块**
//
// 本包实现 AggregatorEventHandler 接口，提供聚合器系统事件处理功能：
// - 处理区块链重组事件，调整聚合器状态
// - 处理网络质量变化事件，优化聚合策略
// - 确保聚合器与区块链状态的一致性
//
// 🏗️ **架构设计**：
// - 委托模式：manager作为薄委托层，具体处理逻辑在独立的处理器中
// - 接口实现：完整实现AggregatorEventHandler接口
// - 状态协调：与聚合器状态管理器协调工作
package event_handler

import (
	"context"

	eventintegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// AggregatorEventHandlerService 聚合器事件处理服务实现
//
// 🎯 **服务职责**：
// - 作为AggregatorEventHandler接口的具体实现
// - 委托具体处理器处理不同类型的系统事件
// - 协调聚合器各组件的事件响应
type AggregatorEventHandlerService struct {
	logger       log.Logger                        // 日志记录器
	stateManager interfaces.AggregatorStateManager // 聚合器状态管理器

	// 具体事件处理器
	reorgHandler          *chainReorganizedHandler // 链重组事件处理器
	networkQualityHandler *networkQualityHandler   // 网络质量变化处理器
}

// NewAggregatorEventHandlerService 创建聚合器事件处理服务实例
//
// 🏗️ **构造函数**：
// 创建聚合器事件处理服务，注入必要依赖并初始化子处理器
//
// 参数：
//   - logger: 日志记录器
//   - stateManager: 聚合器状态管理器，用于协调事件响应与状态变更
//
// 返回：
//   - interfaces.AggregatorEventHandler: 聚合器事件处理器接口实例
func NewAggregatorEventHandlerService(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
) interfaces.AggregatorEventHandler {
	// 创建子处理器
	reorgHandler := newChainReorganizedHandler(logger, stateManager)
	networkQualityHandler := newNetworkQualityHandler(logger, stateManager)

	service := &AggregatorEventHandlerService{
		logger:                logger,
		stateManager:          stateManager,
		reorgHandler:          reorgHandler,
		networkQualityHandler: networkQualityHandler,
	}

	if logger != nil {
		logger.Info("[AggregatorEventHandler] 聚合器事件处理服务已创建")
	}

	return service
}

// ==================== AggregatorEventSubscriber接口实现 ====================

// HandleChainReorganized 处理链重组事件
//
// 🔄 **重组事件处理**：
// 当检测到区块链重组时，聚合器需要重新评估当前状态并清理无效数据
//
// 处理逻辑：
// 1. 解析重组事件数据，获取重组前后的链状态
// 2. 评估当前聚合状态是否受重组影响
// 3. 清理可能无效的候选区块数据
// 4. 重置聚合器到合适的状态
//
// 参数：
//   - ctx: 上下文，支持取消和超时
//   - event: 链重组事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (s *AggregatorEventHandlerService) HandleChainReorganized(ctx context.Context, eventData *types.ChainReorganizedEventData) error {
	if s.logger != nil {
		s.logger.Info("[AggregatorEventHandler] 🔄 收到链重组事件，开始处理...")
	}

	// 委托给专门的重组处理器
	err := s.reorgHandler.handleChainReorganized(ctx, eventData)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[AggregatorEventHandler] 链重组事件处理失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("[AggregatorEventHandler] ✅ 链重组事件处理完成")
	}

	return nil
}

// HandleNetworkQualityChanged 处理网络质量变化事件
//
// 🌐 **网络质量变化处理**：
// 当网络连接质量发生重大变化时，聚合器需要调整聚合策略
//
// 处理逻辑：
// 1. 解析网络质量变化事件数据
// 2. 评估网络质量对聚合过程的影响
// 3. 调整候选区块收集超时时间
// 4. 更新网络评分权重配置
//
// 参数：
//   - ctx: 上下文，支持取消和超时
//   - event: 网络质量变化事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (s *AggregatorEventHandlerService) HandleNetworkQualityChanged(ctx context.Context, eventData *types.NetworkQualityChangedEventData) error {
	if s.logger != nil {
		s.logger.Info("[AggregatorEventHandler] 🌐 收到网络质量变化事件，开始处理...")
	}

	// 委托给专门的网络质量处理器
	err := s.networkQualityHandler.handleNetworkQualityChanged(ctx, eventData)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[AggregatorEventHandler] 网络质量变化事件处理失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("[AggregatorEventHandler] ✅ 网络质量变化事件处理完成")
	}

	return nil
}

// ==================== 编译时接口检查 ====================

// 确保AggregatorEventHandlerService实现了所有必需的接口
var _ interfaces.AggregatorEventHandler = (*AggregatorEventHandlerService)(nil)
var _ eventintegration.AggregatorEventSubscriber = (*AggregatorEventHandlerService)(nil)
