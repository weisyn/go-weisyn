// Package state_manager 实现矿工状态管理器服务
//
// 🎯 **矿工状态管理器模块**
//
// 本包实现 MinerStateManager 接口，提供矿工状态管理功能：
// - 维护矿工当前运行状态
// - 验证状态转换的合法性
// - 支持状态查询和更新
//
// 🏗️ **薄实现设计**：采用委托模式，将具体业务逻辑分离到专门的方法文件中
package state_manager

import (
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// MinerStateService 矿工状态管理服务实现
//
// 🔧 **结构设计**：
// - 线程安全的并发访问支持
// - 轻量级状态管理模型
// - 高性能的状态读写操作
//
// 📊 **性能特性**：
// - 高性能状态读写操作
// - 支持高并发访问
type MinerStateService struct {
	// 基础依赖
	logger log.Logger // 日志记录器

	// 线程安全的状态管理
	mu           sync.RWMutex                  // 读写锁，保护状态访问
	currentState interfaces.MinerInternalState // 当前矿工状态
	lastChanged  time.Time                     // 最后状态变更时间
}

// NewMinerStateService 创建矿工状态服务实例
//
// 🎯 **初始化策略**：
// - 设置初始状态为 Idle
// - 初始化线程安全机制
// - 配置日志记录
//
// 📋 **初始化参数**：
// - logger: 日志记录器，用于状态变更审计
//
// @param logger 日志记录器
// @return interfaces.MinerStateManager 状态管理器实例
func NewMinerStateService(logger log.Logger) interfaces.MinerStateManager {
	service := &MinerStateService{
		logger:       logger,
		currentState: types.MinerStateIdle,
		lastChanged:  time.Now(),
	}

	logger.Info("矿工状态管理器已初始化，初始状态：Idle")
	return service
}

// 编译时确保 MinerStateService 实现了 MinerStateManager 接口
var _ interfaces.MinerStateManager = (*MinerStateService)(nil)

// ===================
// 🔧 **内部辅助方法**
// ===================

// isTransitionAllowed 检查状态转换是否被允许
//
// 🛡️ **转换验证核心**：
// - 基于预定义转换规则验证状态转换
// - 支持业务逻辑的一致性检查
// - 确保系统状态的稳定性
//
// 📋 **转换规则表**：
// 采用优化的状态转换映射，只保留核心必要的转换路径：
// - Idle → Active: 启动挖矿
// - Active → Paused/Stopping: 暂停或停止挖矿
// - Paused → Active/Stopping: 恢复或停止挖矿
// - Stopping → Idle: 停止完成
// - 任何状态 → Error: 错误处理
// - Error → Idle: 错误恢复
// - 任何状态 → Syncing: 开始同步
// - Syncing → Idle/Active: 同步完成
//
// @param from 源状态
// @param to 目标状态
// @return bool 转换是否被允许
func (s *MinerStateService) isTransitionAllowed(from, to interfaces.MinerInternalState) bool {
	// 相同状态转换（幂等操作）检查
	if from == to {
		return true // 所有状态都支持幂等操作
	}

	// 错误和同步状态的特殊转换规则
	if to == types.MinerStateError || to == types.MinerStateSyncing {
		return true // 任何状态都可以转换到错误或同步状态
	}
	if from == types.MinerStateError {
		return to == types.MinerStateIdle // 错误状态只能转换到空闲状态
	}
	if from == types.MinerStateSyncing {
		return to == types.MinerStateIdle || to == types.MinerStateActive // 同步完成后的状态
	}

	// 标准业务流程转换规则
	switch from {
	case types.MinerStateIdle:
		return to == types.MinerStateActive
	case types.MinerStateActive:
		return to == types.MinerStatePaused || to == types.MinerStateStopping
	case types.MinerStatePaused:
		return to == types.MinerStateActive || to == types.MinerStateStopping
	case types.MinerStateStopping:
		return to == types.MinerStateIdle
	default:
		return false
	}
}
