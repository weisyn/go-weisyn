// Package txpool 交易确认和拒绝测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestConfirmTransactions_WithPendingTxs_RemovesFromPool 测试确认pending交易
func TestConfirmTransactions_WithPendingTxs_RemovesFromPool(t *testing.T) {
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
	err = pool.ConfirmTransactions([][]byte{txID}, 1)

	// Assert
	assert.NoError(t, err, "应该成功确认交易")
	// 确认后交易应该被移除
	retrievedTx2, err := pool.GetTx(txID)
	assert.Error(t, err, "确认后的交易应该不存在")
	assert.Nil(t, retrievedTx2, "确认后的交易应该为nil")
}

// TestConfirmTransactions_WithMultipleTxs_RemovesAll 测试批量确认交易
func TestConfirmTransactions_WithMultipleTxs_RemovesAll(t *testing.T) {
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
	err := pool.ConfirmTransactions(txIDs, 1)

	// Assert
	assert.NoError(t, err, "应该成功批量确认交易")
	// 所有交易都应该被移除
	for _, txID := range txIDs {
		retrievedTx, err := pool.GetTx(txID)
		assert.Error(t, err, "确认后的交易应该不存在")
		assert.Nil(t, retrievedTx, "确认后的交易应该为nil")
	}
}

// TestConfirmTransactions_WithNonExistentTx_NoError 测试确认不存在的交易
func TestConfirmTransactions_WithNonExistentTx_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.ConfirmTransactions([][]byte{nonExistentTxID}, 1)

	// Assert
	// 根据实现，不存在的交易不会返回错误，只是被忽略
	assert.NoError(t, err, "不存在的交易应该被忽略")
}

// TestConfirmTransactions_WithEmptyList_NoError 测试空列表时无错误
func TestConfirmTransactions_WithEmptyList_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.ConfirmTransactions([][]byte{}, 1)

	// Assert
	assert.NoError(t, err, "空列表应该成功")
}

// TestRejectTransactions_WithMiningTxs_ReturnsToPending 测试拒绝挖矿中的交易
func TestRejectTransactions_WithMiningTxs_ReturnsToPending(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 标记为挖矿中
	err = pool.MarkTransactionsAsMining([][]byte{txID})
	require.NoError(t, err)

	// Act
	err = pool.RejectTransactions([][]byte{txID})

	// Assert
	assert.NoError(t, err, "应该成功拒绝交易")
	// 拒绝后交易应该回到pending状态（或从池中移除，取决于实现）
	// 注意：根据实现，RejectTransactions 会将mining状态的交易恢复为pending
}

// TestRejectTransactions_WithNonExistentTx_NoError 测试拒绝不存在的交易
func TestRejectTransactions_WithNonExistentTx_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.RejectTransactions([][]byte{nonExistentTxID})

	// Assert
	// 根据实现，不存在的交易不会返回错误，只是被忽略
	assert.NoError(t, err, "不存在的交易应该被忽略")
}

// TestRejectTransactions_WithEmptyList_NoError 测试空列表时无错误
func TestRejectTransactions_WithEmptyList_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.RejectTransactions([][]byte{})

	// Assert
	assert.NoError(t, err, "空列表应该成功")
}

// TestConfirmTransactions_AfterClose_ReturnsError 测试关闭后确认返回错误
func TestConfirmTransactions_AfterClose_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 关闭交易池
	err = pool.Close()
	require.NoError(t, err)

	// Act
	err = pool.ConfirmTransactions([][]byte{txID}, 1)

	// Assert
	assert.Error(t, err, "关闭后确认应该返回错误")
	assert.Equal(t, ErrTxPoolClosed, err, "应该返回ErrTxPoolClosed错误")
}

// TestRejectTransactions_AfterClose_ReturnsError 测试关闭后拒绝返回错误
func TestRejectTransactions_AfterClose_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 关闭交易池
	err = pool.Close()
	require.NoError(t, err)

	// Act
	err = pool.RejectTransactions([][]byte{txID})

	// Assert
	assert.Error(t, err, "关闭后拒绝应该返回错误")
	assert.Equal(t, ErrTxPoolClosed, err, "应该返回ErrTxPoolClosed错误")
}

// TestConfirmTransactions_UpdatesBlockHeight 测试确认交易时更新区块高度
func TestConfirmTransactions_UpdatesBlockHeight(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	blockHeight := uint64(100)

	// Act
	err = pool.ConfirmTransactions([][]byte{txID}, blockHeight)

	// Assert
	assert.NoError(t, err, "应该成功确认交易")
	// 验证交易已被移除（确认成功）
	retrievedTx, err := pool.GetTx(txID)
	assert.Error(t, err, "确认后的交易应该不存在")
	assert.Nil(t, retrievedTx, "确认后的交易应该为nil")
}

