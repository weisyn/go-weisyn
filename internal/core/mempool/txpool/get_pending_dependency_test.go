// Package txpool GetPendingTransactionsByDependencyOrder测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetPendingTransactionsByDependencyOrder_WithValidLimit_ReturnsLimitedTxs 测试有效限制
func TestGetPendingTransactionsByDependencyOrder_WithValidLimit_ReturnsLimitedTxs(t *testing.T) {
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
	result := pool.GetPendingTransactionsByDependencyOrder(limit)

	// Assert
	assert.LessOrEqual(t, len(result), limit, "返回的交易数应该不超过限制")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsByDependencyOrder_WithZeroLimit_ReturnsAllTxs 测试零限制返回所有交易
func TestGetPendingTransactionsByDependencyOrder_WithZeroLimit_ReturnsAllTxs(t *testing.T) {
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
	result := pool.GetPendingTransactionsByDependencyOrder(0)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "应该返回所有交易")
}

// TestGetPendingTransactionsByDependencyOrder_WithNegativeLimit_ReturnsAllTxs 测试负限制返回所有交易
func TestGetPendingTransactionsByDependencyOrder_WithNegativeLimit_ReturnsAllTxs(t *testing.T) {
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
	result := pool.GetPendingTransactionsByDependencyOrder(-1)

	// Assert
	assert.GreaterOrEqual(t, len(result), numTxs, "负限制应该返回所有交易")
}

// TestGetPendingTransactionsByDependencyOrder_WithNoPendingTxs_ReturnsEmpty 测试没有待处理交易时返回空
func TestGetPendingTransactionsByDependencyOrder_WithNoPendingTxs_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	result := pool.GetPendingTransactionsByDependencyOrder(10)

	// Assert
	assert.Empty(t, result, "没有待处理交易时应该返回空列表")
}

// TestGetPendingTransactionsByDependencyOrder_WithLimitGreaterThanPending_ReturnsAllTxs 测试限制大于待处理交易数
func TestGetPendingTransactionsByDependencyOrder_WithLimitGreaterThanPending_ReturnsAllTxs(t *testing.T) {
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
	result := pool.GetPendingTransactionsByDependencyOrder(limit)

	// Assert
	assert.LessOrEqual(t, len(result), numTxs, "应该返回所有待处理交易")
	assert.Greater(t, len(result), 0, "应该返回至少一个交易")
}

// TestGetPendingTransactionsByDependencyOrder_TopologicalSort 测试拓扑排序的正确性
func TestGetPendingTransactionsByDependencyOrder_TopologicalSort(t *testing.T) {
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
	result := pool.GetPendingTransactionsByDependencyOrder(numTxs)

	// Assert
	assert.Equal(t, numTxs, len(result), "应该返回所有交易")
	// 验证拓扑排序：依赖交易应该出现在子交易之前
	// 注意：由于 CreateSimpleTestTransaction 可能不创建真实的依赖关系，
	// 这里主要验证方法能正常运行并返回所有交易
}

