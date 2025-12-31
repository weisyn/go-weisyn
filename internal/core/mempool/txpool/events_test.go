package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockSink struct {
	added          []*TxWrapper
	removed        []*TxWrapper
	confirmed      []struct {
		tx *TxWrapper
		h  uint64
	}
	expired        []*TxWrapper       // P2-6新增
	poolStateChanged []*PoolMetrics   // P2-6新增
}

func (m *mockSink) OnTxAdded(tx *TxWrapper)   { m.added = append(m.added, tx) }
func (m *mockSink) OnTxRemoved(tx *TxWrapper) { m.removed = append(m.removed, tx) }
func (m *mockSink) OnTxConfirmed(tx *TxWrapper, h uint64) {
	m.confirmed = append(m.confirmed, struct {
		tx *TxWrapper
		h  uint64
	}{tx, h})
}

// OnTxExpired 实现交易过期事件回调（P2-6新增）。
func (m *mockSink) OnTxExpired(tx *TxWrapper) {
	m.expired = append(m.expired, tx)
}

// OnPoolStateChanged 实现交易池状态变化事件回调（P2-6新增）。
func (m *mockSink) OnPoolStateChanged(metrics *PoolMetrics) {
	m.poolStateChanged = append(m.poolStateChanged, metrics)
}

func TestTxEventSink_IntegratesWithTxPool(t *testing.T) {
	// 构造最小 TxWrapper
	tx := &TxWrapper{TxID: []byte("tx-1"), ReceivedAt: time.Now(), Status: TxStatusPending}

	// 事件下沉桩
	sink := &mockSink{}

	// 构造 TxPool（仅注入必要字段）
	pool := &TxPool{eventSink: sink, pendingTxs: map[string]struct{}{}, txs: map[string]*TxWrapper{}, pendingQueue: NewPriorityQueue()}

	// 触发添加
	pool.eventSink.OnTxAdded(tx)
	assert.Len(t, sink.added, 1)
	assert.Equal(t, tx, sink.added[0])

	// 触发移除
	pool.eventSink.OnTxRemoved(tx)
	assert.Len(t, sink.removed, 1)
	assert.Equal(t, tx, sink.removed[0])

	// 触发确认
	pool.eventSink.OnTxConfirmed(tx, 100)
	assert.Len(t, sink.confirmed, 1)
	assert.Equal(t, tx, sink.confirmed[0].tx)
	assert.Equal(t, uint64(100), sink.confirmed[0].h)
}
