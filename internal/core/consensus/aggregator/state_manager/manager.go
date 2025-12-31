// Package state_manager 实现聚合器状态管理服务
//
// 🎯 **聚合器状态管理模块**
//
// 本包实现 AggregatorStateManager 接口，提供聚合器的 8 状态转换管理：
// - 8 个聚合状态的转换控制
// - 聚合三阶段流程的状态协调
// - 基本的错误状态检测和恢复
package state_manager

import (
	"sync/atomic"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
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
	currentState := s.GetCurrentState()
	s.logger.Infof("请求状态转换: %s -> %s", currentState.String(), newState.String())

	err := s.transitionManager.transitionTo(newState)
	if err != nil {
		s.logger.Errorf("状态转换失败: %s -> %s, 错误: %v", currentState.String(), newState.String(), err)
		return err
	}

	s.logger.Infof("状态转换成功: %s -> %s", currentState.String(), newState.String())
	return nil
}

// EnsureState 确保处于目标状态（幂等操作）
// 用于错误恢复、状态修复等场景，如果已经是目标状态则直接返回成功
func (s *AggregatorStateManagerService) EnsureState(targetState interfaces.AggregationState) error {
	return s.transitionManager.ensureState(targetState)
}

// EnsureIdle 确保处于 Idle 状态的便捷方法
// 用于只读模式弃权、停止聚合、链重组恢复等场景
func (s *AggregatorStateManagerService) EnsureIdle() error {
	return s.transitionManager.ensureIdle()
}

// Deprecated: TransitionToIdleIfNeeded 已废弃，请使用 EnsureIdle()
// 保留此方法用于向后兼容，将在未来版本中移除
func (s *AggregatorStateManagerService) TransitionToIdleIfNeeded() error {
	s.logger.Warnf("TransitionToIdleIfNeeded 已废弃，请使用 EnsureIdle()")
	return s.EnsureIdle()
}

// IsValidTransition 验证状态转换
func (s *AggregatorStateManagerService) IsValidTransition(from, to interfaces.AggregationState) bool {
	return s.transitionManager.isValidTransition(from, to)
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
