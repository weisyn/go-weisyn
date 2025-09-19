// 文件说明：
// 本文件定义交易池（TxPool）的事件下沉接口（TxEventSink）及默认空实现。
// 职责：将 TxPool 内部产生的事件（添加、移除、确认）与事件总线解耦，
// 由 integration/event/outgoing 层注入具体发布逻辑，实现统一对外事件标准化。
package txpool

// TxEventSink 定义交易池对外的事件下沉接口。
// 说明：
// - 由集成层实现并注入到 TxPool；
// - 若为 Noop 实现，则表示不对外发布事件。
// 方法参数：
// - OnTxAdded：tx 为被添加的交易包装器；
// - OnTxRemoved：tx 为被移除的交易包装器；
// - OnTxConfirmed：tx 为被确认的交易包装器，h 为确认区块高度。
type TxEventSink interface {
	OnTxAdded(tx *TxWrapper)
	OnTxRemoved(tx *TxWrapper)
	OnTxConfirmed(tx *TxWrapper, blockHeight uint64)
}

// NoopTxEventSink 为默认空实现，不执行任何操作。
// 适用于未开启事件对外发布的场景。
type NoopTxEventSink struct{}

// OnTxAdded 空实现。
// 参数：tx 交易包装器。
// 返回：无。
func (NoopTxEventSink) OnTxAdded(tx *TxWrapper) {}

// OnTxRemoved 空实现。
// 参数：tx 交易包装器。
// 返回：无。
func (NoopTxEventSink) OnTxRemoved(tx *TxWrapper) {}

// OnTxConfirmed 空实现。
// 参数：
// - tx：交易包装器；
// - blockHeight：确认区块高度。
// 返回：无。
func (NoopTxEventSink) OnTxConfirmed(tx *TxWrapper, blockHeight uint64) {}
