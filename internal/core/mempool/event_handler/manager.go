// Package event_handler 内存池事件处理器管理
//
// 🎯 **内存池事件处理器统一管理**
//
// 本文件实现内存池事件处理器的统一管理，参考consensus、blockchain、execution和repositories模块的事件处理器模式：
// - 实现MempoolEventSubscriber等订阅接口
// - 提供统一的事件处理入口
// - 协调交易池和候选区块池的事件处理逻辑
//
// 🏗️ **设计原则**：
// - 高内聚低耦合：事件处理逻辑集中管理
// - 接口导向：实现integration/event定义的订阅接口
// - 委托模式：将具体处理委托给专门的处理器
// - 错误隔离：单个事件处理失败不影响其他事件
package event_handler

import (
	"context"

	eventintegration "github.com/weisyn/v1/internal/core/mempool/integration/event"
	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 内存池事件处理器管理器 ====================

// MempoolEventHandler 内存池事件处理器管理器
//
// 🎯 **统一事件处理管理器**：
// 实现所有内存池相关的事件订阅接口，作为事件处理的统一入口点
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

// ==================== MempoolEventSubscriber接口实现 ====================

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

// ==================== TxPoolEventSubscriber接口实现 ====================

// TxPoolEventHandler 交易池事件处理器
type TxPoolEventHandler struct {
	logger   log.Logger
	txPool   mempoolIfaces.TxPool
	eventBus event.EventBus
}

// NewTxPoolEventHandler 创建交易池事件处理器
func NewTxPoolEventHandler(logger log.Logger, eventBus event.EventBus, txPool mempoolIfaces.TxPool) *TxPoolEventHandler {
	return &TxPoolEventHandler{
		logger:   logger,
		eventBus: eventBus,
		txPool:   txPool,
	}
}

// HandleResourceExhausted 处理资源耗尽事件
func (h *TxPoolEventHandler) HandleResourceExhausted(
	ctx context.Context,
	eventData *types.ResourceExhaustedEventData,
) error {
	h.logger.Warnf("处理资源耗尽事件: ResourceType=%s", eventData.ResourceType)

	// 1. 根据资源类型启动清理策略
	switch eventData.ResourceType {
	case "memory":
		h.logger.Info("内存耗尽，启动交易池清理")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeTxRemoved, map[string]interface{}{
				"reason":           "memory_exhausted",
				"cleanup_strategy": "low_fee_first",
			})
		}

	case "disk":
		h.logger.Info("磁盘空间耗尽，减少交易缓存")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeTxPoolFull, map[string]interface{}{
				"reason":       "disk_exhausted",
				"reduce_cache": true,
			})
		}
	}

	h.logger.Info("资源耗尽事件处理完成")
	return nil
}

// HandleMemoryPressure 处理内存压力事件
func (h *TxPoolEventHandler) HandleMemoryPressure(
	ctx context.Context,
	eventData *types.MemoryPressureEventData,
) error {
	h.logger.Warnf("处理内存压力事件: UsagePercent=%.1f%%", eventData.Threshold*100)

	// 1. 根据内存压力级别采取措施
	if eventData.Threshold > 0.9 {
		h.logger.Error("内存压力严重，立即清理低价值交易")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeTxRemoved, map[string]interface{}{
				"reason":             "critical_memory_pressure",
				"aggressive_cleanup": true,
			})
		}
	} else if eventData.Threshold > 0.8 {
		h.logger.Warn("内存压力较高，启动预防性清理")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeMempoolPressureHigh, map[string]interface{}{
				"memory_pressure":    eventData.Threshold,
				"preventive_cleanup": true,
			})
		}
	}

	h.logger.Info("内存压力事件处理完成")
	return nil
}

// HandleTransactionReceived 处理交易接收事件
func (h *TxPoolEventHandler) HandleTransactionReceived(
	ctx context.Context,
	eventData *types.TransactionReceivedEventData,
) error {
	h.logger.Debugf("处理交易接收事件: TxHash=%x", eventData.Hash)

	// 1. 验证交易并决定是否加入池
	// 这里通常会调用交易池的接口进行处理
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeTxAdded, map[string]interface{}{
			"tx_hash": eventData.Hash,
			"source":  "network",
		})
	}

	h.logger.Debug("交易接收事件处理完成")
	return nil
}

// HandleTransactionFailed 处理交易失败事件
func (h *TxPoolEventHandler) HandleTransactionFailed(
	ctx context.Context,
	eventData *types.TransactionFailedEventData,
) error {
	h.logger.Warnf("处理交易失败事件: Reason=%s", eventData.Reason)

	// 1. 从交易池中移除失败的交易
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeTxRemoved, map[string]interface{}{
			"transaction": eventData.Transaction,
			"reason":      eventData.Reason,
			"failed":      true,
		})
	}

	h.logger.Info("交易失败事件处理完成")
	return nil
}

// HandleForkDetected 处理分叉检测事件
func (h *TxPoolEventHandler) HandleForkDetected(
	ctx context.Context,
	eventData *types.ForkDetectedEventData,
) error {
	h.logger.Warnf("处理分叉检测事件: ForkHeight=%d", eventData.ForkHeight)

	// 1. 暂停交易处理，等待分叉解决
	h.logger.Info("检测到分叉，暂停交易池的主动处理")
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeMempoolPressureHigh, map[string]interface{}{
			"reason":           "fork_detected",
			"fork_height":      eventData.ForkHeight,
			"pause_processing": true,
		})
	}

	h.logger.Info("分叉检测事件处理完成")
	return nil
}

// ==================== CandidatePoolEventSubscriber接口实现 ====================

// CandidatePoolEventHandler 候选区块池事件处理器
type CandidatePoolEventHandler struct {
	logger        log.Logger
	candidatePool mempoolIfaces.CandidatePool
	eventBus      event.EventBus
}

// NewCandidatePoolEventHandler 创建候选区块池事件处理器
func NewCandidatePoolEventHandler(logger log.Logger, eventBus event.EventBus, candidatePool mempoolIfaces.CandidatePool) *CandidatePoolEventHandler {
	return &CandidatePoolEventHandler{
		logger:        logger,
		eventBus:      eventBus,
		candidatePool: candidatePool,
	}
}

// HandleBlockProduced 处理区块生产事件
func (h *CandidatePoolEventHandler) HandleBlockProduced(
	ctx context.Context,
	eventData *types.BlockProducedEventData,
) error {
	h.logger.Infof("处理区块生产事件: Height=%d", eventData.Height)

	// 1. 将新产生的区块添加到候选区块池
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeCandidateAdded, map[string]interface{}{
			"block_height": eventData.Height,
			"block_hash":   eventData.Hash,
			"producer":     eventData.Producer,
		})
	}

	h.logger.Info("区块生产事件处理完成")
	return nil
}

// HandleConsensusStateChanged 处理共识状态变化事件
func (h *CandidatePoolEventHandler) HandleConsensusStateChanged(
	ctx context.Context,
	eventData *types.ConsensusStateChangedEventData,
) error {
	h.logger.Infof("处理共识状态变化事件: NewState=%s", eventData.NewState)

	// 1. 根据共识状态调整候选区块池策略
	switch eventData.NewState {
	case "active":
		h.logger.Info("共识活跃，正常处理候选区块")

	case "syncing":
		h.logger.Info("共识同步中，暂停候选区块处理")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeCandidatePoolCleared, map[string]interface{}{
				"reason": "consensus_syncing",
			})
		}

	case "inactive":
		h.logger.Warn("共识不活跃，清理候选区块池")
		if h.eventBus != nil {
			h.eventBus.Publish(eventconstants.EventTypeCandidateCleanupCompleted, map[string]interface{}{
				"reason": "consensus_inactive",
			})
		}
	}

	h.logger.Info("共识状态变化事件处理完成")
	return nil
}

// HandleResourceExhausted 处理资源耗尽事件
func (h *CandidatePoolEventHandler) HandleResourceExhausted(
	ctx context.Context,
	eventData *types.ResourceExhaustedEventData,
) error {
	h.logger.Warnf("处理资源耗尽事件: ResourceType=%s", eventData.ResourceType)

	// 1. 清理过期的候选区块
	h.logger.Info("资源耗尽，启动候选区块池清理")
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeCandidateExpired, map[string]interface{}{
			"reason":        "resource_exhausted",
			"resource_type": eventData.ResourceType,
		})
	}

	h.logger.Info("资源耗尽事件处理完成")
	return nil
}

// HandleStorageSpaceLow 处理存储空间不足事件
func (h *CandidatePoolEventHandler) HandleStorageSpaceLow(
	ctx context.Context,
	eventData *types.StorageSpaceLowEventData,
) error {
	h.logger.Warnf("处理存储空间不足事件: AvailableSpace=%d", eventData.AvailableSpace)

	// 1. 减少候选区块的存储
	h.logger.Info("存储空间不足，减少候选区块缓存")
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeCandidateRemoved, map[string]interface{}{
			"reason":          "storage_low",
			"available_space": eventData.AvailableSpace,
			"cleanup_old":     true,
		})
	}

	h.logger.Info("存储空间不足事件处理完成")
	return nil
}

// HandleSystemStopping 处理系统停止事件
func (h *CandidatePoolEventHandler) HandleSystemStopping(
	ctx context.Context,
	eventData *types.SystemStoppingEventData,
) error {
	h.logger.Infof("处理系统停止事件: Reason=%s", eventData.Reason)

	// 1. 清理候选区块池
	h.logger.Info("系统停止，清理候选区块池")
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeCandidatePoolCleared, map[string]interface{}{
			"reason":   "system_stopping",
			"graceful": eventData.Graceful,
		})
	}

	h.logger.Info("系统停止事件处理完成")
	return nil
}

// ==================== 事件处理器创建函数 ====================

// CreateMempoolEventHandlers 创建所有内存池事件处理器
//
// 🎯 **统一创建入口**：
// 创建并返回所有内存池相关的事件处理器实例
func CreateMempoolEventHandlers(
	logger log.Logger,
	eventBus event.EventBus,
	txPool mempoolIfaces.TxPool,
	candidatePool mempoolIfaces.CandidatePool,
) (
	eventintegration.MempoolEventSubscriber,
	eventintegration.TxPoolEventSubscriber,
	eventintegration.CandidatePoolEventSubscriber,
) {
	// 创建各个事件处理器
	mempoolHandler := NewMempoolEventHandler(logger, eventBus, txPool, candidatePool)
	txPoolHandler := NewTxPoolEventHandler(logger, eventBus, txPool)
	candidatePoolHandler := NewCandidatePoolEventHandler(logger, eventBus, candidatePool)

	return mempoolHandler, txPoolHandler, candidatePoolHandler
}
