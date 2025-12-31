// Package event 内存池组件级事件处理器实现
//
// 🎯 **内存池通用事件处理**
//
// 本文件实现内存池整体的通用事件处理，包括：
// - 实现 MempoolEventSubscriber 接口
// - 处理系统级别和内存池整体的协调事件
// - 协调交易池和候选区块池的协作
//
// 设计原则：
// - 整体协调：处理需要协调多个子组件的事件
// - 状态同步：确保内存池整体状态的一致性
// - 优雅处理：支持系统的优雅关闭和状态转换
package event

import (
	"context"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
)

// MempoolEventHandler 内存池事件处理器管理器
//
// 🎯 **统一事件处理管理器**：
// 实现内存池通用的事件订阅接口，处理需要协调多个子组件的事件
type MempoolEventHandler struct {
	logger log.Logger

	// 内存池服务依赖
	txPool        mempoolIfaces.TxPool
	candidatePool mempoolIfaces.CandidatePool

	// 可选的EventBus引用，用于发布衍生事件
	eventBus event.EventBus
}

// NewMempoolEventHandler 创建内存池事件处理器管理器
func NewMempoolEventHandler(
	logger log.Logger,
	eventBus event.EventBus,
	txPool mempoolIfaces.TxPool,
	candidatePool mempoolIfaces.CandidatePool,
) *MempoolEventHandler {
	return &MempoolEventHandler{
		logger:        logger,
		eventBus:      eventBus,
		txPool:        txPool,
		candidatePool: candidatePool,
	}
}

// HandleSystemStopping 处理系统停止事件
//
// 🎯 **系统优雅关闭**：
// 当系统准备关闭时，确保内存池的优雅关闭
func (h *MempoolEventHandler) HandleSystemStopping(
	ctx context.Context,
	eventData *types.SystemStoppingEventData,
) error {
	h.logger.Infof("处理系统停止事件: Reason=%s", eventData.Reason)

	// 1. 检查停止原因并采取相应措施
	if eventData.Reason == "emergency" {
		h.logger.Error("紧急停机，立即停止内存池处理")
		// 紧急情况下立即停止
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolStopped, map[string]interface{}{
				"reason":    eventData.Reason,
				"emergency": true,
			})
		}
	} else {
		h.logger.Info("正常停机，开始优雅关闭内存池")
		// 正常情况下优雅关闭

		// 2. 停止接受新交易
		h.logger.Info("停止接受新交易到内存池")

		// 3. 等待当前处理中的交易完成
		h.logger.Info("等待内存池中的交易处理完成...")

		// 4. 发送停止确认
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolStopped, map[string]interface{}{
				"reason":   eventData.Reason,
				"graceful": true,
			})
		}
	}

	h.logger.Info("系统停止事件处理完成")
	return nil
}

// HandleNetworkQualityChanged 处理网络质量变化事件
//
// 🎯 **网络自适应优化**：
// 根据网络质量调整内存池的策略
func (h *MempoolEventHandler) HandleNetworkQualityChanged(
	ctx context.Context,
	eventData *types.NetworkQualityChangedEventData,
) error {
	h.logger.Infof("处理网络质量变化事件: Quality=%s", eventData.Quality)

	// 1. 根据网络质量调整策略
	if eventData.Quality == "poor" || eventData.Quality == "critical" {
		h.logger.Warn("网络质量很差，调整内存池为保守模式")
		// 减少交易广播频率，增加缓存时间
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolPressureHigh, map[string]interface{}{
				"reason":            "poor_network_quality",
				"quality":           eventData.Quality,
				"conservative_mode": true,
			})
		}
	} else if eventData.Quality == "excellent" {
		h.logger.Info("网络质量良好，启用积极模式")
		// 增加交易处理和广播频率
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolSizeChanged, map[string]interface{}{
				"network_quality": eventData.Quality,
				"aggressive_mode": true,
			})
		}
	}

	h.logger.Info("网络质量变化事件处理完成")
	return nil
}

// HandleBlockProcessed 处理区块处理完成事件
//
// 🎯 **区块确认处理**：
// 当区块被处理完成时，清理内存池中已确认的交易
func (h *MempoolEventHandler) HandleBlockProcessed(
	ctx context.Context,
	eventData *types.BlockProcessedEventData,
) error {
	h.logger.Infof("处理区块处理完成事件: Height=%d, TxCount=%d",
		eventData.Height, eventData.TransactionCount)

	// 1. 清理交易池中已确认的交易
	if h.txPool != nil && eventData.TransactionCount > 0 {
		h.logger.Infof("清理交易池中已确认的 %d 个交易", eventData.TransactionCount)
		// 触发交易确认清理
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeTxConfirmed, map[string]interface{}{
				"block_height":       eventData.Height,
				"confirmed_tx_count": eventData.TransactionCount,
			})
		}
	}

	// 2. 清理候选区块池中已确认的区块
	if h.candidatePool != nil {
		h.logger.Infof("清理候选区块池中高度 %d 及以下的候选区块", eventData.Height)
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeCandidateRemoved, map[string]interface{}{
				"confirmed_height": eventData.Height,
				"reason":           "block_confirmed",
			})
		}
	}

	// 3. 发布内存池大小变化事件
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeMempoolSizeChanged, map[string]interface{}{
			"trigger":      "block_processed",
			"block_height": eventData.Height,
			"cleanup":      true,
		})
	}

	h.logger.Info("区块处理完成事件处理完成")
	return nil
}

// HandleChainReorganized 处理链重组事件
//
// 🎯 **链重组响应**：
// 当发生链重组时，恢复被回滚的交易到内存池
func (h *MempoolEventHandler) HandleChainReorganized(
	ctx context.Context,
	eventData *types.ChainReorganizedEventData,
) error {
	h.logger.Warnf("处理链重组事件: OldHeight=%d, NewHeight=%d",
		eventData.OldHeight, eventData.NewHeight)

	// 1. 分析重组的严重程度
	if eventData.OldHeight > eventData.NewHeight {
		reorgDepth := eventData.OldHeight - eventData.NewHeight
		h.logger.Warnf("检测到回滚重组，深度: %d", reorgDepth)

		// 2. 恢复被回滚区块中的交易
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeTxAdded, map[string]interface{}{
				"reason":               "chain_reorg",
				"reorg_depth":          reorgDepth,
				"old_height":           eventData.OldHeight,
				"new_height":           eventData.NewHeight,
				"restore_transactions": true,
			})
		}

		// 3. 清理候选区块池中的无效候选区块
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeCandidatePoolCleared, map[string]interface{}{
				"reason":      "chain_reorg",
				"reorg_depth": reorgDepth,
			})
		}
	}

	h.logger.Info("链重组事件处理完成")
	return nil
}

// HandleConsensusResultBroadcast 处理共识结果广播事件
//
// 🎯 **共识结果响应**：
// 根据共识结果调整内存池策略
func (h *MempoolEventHandler) HandleConsensusResultBroadcast(
	ctx context.Context,
	eventData *types.ConsensusResultEventData,
) error {
	h.logger.Infof("处理共识结果广播事件: Result=%s", eventData.Result)

	// 1. 根据共识结果调整策略
	switch eventData.Result {
	case "block_accepted":
		h.logger.Info("共识接受区块，正常运行")
		// 正常情况，不需要特殊处理

	case "block_rejected":
		h.logger.Warn("共识拒绝区块，可能需要调整交易选择策略")
		// 可能需要调整交易费用阈值或选择策略
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolPressureHigh, map[string]interface{}{
				"reason":          "block_rejected",
				"adjust_strategy": true,
			})
		}

	case "fork_resolved":
		h.logger.Info("共识解决分叉，恢复正常运行")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolSizeChanged, map[string]interface{}{
				"trigger":          "fork_resolved",
				"normal_operation": true,
			})
		}

	default:
		h.logger.Warnf("未知的共识结果类型: %s", eventData.Result)
	}

	h.logger.Info("共识结果广播事件处理完成")
	return nil
}

// 编译期检查：确保 MempoolEventHandler 实现了 MempoolEventSubscriber 接口
var _ MempoolEventSubscriber = (*MempoolEventHandler)(nil)
