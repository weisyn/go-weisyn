// Package txpool 其他方法测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetTransactionByID_WithExistingTx_ReturnsTransaction 测试GetTransactionByID
func TestGetTransactionByID_WithExistingTx_ReturnsTransaction(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	retrievedTx, err := pool.GetTransactionByID(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取交易")
	assert.NotNil(t, retrievedTx, "交易不应为nil")
	assert.Equal(t, tx.Nonce, retrievedTx.Nonce, "交易Nonce应该相同")
}

// TestGetTransactionByID_WithNonExistentTx_ReturnsNil 测试不存在的交易
func TestGetTransactionByID_WithNonExistentTx_ReturnsNil(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	tx, err := pool.GetTransactionByID(nonExistentTxID)

	// Assert
	// 注意：根据实现，GetTransactionByID在交易不存在时返回nil, nil而不是错误
	assert.NoError(t, err, "应该不返回错误")
	assert.Nil(t, tx, "交易应为nil")
}

// TestGetPendingTransactions_ReturnsAllPending 测试GetPendingTransactions
func TestGetPendingTransactions_ReturnsAllPending(t *testing.T) {
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

	// Act
	pendingTxs, err := pool.GetPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功获取pending交易")
	assert.GreaterOrEqual(t, len(pendingTxs), numTxs, "应该返回至少numTxs个交易")
}

// TestGetPendingTransactions_WithNoPending_ReturnsEmpty 测试没有pending交易时返回空
func TestGetPendingTransactions_WithNoPending_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	pendingTxs, err := pool.GetPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功返回空列表")
	assert.Len(t, pendingTxs, 0, "应该返回空列表")
}

