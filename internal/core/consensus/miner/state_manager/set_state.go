// Package state_manager 实现矿工状态管理器的状态设置功能
//
// ⚙️ **状态设置功能模块**
//
// 实现 SetMinerState 方法，提供安全的矿工状态更新能力。
// 该模块确保状态转换的原子性、一致性和业务规则合规性。
package state_manager

import (
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
)

// SetMinerState 设置矿工状态
//
// 🔧 **原子状态更新**：
// - 写锁保护，确保状态更新的原子性
// - 状态转换验证，确保业务规则合规
// - 完整的变更日志记录
//
// 🎯 **业务场景**：
// - 挖矿启动：Idle → Active
// - 挖矿暂停：Active → Paused
// - 挖矿停止：Active/Paused → Stopping → Idle
// - 错误处理：任何状态 → Error → Idle
// - 同步处理：任何状态 → Syncing → Idle/Active
//
// 🛡️ **安全保证**：
// - 非法状态转换拒绝
// - 状态变更完整审计日志
// - 并发安全的状态更新
//
// @param newState 目标状态
// @return error 状态设置错误（包括非法转换、系统错误等）
func (s *MinerStateService) SetMinerState(newState interfaces.MinerInternalState) error {
	return s.performStateTransitionWithValidation(newState)
}

// performStateTransitionWithValidation 执行带验证的状态转换
//
// 🔄 **完整转换流程**：
// 1. 获取写锁保护
// 2. 验证转换合法性
// 3. 执行状态更新
// 4. 记录变更日志
// 5. 释放锁并返回结果
//
// 🎯 **原子性保证**：
// - 整个转换过程在写锁保护下执行
// - 要么完全成功，要么完全失败
// - 不存在中间不一致状态
//
// @param targetState 目标状态
// @return error 转换过程中的任何错误
func (s *MinerStateService) performStateTransitionWithValidation(targetState interfaces.MinerInternalState) error {
	// 获取写锁确保状态更新的原子性
	s.mu.Lock()
	defer s.mu.Unlock()

	// 记录转换前的状态
	previousState := s.currentState

	// 验证状态转换的合法性
	if !s.validateTransition(previousState, targetState) {
		errorMsg := s.buildTransitionErrorMessage(previousState, targetState)
		s.logTransitionError(previousState, targetState, errorMsg)
		return fmt.Errorf("invalid state transition: %s", errorMsg)
	}

	// 执行状态更新
	s.executeStateUpdate(targetState)

	// 记录成功的状态变更
	s.logSuccessfulTransition(previousState, targetState)

	return nil
}

// executeStateUpdate 执行状态更新操作
//
// 🔄 **状态更新逻辑**：
// - 更新当前状态字段
// - 更新最后变更时间戳
// - 确保内部状态一致性
//
// @param newState 新状态值
func (s *MinerStateService) executeStateUpdate(newState interfaces.MinerInternalState) {
	s.currentState = newState
	s.lastChanged = time.Now()
}

// buildTransitionErrorMessage 构建状态转换错误消息
//
// 📝 **错误消息格式**：
// - 包含源状态和目标状态信息
// - 提供清晰的错误描述
// - 便于调试和问题定位
//
// @param from 源状态
// @param to 目标状态
// @return string 格式化的错误消息
func (s *MinerStateService) buildTransitionErrorMessage(from, to interfaces.MinerInternalState) string {
	return fmt.Sprintf("cannot transition from %s to %s", from.String(), to.String())
}

// logTransitionError 记录状态转换错误日志
//
// 📊 **错误日志内容**：
// - 转换失败的源状态和目标状态
// - 详细的错误原因
// - 转换尝试的时间戳
//
// @param from 源状态
// @param to 目标状态
// @param errorMsg 错误消息
func (s *MinerStateService) logTransitionError(from, to interfaces.MinerInternalState, errorMsg string) {
	s.logger.Info(fmt.Sprintf("矿工状态转换失败: %s -> %s, 原因: %s",
		from.String(), to.String(), errorMsg))
}

// logSuccessfulTransition 记录成功的状态转换日志
//
// ✅ **成功日志内容**：
// - 转换前后的状态信息
// - 转换成功的时间戳
// - 便于监控和审计
//
// @param from 源状态
// @param to 目标状态
func (s *MinerStateService) logSuccessfulTransition(from, to interfaces.MinerInternalState) {
	s.logger.Info(fmt.Sprintf("矿工状态转换成功: %s -> %s",
		from.String(), to.String()))
}

// validateTransition 验证状态转换是否合法
//
// 🛡️ **转换验证逻辑**：
// - 委托给专门的验证模块
// - 基于预定义的转换规则
// - 支持业务规则的一致性检查
//
// @param from 源状态
// @param to 目标状态
// @return bool 转换是否合法
func (s *MinerStateService) validateTransition(from, to interfaces.MinerInternalState) bool {
	return s.isTransitionAllowed(from, to)
}
