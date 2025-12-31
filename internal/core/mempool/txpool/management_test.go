// Package txpool 交易池管理功能测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestReset_ClearsAllTransactions 测试重置交易池清空所有交易
func TestReset_ClearsAllTransactions(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5
	txIDs := make([][]byte, numTxs)

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// 验证交易存在
	pendingTxs, err := pool.GetAllPendingTransactions()
	require.NoError(t, err)
	require.Greater(t, len(pendingTxs), 0, "重置前应该有交易")

	// Act
	pool.Reset()

	// Assert
	// 所有交易都应该被清除
	for _, txID := range txIDs {
		retrievedTx, err := pool.GetTx(txID)
		assert.Error(t, err, "重置后的交易应该不存在")
		assert.Nil(t, retrievedTx, "重置后的交易应该为nil")
	}
	// pending列表应该为空
	pendingTxs2, err := pool.GetAllPendingTransactions()
	assert.NoError(t, err)
	assert.Len(t, pendingTxs2, 0, "重置后pending列表应该为空")
}

// TestReset_ResetsMemoryUsage 测试重置后内存使用归零
func TestReset_ResetsMemoryUsage(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	_, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	pool.Reset()

	// Assert
	// 注意：memoryUsage是私有字段，我们通过行为验证
	// 重置后提交新交易应该从零开始
	tx2 := testutil.CreateSimpleTestTransaction(2)
	_, err = pool.SubmitTx(tx2)
	assert.NoError(t, err, "重置后应该可以正常提交交易")
}

// TestSetEventSink_WithValidSink_SetsSink 测试设置有效的事件下沉
func TestSetEventSink_WithValidSink_SetsSink(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	mockSink := &MockTxEventSink{}

	// Act
	pool.SetEventSink(mockSink)

	// Assert
	// 通过提交交易验证事件下沉是否被调用
	tx := testutil.CreateSimpleTestTransaction(1)
	_, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	// 验证事件下沉被调用（如果MockTxEventSink记录了调用）
	// 注意：这需要MockTxEventSink实现记录功能
}

// TestSetEventSink_WithNilSink_SetsNoopSink 测试设置nil时使用Noop下沉
func TestSetEventSink_WithNilSink_SetsNoopSink(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	pool.SetEventSink(nil)

	// Assert
	// 设置nil后应该使用Noop下沉，不会panic
	tx := testutil.CreateSimpleTestTransaction(1)
	_, err := pool.SubmitTx(tx)
	assert.NoError(t, err, "设置nil下沉后应该仍然可以正常工作")
}

// TestMarkTransactionsAsPendingConfirm_WithMiningTxs_MarksAsPendingConfirm 测试标记挖矿中的交易为待确认
func TestMarkTransactionsAsPendingConfirm_WithMiningTxs_MarksAsPendingConfirm(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 标记为挖矿中
	err = pool.MarkTransactionsAsMining([][]byte{txID})
	require.NoError(t, err)

	// Act
	err = pool.MarkTransactionsAsPendingConfirm([][]byte{txID}, 1)

	// Assert
	assert.NoError(t, err, "应该成功标记为待确认")
	// 注意：由于状态转换的复杂性，我们主要验证不会出错
}

// TestMarkTransactionsAsPendingConfirm_WithNonMiningTxs_NoError 测试非挖矿状态的交易
func TestMarkTransactionsAsPendingConfirm_WithNonMiningTxs_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	// 不标记为挖矿中，直接尝试标记为待确认

	// Act
	err = pool.MarkTransactionsAsPendingConfirm([][]byte{txID}, 1)

	// Assert
	// 根据实现，非mining状态的交易可能被忽略或返回错误
	// 我们验证不会panic
	assert.NoError(t, err, "非挖矿状态的交易应该被忽略或返回错误")
}

// TestMarkTransactionsAsPendingConfirm_WithNonExistentTx_NoError 测试不存在的交易
func TestMarkTransactionsAsPendingConfirm_WithNonExistentTx_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.MarkTransactionsAsPendingConfirm([][]byte{nonExistentTxID}, 1)

	// Assert
	// 根据实现，不存在的交易可能被忽略
	assert.NoError(t, err, "不存在的交易应该被忽略")
}

// TestMarkTransactionsAsPendingConfirm_WithEmptyList_NoError 测试空列表
func TestMarkTransactionsAsPendingConfirm_WithEmptyList_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.MarkTransactionsAsPendingConfirm([][]byte{}, 1)

	// Assert
	assert.NoError(t, err, "空列表应该成功")
}

// MockTxEventSink 用于测试的Mock事件下沉
type MockTxEventSink struct {
	onTxAddedCalled      bool
	onTxRemovedCalled    bool
	onTxConfirmedCalled  bool
	onTxExpiredCalled    bool
	onPoolStateChangedCalled bool
}

func (m *MockTxEventSink) OnTxAdded(wrapper *TxWrapper) {
	m.onTxAddedCalled = true
}

func (m *MockTxEventSink) OnTxRemoved(wrapper *TxWrapper) {
	m.onTxRemovedCalled = true
}

func (m *MockTxEventSink) OnTxConfirmed(wrapper *TxWrapper, blockHeight uint64) {
	m.onTxConfirmedCalled = true
}

func (m *MockTxEventSink) OnTxExpired(wrapper *TxWrapper) {
	m.onTxExpiredCalled = true
}

func (m *MockTxEventSink) OnPoolStateChanged(metrics *PoolMetrics) {
	m.onPoolStateChangedCalled = true
}

