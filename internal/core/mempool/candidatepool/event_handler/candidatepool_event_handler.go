// Package event_handler 候选区块池事件处理器
//
// 🎯 **候选区块池事件处理**
//
// 本文件实现候选区块池的事件处理功能，包括：
// - 实现 CandidatePoolEventSubscriber 接口（事件订阅）
// - 实现 CandidateEventSink 接口（事件发布）
// - 处理候选区块池相关的外部事件
//
// 设计原则：
// - 专注候选区块池：只处理与候选区块池相关的事件
// - 状态协调：确保候选区块池状态与外部事件保持一致
// - 自动调整：根据资源状况自动调整候选区块池策略
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

// CandidatePoolEventHandler 候选区块池事件处理器
// 实现 CandidatePoolEventSubscriber 接口，处理候选区块池相关的外部事件
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

// 编译期检查：确保 CandidatePoolEventHandler 实现了 CandidatePoolEventSubscriber 接口
var _ eventintegration.CandidatePoolEventSubscriber = (*CandidatePoolEventHandler)(nil)

