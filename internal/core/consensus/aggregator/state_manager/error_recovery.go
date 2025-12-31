// error_recovery.go
// 基本的错误状态检测和恢复实现
//
// 核心业务功能：
// 1. 错误状态的检测和诊断
// 2. 基本的错误恢复策略
// 3. 状态一致性验证
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package state_manager

import (
	"context"
	"errors"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// errorRecoveryManager 错误恢复管理器
type errorRecoveryManager struct {
	logger            log.Logger
	transitionManager *stateTransitionManager
}

// newErrorRecoveryManager 创建错误恢复管理器
func newErrorRecoveryManager(
	logger log.Logger,
	transitionManager *stateTransitionManager,
) *errorRecoveryManager {
	return &errorRecoveryManager{
		logger:            logger,
		transitionManager: transitionManager,
	}
}

// detectStateError 检测状态错误
func (r *errorRecoveryManager) detectStateError(ctx context.Context) error {
	current := r.transitionManager.getCurrentState()

	// 检查是否已经在错误状态
	if current == types.AggregationStateError {
		return errors.New("聚合器处于错误状态")
	}

	// 检查状态持续时间是否异常
	duration := r.transitionManager.getStateDuration()
	if r.isStateTimeoutExceeded(current, duration) {
		r.logger.Info("检测到状态超时")
		return r.transitionToErrorState(ctx)
	}

	return nil
}

// isStateTimeoutExceeded 判断状态是否超时
func (r *errorRecoveryManager) isStateTimeoutExceeded(state types.AggregationState, duration time.Duration) bool {
	// 定义各状态的最大允许持续时间
	maxDurations := map[types.AggregationState]time.Duration{
		types.AggregationStateListening:    5 * time.Minute, // 监听最多5分钟
		types.AggregationStateCollecting:   3 * time.Minute, // 收集最多3分钟
		types.AggregationStateEvaluating:   2 * time.Minute, // 评估最多2分钟
		types.AggregationStateSelecting:    1 * time.Minute, // 选择最多1分钟
		types.AggregationStateDistributing: 2 * time.Minute, // 分发最多2分钟
	}

	maxDuration, exists := maxDurations[state]
	if !exists {
		return false // 空闲和暂停状态不设超时限制
	}

	return duration > maxDuration
}

// transitionToErrorState 转换到错误状态
func (r *errorRecoveryManager) transitionToErrorState(ctx context.Context) error {
	if err := r.transitionManager.transitionTo(types.AggregationStateError); err != nil {
		return err
	}

	r.logger.Info("转换到错误状态")
	return nil
}

// attemptRecovery 尝试错误恢复
func (r *errorRecoveryManager) attemptRecovery(ctx context.Context) error {
	current := r.transitionManager.getCurrentState()

	// 只有在错误状态才能恢复
	if current != types.AggregationStateError {
		return errors.New("当前不是错误状态无需恢复")
	}

	// 尝试恢复到空闲状态
	if err := r.transitionManager.transitionTo(types.AggregationStateIdle); err != nil {
		return err
	}

	r.logger.Info("错误恢复完成转为空闲状态")
	return nil
}

// forceReset 强制重置到空闲状态
func (r *errorRecoveryManager) forceReset(ctx context.Context) error {
	// 直接重置到空闲状态，忽略转换规则
	if err := r.transitionManager.transitionTo(types.AggregationStateIdle); err != nil {
		return err
	}

	r.logger.Info("强制重置到空闲状态")
	return nil
}

// validateStateConsistency 验证状态一致性
func (r *errorRecoveryManager) validateStateConsistency(ctx context.Context) error {
	current := r.transitionManager.getCurrentState()

	// 验证状态值是否在有效范围内
	if !r.isValidStateValue(current) {
		r.logger.Info("检测到无效的状态值")
		return r.transitionToErrorState(ctx)
	}

	return nil
}

// isValidStateValue 检查状态值是否有效
func (r *errorRecoveryManager) isValidStateValue(state types.AggregationState) bool {
	validStates := []types.AggregationState{
		types.AggregationStateIdle,
		types.AggregationStateListening,
		types.AggregationStateCollecting,
		types.AggregationStateEvaluating,
		types.AggregationStateSelecting,
		types.AggregationStateDistributing,
		types.AggregationStatePaused,
		types.AggregationStateError,
	}

	for _, validState := range validStates {
		if state == validState {
			return true
		}
	}

	return false
}

// getRecoveryStrategy 获取恢复策略
func (r *errorRecoveryManager) getRecoveryStrategy(ctx context.Context) string {
	current := r.transitionManager.getCurrentState()

	switch current {
	case types.AggregationStateError:
		return "重置到空闲状态"
	case types.AggregationStatePaused:
		return "恢复到监听状态"
	default:
		return "保持当前状态"
	}
}

// canRecover 判断是否可以恢复
func (r *errorRecoveryManager) canRecover(ctx context.Context) bool {
	current := r.transitionManager.getCurrentState()
	return current == types.AggregationStateError || current == types.AggregationStatePaused
}

// getHealthStatus 获取状态健康状况
func (r *errorRecoveryManager) getHealthStatus() string {
	current := r.transitionManager.getCurrentState()
	duration := r.transitionManager.getStateDuration()

	switch current {
	case types.AggregationStateError:
		return "错误状态"
	case types.AggregationStateIdle:
		return "健康空闲"
	default:
		if r.isStateTimeoutExceeded(current, duration) {
			return "状态超时"
		}
		return "运行正常"
	}
}
