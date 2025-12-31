// Package event_handler 链重组事件处理器
//
// 🔄 **链重组事件专门处理器**
//
// 本文件实现聚合器对区块链重组事件的响应逻辑：
// - 评估重组对当前聚合状态的影响
// - 清理可能无效的候选区块数据
// - 重置聚合器到合适的安全状态
// - 确保聚合决策的一致性和正确性
package event_handler

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// chainReorganizedHandler 链重组事件处理器
//
// 🎯 **专门职责**：
// 处理区块链重组事件，确保聚合器状态与重组后的链状态保持一致
type chainReorganizedHandler struct {
	logger       log.Logger                        // 日志记录器
	stateManager interfaces.AggregatorStateManager // 状态管理器
}

// newChainReorganizedHandler 创建链重组事件处理器
//
// 🏗️ **内部构造器**：
// 仅供manager.go使用的内部构造函数
func newChainReorganizedHandler(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
) *chainReorganizedHandler {
	return &chainReorganizedHandler{
		logger:       logger,
		stateManager: stateManager,
	}
}

// handleChainReorganized 处理链重组事件的核心逻辑
//
// 🔄 **重组响应流程**：
//
// 1. **事件数据解析**：
//   - 解析重组事件中的前后链状态信息
//   - 提取重组长度和影响的区块高度范围
//
// 2. **影响评估**：
//   - 检查当前聚合高度是否在重组影响范围内
//   - 评估已收集的候选区块是否还有效
//
// 3. **状态清理**：
//   - 如果重组影响当前聚合，清理无效候选数据
//   - 重置聚合器到等待状态，避免基于无效数据做决策
//
// 4. **状态重置**：
//   - 根据重组情况调整聚合器当前高度
//   - 如果需要，触发新一轮聚合流程
//
// 参数：
//   - ctx: 处理上下文
//   - event: 链重组事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (h *chainReorganizedHandler) handleChainReorganized(ctx context.Context, reorgData *types.ChainReorganizedEventData) error {
	if h.logger != nil {
		h.logger.Infof("[ChainReorgHandler] 处理重组事件: old_height=%d, new_height=%d, reorg_length=%d",
			reorgData.OldHeight, reorgData.NewHeight, reorgData.ReorgLength)
	}

	// ==================== 2. 影响评估 ====================
	currentHeight := h.stateManager.GetCurrentHeight()

	// 检查重组是否影响当前聚合高度
	isAffected := h.isAggregationAffectedByReorg(currentHeight, reorgData)

	if h.logger != nil {
		h.logger.Infof("[ChainReorgHandler] 重组影响评估: current_height=%d, affected=%v",
			currentHeight, isAffected)
	}

	// ==================== 3. 状态清理和重置 ====================
	if isAffected {
		if h.logger != nil {
			h.logger.Warnf("[ChainReorgHandler] 当前聚合受重组影响，执行状态重置...")
		}

		// 重置聚合器到等待状态，清理可能无效的数据
		err := h.resetAggregatorForReorg(ctx, reorgData)
		if err != nil {
			return fmt.Errorf("重组状态重置失败: %w", err)
		}

		if h.logger != nil {
			h.logger.Info("[ChainReorgHandler] ✅ 聚合器状态重置完成")
		}
	} else {
		if h.logger != nil {
			h.logger.Info("[ChainReorgHandler] 当前聚合未受重组影响，继续正常流程")
		}
	}

	return nil
}

// isAggregationAffectedByReorg 判断当前聚合是否受重组影响
//
// 🔍 **影响评估逻辑**：
// - 如果当前聚合高度在重组影响范围内，则受影响
// - 如果当前聚合高度等于或接近重组分叉点，也可能受影响
func (h *chainReorganizedHandler) isAggregationAffectedByReorg(currentHeight uint64, reorgData *types.ChainReorganizedEventData) bool {
	// 计算重组的起始高度（分叉点）
	forkPoint := reorgData.OldHeight - uint64(reorgData.ReorgLength)

	// 如果当前聚合高度在分叉点之后，则可能受影响
	if currentHeight > forkPoint {
		return true
	}

	// 如果当前聚合高度等于分叉点，也需要谨慎处理
	if currentHeight == forkPoint {
		return true
	}

	// 其他情况不受影响
	return false
}

// resetAggregatorForReorg 为重组重置聚合器状态
//
// 🔄 **重置策略**：
//
// 1. **状态转换**：将聚合器转换到等待状态
// 2. **高度调整**：根据重组结果调整当前聚合高度
// 3. **数据清理**：清理可能无效的中间数据
//
// 参数：
//   - ctx: 处理上下文
//   - reorgData: 重组事件数据
//
// 返回：
//   - error: 重置过程中的错误
func (h *chainReorganizedHandler) resetAggregatorForReorg(ctx context.Context, reorgData *types.ChainReorganizedEventData) error {
	// 1. 确保处于等待状态，暂停当前聚合流程
	if err := h.stateManager.EnsureIdle(); err != nil {
		return fmt.Errorf("链重组恢复失败，无法确保Idle状态: %w", err)
	}

	// 2. 调整聚合高度到重组后的安全高度
	// 选择重组后的新高度，确保基于正确的链状态进行聚合
	newHeight := reorgData.NewHeight
	err := h.stateManager.SetCurrentHeight(newHeight)
	if err != nil {
		return fmt.Errorf("设置聚合高度失败: %w", err)
	}

	if h.logger != nil {
		h.logger.Infof("[ChainReorgHandler] 聚合器状态重置完成: new_height=%d, state=waiting",
			newHeight)
	}

	// 注意：候选区块数据的清理由各子组件在下次聚合开始时自动处理
	// 这里只负责核心状态的重置，避免过度耦合

	return nil
}
