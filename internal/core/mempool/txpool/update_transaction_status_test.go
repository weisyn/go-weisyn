// Package txpool UpdateTransactionStatus覆盖率提升测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// TestUpdateTransactionStatus_WithMiningToPendingConfirm_UpdatesStatus 测试从Mining到PendingConfirm
func TestUpdateTransactionStatus_WithMiningToPendingConfirm_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// 标记为挖矿中
	err = pool.MarkTransactionsAsMining([][]byte{txID})
	require.NoError(t, err)
	
	// Act - 注意：UpdateTransactionStatus不支持PendingConfirm状态，需要通过MarkTransactionsAsPendingConfirm
	// 这里我们测试从Mining到其他状态的转换
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "应该是Pending状态")
}

// TestUpdateTransactionStatus_WithInvalidStatus_ReturnsError_Advanced 测试无效状态
func TestUpdateTransactionStatus_WithInvalidStatus_ReturnsError_Advanced(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act - 使用无效状态（TxStatusUnknown）
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusUnknown)
	
	// Assert
	assert.Error(t, err, "无效状态应该返回错误")
	assert.Contains(t, err.Error(), "无效的交易状态", "错误信息应该包含'无效的交易状态'")
}

// TestUpdateTransactionStatus_WithSameStatus_ReturnsNoError 测试相同状态
func TestUpdateTransactionStatus_WithSameStatus_ReturnsNoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act - 更新为相同状态
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)
	
	// Assert
	assert.NoError(t, err, "相同状态应该不返回错误")
}

// TestUpdateTransactionStatus_WithPendingToRejected_UpdatesStatus_Advanced 测试从Pending到Rejected
func TestUpdateTransactionStatus_WithPendingToRejected_UpdatesStatus_Advanced(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusRejected)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusRejected, status, "应该是Rejected状态")
}

// TestUpdateTransactionStatus_WithPendingToConfirmed_UpdatesStatus_Advanced 测试从Pending到Confirmed
func TestUpdateTransactionStatus_WithPendingToConfirmed_UpdatesStatus_Advanced(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusConfirmed)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusConfirmed, status, "应该是Confirmed状态")
}

// TestUpdateTransactionStatus_WithPendingToExpired_UpdatesStatus_Advanced 测试从Pending到Expired
func TestUpdateTransactionStatus_WithPendingToExpired_UpdatesStatus_Advanced(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusExpired)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusExpired, status, "应该是Expired状态")
}

// TestUpdateTransactionStatus_WithRejectedToPending_UpdatesStatus 测试从Rejected到Pending
func TestUpdateTransactionStatus_WithRejectedToPending_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// 先标记为Rejected
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusRejected)
	require.NoError(t, err)
	
	// Act - 从Rejected回到Pending
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "应该是Pending状态")
}

// TestUpdateTransactionStatus_WithExpiredToPending_UpdatesStatus 测试从Expired到Pending
func TestUpdateTransactionStatus_WithExpiredToPending_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// 先标记为Expired
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusExpired)
	require.NoError(t, err)
	
	// Act - 从Expired回到Pending
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "应该是Pending状态")
}

// TestUpdateTransactionStatus_WithConfirmedToPending_UpdatesStatus 测试从Confirmed到Pending
func TestUpdateTransactionStatus_WithConfirmedToPending_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// 先标记为Confirmed
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusConfirmed)
	require.NoError(t, err)
	
	// Act - 从Confirmed回到Pending
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)
	
	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	status, err := pool.GetTransactionStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "应该是Pending状态")
}

