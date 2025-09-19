// stop_aggregation.go
// 停止聚合轮次的业务逻辑实现
//
// 核心业务功能：
// 1. 安全停止当前聚合轮次
// 2. 清理聚合状态和资源
// 3. 处理紧急停止情况
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package controller

import (
	"context"
	"errors"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// aggregationStopper 聚合轮次停止器
type aggregationStopper struct {
	logger       log.Logger
	stateManager interfaces.AggregatorStateManager
}

// newAggregationStopper 创建聚合轮次停止器
func newAggregationStopper(logger log.Logger, stateManager interfaces.AggregatorStateManager) *aggregationStopper {
	return &aggregationStopper{
		logger:       logger,
		stateManager: stateManager,
	}
}

// stopAggregatorService 停止聚合器服务
func (s *aggregationStopper) stopAggregatorService(ctx context.Context) error {
	s.logger.Info("停止聚合器服务")

	// 获取当前状态
	currentState := s.stateManager.GetCurrentState()

	// 如果已经是空闲状态，直接返回
	if currentState == types.AggregationStateIdle {
		s.logger.Info("聚合器服务已停止")
		return nil
	}

	// 执行安全停止流程
	if err := s.performSafeStop(ctx, currentState); err != nil {
		return err
	}

	s.logger.Info("聚合器服务停止完成")
	return nil
}

// performSafeStop 执行安全停止流程
func (s *aggregationStopper) performSafeStop(ctx context.Context, currentState types.AggregationState) error {
	// 根据当前状态选择合适的停止策略
	switch currentState {
	case types.AggregationStateListening:
		// 监听状态可以直接停止
		return s.transitionToIdle()

	case types.AggregationStateCollecting:
		// 收集状态可以暂停后停止
		if err := s.pauseAndStop(); err != nil {
			return err
		}

	case types.AggregationStateEvaluating, types.AggregationStateSelecting:
		// 决策过程中不建议强制停止，等待完成或转为错误状态
		s.logger.Info("决策过程中，等待完成后停止")
		return errors.New("决策过程中无法立即停止")

	case types.AggregationStateDistributing:
		// 分发状态应该让其自然完成
		s.logger.Info("分发过程中，等待完成后停止")
		return errors.New("分发过程中无法立即停止")

	case types.AggregationStatePaused:
		// 暂停状态可以直接停止
		return s.transitionToIdle()

	case types.AggregationStateError:
		// 错误状态直接停止
		return s.transitionToIdle()

	default:
		return errors.New("未知状态无法停止")
	}

	return nil
}

// transitionToIdle 转换到空闲状态
func (s *aggregationStopper) transitionToIdle() error {
	if err := s.stateManager.TransitionTo(types.AggregationStateIdle); err != nil {
		return err
	}

	// 清理高度信息
	if err := s.stateManager.SetCurrentHeight(0); err != nil {
		s.logger.Info("清理高度信息失败")
		// 不返回错误，因为主要目标是停止服务
	}

	return nil
}

// pauseAndStop 暂停后停止
func (s *aggregationStopper) pauseAndStop() error {
	// 先暂停
	if err := s.stateManager.TransitionTo(types.AggregationStatePaused); err != nil {
		return err
	}

	// 然后停止
	return s.transitionToIdle()
}
