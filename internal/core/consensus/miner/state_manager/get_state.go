// Package state_manager 实现矿工状态管理器的状态获取功能
//
// 📋 **状态获取功能模块**
//
// 实现 GetMinerState 方法，提供线程安全的矿工状态读取能力。
// 该模块专注于高性能、并发安全的状态访问。
package state_manager

import (
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
)

// GetMinerState 获取当前矿工状态
//
// 提供线程安全的矿工状态读取，使用读锁保护支持高并发访问。
//
// 主要使用场景：
// - 挖矿启动前状态检查
// - 外部监控查询
// - 状态转换验证
//
// @return MinerInternalState 当前矿工内部状态
func (s *MinerStateService) GetMinerState() interfaces.MinerInternalState {
	return s.getCurrentStateThreadSafe()
}

// getCurrentStateThreadSafe 线程安全地获取当前状态
//
// 🔒 **并发安全设计**：
// - 使用读锁保护状态读取
// - 确保读取的原子性和一致性
// - 避免读写竞争条件
//
// 📈 **优化策略**：
// - 读锁持有时间最短
// - 避免在锁内执行其他操作
// - 快速返回状态值
//
// @return MinerInternalState 当前状态的安全副本
func (s *MinerStateService) getCurrentStateThreadSafe() interfaces.MinerInternalState {
	// 获取读锁确保线程安全
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回当前状态
	return s.currentState
}
