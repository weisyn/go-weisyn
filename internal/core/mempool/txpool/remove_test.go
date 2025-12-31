// Package txpool 移除交易测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestRemoveTransaction_WithExistingTransaction_RemovesFromPool 测试移除存在的交易
func TestRemoveTransaction_WithExistingTransaction_RemovesFromPool(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 验证交易存在
	retrievedTx, err := pool.GetTx(txID)
	require.NoError(t, err)
	require.NotNil(t, retrievedTx)

	// Act
	err = pool.RemoveTransaction(txID)

	// Assert
	assert.NoError(t, err, "应该成功移除交易")
	// 移除后交易应该不存在
	retrievedTx2, err := pool.GetTx(txID)
	assert.Error(t, err, "移除后的交易应该不存在")
	assert.Nil(t, retrievedTx2, "移除后的交易应该为nil")
}

// TestRemoveTransaction_WithNonExistentTransaction_ReturnsError 测试移除不存在的交易
func TestRemoveTransaction_WithNonExistentTransaction_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.RemoveTransaction(nonExistentTxID)

	// Assert
	assert.Error(t, err, "移除不存在的交易应该返回错误")
}

// TestRemoveTransaction_WithNilTxID_ReturnsError 测试nil交易ID时返回错误
func TestRemoveTransaction_WithNilTxID_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.RemoveTransaction(nil)

	// Assert
	assert.Error(t, err, "nil交易ID应该返回错误")
}

// TestBatchRemoveTransactions_WithMultipleTxs_RemovesAll 测试批量移除交易
func TestBatchRemoveTransactions_WithMultipleTxs_RemovesAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 3
	txIDs := make([][]byte, numTxs)

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// Act
	errors := pool.BatchRemoveTransactions(txIDs)

	// Assert
	// 所有移除操作应该成功
	for i, err := range errors {
		assert.NoError(t, err, "第%d个交易应该成功移除", i)
	}
	// 所有交易都应该被移除
	for _, txID := range txIDs {
		retrievedTx, err := pool.GetTx(txID)
		assert.Error(t, err, "移除后的交易应该不存在")
		assert.Nil(t, retrievedTx, "移除后的交易应该为nil")
	}
}

// TestBatchRemoveTransactions_WithEmptyList_ReturnsEmpty 测试空列表时返回空结果
func TestBatchRemoveTransactions_WithEmptyList_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	errors := pool.BatchRemoveTransactions([][]byte{})

	// Assert
	assert.Len(t, errors, 0, "空列表应该返回空错误列表")
}

// TestRemoveTxs_WithMultipleTxs_RemovesAll 测试RemoveTxs批量移除
func TestRemoveTxs_WithMultipleTxs_RemovesAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 3
	txIDs := make([][]byte, numTxs)

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// Act
	err := pool.RemoveTxs(txIDs)

	// Assert
	assert.NoError(t, err, "应该成功批量移除交易")
	// 所有交易都应该被移除
	for _, txID := range txIDs {
		retrievedTx, err := pool.GetTx(txID)
		assert.Error(t, err, "移除后的交易应该不存在")
		assert.Nil(t, retrievedTx, "移除后的交易应该为nil")
	}
}

// TestRemoveTxs_WithNonExistentTx_ReturnsError 测试移除不存在的交易
func TestRemoveTxs_WithNonExistentTx_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.RemoveTxs([][]byte{nonExistentTxID})

	// Assert
	assert.Error(t, err, "移除不存在的交易应该返回错误")
}

// TestRemoveTransaction_AfterConfirm_ReturnsError 测试确认后移除返回错误
func TestRemoveTransaction_AfterConfirm_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 确认交易
	err = pool.ConfirmTransactions([][]byte{txID}, 1)
	require.NoError(t, err)

	// Act - 尝试移除已确认的交易
	err = pool.RemoveTransaction(txID)

	// Assert
	assert.Error(t, err, "移除已确认的交易应该返回错误（因为交易已被移除）")
}

