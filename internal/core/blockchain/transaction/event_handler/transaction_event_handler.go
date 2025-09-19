// Package event_handler provides event handling capabilities for the transaction module
//
// 🎯 **Transaction模块事件处理器**
//
// 专门处理transaction模块相关的事件：
// - 交易接收事件（TransactionReceived）
// - 交易验证事件（TransactionValidated）
// - 交易执行事件（TransactionExecuted）
// - 交易确认事件（TransactionConfirmed）
// - 交易失败事件（TransactionFailed）
// - UTXO状态变化事件（UTXOStateChanged）
// - 内存池事件（来自mempool的通知）
//
// 设计原则：
// - 专注交易：只处理与交易生命周期相关的事件
// - 状态追踪：维护交易状态变化的完整记录
// - 性能优化：批量处理和异步更新
package event_handler

import (
	"time"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	eventIf "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// TransactionEventHandler transaction模块事件处理器
//
// 🔧 **交易生命周期事件处理**
//
// 核心职责：
// - 跟踪交易从接收到确认的完整生命周期
// - 响应内存池的交易状态变化通知
// - 处理UTXO状态变化对交易的影响
// - 维护交易处理的性能指标和错误统计
//
// 事件流程：
// TransactionReceived → TransactionValidated → TransactionExecuted → TransactionConfirmed
//
//	↘                      ↘
//	  TransactionFailed     TransactionFailed
type TransactionEventHandler struct {
	logger   log.Logger
	eventBus eventIf.EventBus

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

	// UTXO状态跟踪
	utxoCreated uint64 // 创建的UTXO数量
	utxoSpent   uint64 // 花费的UTXO数量
}

// NewTransactionEventHandler 创建transaction事件处理器
func NewTransactionEventHandler(logger log.Logger, eventBus eventIf.EventBus) *TransactionEventHandler {
	return &TransactionEventHandler{
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
// 3. 触发交易验证流程
// 4. 发布交易接收确认事件
func (h *TransactionEventHandler) HandleTransactionReceived(eventData *types.TransactionReceivedEventData) error {
	h.receivedCount++
	h.lastProcessTime = time.Now()

	if h.logger != nil {
		h.logger.Infof("[TxHandler] 📨 接收交易: %s, 发送者: %s, 金额: %d, 手续费: %d",
			eventData.Hash, eventData.From, eventData.Value, eventData.Fee)
	}

	// 发布交易接收确认事件
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

	return nil
}

// HandleTransactionValidated 处理交易验证事件
//
// ✅ **交易验证结果处理**
//
// 验证处理：
// 1. 检查验证结果，更新相应统计
// 2. 计算和更新平均验证时间
// 3. 对验证失败的交易记录错误原因
// 4. 为验证通过的交易触发执行流程
func (h *TransactionEventHandler) HandleTransactionValidated(eventData *types.TransactionValidatedEventData) error {
	if eventData.Valid {
		h.validatedCount++
		if h.logger != nil {
			h.logger.Infof("[TxHandler] ✅ 交易验证通过: %s", eventData.Hash)
		}

		// 发布验证通过事件
		validData := map[string]interface{}{
			"tx_hash":      eventData.Hash,
			"validated_at": eventData.Timestamp,
			"status":       "validated",
		}

		h.eventBus.Publish("transaction.status.validated", validData)

	} else {
		h.failedCount++
		if h.logger != nil {
			h.logger.Warnf("[TxHandler] ❌ 交易验证失败: %s, 错误: %v", eventData.Hash, eventData.Errors)
		}

		// 发布验证失败事件
		failData := map[string]interface{}{
			"tx_hash":   eventData.Hash,
			"failed_at": eventData.Timestamp,
			"status":    "validation_failed",
			"errors":    eventData.Errors,
		}

		h.eventBus.Publish("transaction.status.failed", failData)
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
// 3. 对执行成功的交易更新状态
// 4. 对执行失败的交易记录详细错误信息
func (h *TransactionEventHandler) HandleTransactionExecuted(eventData *types.TransactionExecutedEventData) error {
	if eventData.Success {
		h.executedCount++
		if h.logger != nil {
			h.logger.Infof("[TxHandler] ⚙️ 交易执行成功: %s, 执行费用: %d, 结果: %s",
				eventData.Hash, eventData.ExecutionFeeUsed, eventData.Result)
		}

		// 发布执行成功事件
		successData := map[string]interface{}{
			"tx_hash":      eventData.Hash,
			"block_height": eventData.BlockHeight,
			"执行费用_used":     eventData.ExecutionFeeUsed,
			"result":       eventData.Result,
			"executed_at":  eventData.Timestamp,
			"status":       "executed",
		}

		h.eventBus.Publish("transaction.status.executed", successData)

	} else {
		h.failedCount++
		if h.logger != nil {
			h.logger.Warnf("[TxHandler] ❌ 交易执行失败: %s, 执行费用: %d",
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
// 3. 发布失败通知给相关组件
// 4. 执行失败交易的清理操作
func (h *TransactionEventHandler) HandleTransactionFailed(eventData *types.TransactionFailedEventData) error {
	h.failedCount++

	if h.logger != nil {
		h.logger.Errorf("[TxHandler] 💥 交易处理失败: %s, 区块: %d, 错误: %s, 执行费用消耗: %d",
			eventData.Hash, eventData.BlockHeight, eventData.Error, eventData.ExecutionFeeUsed)
	}

	// 发布详细的失败事件
	failureData := map[string]interface{}{
		"tx_hash":      eventData.Hash,
		"block_height": eventData.BlockHeight,
		"error":        eventData.Error,
		"执行费用_used":     eventData.ExecutionFeeUsed,
		"failed_at":    eventData.Timestamp,
		"status":       "failed",
	}

	h.eventBus.Publish("transaction.status.failed", failureData)

	// 通知mempool移除失败交易
	removeData := map[string]interface{}{
		"tx_hash": eventData.Hash,
		"reason":  "execution_failed",
	}

	h.eventBus.Publish(eventconstants.EventTypeTxRemoved, removeData)

	return nil
}

// HandleTransactionConfirmed 处理交易确认事件
//
// 🎯 **交易最终确认处理**
//
// 确认处理：
// 1. 记录交易最终确认状态
// 2. 更新确认统计计数
// 3. 发布确认通知给用户层
// 4. 触发相关的后续处理流程
func (h *TransactionEventHandler) HandleTransactionConfirmed(eventData *types.TransactionConfirmedEventData) error {
	h.confirmedCount++

	if h.logger != nil {
		finalStatus := ""
		if eventData.Final {
			finalStatus = " (最终确认)"
		}

		h.logger.Infof("[TxHandler] 🎯 交易确认: %s, 区块: %d, 确认数: %d%s",
			eventData.Hash, eventData.BlockHeight, eventData.Confirmations, finalStatus)
	}

	// 发布确认事件
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

	return nil
}

// HandleUTXOStateChanged 处理UTXO状态变化事件
//
// 💰 **UTXO状态变化处理**
//
// UTXO处理：
// 1. 跟踪UTXO的创建和消费
// 2. 维护UTXO状态统计
// 3. 发布UTXO变化通知
// 4. 更新相关交易的UTXO引用
func (h *TransactionEventHandler) HandleUTXOStateChanged(eventData *types.UTXOStateChangedEventData) error {
	switch eventData.Operation {
	case "created":
		h.utxoCreated++
		if h.logger != nil {
			h.logger.Infof("[TxHandler] 💰 UTXO创建: %s, 交易: %s, 区块: %d",
				eventData.UTXOHash, eventData.TxHash, eventData.BlockHeight)
		}

	case "spent":
		h.utxoSpent++
		if h.logger != nil {
			h.logger.Infof("[TxHandler] 💸 UTXO消费: %s, 交易: %s, 区块: %d",
				eventData.UTXOHash, eventData.TxHash, eventData.BlockHeight)
		}

	case "locked":
		if h.logger != nil {
			h.logger.Infof("[TxHandler] 🔒 UTXO锁定: %s, 交易: %s",
				eventData.UTXOHash, eventData.TxHash)
		}

	case "unlocked":
		if h.logger != nil {
			h.logger.Infof("[TxHandler] 🔓 UTXO解锁: %s, 交易: %s",
				eventData.UTXOHash, eventData.TxHash)
		}
	}

	// 发布UTXO状态变化事件
	utxoData := map[string]interface{}{
		"utxo_hash":    eventData.UTXOHash,
		"operation":    eventData.Operation,
		"tx_hash":      eventData.TxHash,
		"block_height": eventData.BlockHeight,
		"changed_at":   eventData.Timestamp,
	}

	h.eventBus.Publish("utxo.state.changed", utxoData)

	return nil
}

// HandleMempoolTransactionRemoved 处理内存池交易移除事件
//
// 🗑️ **内存池交易移除处理**
//
// 移除处理：
// 1. 响应内存池的交易移除通知
// 2. 根据移除原因执行相应处理
// 3. 更新本地交易状态跟踪
// 4. 清理相关的临时数据
func (h *TransactionEventHandler) HandleMempoolTransactionRemoved(eventData *types.TransactionRemovedEventData) error {
	if h.logger != nil {
		h.logger.Infof("[TxHandler] 🗑️ 内存池移除交易: %s, 原因: %s, 池: %s",
			eventData.Hash, eventData.Reason, eventData.Pool)
	}

	// 根据移除原因采取不同行动
	switch eventData.Reason {
	case "expired":
		// 交易过期，无需特殊处理

	case "included":
		// 交易已被打包，这是正常流程
		if h.logger != nil {
			h.logger.Infof("[TxHandler] ✅ 交易已被打包: %s", eventData.Hash)
		}

	case "invalid":
		// 交易无效，记录错误
		if h.logger != nil {
			h.logger.Warnf("[TxHandler] ❌ 交易被标记为无效: %s", eventData.Hash)
		}

	case "replaced":
		// 交易被替换，记录信息
		if h.logger != nil {
			h.logger.Infof("[TxHandler] 🔄 交易被替换: %s", eventData.Hash)
		}
	}

	return nil
}

// GetTransactionStats 获取交易处理统计信息
func (h *TransactionEventHandler) GetTransactionStats() map[string]interface{} {
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
		"utxo_created":      h.utxoCreated,
		"utxo_spent":        h.utxoSpent,
		"last_process_time": h.lastProcessTime,
	}
}
