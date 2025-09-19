// Package event_handler 实现矿工事件处理服务
//
// 🎯 **矿工事件处理服务模块**
//
// 本包实现 MinerEventHandler 接口，提供矿工系统事件处理功能：
// - 处理分叉检测事件，立即暂停挖矿避免冲突
// - 处理分叉处理中事件，维持暂停状态等待完成
// - 处理分叉完成事件，根据结果决定恢复挖矿
// - 确保矿工与区块链状态的一致性，防止冲突挖矿
//
// 🏗️ **架构设计**：
// - 委托模式：manager作为薄委托层，具体处理逻辑在独立的处理器中
// - 接口实现：完整实现MinerEventHandler接口
// - 状态协调：与矿工状态管理器和控制器协调工作
// - 分叉响应：基于原integration/event/fork_handler.go的逻辑重构
package event_handler

import (
	"context"

	eventintegration "github.com/weisyn/v1/internal/core/consensus/integration/event"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// MinerEventHandlerService 矿工事件处理服务实现
//
// 🎯 **服务职责**：
// - 作为MinerEventHandler接口的具体实现
// - 委托具体处理器处理不同类型的分叉事件
// - 协调矿工各组件的事件响应和状态管理
// - 确保分叉期间挖矿安全暂停和恢复
type MinerEventHandlerService struct {
	logger          log.Logger                   // 日志记录器
	minerController interfaces.MinerController   // 矿工控制器（用于启停挖矿）
	stateManager    interfaces.MinerStateManager // 矿工状态管理器

	// 具体事件处理器
	forkEventsHandler *forkEventsHandler // 分叉事件统一处理器
}

// NewMinerEventHandlerService 创建矿工事件处理服务实例
//
// 🏗️ **构造函数**：
// 创建矿工事件处理服务，注入必要依赖并初始化子处理器
//
// @param logger 日志记录器
// @param minerController 矿工控制器，用于启停挖矿操作
// @param stateManager 矿工状态管理器，用于状态协调
// @return interfaces.MinerEventHandler 矿工事件处理器接口实例
func NewMinerEventHandlerService(
	logger log.Logger,
	minerController interfaces.MinerController,
	stateManager interfaces.MinerStateManager,
) interfaces.MinerEventHandler {
	// 创建分叉事件处理器
	forkEventsHandler := newForkEventsHandler(logger, minerController, stateManager)

	service := &MinerEventHandlerService{
		logger:            logger,
		minerController:   minerController,
		stateManager:      stateManager,
		forkEventsHandler: forkEventsHandler,
	}

	if logger != nil {
		logger.Info("[MinerEventHandler] 矿工事件处理服务已创建")
	}

	return service
}

// ==================== MinerEventSubscriber接口实现 ====================

// HandleForkDetected 处理分叉检测事件
//
// 🔀 **分叉检测响应**：
// 当检测到区块链分叉时，矿工必须立即暂停挖矿以避免产生冲突区块
//
// 处理逻辑：
// 1. 解析分叉检测事件数据，获取分叉信息
// 2. 检查当前矿工状态，如果正在挖矿则立即暂停
// 3. 保存当前挖矿状态（如矿工地址）用于后续恢复
// 4. 设置分叉暂停标志，等待分叉处理完成
//
// @param ctx 上下文，支持取消和超时
// @param eventData 分叉检测事件数据
// @return error 处理过程中的错误
func (s *MinerEventHandlerService) HandleForkDetected(ctx context.Context, eventData *types.ForkDetectedEventData) error {
	if s.logger != nil {
		s.logger.Info("[MinerEventHandler] 🔀 收到分叉检测事件，开始处理...")
	}

	// 委托给专门的分叉事件处理器
	err := s.forkEventsHandler.handleForkDetected(ctx, eventData)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[MinerEventHandler] 分叉检测事件处理失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("[MinerEventHandler] ✅ 分叉检测事件处理完成")
	}

	return nil
}

// HandleForkProcessing 处理分叉处理中事件
//
// 🔄 **分叉处理进度响应**：
// 在分叉处理过程中，矿工需要保持暂停状态直到处理完成
//
// 处理逻辑：
// 1. 解析分叉处理进度事件数据
// 2. 确认矿工仍处于暂停状态
// 3. 记录处理进度信息用于监控
// 4. 如果检测到异常状态，进行纠正
//
// 参数：
//   - ctx: 上下文，支持取消和超时
//   - eventData: 分叉处理中事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (s *MinerEventHandlerService) HandleForkProcessing(ctx context.Context, eventData *types.ForkProcessingEventData) error {
	if s.logger != nil {
		s.logger.Debug("[MinerEventHandler] 🔄 收到分叉处理中事件，检查状态...")
	}

	// 委托给专门的分叉事件处理器
	err := s.forkEventsHandler.handleForkProcessing(ctx, eventData)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[MinerEventHandler] 分叉处理中事件处理失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Debug("[MinerEventHandler] ✅ 分叉处理中事件处理完成")
	}

	return nil
}

// HandleForkCompleted 处理分叉完成事件
//
// ✅ **分叉处理完成响应**：
// 分叉处理完成后，根据处理结果决定是否恢复挖矿
//
// 处理逻辑：
// 1. 解析分叉完成事件数据，获取处理结果
// 2. 如果处理成功，使用保存的状态恢复挖矿
// 3. 如果处理失败，保持暂停状态等待人工干预
// 4. 清理分叉暂停标志和状态数据
//
// 参数：
//   - ctx: 上下文，支持取消和超时
//   - eventData: 分叉完成事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (s *MinerEventHandlerService) HandleForkCompleted(ctx context.Context, eventData *types.ForkCompletedEventData) error {
	if s.logger != nil {
		s.logger.Info("[MinerEventHandler] ✅ 收到分叉完成事件，开始处理...")
	}

	// 委托给专门的分叉事件处理器
	err := s.forkEventsHandler.handleForkCompleted(ctx, eventData)
	if err != nil {
		if s.logger != nil {
			s.logger.Errorf("[MinerEventHandler] 分叉完成事件处理失败: %v", err)
		}
		return err
	}

	if s.logger != nil {
		s.logger.Info("[MinerEventHandler] ✅ 分叉完成事件处理完成")
	}

	return nil
}

// ==================== 编译时接口检查 ====================

// 确保MinerEventHandlerService实现了所有必需的接口
var _ interfaces.MinerEventHandler = (*MinerEventHandlerService)(nil)
var _ eventintegration.MinerEventSubscriber = (*MinerEventHandlerService)(nil)
