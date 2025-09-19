// Package orchestrator 实现挖矿编排器的确认等待和同步触发功能
//
// ⏳ **确认等待模块**
//
// 本文件实现区块提交后的确认等待和同步触发逻辑：
// 1. 区块确认等待 - 等待区块在网络中的确认
// 2. 确认超时处理 - 设置合理的等待超时并处理超时情况
// 3. 同步触发机制 - 确认超时时主动触发同步以获取最新状态
// 4. 高度门闸更新 - 确认成功或超时后更新已处理高度
// 5. 状态协调管理 - 与其他组件协调挖矿后续处理
package orchestrator

import (
	"context"
	"fmt"
	"time"

	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
)

// 注意：确认超时和检查间隔现在从配置中获取，不再使用硬编码常量

// waitForConfirmation 等待区块确认或超时触发同步
// 这是确认等待的主入口方法，被 execute_mining_round.go 调用
func (s *MiningOrchestratorService) waitForConfirmation(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("开始等待区块确认")

	// 从配置获取确认超时时间
	confirmationTimeout := s.minerConfig.ConfirmationTimeout
	if confirmationTimeout <= 0 {
		// 如果配置未设置，使用30秒作为兜底默认值
		confirmationTimeout = 30 * time.Second
	}

	// 创建带超时的上下文
	confirmCtx, cancel := context.WithTimeout(ctx, confirmationTimeout)
	defer cancel()

	// 监听区块确认状态
	if err := s.waitForBlockConfirmation(confirmCtx, minedBlock); err != nil {
		// 确认失败或超时，触发同步
		s.logger.Info("区块确认失败或超时，触发同步")
		if syncErr := s.triggerSyncIfNeeded(ctx); syncErr != nil {
			s.logger.Info("触发同步失败")
			return fmt.Errorf("确认失败且同步失败: 确认错误=%v, 同步错误=%v", err, syncErr)
		}
		// 🔧 修复：确认失败时不更新高度门闸，避免门闸与链高度不一致
		s.logger.Info("区块确认失败，不更新高度门闸")
	} else {
		// 确认成功，但在更新门闸前进行二次验证
		s.logger.Info("区块确认成功，准备更新高度门闸")
		if err := s.validateChainHeightBeforeGateUpdate(ctx, minedBlock.Header.Height); err != nil {
			s.logger.Warnf("门闸更新前验证失败: %v", err)
			return fmt.Errorf("确认成功但门闸更新验证失败: %v", err)
		}
		s.updateHeightGate(minedBlock.Header.Height)
	}

	return nil
}

// waitForBlockConfirmation 等待区块确认
// 通过定期检查链高度来判断区块是否已被网络确认
func (s *MiningOrchestratorService) waitForBlockConfirmation(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("开始监听区块确认")

	expectedHeight := minedBlock.Header.Height
	s.logger.Debugf("等待区块确认，期望高度: %d", expectedHeight)

	// 从配置获取检查间隔（配置必须提供有效值）
	checkInterval := s.minerConfig.ConfirmationCheckInterval
	if checkInterval <= 0 {
		return fmt.Errorf("配置错误：ConfirmationCheckInterval必须大于0，当前值: %v", checkInterval)
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 上下文超时或取消
			return fmt.Errorf("等待区块确认超时: %v", ctx.Err())

		case <-ticker.C:
			// 使用ChainService检查当前链高度
			if err := s.checkBlockConfirmation(ctx, expectedHeight); err != nil {
				s.logger.Debugf("区块确认检查失败: %v", err)
				continue // 继续等待
			}

			// 确认成功
			s.logger.Infof("区块确认成功，高度: %d", expectedHeight)
			return nil
		}
	}
}

// handleConfirmationTimeout 处理确认超时
// 当区块确认超时时的处理逻辑
func (s *MiningOrchestratorService) handleConfirmationTimeout(ctx context.Context, minedBlock *blocktypes.Block) error {
	s.logger.Info("处理区块确认超时")

	// 1. 获取当前链状态进行诊断
	if s.chainService != nil {
		chainInfo, err := s.chainService.GetChainInfo(ctx)
		if err != nil {
			s.logger.Errorf("获取链状态失败: %v", err)
		} else {
			s.logger.Infof("确认超时诊断 - 当前高度: %d, 期望高度: %d, 链状态: %s",
				chainInfo.Height, minedBlock.Header.Height, chainInfo.Status)
		}
	}

	// 2. 返回超时错误
	return fmt.Errorf("区块确认超时，高度: %d", minedBlock.Header.Height)
}

// triggerSyncIfNeeded 触发同步
// 当确认失败时，主动触发同步以获取网络最新状态
func (s *MiningOrchestratorService) triggerSyncIfNeeded(ctx context.Context) error {
	s.logger.Info("触发网络同步以获取最新状态")

	// 调用同步服务触发同步（直接使用公共接口，不重复封装）
	if err := s.syncService.TriggerSync(ctx); err != nil {
		return fmt.Errorf("触发同步失败: %v", err)
	}

	s.logger.Info("同步已成功触发")
	return nil
}

// updateHeightGate 更新高度门闸
// 无论确认成功与否，都需要更新已处理高度以防止重复挖矿
func (s *MiningOrchestratorService) updateHeightGate(height uint64) {
	s.logger.Info("更新高度门闸")

	// 更新已处理的最高高度
	s.heightGateService.UpdateLastProcessedHeight(height)

	s.logger.Info("高度门闸更新完成")
}

// ==================== 区块确认检查 ====================

// checkBlockConfirmation 检查区块是否已被确认
//
// 🎯 **确认检查逻辑**
//
// 通过ChainService检查当前链的状态，判断指定高度的区块是否已被网络确认。
//
// 参数：
//
//	ctx: 上下文对象
//	expectedHeight: 期望确认的区块高度
//
// 返回值：
//
//	error: 确认失败时的错误，nil表示已确认
func (s *MiningOrchestratorService) checkBlockConfirmation(ctx context.Context, expectedHeight uint64) error {
	// 1. 检查ChainService是否可用
	if s.chainService == nil {
		return fmt.Errorf("ChainService未注入")
	}

	// 2. 获取当前链信息
	chainInfo, err := s.chainService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	currentHeight := chainInfo.Height
	s.logger.Debugf("当前链高度: %d, 期望高度: %d", currentHeight, expectedHeight)

	// 3. 检查高度是否已达到或超过期望高度
	if currentHeight >= expectedHeight {
		// 区块已确认
		return nil
	}

	// 4. 高度未达到，继续等待
	return fmt.Errorf("区块尚未确认，当前高度: %d, 期望高度: %d", currentHeight, expectedHeight)
}

// validateChainHeightBeforeGateUpdate 在更新门闸前验证链高度
//
// 🔒 **防御性验证**
//
// 在确认成功后，更新门闸前再次验证链高度，确保门闸不会超前于实际链高度。
// 这是防止门闸与链状态不一致的最后一道防线。
//
// 参数：
//
//	ctx: 上下文对象
//	expectedHeight: 期望的区块高度
//
// 返回值：
//
//	error: 验证失败时的错误，nil表示验证通过
func (s *MiningOrchestratorService) validateChainHeightBeforeGateUpdate(ctx context.Context, expectedHeight uint64) error {
	// 获取当前链信息
	if s.chainService == nil {
		return fmt.Errorf("ChainService未注入")
	}

	chainInfo, err := s.chainService.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %v", err)
	}

	currentHeight := chainInfo.Height
	s.logger.Infof("门闸更新前验证 - 当前链高度: %d, 期望高度: %d", currentHeight, expectedHeight)

	// 严格验证：链高度必须大于等于期望高度
	if currentHeight < expectedHeight {
		return fmt.Errorf("链高度验证失败：当前高度 %d 小于期望高度 %d", currentHeight, expectedHeight)
	}

	s.logger.Info("链高度验证通过，允许更新门闸")
	return nil
}
