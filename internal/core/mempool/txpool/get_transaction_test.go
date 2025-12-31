// Package txpool GetTransaction方法测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetTransaction_WithExistingTx_ReturnsTransaction 测试获取存在的交易
func TestGetTransaction_WithExistingTx_ReturnsTransaction(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	retrievedTx, err := pool.GetTransaction(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取交易")
	assert.NotNil(t, retrievedTx, "交易不应为nil")
	assert.Equal(t, tx.Nonce, retrievedTx.Nonce, "交易Nonce应该相同")
}

// TestGetTransaction_WithNonExistentTx_ReturnsError 测试获取不存在的交易
func TestGetTransaction_WithNonExistentTx_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	tx, err := pool.GetTransaction(nonExistentTxID)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应为nil")
	assert.Contains(t, err.Error(), "交易不存在", "错误信息应该包含'交易不存在'")
}

// TestGetTransaction_WithNilTxID_ReturnsError 测试nil交易ID
func TestGetTransaction_WithNilTxID_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	tx, err := pool.GetTransaction(nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应为nil")
}

// TestGetTransaction_WithEmptyTxID_ReturnsError 测试空交易ID
func TestGetTransaction_WithEmptyTxID_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	tx, err := pool.GetTransaction([]byte{})

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应为nil")
}

// TestGetTransaction_AfterRemoval_ReturnsError 测试移除后获取交易
func TestGetTransaction_AfterRemoval_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 移除交易
	err = pool.RemoveTransaction(txID)
	require.NoError(t, err)

	// Act
	retrievedTx, err := pool.GetTransaction(txID)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, retrievedTx, "交易应为nil")
}

// TestGetTx_IsAliasOfGetTransaction 测试GetTx是GetTransaction的别名
func TestGetTx_IsAliasOfGetTransaction(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	tx1, err1 := pool.GetTransaction(txID)
	tx2, err2 := pool.GetTx(txID)

	// Assert
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, tx1, tx2, "GetTx应该返回与GetTransaction相同的结果")
}

