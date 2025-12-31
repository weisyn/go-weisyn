// Package event_handler 分叉事件专门处理器
//
// 🔀 **分叉事件统一处理器**
//
// 本文件实现矿工对分叉事件的响应逻辑，基于原integration/event/fork_handler.go重构：
// - 监听分叉检测、处理中、完成事件
// - 自动暂停/恢复挖矿以避免在分叉期间产生无效区块
// - 协调挖矿状态与区块链状态的一致性
// - 与矿工状态管理器和控制器协调工作
//
// 🎯 **事件响应策略**：
// 1. ForkDetected → 立即暂停挖矿，避免产生冲突区块
// 2. ForkProcessing → 保持暂停状态，等待处理完成
// 3. ForkCompleted → 根据处理结果决定是否恢复挖矿
package event_handler

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// forkEventsHandler 分叉事件统一处理器
//
// 🎯 **专门职责**：
// 处理所有分叉相关事件，确保挖矿状态与区块链分叉处理的一致性
//
// 基于原integration/event/fork_handler.go的核心逻辑重构
type forkEventsHandler struct {
	logger          log.Logger                   // 日志记录器
	minerController interfaces.MinerController   // 矿工控制器（用于启停挖矿）
	stateManager    interfaces.MinerStateManager // 矿工状态管理器

	// 分叉状态管理
	isPausedForFork   bool      // 是否因分叉而暂停
	forkStartTime     time.Time // 分叉开始时间
	lastForkHeight    uint64    // 最后处理的分叉高度
	savedMinerAddress []byte    // 暂停前保存的矿工地址，用于恢复
}

// newForkEventsHandler 创建分叉事件处理器
//
// 🏗️ **内部构造器**：
// 仅供manager.go使用的内部构造函数
func newForkEventsHandler(
	logger log.Logger,
	minerController interfaces.MinerController,
	stateManager interfaces.MinerStateManager,
) *forkEventsHandler {
	return &forkEventsHandler{
		logger:          logger,
		minerController: minerController,
		stateManager:    stateManager,
	}
}

// ==================== 分叉事件处理方法 ====================

// handleForkDetected 处理分叉检测事件的核心逻辑
//
// 🔀 **分叉检测响应流程**：
//
// 1. **事件数据解析**：
//   - 解析分叉检测事件数据，获取分叉信息
//   - 记录分叉高度和检测时间
//
// 2. **挖矿状态检查**：
//   - 检查当前挖矿状态，如果正在挖矿则立即暂停
//   - 保存当前挖矿地址用于后续恢复
//
// 3. **状态标记**：
//   - 设置分叉暂停标志
//   - 记录分叉开始时间用于监控
//
// 参数：
//   - ctx: 处理上下文
//   - forkData: 分叉检测事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (h *forkEventsHandler) handleForkDetected(ctx context.Context, forkData *types.ForkDetectedEventData) error {
	if h.logger != nil {
		h.logger.Infof("[ForkEventHandler] 🔀 分叉检测详情: height=%d, type=%s, message=%s",
			forkData.ForkHeight, forkData.ForkType, forkData.Message)
	}

	// 记录分叉信息
	h.lastForkHeight = forkData.ForkHeight
	h.forkStartTime = time.Unix(forkData.DetectedAt, 0)

	// ==================== 2. 暂停挖矿 ====================
	return h.pauseMiningForFork(ctx, fmt.Sprintf("检测到分叉: %s", forkData.ForkType))
}

// handleForkProcessing 处理分叉处理中事件的核心逻辑
//
// 🔄 **分叉处理进度响应流程**：
//
// 1. **进度信息记录**：
//   - 解析分叉处理进度事件数据
//   - 记录处理阶段和进度信息
//
// 2. **状态一致性检查**：
//   - 确认挖矿仍然处于暂停状态
//   - 如果检测到异常状态，进行纠正
//
// 参数：
//   - ctx: 处理上下文
//   - processingData: 分叉处理中事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (h *forkEventsHandler) handleForkProcessing(ctx context.Context, processingData *types.ForkProcessingEventData) error {
	if h.logger != nil {
		h.logger.Debugf("[ForkEventHandler] 🔄 分叉处理进度: stage=%s, progress=%.1f%%, message=%s",
			processingData.ProcessStage, processingData.Progress*100, processingData.Message)
	}

	// ==================== 2. 状态一致性检查 ====================
	// 确保挖矿仍然处于暂停状态
	if !h.isPausedForFork {
		if h.logger != nil {
			h.logger.Warnf("[ForkEventHandler] 分叉处理中但挖矿未暂停，立即暂停")
		}
		return h.pauseMiningForFork(ctx, fmt.Sprintf("分叉处理中: %s", processingData.ProcessStage))
	}

	return nil
}

// handleForkCompleted 处理分叉完成事件的核心逻辑
//
// ✅ **分叉处理完成响应流程**：
//
// 1. **结果评估**：
//   - 解析分叉完成事件数据，获取处理结果
//   - 记录处理耗时和结果状态
//
// 2. **恢复决策**：
//   - 如果处理成功，使用保存的状态恢复挖矿
//   - 如果处理失败，保持暂停状态等待人工干预
//
// 3. **状态清理**：
//   - 清理分叉暂停标志和状态数据
//   - 重置分叉相关状态
//
// 参数：
//   - ctx: 处理上下文
//   - completedData: 分叉完成事件数据
//
// 返回：
//   - error: 处理过程中的错误
func (h *forkEventsHandler) handleForkCompleted(ctx context.Context, completedData *types.ForkCompletedEventData) error {
	// 记录分叉处理结果
	processingDuration := time.Duration(completedData.ProcessingTime) * time.Millisecond

	if h.logger != nil {
		h.logger.Infof("[ForkEventHandler] 分叉处理结果: success=%v, chain_switched=%v, duration=%v",
			completedData.Success, completedData.ChainSwitched, processingDuration)

		if completedData.Error != "" {
			h.logger.Warnf("[ForkEventHandler] 分叉处理错误: %s", completedData.Error)
		}
	}

	// ==================== 2. 根据处理结果决定是否恢复挖矿 ====================
	if completedData.Success {
		// 分叉处理成功，恢复挖矿
		message := fmt.Sprintf("分叉处理成功完成 (耗时: %v)", processingDuration)
		if completedData.ChainSwitched {
			message += ", 已切换到新链"
		}

		return h.resumeMiningIfNeeded(ctx, message)
	} else {
		// 分叉处理失败，保持暂停状态，等待系统恢复
		if h.logger != nil {
			h.logger.Warnf("[ForkEventHandler] 分叉处理失败，继续保持挖矿暂停状态")
		}
		return nil
	}
}

// ==================== 挖矿控制辅助方法 ====================

// pauseMiningForFork 因分叉而暂停挖矿
//
// 🔒 **暂停挖矿逻辑**：
// 基于原fork_handler.go的pauseMiningForFork方法重构
func (h *forkEventsHandler) pauseMiningForFork(ctx context.Context, reason string) error {
	// 检查是否已经暂停
	if h.isPausedForFork {
		if h.logger != nil {
			h.logger.Debugf("[ForkEventHandler] 挖矿已因分叉暂停，跳过重复暂停")
		}
		return nil
	}

	// 获取当前挖矿状态
	isRunning, minerAddress, err := h.minerController.GetMiningStatus(ctx)
	if err != nil {
		return fmt.Errorf("获取矿工状态失败: %w", err)
	}

	// 只有在挖矿运行时才需要暂停
	if !isRunning {
		if h.logger != nil {
			h.logger.Debugf("[ForkEventHandler] 矿工当前未运行，无需暂停")
		}
		return nil
	}

	// 保存当前的矿工地址，用于后续恢复
	h.savedMinerAddress = make([]byte, len(minerAddress))
	copy(h.savedMinerAddress, minerAddress)

	// 暂停挖矿（通过停止挖矿实现）
	if h.logger != nil {
		h.logger.Infof("[ForkEventHandler] ⏸️ 因分叉暂停挖矿: %s", reason)
	}

	err = h.minerController.StopMining(ctx)
	if err != nil {
		return fmt.Errorf("暂停挖矿失败: %w", err)
	}

	h.isPausedForFork = true
	if h.logger != nil {
		h.logger.Infof("[ForkEventHandler] ✅ 挖矿已暂停")
	}

	return nil
}

// resumeMiningIfNeeded 在需要时恢复挖矿
//
// ▶️ **恢复挖矿逻辑**：
// 基于原fork_handler.go的resumeMiningIfNeeded方法重构
func (h *forkEventsHandler) resumeMiningIfNeeded(ctx context.Context, reason string) error {
	// 检查是否因分叉而暂停
	if !h.isPausedForFork {
		if h.logger != nil {
			h.logger.Debugf("[ForkEventHandler] 挖矿未因分叉暂停，无需恢复")
		}
		return nil
	}

	// 检查是否有保存的矿工地址
	if len(h.savedMinerAddress) == 0 {
		if h.logger != nil {
			h.logger.Warnf("[ForkEventHandler] 没有保存的矿工地址，无法恢复挖矿")
		}
		h.isPausedForFork = false // 重置标志
		return nil
	}

	// 获取当前挖矿状态
	isRunning, _, err := h.minerController.GetMiningStatus(ctx)
	if err != nil {
		return fmt.Errorf("获取矿工状态失败: %w", err)
	}

	// 如果已经在运行，只需重置标志
	if isRunning {
		if h.logger != nil {
			h.logger.Debugf("[ForkEventHandler] 矿工已在运行，只需重置分叉标志")
		}
		h.isPausedForFork = false
		return nil
	}

	// 恢复挖矿（使用保存的矿工地址重新启动）
	if h.logger != nil {
		h.logger.Infof("[ForkEventHandler] ▶️ 恢复挖矿: %s", reason)
	}

	err = h.minerController.StartMining(ctx, h.savedMinerAddress)
	if err != nil {
		return fmt.Errorf("恢复挖矿失败: %w", err)
	}

	h.isPausedForFork = false
	h.savedMinerAddress = nil // 清空保存的地址

	if h.logger != nil {
		forkDuration := time.Since(h.forkStartTime)
		h.logger.Infof("[ForkEventHandler] ✅ 挖矿已恢复 (分叉处理总耗时: %v)", forkDuration)
	}

	return nil
}

// ==================== 状态查询接口 ====================

// GetForkHandlerStatus 获取分叉处理器状态
//
// 📊 **状态查询接口**：
// 提供分叉事件处理器的当前状态信息，用于监控和调试
func (h *forkEventsHandler) GetForkHandlerStatus() ForkHandlerStatus {
	return ForkHandlerStatus{
		IsPausedForFork:   h.isPausedForFork,
		ForkStartTime:     h.forkStartTime,
		LastForkHeight:    h.lastForkHeight,
		SavedMinerAddress: h.savedMinerAddress,
	}
}

// ForkHandlerStatus 分叉处理器状态
//
// 📊 **状态数据结构**：
// 基于原fork_handler.go的ForkHandlerStatus结构重构
type ForkHandlerStatus struct {
	IsPausedForFork   bool      `json:"is_paused_for_fork"`  // 是否因分叉而暂停
	ForkStartTime     time.Time `json:"fork_start_time"`     // 分叉开始时间
	LastForkHeight    uint64    `json:"last_fork_height"`    // 最后处理的分叉高度
	SavedMinerAddress []byte    `json:"saved_miner_address"` // 暂停前保存的矿工地址
}
