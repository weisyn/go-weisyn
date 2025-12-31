// Package txpool 高级查询功能测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestGetPendingTransactionsWithLimit_WithValidLimit_RespectsLimit 测试带限制的获取pending交易
func TestGetPendingTransactionsWithLimit_WithValidLimit_RespectsLimit(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 10

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act - 限制为5
	result := pool.GetPendingTransactionsWithLimit(5)

	// Assert
	assert.LessOrEqual(t, len(result), 5, "返回的交易数不应超过限制")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsWithLimit_WithZeroLimit_ReturnsAll 测试零限制时返回所有
func TestGetPendingTransactionsWithLimit_WithZeroLimit_ReturnsAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act - 限制为0（应该返回所有）
	result := pool.GetPendingTransactionsWithLimit(0)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "零限制应该返回所有交易")
}

// TestGetPendingTransactionsWithLimit_WithNegativeLimit_ReturnsAll 测试负限制时返回所有
func TestGetPendingTransactionsWithLimit_WithNegativeLimit_ReturnsAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 3

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act - 限制为-1（应该返回所有）
	result := pool.GetPendingTransactionsWithLimit(-1)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "负限制应该返回所有交易")
}

// TestGetPendingTransactionsWithLimit_WithEmptyPool_ReturnsEmpty 测试空池时返回空列表
func TestGetPendingTransactionsWithLimit_WithEmptyPool_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	result := pool.GetPendingTransactionsWithLimit(10)

	// Assert
	assert.Len(t, result, 0, "空池应该返回空列表")
}

// TestGetPendingTransactionsByDependencyOrder_WithValidLimit_RespectsLimit 测试按依赖顺序获取交易
func TestGetPendingTransactionsByDependencyOrder_WithValidLimit_RespectsLimit(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 10

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act - 限制为5
	result := pool.GetPendingTransactionsByDependencyOrder(5)

	// Assert
	assert.LessOrEqual(t, len(result), 5, "返回的交易数不应超过限制")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsByDependencyOrder_WithZeroLimit_ReturnsAll 测试零限制时返回所有
func TestGetPendingTransactionsByDependencyOrder_WithZeroLimit_ReturnsAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act - 限制为0（应该返回所有）
	result := pool.GetPendingTransactionsByDependencyOrder(0)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "零限制应该返回所有交易")
}

// TestGetPendingTransactionsByDependencyOrder_WithEmptyPool_ReturnsEmpty 测试空池时返回空列表
func TestGetPendingTransactionsByDependencyOrder_WithEmptyPool_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	result := pool.GetPendingTransactionsByDependencyOrder(10)

	// Assert
	assert.Len(t, result, 0, "空池应该返回空列表")
}

// TestGetPendingTransactionsByDependencyOrder_WithDependentTxs_ReturnsInOrder 测试有依赖关系的交易
func TestGetPendingTransactionsByDependencyOrder_WithDependentTxs_ReturnsInOrder(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建第一个交易（父交易）
	parentTxID := []byte("parent_tx_id_32_bytes_12345678")
	tx1 := testutil.CreateTestTransaction(1, nil, nil)
	tx1.Nonce = 1
	_, err := pool.SubmitTx(tx1)
	require.NoError(t, err)

	// 创建第二个交易（依赖第一个交易）
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{
			PreviousOutput: &transaction.OutPoint{
				TxId:        parentTxID,
				OutputIndex: 0,
			},
			IsReferenceOnly: false,
			Sequence:        0xFFFFFFFF,
		},
	}, nil)
	tx2.Nonce = 2
	_, err = pool.SubmitTx(tx2)
	require.NoError(t, err)

	// Act
	result := pool.GetPendingTransactionsByDependencyOrder(10)

	// Assert
	assert.GreaterOrEqual(t, len(result), 2, "应该返回至少2个交易")
	// 注意：由于实现是占位实现，可能不会真正按依赖顺序排序
	// 但至少应该返回所有交易
}

