// Package controller 实现矿工控制器的状态查询功能
//
// 📊 **状态查询功能模块**
//
// 本文件实现 getMiningStatus 方法的具体业务逻辑，包括：
// - 线程安全的状态读取
// - 原子状态和内部状态的一致性检查
// - 矿工地址的安全拷贝返回
// - 状态信息的校验和格式化
package controller

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// getMiningStatus 获取挖矿状态的具体实现
//
// 🎯 **核心功能**：
// 1. 线程安全读取：使用原子操作和读锁保护状态读取
// 2. 状态一致性：检查原子状态与内部状态管理器的一致性
// 3. 地址拷贝：安全返回矿工地址的副本，避免修改
// 4. 错误处理：处理状态不一致等异常情况
//
// @param ctx 上下文（当前实现暂不使用，为未来扩展预留）
// @return bool 挖矿运行状态，true表示正在挖矿
// @return []byte 矿工地址的安全副本
// @return error 查询过程中的错误
func (s *MinerControllerService) getMiningStatus(ctx context.Context) (bool, []byte, error) {
	s.logger.Info("查询挖矿状态")

	// 1. 获取原子运行状态
	isRunning := s.isRunning.Load()

	// 2. 获取矿工地址的安全副本
	minerAddress := s.getSafeMinerAddress()

	// 3. 验证状态一致性（可选，用于调试和监控）
	if err := s.validateStatusConsistency(isRunning); err != nil {
		s.logger.Info(fmt.Sprintf("状态一致性检查警告: %v", err))
		// 状态不一致警告，但不影响返回结果
	}

	// 4. 记录查询结果日志
	s.logStatusQuery(isRunning, minerAddress)

	return isRunning, minerAddress, nil
}

// getSafeMinerAddress 获取矿工地址的安全副本
func (s *MinerControllerService) getSafeMinerAddress() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 如果地址为空，返回nil
	if s.minerAddress == nil {
		return nil
	}

	// 返回地址的安全拷贝，避免外部修改原始数据
	minerAddressCopy := make([]byte, len(s.minerAddress))
	copy(minerAddressCopy, s.minerAddress)
	return minerAddressCopy
}

// validateStatusConsistency 验证状态一致性
func (s *MinerControllerService) validateStatusConsistency(atomicIsRunning bool) error {
	// 获取状态管理器中的状态
	internalState := s.stateManagerService.GetMinerState()

	// 检查原子状态与内部状态的一致性
	expectedInternalActive := (internalState == types.MinerStateActive)

	if atomicIsRunning != expectedInternalActive {
		return fmt.Errorf("状态不一致: 原子状态=%v, 内部状态=%s",
			atomicIsRunning, internalState.String())
	}

	// 额外检查：如果正在运行，矿工地址应该不为空
	if atomicIsRunning {
		s.mu.RLock()
		hasAddress := len(s.minerAddress) > 0
		s.mu.RUnlock()

		if !hasAddress {
			return fmt.Errorf("状态异常: 正在运行但缺少矿工地址")
		}
	}

	return nil
}

// logStatusQuery 记录状态查询日志
func (s *MinerControllerService) logStatusQuery(isRunning bool, minerAddress []byte) {
	if isRunning {
		if len(minerAddress) > 0 {
			// 仅记录地址长度，避免在日志中暴露完整地址
			s.logger.Info(fmt.Sprintf("挖矿状态: 运行中, 矿工地址长度: %d字节", len(minerAddress)))
		} else {
			s.logger.Info("挖矿状态: 运行中, 但矿工地址为空")
		}
	} else {
		s.logger.Info("挖矿状态: 已停止")
	}
}
