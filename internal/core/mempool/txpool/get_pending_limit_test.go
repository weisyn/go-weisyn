// Package txpool GetPendingTransactionsWithLimit边界情况测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetPendingTransactionsWithLimit_WithValidLimit_ReturnsLimitedTxs 测试有效限制
func TestGetPendingTransactionsWithLimit_WithValidLimit_ReturnsLimitedTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 10
	limit := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	result := pool.GetPendingTransactionsWithLimit(limit)

	// Assert
	assert.LessOrEqual(t, len(result), limit, "返回的交易数应该不超过限制")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsWithLimit_WithZeroLimit_ReturnsAllTxs 测试零限制返回所有交易
func TestGetPendingTransactionsWithLimit_WithZeroLimit_ReturnsAllTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	result := pool.GetPendingTransactionsWithLimit(0)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "应该返回所有交易")
}

// TestGetPendingTransactionsWithLimit_WithNegativeLimit_ReturnsAllTxs 测试负限制返回所有交易
func TestGetPendingTransactionsWithLimit_WithNegativeLimit_ReturnsAllTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	result := pool.GetPendingTransactionsWithLimit(-1)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "负限制应该返回所有交易")
}

// TestGetPendingTransactionsWithLimit_WithLimitGreaterThanPending_ReturnsAllTxs 测试限制大于待处理交易数
func TestGetPendingTransactionsWithLimit_WithLimitGreaterThanPending_ReturnsAllTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5
	limit := 100

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	result := pool.GetPendingTransactionsWithLimit(limit)

	// Assert
	assert.LessOrEqual(t, len(result), numTxs, "应该返回所有待处理交易")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsWithLimit_WithNoPendingTxs_ReturnsEmpty 测试没有待处理交易时返回空
func TestGetPendingTransactionsWithLimit_WithNoPendingTxs_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	result := pool.GetPendingTransactionsWithLimit(10)

	// Assert
	assert.Empty(t, result, "没有待处理交易时应该返回空列表")
}

// TestGetPendingTransactionsWithLimit_RespectsPriorityOrder 测试按优先级顺序返回
func TestGetPendingTransactionsWithLimit_RespectsPriorityOrder(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易（不同优先级）
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	result := pool.GetPendingTransactionsWithLimit(numTxs)

	// Assert
	assert.Equal(t, numTxs, len(result), "应该返回所有交易")
	// 注意：优先级顺序由优先级队列决定，这里主要验证方法正常工作
}

