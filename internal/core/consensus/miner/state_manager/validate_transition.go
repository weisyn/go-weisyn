// Package state_manager 实现矿工状态管理器的状态转换验证功能
//
// ✅ **状态转换验证功能模块**
//
// 实现 ValidateStateTransition 方法，提供专业的状态转换规则验证。
// 该模块基于矿工业务模型，确保所有状态转换符合业务逻辑和安全要求。
package state_manager

import (
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/types"
)

// ValidateStateTransition 验证状态转换的合法性
//
// 🛡️ **转换规则验证**：
// - 基于矿工业务模型的状态机设计
// - 防止非法状态转换导致的系统不一致
// - 支持外部调用进行转换合法性检查
//
// 🎯 **验证场景**：
// - 状态转换前的预检查
// - 业务逻辑中的状态依赖验证
// - 系统监控和诊断中的状态合规检查
// - 测试用例中的状态转换测试
//
// 📋 **支持的转换规则**：
// - Idle → Active: 启动挖矿
// - Active → Paused: 暂停挖矿（同步、分叉处理）
// - Active → Stopping: 停止挖矿
// - Paused → Active: 恢复挖矿
// - Paused → Stopping: 停止挖矿
// - Stopping → Idle: 停止完成
// - 任何状态 → Error: 错误处理
// - Error → Idle: 错误恢复
// - 任何状态 → Syncing: 开始同步
// - Syncing → Idle/Active: 同步完成
//
// @param from 源状态
// @param to 目标状态
// @return bool 转换是否合法（true=合法，false=非法）
func (s *MinerStateService) ValidateStateTransition(from, to interfaces.MinerInternalState) bool {
	return s.checkTransitionByBusinessRules(from, to)
}

// checkTransitionByBusinessRules 基于业务规则检查状态转换
//
// 🎯 **业务规则实现**：
// - 基于矿工生命周期的状态转换规则
// - 支持正常流程和异常流程的转换
// - 确保系统在各种场景下的状态一致性
//
// 📊 **规则分类**：
// - 正常业务流程转换
// - 异常处理流程转换
// - 系统维护流程转换
// - 特殊情况处理转换
//
// @param fromState 源状态
// @param toState 目标状态
// @return bool 基于业务规则的转换合法性
func (s *MinerStateService) checkTransitionByBusinessRules(fromState, toState interfaces.MinerInternalState) bool {
	// 检查相同状态转换（幂等操作）
	if fromState == toState {
		return s.isIdempotentTransitionAllowed(fromState)
	}

	// 检查特殊状态转换
	if s.isSpecialStateTransition(fromState, toState) {
		return s.validateSpecialTransition(fromState, toState)
	}

	// 检查标准业务流程转换
	return s.validateStandardBusinessTransition(fromState, toState)
}

// validateStandardBusinessTransition 验证标准业务流程转换
//
// 📈 **标准流程覆盖**：
// - 挖矿启动停止流程
// - 挖矿暂停恢复流程
// - 正常状态之间的转换
//
// @param from 源状态
// @param to 目标状态
// @return bool 标准转换是否合法
func (s *MinerStateService) validateStandardBusinessTransition(from, to interfaces.MinerInternalState) bool {
	switch from {
	case types.MinerStateIdle:
		return s.validateFromIdleTransitions(to)
	case types.MinerStateActive:
		return s.validateFromActiveTransitions(to)
	case types.MinerStatePaused:
		return s.validateFromPausedTransitions(to)
	case types.MinerStateStopping:
		return s.validateFromStoppingTransitions(to)
	default:
		return false
	}
}

// validateFromIdleTransitions 验证从空闲状态的转换
//
// 📋 **空闲状态允许的转换**：
// - Idle → Active: 启动挖矿
//
// @param to 目标状态
// @return bool 转换是否合法
func (s *MinerStateService) validateFromIdleTransitions(to interfaces.MinerInternalState) bool {
	allowedStates := []interfaces.MinerInternalState{
		types.MinerStateActive, // 启动挖矿
	}
	return s.isStateInAllowedList(to, allowedStates)
}

// validateFromActiveTransitions 验证从活跃状态的转换
//
// 📋 **活跃状态允许的转换**：
// - Active → Paused: 暂停挖矿
// - Active → Stopping: 停止挖矿
//
// @param to 目标状态
// @return bool 转换是否合法
func (s *MinerStateService) validateFromActiveTransitions(to interfaces.MinerInternalState) bool {
	allowedStates := []interfaces.MinerInternalState{
		types.MinerStatePaused,   // 暂停挖矿
		types.MinerStateStopping, // 停止挖矿
	}
	return s.isStateInAllowedList(to, allowedStates)
}

// validateFromPausedTransitions 验证从暂停状态的转换
//
// 📋 **暂停状态允许的转换**：
// - Paused → Active: 恢复挖矿
// - Paused → Stopping: 停止挖矿
//
// @param to 目标状态
// @return bool 转换是否合法
func (s *MinerStateService) validateFromPausedTransitions(to interfaces.MinerInternalState) bool {
	allowedStates := []interfaces.MinerInternalState{
		types.MinerStateActive,   // 恢复挖矿
		types.MinerStateStopping, // 停止挖矿
	}
	return s.isStateInAllowedList(to, allowedStates)
}

// validateFromStoppingTransitions 验证从停止中状态的转换
//
// 📋 **停止中状态允许的转换**：
// - Stopping → Idle: 停止完成
//
// @param to 目标状态
// @return bool 转换是否合法
func (s *MinerStateService) validateFromStoppingTransitions(to interfaces.MinerInternalState) bool {
	allowedStates := []interfaces.MinerInternalState{
		types.MinerStateIdle, // 停止完成
	}
	return s.isStateInAllowedList(to, allowedStates)
}

// isSpecialStateTransition 检查是否为特殊状态转换
//
// 🚨 **特殊转换识别**：
// - 涉及错误状态的转换
// - 涉及同步状态的转换
// - 其他需要特殊处理的转换
//
// @param from 源状态
// @param to 目标状态
// @return bool 是否为特殊转换
func (s *MinerStateService) isSpecialStateTransition(from, to interfaces.MinerInternalState) bool {
	return from == types.MinerStateError || to == types.MinerStateError ||
		from == types.MinerStateSyncing || to == types.MinerStateSyncing
}

// validateSpecialTransition 验证特殊状态转换
//
// 🔧 **特殊转换规则**：
// - 任何状态都可以转换到错误状态（系统保护）
// - 错误状态只能转换到空闲状态（恢复流程）
// - 任何状态都可以转换到同步状态（系统需要）
// - 同步状态可以转换到空闲或活跃状态（同步完成）
//
// @param from 源状态
// @param to 目标状态
// @return bool 特殊转换是否合法
func (s *MinerStateService) validateSpecialTransition(from, to interfaces.MinerInternalState) bool {
	// 任何状态 → Error（系统保护机制）
	if to == types.MinerStateError {
		return true
	}

	// Error → Idle（错误恢复）
	if from == types.MinerStateError && to == types.MinerStateIdle {
		return true
	}

	// 任何状态 → Syncing（系统同步需要）
	if to == types.MinerStateSyncing {
		return true
	}

	// Syncing → Idle/Active（同步完成）
	if from == types.MinerStateSyncing {
		return to == types.MinerStateIdle || to == types.MinerStateActive
	}

	return false
}

// isIdempotentTransitionAllowed 检查幂等转换是否允许
//
// 🔄 **幂等转换策略**：
// - 所有状态都支持幂等操作（重复设置相同状态）
// - 降低客户端复杂度，无需预先检查状态
//
// @param state 状态值
// @return bool 是否允许幂等转换（总是返回 true）
func (s *MinerStateService) isIdempotentTransitionAllowed(state interfaces.MinerInternalState) bool {
	// 所有状态都支持幂等操作，降低客户端复杂度
	return true
}

// isStateInAllowedList 检查状态是否在允许列表中
//
// 📋 **列表匹配工具**：
// - 通用的状态列表匹配功能
// - 支持多个允许状态的检查
//
// @param targetState 目标状态
// @param allowedStates 允许的状态列表
// @return bool 目标状态是否在允许列表中
func (s *MinerStateService) isStateInAllowedList(targetState interfaces.MinerInternalState, allowedStates []interfaces.MinerInternalState) bool {
	for _, allowedState := range allowedStates {
		if targetState == allowedState {
			return true
		}
	}
	return false
}
