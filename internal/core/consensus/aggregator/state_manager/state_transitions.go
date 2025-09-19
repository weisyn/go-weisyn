// state_transitions.go
// 8个聚合状态的转换规则实现
//
// 核心业务功能：
// 1. 定义8个状态间的合法转换规则
// 2. 实现原子性的状态转换操作
// 3. 基本的转换条件验证
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package state_manager

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// stateTransitionManager 状态转换管理器
type stateTransitionManager struct {
	logger       log.Logger
	currentState int64     // 使用atomic操作的当前状态
	lastUpdate   time.Time // 最后更新时间
}

// newStateTransitionManager 创建状态转换管理器
func newStateTransitionManager(logger log.Logger) *stateTransitionManager {
	return &stateTransitionManager{
		logger:       logger,
		currentState: int64(types.AggregationStateIdle),
		lastUpdate:   time.Now(),
	}
}

// getCurrentState 获取当前状态
func (m *stateTransitionManager) getCurrentState() types.AggregationState {
	return types.AggregationState(atomic.LoadInt64(&m.currentState))
}

// transitionTo 转换到指定状态
func (m *stateTransitionManager) transitionTo(target types.AggregationState) error {
	current := m.getCurrentState()

	// 验证转换是否合法
	if !m.isValidTransition(current, target) {
		return errors.New("无效的状态转换")
	}

	// 执行原子状态转换
	atomic.StoreInt64(&m.currentState, int64(target))
	m.lastUpdate = time.Now()

	m.logger.Info("状态转换完成")
	return nil
}

// isValidTransition 检查状态转换是否合法
func (m *stateTransitionManager) isValidTransition(from, to types.AggregationState) bool {
	// ABS正常业务流程转换规则
	validTransitions := map[types.AggregationState][]types.AggregationState{
		types.AggregationStateIdle: {
			types.AggregationStateListening, // 开始新的聚合轮次
			types.AggregationStateError,     // 异常情况
		},
		types.AggregationStateListening: {
			types.AggregationStateCollecting, // 检测到新高度，开始收集
			types.AggregationStateIdle,       // 取消聚合
			types.AggregationStatePaused,     // 暂停监听
			types.AggregationStateError,      // 异常情况
		},
		types.AggregationStateCollecting: {
			types.AggregationStateEvaluating, // 收集完成，开始评估
			types.AggregationStatePaused,     // 暂停收集
			types.AggregationStateError,      // 异常情况
		},
		types.AggregationStateEvaluating: {
			types.AggregationStateSelecting, // 评估完成，开始选择
			types.AggregationStateError,     // 异常情况
		},
		types.AggregationStateSelecting: {
			types.AggregationStateDistributing, // 选择完成，开始分发
			types.AggregationStateError,        // 异常情况
		},
		types.AggregationStateDistributing: {
			types.AggregationStateIdle,  // 分发完成，回到空闲
			types.AggregationStateError, // 异常情况
		},
		types.AggregationStatePaused: {
			types.AggregationStateListening,  // 恢复到监听
			types.AggregationStateCollecting, // 恢复到收集
			types.AggregationStateIdle,       // 取消聚合
			types.AggregationStateError,      // 异常情况
		},
		types.AggregationStateError: {
			types.AggregationStateIdle,      // 错误恢复到空闲
			types.AggregationStateListening, // 错误恢复到监听
		},
	}

	// 检查转换是否在有效列表中
	allowedStates, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowedState := range allowedStates {
		if allowedState == to {
			return true
		}
	}

	return false
}

// getStateDuration 获取当前状态持续时间
func (m *stateTransitionManager) getStateDuration() time.Duration {
	return time.Since(m.lastUpdate)
}

// getLastUpdateTime 获取最后更新时间
func (m *stateTransitionManager) getLastUpdateTime() time.Time {
	return m.lastUpdate
}

// isInActiveState 判断是否处于活跃状态
func (m *stateTransitionManager) isInActiveState() bool {
	current := m.getCurrentState()
	activeStates := []types.AggregationState{
		types.AggregationStateListening,
		types.AggregationStateCollecting,
		types.AggregationStateEvaluating,
		types.AggregationStateSelecting,
		types.AggregationStateDistributing,
	}

	for _, state := range activeStates {
		if current == state {
			return true
		}
	}

	return false
}

// isInErrorState 判断是否处于错误状态
func (m *stateTransitionManager) isInErrorState() bool {
	return m.getCurrentState() == types.AggregationStateError
}

// canStartAggregation 判断是否可以开始聚合
func (m *stateTransitionManager) canStartAggregation() bool {
	current := m.getCurrentState()
	return current == types.AggregationStateIdle
}

// mustStopAggregation 判断是否必须停止聚合
func (m *stateTransitionManager) mustStopAggregation() bool {
	current := m.getCurrentState()
	return current == types.AggregationStateError
}
