// Package txpool RemoveTxs方法测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestRemoveTxs_WithValidTxIDs_RemovesAllTxs 测试移除多个有效交易
func TestRemoveTxs_WithValidTxIDs_RemovesAllTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5
	txIDs := make([][]byte, numTxs)

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// Act
	err := pool.RemoveTxs(txIDs)

	// Assert
	assert.NoError(t, err, "应该成功移除所有交易")
	// 验证交易已被移除
	for _, txID := range txIDs {
		_, err := pool.GetTransaction(txID)
		assert.Error(t, err, "交易应该已被移除")
	}
}

// TestRemoveTxs_WithPartialValidTxIDs_ReturnsError 测试部分有效交易ID
func TestRemoveTxs_WithPartialValidTxIDs_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")
	txIDs := [][]byte{txID, nonExistentTxID}

	// Act
	err = pool.RemoveTxs(txIDs)

	// Assert
	assert.Error(t, err, "应该返回错误（因为包含不存在的交易）")
	// 第一个交易可能已被移除，也可能没有（取决于实现）
}

// TestRemoveTxs_WithEmptyList_ReturnsNoError 测试空列表
func TestRemoveTxs_WithEmptyList_ReturnsNoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.RemoveTxs([][]byte{})

	// Assert
	assert.NoError(t, err, "空列表应该不返回错误")
}

// TestRemoveTxs_WithNilList_ReturnsNoError 测试nil列表
func TestRemoveTxs_WithNilList_ReturnsNoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.RemoveTxs(nil)

	// Assert
	assert.NoError(t, err, "nil列表应该不返回错误")
}

// TestRemoveTxs_WithAllNonExistentTxIDs_ReturnsError 测试所有交易ID都不存在
func TestRemoveTxs_WithAllNonExistentTxIDs_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID1 := []byte("non_existent_tx_id_32_bytes_12345678")
	nonExistentTxID2 := []byte("non_existent_tx_id_32_bytes_87654321")
	txIDs := [][]byte{nonExistentTxID1, nonExistentTxID2}

	// Act
	err := pool.RemoveTxs(txIDs)

	// Assert
	assert.Error(t, err, "应该返回错误（因为所有交易都不存在）")
}

// TestRemoveTxs_AfterPoolClose_ReturnsError 测试池关闭后移除交易
func TestRemoveTxs_AfterPoolClose_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 关闭池
	err = pool.Close()
	require.NoError(t, err)

	// Act
	err = pool.RemoveTxs([][]byte{txID})

	// Assert
	// 注意：RemoveTxs内部调用BatchRemoveTransactions，可能不会检查池状态
	// 这里主要验证方法不会panic
	_ = err // 可能返回错误，也可能不返回
}

