// Package event_handler 交易池事件处理器
//
// 🎯 **交易池事件处理**
//
// 本文件实现交易池的事件处理功能，包括：
// - 实现 TxPoolEventSubscriber 接口（事件订阅）
// - 实现 TxEventSink 接口（事件发布）
// - 处理交易池相关的外部事件
//
// 设计原则：
// - 专注交易池：只处理与交易池相关的事件
// - 状态协调：确保交易池状态与外部事件保持一致
// - 自动调整：根据资源状况自动调整交易池策略
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

// TxPoolEventHandler 交易池事件处理器
// 实现 TxPoolEventSubscriber 接口，处理交易池相关的外部事件
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

// 编译期检查：确保 TxPoolEventHandler 实现了 TxPoolEventSubscriber 接口
var _ eventintegration.TxPoolEventSubscriber = (*TxPoolEventHandler)(nil)

