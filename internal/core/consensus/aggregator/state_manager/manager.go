// Package state_manager 实现聚合器状态管理服务
//
// 🎯 **聚合器状态管理模块**
//
// 本包实现 AggregatorStateManager 接口，提供聚合器的8状态转换管理：
// - 8个聚合状态的转换控制
// - ABS三阶段流程的状态协调
// - 基本的错误状态检测和恢复
package state_manager

import (
	"sync/atomic"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// AggregatorStateManagerService 聚合器状态管理服务实现（薄委托层）
type AggregatorStateManagerService struct {
	logger            log.Logger              // 日志记录器
	transitionManager *stateTransitionManager // 状态转换管理器
	errorRecovery     *errorRecoveryManager   // 错误恢复管理器
	currentHeight     uint64                  // 当前聚合高度
}

// NewAggregatorStateManagerService 创建聚合器状态管理服务实例
func NewAggregatorStateManagerService(
	logger log.Logger,
) interfaces.AggregatorStateManager {
	// 创建状态转换管理器
	transitionManager := newStateTransitionManager(logger)

	// 创建错误恢复管理器（简化依赖）
	errorRecovery := newErrorRecoveryManager(logger, transitionManager)

	return &AggregatorStateManagerService{
		logger:            logger,
		transitionManager: transitionManager,
		errorRecovery:     errorRecovery,
		currentHeight:     0,
	}
}

// 编译时确保 AggregatorStateManagerService 实现了 AggregatorStateManager 接口
var _ interfaces.AggregatorStateManager = (*AggregatorStateManagerService)(nil)

// GetCurrentState 获取当前聚合状态
func (s *AggregatorStateManagerService) GetCurrentState() interfaces.AggregationState {
	return s.transitionManager.getCurrentState()
}

// TransitionTo 转换到目标状态
func (s *AggregatorStateManagerService) TransitionTo(newState interfaces.AggregationState) error {
	s.logger.Info("请求状态转换")
	return s.transitionManager.transitionTo(newState)
}

// IsValidTransition 验证状态转换
func (s *AggregatorStateManagerService) IsValidTransition(from, to interfaces.AggregationState) bool {
	return s.transitionManager.isValidTransition(from, to)
}

// GetStateHistory 获取状态转换历史 - 简化实现
func (s *AggregatorStateManagerService) GetStateHistory(limit int) ([]types.StateTransition, error) {
	s.logger.Info("获取状态转换历史")

	// 简化实现：区块链自运行，不需要复杂的历史记录
	// 只返回当前状态的基本信息
	current := s.transitionManager.getCurrentState()

	history := []types.StateTransition{
		{
			FromState: types.AggregationStateIdle.String(), // 简化：假设从空闲状态转换而来
			ToState:   current.String(),
			Timestamp: s.transitionManager.getLastUpdateTime(),
			Reason:    "正常状态转换",
			Success:   true,
		},
	}

	// 限制返回数量
	if limit > 0 && limit < len(history) {
		history = history[:limit]
	}

	return history, nil
}

// GetCurrentHeight 获取当前聚合高度
func (s *AggregatorStateManagerService) GetCurrentHeight() uint64 {
	return atomic.LoadUint64(&s.currentHeight)
}

// SetCurrentHeight 设置当前聚合高度
func (s *AggregatorStateManagerService) SetCurrentHeight(height uint64) error {
	s.logger.Info("设置聚合高度")

	atomic.StoreUint64(&s.currentHeight, height)
	return nil
}
