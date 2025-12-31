// Package txpool 查询交易测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// TestGetTx_WithExistingTransaction_ReturnsTransaction 测试获取存在的交易
func TestGetTx_WithExistingTransaction_ReturnsTransaction(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	retrievedTx, err := pool.GetTx(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取交易")
	assert.NotNil(t, retrievedTx, "交易不应为nil")
	assert.Equal(t, tx.Version, retrievedTx.Version, "交易版本应该相同")
	assert.Equal(t, tx.Nonce, retrievedTx.Nonce, "交易Nonce应该相同")
}

// TestGetTx_WithNonExistentTransaction_ReturnsError 测试获取不存在的交易
func TestGetTx_WithNonExistentTransaction_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	tx, err := pool.GetTx(nonExistentTxID)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应为nil")
	assert.Contains(t, err.Error(), "交易不存在", "错误信息应该包含相关描述")
}

// TestGetTx_WithNilTxID_ReturnsError 测试nil交易ID时返回错误
func TestGetTx_WithNilTxID_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	tx, err := pool.GetTx(nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应为nil")
}

// TestGetTxStatus_WithPendingTransaction_ReturnsPending 测试获取pending状态的交易
func TestGetTxStatus_WithPendingTransaction_ReturnsPending(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	status, err := pool.GetTxStatus(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取状态")
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "状态应该是Pending")
}

// TestGetTxStatus_WithNonExistentTransaction_ReturnsError 测试获取不存在交易的状态
func TestGetTxStatus_WithNonExistentTransaction_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	status, err := pool.GetTxStatus(nonExistentTxID)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, mempoolIfaces.TxStatusUnknown, status, "状态应该是Unknown")
}

// TestGetAllPendingTransactions_WithPendingTxs_ReturnsAll 测试获取所有pending交易
func TestGetAllPendingTransactions_WithPendingTxs_ReturnsAll(t *testing.T) {
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

	// Act
	pendingTxs, err := pool.GetAllPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功获取所有pending交易")
	assert.GreaterOrEqual(t, len(pendingTxs), numTxs, "应该返回至少numTxs个交易")
}

// TestGetAllPendingTransactions_WithNoPendingTxs_ReturnsEmpty 测试没有pending交易时返回空列表
func TestGetAllPendingTransactions_WithNoPendingTxs_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	pendingTxs, err := pool.GetAllPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功返回空列表")
	assert.Len(t, pendingTxs, 0, "应该返回空列表")
}

// TestGetAllPendingTransactions_AfterConfirm_ExcludesConfirmed 测试确认后不再包含在pending列表中
func TestGetAllPendingTransactions_AfterConfirm_ExcludesConfirmed(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 验证交易在pending列表中
	pendingTxs1, err := pool.GetAllPendingTransactions()
	require.NoError(t, err)
	require.Greater(t, len(pendingTxs1), 0, "提交后应该有pending交易")

	// Act - 确认交易
	err = pool.ConfirmTransactions([][]byte{txID}, 1)
	require.NoError(t, err)

	// Act - 再次获取pending交易
	pendingTxs2, err := pool.GetAllPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功获取pending交易列表")
	// 确认后的交易不应该在pending列表中
	found := false
	for _, pendingTx := range pendingTxs2 {
		if pendingTx.Nonce == tx.Nonce {
			found = true
			break
		}
	}
	assert.False(t, found, "确认后的交易不应该在pending列表中")
}

