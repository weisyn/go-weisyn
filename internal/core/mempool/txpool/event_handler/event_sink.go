// Package event_handler 交易池事件发布下沉
//
// 本文件实现交易池的事件发布下沉（Event Sink），负责将 TxPool 的内部事件
// 转换为标准化的事件总线消息并发布。
//
// 职责：
// - 实现 TxEventSink 接口
// - 将本地事件转换为全局事件常量并发布到事件总线
// - 确保事件发布的类型安全和标准化
package event_handler

import (
	"encoding/hex"

	"github.com/weisyn/v1/internal/core/mempool/txpool"
	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// txSink 是 TxPool 的事件下沉实现。
// 作用：将交易相关本地事件转换为标准化的事件总线消息。
// 线程安全：事件总线接口自身应保证并发安全；本实现不持有可变共享状态。
type txSink struct {
	eventBus event.EventBus
	logger   log.Logger
}

// OnTxAdded 交易添加事件回调。
// 参数：
// - tx：交易包装器。
// 返回：无。
func (s *txSink) OnTxAdded(tx *txpool.TxWrapper) {
	if s.eventBus != nil {
		// 🔧 修复：将 TxWrapper 转换为 TransactionReceivedEventData
		// 避免类型不匹配导致的 panic
		
		// 从交易中提取基本信息
		var from, to string
		var value uint64
		
	if tx.Tx != nil {
		// 提取发送方地址（从第一个输入）
		if len(tx.Tx.Inputs) > 0 && tx.Tx.Inputs[0].PreviousOutput != nil {
			txId := tx.Tx.Inputs[0].PreviousOutput.TxId
			if len(txId) >= 8 {
				from = hex.EncodeToString(txId[:8])
			} else if len(txId) > 0 {
				from = hex.EncodeToString(txId)
			}
		}
		
		// 提取接收方地址和金额（从第一个输出）
		if len(tx.Tx.Outputs) > 0 && tx.Tx.Outputs[0] != nil {
			owner := tx.Tx.Outputs[0].Owner
			if len(owner) >= 8 {
				to = hex.EncodeToString(owner[:8])
			} else if len(owner) > 0 {
				to = hex.EncodeToString(owner)
			}
			
			// 尝试提取金额（如果是资产输出）
			if assetOutput := tx.Tx.Outputs[0].GetAsset(); assetOutput != nil {
				if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
					// 简化处理：无法直接解析 string 为 uint64，使用0
					value = 0
				}
			}
		}
	}
		
		eventData := &types.TransactionReceivedEventData{
			Hash:      hex.EncodeToString(tx.TxID),
			From:      from,
			To:        to,
			Value:     value,
			Fee:       0, // 手续费需要复杂计算，暂时使用0
			Timestamp: tx.ReceivedAt.Unix(),
		}
		s.eventBus.Publish(eventconstants.EventTypeTxAdded, eventData)
	}
}

// OnTxRemoved 交易移除事件回调。
// 参数：
// - tx：交易包装器。
// 返回：无。
func (s *txSink) OnTxRemoved(tx *txpool.TxWrapper) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeTxRemoved, tx)
	}
}

// OnTxConfirmed 交易确认事件回调。
// 参数：
// - tx：交易包装器；
// - h：确认区块高度。
// 返回：无。
func (s *txSink) OnTxConfirmed(tx *txpool.TxWrapper, h uint64) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.Publish(eventconstants.EventTypeTxConfirmed, &struct {
		Tx          *txpool.TxWrapper
		BlockHeight uint64
	}{Tx: tx, BlockHeight: h})
}

// OnTxExpired 交易过期事件回调。
// 参数：
// - tx：交易包装器。
// 返回：无。
func (s *txSink) OnTxExpired(tx *txpool.TxWrapper) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeTxExpired, tx)
	}
}

// OnPoolStateChanged 交易池状态变化事件回调。
// 参数：
// - metrics：交易池监控指标。
// 返回：无。
// 注意：使用 EventTypeMempoolSizeChanged 发布交易池状态变化事件。
// 如需更细粒度的事件类型，可在 pkg/constants/events/system_events.go 中新增常量。
func (s *txSink) OnPoolStateChanged(metrics *txpool.PoolMetrics) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeMempoolSizeChanged, metrics)
	}
}

// SetupTxPoolEventSink 设置交易池事件发布下沉。
// 将事件发布实现注入到 TxPool 中，使它们能够发布事件到事件总线。
//
// 参数：
// - eventBus：事件总线接口（可选，nil 时事件发布将被禁用）
// - logger：日志接口（可选）
// - extendedTxPool：扩展的交易池接口
//
// 说明：
// - 如果 eventBus 为 nil，事件发布将被禁用（池会使用 Noop 实现）
// - 使用类型断言确保类型安全
func SetupTxPoolEventSink(
	eventBus event.EventBus,
	logger log.Logger,
	extendedTxPool txpool.ExtendedTxPool,
) {
	// 注入 TxPool 事件下沉
	if pool, ok := extendedTxPool.(*txpool.TxPool); ok {
		pool.SetEventSink(&txSink{eventBus: eventBus, logger: logger})
		if logger != nil {
			logger.Debug("✅ TxPool 事件发布下沉已配置")
		}
	}
}

// 编译期检查：确保 txSink 实现了 TxEventSink 接口
var _ txpool.TxEventSink = (*txSink)(nil)

