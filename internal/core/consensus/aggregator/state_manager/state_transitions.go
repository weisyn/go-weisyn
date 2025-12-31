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
	currentState int64         // 使用atomic操作的当前状态
	lastUpdate   atomic.Value  // 最后更新时间（使用atomic.Value存储time.Time，避免数据竞争）
}

// newStateTransitionManager 创建状态转换管理器
func newStateTransitionManager(logger log.Logger) *stateTransitionManager {
	m := &stateTransitionManager{
		logger:       logger,
		currentState: int64(types.AggregationStateIdle),
	}
	// 使用 atomic.Value 存储初始时间，确保并发读取安全
	m.lastUpdate.Store(time.Now())
	return m
}

// getCurrentState 获取当前状态
func (m *stateTransitionManager) getCurrentState() types.AggregationState {
	return types.AggregationState(atomic.LoadInt64(&m.currentState))
}

// transitionTo 转换到指定状态
//
// 🆕 2025-12-18 优化：
// - 支持状态自转换（幂等性）：当 from == to 时直接返回成功
// - 使用 CAS 实现原子状态转换，避免并发竞态
// - 增强日志输出，包含更多上下文信息
func (m *stateTransitionManager) transitionTo(target types.AggregationState) error {
	current := m.getCurrentState()

	// 🆕 幂等性支持：如果已经是目标状态，直接返回成功
	// 这解决了 "Listening -> Listening 不允许" 的问题
	if current == target {
		m.logger.Debugf("状态自转换（幂等）: %s -> %s，无需转换", current.String(), target.String())
		return nil
	}

	m.logger.Infof("状态转换验证: 当前状态=%s, 目标状态=%s", current.String(), target.String())

	// 验证转换是否合法
	if !m.isValidTransition(current, target) {
		m.logger.Errorf("状态转换验证失败: %s -> %s 不在允许的转换列表中", current.String(), target.String())
		return errors.New("无效的状态转换")
	}

	// 🆕 使用 CAS 实现原子状态转换，避免并发竞态
	// 这解决了并发场景下状态被意外修改的问题
	if !atomic.CompareAndSwapInt64(&m.currentState, int64(current), int64(target)) {
		// CAS 失败，说明有并发修改，重新获取当前状态并重试
		newCurrent := m.getCurrentState()
		m.logger.Warnf("状态转换 CAS 失败（并发修改）: 期望=%s 实际=%s 目标=%s，将重试",
			current.String(), newCurrent.String(), target.String())
		
		// 如果新的当前状态已经是目标状态，则视为成功（幂等）
		if newCurrent == target {
			m.logger.Infof("状态转换并发完成（已达到目标）: %s", target.String())
			return nil
		}
		
		// 否则返回错误，让调用方决定是否重试
		return errors.New("状态转换失败：并发修改")
	}
	
	// 使用 atomic.Value 更新最后更新时间，避免与并发读取产生数据竞争
	m.lastUpdate.Store(time.Now())

	m.logger.Infof("状态转换完成: %s -> %s", current.String(), target.String())
	return nil
}

// ensureState 确保处于目标状态（幂等操作）
// 用于错误恢复、状态修复等场景，不关心当前状态，只关心最终状态
func (m *stateTransitionManager) ensureState(target types.AggregationState) error {
	current := m.getCurrentState()

	// 如果已经是目标状态，直接返回成功（幂等）
	if current == target {
		m.logger.Debugf("状态已满足期望: %s", target.String())
		return nil
	}

	// 需要转换，尝试通过合法路径到达目标状态
	m.logger.Infof("确保状态: %s -> %s", current.String(), target.String())
	return m.transitionTo(target)
}

// ensureIdle 确保处于 Idle 状态的便捷方法
func (m *stateTransitionManager) ensureIdle() error {
	return m.ensureState(types.AggregationStateIdle)
}

// Deprecated: transitionToIdleIfNeeded 已废弃，请使用 ensureIdle()
// 保留此方法用于向后兼容，将在未来版本中移除
func (m *stateTransitionManager) transitionToIdleIfNeeded() error {
	m.logger.Warnf("transitionToIdleIfNeeded 已废弃，请使用 ensureIdle()")
	return m.ensureIdle()
}

// isValidTransition 检查状态转换是否合法
func (m *stateTransitionManager) isValidTransition(from, to types.AggregationState) bool {
	// 聚合器正常业务流程转换规则（与具体共识算法无关）
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
		m.logger.Warnf("状态转换验证: 源状态 %s 不在转换规则表中", from.String())
		return false
	}

	m.logger.Debugf("状态转换验证: 源状态 %s 的允许目标状态: %v", from.String(), allowedStates)

	for _, allowedState := range allowedStates {
		if allowedState == to {
			m.logger.Debugf("状态转换验证: %s -> %s 转换合法", from.String(), to.String())
			return true
		}
	}

	m.logger.Warnf("状态转换验证: %s -> %s 转换不合法，允许的目标状态: %v", from.String(), to.String(), allowedStates)
	return false
}

// getStateDuration 获取当前状态持续时间
func (m *stateTransitionManager) getStateDuration() time.Duration {
	if v := m.lastUpdate.Load(); v != nil {
		if t, ok := v.(time.Time); ok {
			return time.Since(t)
		}
	}
	// 如果尚未初始化或类型不匹配，返回0作为保守值
	return 0
}

// getLastUpdateTime 获取最后更新时间
func (m *stateTransitionManager) getLastUpdateTime() time.Time {
	if v := m.lastUpdate.Load(); v != nil {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	// 未初始化时返回零值
	return time.Time{}
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
