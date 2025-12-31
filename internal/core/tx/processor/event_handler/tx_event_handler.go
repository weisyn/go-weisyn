// Package event_handler 实现交易事件订阅处理服务
//
// 🎯 **交易事件订阅处理服务模块**
//
// 本包实现 TransactionEventSubscriber 接口，提供交易事件订阅处理功能：
// - 监听交易生命周期事件（接收、验证、执行、确认、失败）
// - 监听内存池事件（添加、移除）
// - 维护交易处理统计信息
//
// 设计理念：
// - 被动监听：只响应事件，不主动发起
// - 统计追踪：维护交易处理的统计和性能指标
// - 无副作用：不修改交易状态，只做记录和统计
package event_handler

import (
	"sync"
	"time"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	eventIf "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// EventHandler 交易事件处理器
//
// 🔧 **交易生命周期事件处理**
//
// 核心职责：
// - 跟踪交易从接收到确认的完整生命周期
// - 响应内存池的交易状态变化通知
// - 维护交易处理的性能指标和错误统计
//
// 事件流程：
// TransactionReceived → TransactionValidated → TransactionExecuted → TransactionConfirmed
//
//	↘                      ↘
//	  TransactionFailed     TransactionFailed
type EventHandler struct {
	logger   log.Logger
	eventBus eventIf.EventBus

	mu sync.RWMutex // 保护统计数据

	// 交易状态统计
	receivedCount  uint64 // 接收交易总数
	validatedCount uint64 // 验证通过交易数
	executedCount  uint64 // 执行成功交易数
	confirmedCount uint64 // 确认交易数
	failedCount    uint64 // 失败交易数

	// 性能统计
	avgValidationTime time.Duration // 平均验证时间
	avgExecutionTime  time.Duration // 平均执行时间
	lastProcessTime   time.Time     // 最后处理时间
}

// NewEventHandler 创建交易事件处理器
func NewEventHandler(logger log.Logger, eventBus eventIf.EventBus) *EventHandler {
	return &EventHandler{
		logger:          logger,
		eventBus:        eventBus,
		lastProcessTime: time.Now(),
	}
}

// HandleTransactionReceived 处理交易接收事件
//
// 📨 **交易接收处理**
//
// 处理逻辑：
// 1. 记录交易基本信息（发送者、接收者、金额）
// 2. 更新接收统计计数
// 3. 发布交易接收确认事件
func (h *EventHandler) HandleTransactionReceived(eventData *types.TransactionReceivedEventData) error {
	h.mu.Lock()
	h.receivedCount++
	h.lastProcessTime = time.Now()
	h.mu.Unlock()

	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Event] 📨 接收交易: %s, 发送者: %s, 金额: %d, 手续费: %d",
			eventData.Hash, eventData.From, eventData.Value, eventData.Fee)
	}

	// 发布交易接收确认事件（如果需要）
	if h.eventBus != nil {
		confirmData := map[string]interface{}{
			"tx_hash":      eventData.Hash,
			"received_at":  eventData.Timestamp,
			"from_address": eventData.From,
			"to_address":   eventData.To,
			"amount":       eventData.Value,
			"fee":          eventData.Fee,
			"status":       "received",
		}
		h.eventBus.Publish("transaction.status.received", confirmData)
	}

	return nil
}

// HandleTransactionValidated 处理交易验证事件
//
// ✅ **交易验证结果处理**
//
// 验证处理：
// 1. 检查验证结果，更新相应统计
// 2. 对验证失败的交易记录错误原因
// 3. 发布验证结果事件
func (h *EventHandler) HandleTransactionValidated(eventData *types.TransactionValidatedEventData) error {
	h.mu.Lock()
	if eventData.Valid {
		h.validatedCount++
	} else {
		h.failedCount++
	}
	h.mu.Unlock()

	if eventData.Valid {
		if h.logger != nil {
			h.logger.Infof("[TxProcessor/Event] ✅ 交易验证通过: %s", eventData.Hash)
		}

		// 发布验证通过事件
		if h.eventBus != nil {
			validData := map[string]interface{}{
				"tx_hash":      eventData.Hash,
				"validated_at": eventData.Timestamp,
				"status":       "validated",
			}
			h.eventBus.Publish("transaction.status.validated", validData)
		}
	} else {
		if h.logger != nil {
			h.logger.Warnf("[TxProcessor/Event] 🚫 交易验证失败: %s, 错误: %v", eventData.Hash, eventData.Errors)
		}

		// 发布验证失败事件
		if h.eventBus != nil {
			failData := map[string]interface{}{
				"tx_hash":   eventData.Hash,
				"failed_at": eventData.Timestamp,
				"status":    "validation_failed",
				"errors":    eventData.Errors,
			}
			h.eventBus.Publish("transaction.status.failed", failData)
		}
	}

	return nil
}

// HandleTransactionExecuted 处理交易执行事件
//
// ⚙️ **交易执行结果处理**
//
// 执行处理：
// 1. 记录执行结果和执行费用消耗
// 2. 更新执行统计和性能指标
// 3. 发布执行结果事件
func (h *EventHandler) HandleTransactionExecuted(eventData *types.TransactionExecutedEventData) error {
	h.mu.Lock()
	if eventData.Success {
		h.executedCount++
	} else {
		h.failedCount++
	}
	h.mu.Unlock()

	if eventData.Success {
		if h.logger != nil {
			h.logger.Infof("[TxProcessor/Event] ⚙️ 交易执行成功: %s, 执行费用: %d, 结果: %s",
				eventData.Hash, eventData.ExecutionFeeUsed, eventData.Result)
		}

		// 发布执行成功事件
		if h.eventBus != nil {
			successData := map[string]interface{}{
				"tx_hash":            eventData.Hash,
				"block_height":       eventData.BlockHeight,
				"execution_fee_used": eventData.ExecutionFeeUsed,
				"result":             eventData.Result,
				"executed_at":        eventData.Timestamp,
				"status":             "executed",
			}
			h.eventBus.Publish("transaction.status.executed", successData)
		}
	} else {
		if h.logger != nil {
			h.logger.Warnf("[TxProcessor/Event] ⚠️ 交易执行失败: %s, 执行费用: %d",
				eventData.Hash, eventData.ExecutionFeeUsed)
		}
	}

	return nil
}

// HandleTransactionFailed 处理交易失败事件
//
// 💥 **交易失败处理**
//
// 失败处理：
// 1. 记录失败原因和上下文
// 2. 更新失败统计计数
// 3. 发布失败通知
func (h *EventHandler) HandleTransactionFailed(eventData *types.TransactionFailedEventData) error {
	h.mu.Lock()
	h.failedCount++
	h.mu.Unlock()

	if h.logger != nil {
		h.logger.Errorf("[TxProcessor/Event] 💥 交易处理失败: %s, 区块: %d, 错误: %s, 执行费用消耗: %d",
			eventData.Hash, eventData.BlockHeight, eventData.Error, eventData.ExecutionFeeUsed)
	}

	// 发布详细的失败事件
	if h.eventBus != nil {
		failureData := map[string]interface{}{
			"tx_hash":            eventData.Hash,
			"block_height":       eventData.BlockHeight,
			"error":              eventData.Error,
			"execution_fee_used": eventData.ExecutionFeeUsed,
			"failed_at":          eventData.Timestamp,
			"status":             "failed",
		}
		h.eventBus.Publish("transaction.status.failed", failureData)

		// 通知mempool移除失败交易
		removeData := map[string]interface{}{
			"tx_hash": eventData.Hash,
			"reason":  "execution_failed",
		}
		h.eventBus.Publish(eventconstants.EventTypeTxRemoved, removeData)
	}

	return nil
}

// HandleTransactionConfirmed 处理交易确认事件
//
// 🎯 **交易最终确认处理**
//
// 确认处理：
// 1. 记录交易最终确认状态
// 2. 更新确认统计计数
// 3. 发布确认通知
func (h *EventHandler) HandleTransactionConfirmed(eventData *types.TransactionConfirmedEventData) error {
	h.mu.Lock()
	h.confirmedCount++
	h.mu.Unlock()

	if h.logger != nil {
		finalStatus := ""
		if eventData.Final {
			finalStatus = " (最终确认)"
		}

		h.logger.Infof("[TxProcessor/Event] 🎯 交易确认: %s, 区块: %d, 确认数: %d%s",
			eventData.Hash, eventData.BlockHeight, eventData.Confirmations, finalStatus)
	}

	// 发布确认事件
	if h.eventBus != nil {
		confirmData := map[string]interface{}{
			"tx_hash":       eventData.Hash,
			"block_height":  eventData.BlockHeight,
			"block_hash":    eventData.BlockHash,
			"confirmations": eventData.Confirmations,
			"final":         eventData.Final,
			"confirmed_at":  eventData.Timestamp,
			"status":        "confirmed",
		}
		h.eventBus.Publish("transaction.status.confirmed", confirmData)

		// 如果是最终确认，发布特殊事件
		if eventData.Final {
			finalData := map[string]interface{}{
				"tx_hash":      eventData.Hash,
				"block_height": eventData.BlockHeight,
				"finalized_at": eventData.Timestamp,
			}
			h.eventBus.Publish("transaction.status.finalized", finalData)
		}
	}

	return nil
}

// HandleMempoolTransactionAdded 处理交易添加到内存池事件
//
// ➕ **内存池交易添加处理**
//
// 处理流程：
// 1. 记录交易进入内存池
// 2. 更新统计计数器
func (h *EventHandler) HandleMempoolTransactionAdded(eventData *types.TransactionReceivedEventData) error {
	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Event] ➕ 交易添加到内存池: %s", eventData.Hash)
	}

	h.mu.Lock()
	h.receivedCount++
	h.lastProcessTime = time.Now()
	h.mu.Unlock()

	// 发布内部事件（如果需要）
	if h.eventBus != nil {
		h.eventBus.Publish(eventconstants.EventTypeTxAdded, eventData)
	}

	return nil
}

// HandleMempoolTransactionRemoved 处理内存池交易移除事件
//
// 🗑️ **内存池交易移除处理**
//
// 移除处理：
// 1. 响应内存池的交易移除通知
// 2. 根据移除原因执行相应处理
func (h *EventHandler) HandleMempoolTransactionRemoved(eventData *types.TransactionRemovedEventData) error {
	if h.logger != nil {
		h.logger.Infof("[TxProcessor/Event] 🗑️ 内存池移除交易: %s, 原因: %s, 池: %s",
			eventData.Hash, eventData.Reason, eventData.Pool)
	}

	// 根据移除原因采取不同行动
	switch eventData.Reason {
	case "expired":
		// 交易过期，无需特殊处理

	case "included":
		// 交易已被打包，这是正常流程
		if h.logger != nil {
			h.logger.Infof("[TxProcessor/Event] ✅ 交易已被打包: %s", eventData.Hash)
		}

	case "invalid":
		// 交易无效，记录错误
		if h.logger != nil {
		h.logger.Warnf("[TxProcessor/Event] 🚫 交易被标记为无效: %s", eventData.Hash)
		}

	case "replaced":
		// 交易被替换，记录信息
		if h.logger != nil {
			h.logger.Infof("[TxProcessor/Event] 🔄 交易被替换: %s", eventData.Hash)
		}
	}

	return nil
}

// GetTransactionStats 获取交易处理统计信息
func (h *EventHandler) GetTransactionStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := h.receivedCount
	successRate := float64(0)
	if total > 0 {
		successRate = float64(h.confirmedCount) / float64(total) * 100
	}

	return map[string]interface{}{
		"received_count":    h.receivedCount,
		"validated_count":   h.validatedCount,
		"executed_count":    h.executedCount,
		"confirmed_count":   h.confirmedCount,
		"failed_count":      h.failedCount,
		"success_rate":      successRate,
		"last_process_time": h.lastProcessTime,
	}
}
