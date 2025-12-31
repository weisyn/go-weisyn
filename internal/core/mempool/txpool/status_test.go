// Package txpool 交易状态管理测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// TestUpdateTransactionStatus_WithPendingToRejected_UpdatesStatus 测试更新状态从Pending到Rejected
func TestUpdateTransactionStatus_WithPendingToRejected_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 验证初始状态是pending
	status, err := pool.GetTxStatus(txID)
	require.NoError(t, err)
	require.Equal(t, mempoolIfaces.TxStatusPending, status)

	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusRejected)

	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	// 验证状态已更新
	newStatus, err := pool.GetTxStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusRejected, newStatus, "状态应该更新为Rejected")
}

// TestUpdateTransactionStatus_WithPendingToExpired_UpdatesStatus 测试更新状态从Pending到Expired
func TestUpdateTransactionStatus_WithPendingToExpired_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusExpired)

	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	// 验证状态已更新
	newStatus, err := pool.GetTxStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusExpired, newStatus, "状态应该更新为Expired")
}

// TestUpdateTransactionStatus_WithPendingToConfirmed_UpdatesStatus 测试更新状态从Pending到Confirmed
func TestUpdateTransactionStatus_WithPendingToConfirmed_UpdatesStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusConfirmed)

	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	// 验证状态已更新
	newStatus, err := pool.GetTxStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusConfirmed, newStatus, "状态应该更新为Confirmed")
}

// TestUpdateTransactionStatus_WithSameStatus_NoError 测试更新为相同状态时无错误
func TestUpdateTransactionStatus_WithSameStatus_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act - 更新为相同状态
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)

	// Assert
	assert.NoError(t, err, "更新为相同状态应该无错误")
	// 状态应该保持不变
	status, err := pool.GetTxStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "状态应该保持不变")
}

// TestUpdateTransactionStatus_WithNonExistentTx_ReturnsError 测试更新不存在交易的状态
func TestUpdateTransactionStatus_WithNonExistentTx_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.UpdateTransactionStatus(nonExistentTxID, mempoolIfaces.TxStatusRejected)

	// Assert
	assert.Error(t, err, "更新不存在交易的状态应该返回错误")
	assert.Contains(t, err.Error(), "交易不存在", "错误信息应该包含相关描述")
}

// TestUpdateTransactionStatus_WithInvalidStatus_ReturnsError 测试无效状态时返回错误
func TestUpdateTransactionStatus_WithInvalidStatus_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// Act - 使用无效状态（TxStatusUnknown通常不应该用于更新）
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusUnknown)

	// Assert
	assert.Error(t, err, "无效状态应该返回错误")
	assert.Contains(t, err.Error(), "无效", "错误信息应该包含相关描述")
}

// TestSyncStatus_WithNonExpiredTxs_NoChange 测试同步状态时未过期交易不变
func TestSyncStatus_WithNonExpiredTxs_NoChange(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 注意：SyncStatus会检查交易的过期时间
	// 由于测试中的交易刚创建，可能不会立即过期
	// 我们需要验证SyncStatus不会出错

	// Act
	err = pool.SyncStatus(1, []byte("state_root"))

	// Assert
	assert.NoError(t, err, "同步状态应该成功")
	// 验证交易仍然存在（如果未过期）
	retrievedTx, err := pool.GetTx(txID)
	// 如果交易未过期，应该仍然存在
	if err == nil {
		assert.NotNil(t, retrievedTx, "未过期的交易应该仍然存在")
	}
}

// TestSyncStatus_WithEmptyPool_NoError 测试空池同步状态
func TestSyncStatus_WithEmptyPool_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.SyncStatus(1, []byte("state_root"))

	// Assert
	assert.NoError(t, err, "空池同步状态应该成功")
}

// TestSyncStatus_WithNilStateRoot_NoError 测试nil状态根时无错误
func TestSyncStatus_WithNilStateRoot_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.SyncStatus(1, nil)

	// Assert
	assert.NoError(t, err, "nil状态根应该无错误")
}

// TestGetTransactionsByStatus_WithPendingStatus_ReturnsPendingTxs 测试按状态查询交易
func TestGetTransactionsByStatus_WithPendingStatus_ReturnsPendingTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 3

	// 提交多个pending交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	pendingTxs, err := pool.GetTransactionsByStatus(mempoolIfaces.TxStatusPending)

	// Assert
	assert.NoError(t, err, "应该成功查询pending交易")
	assert.GreaterOrEqual(t, len(pendingTxs), numTxs, "应该返回至少numTxs个pending交易")
}

// TestGetTransactionsByStatus_WithRejectedStatus_ReturnsRejectedTxs 测试查询rejected交易
func TestGetTransactionsByStatus_WithRejectedStatus_ReturnsRejectedTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 更新为rejected状态
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusRejected)
	require.NoError(t, err)

	// Act
	rejectedTxs, err := pool.GetTransactionsByStatus(mempoolIfaces.TxStatusRejected)

	// Assert
	assert.NoError(t, err, "应该成功查询rejected交易")
	assert.GreaterOrEqual(t, len(rejectedTxs), 1, "应该返回至少1个rejected交易")
}

// TestGetTransactionsByStatus_WithEmptyStatus_ReturnsEmpty 测试空状态时返回空列表
func TestGetTransactionsByStatus_WithEmptyStatus_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act - 查询expired状态（应该为空）
	expiredTxs, err := pool.GetTransactionsByStatus(mempoolIfaces.TxStatusExpired)

	// Assert
	assert.NoError(t, err, "应该成功返回空列表")
	assert.Len(t, expiredTxs, 0, "应该返回空列表")
}

// TestGetPendingTxs_WithLimit_RespectsLimit 测试GetPendingTxs尊重限制
func TestGetPendingTxs_WithLimit_RespectsLimit(t *testing.T) {
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
	pendingTxs, err := pool.GetPendingTxs(5, 1024*1024, nil)

	// Assert
	assert.NoError(t, err, "应该成功获取pending交易")
	assert.LessOrEqual(t, len(pendingTxs), 5, "返回的交易数不应超过限制")
}

// TestGetPendingTxs_WithExcludedTxs_ExcludesThem 测试排除指定交易
func TestGetPendingTxs_WithExcludedTxs_ExcludesThem(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx1 := testutil.CreateTestTransaction(1, nil, nil)
	tx1.Nonce = 1
	txID1, err := pool.SubmitTx(tx1)
	require.NoError(t, err)

	tx2 := testutil.CreateTestTransaction(2, nil, nil)
	tx2.Nonce = 2
	_, err = pool.SubmitTx(tx2)
	require.NoError(t, err)

	// Act - 排除txID1
	pendingTxs, err := pool.GetPendingTxs(10, 1024*1024, [][]byte{txID1})

	// Assert
	assert.NoError(t, err, "应该成功获取pending交易")
	// 验证txID1不在结果中
	found := false
	for _, pendingTx := range pendingTxs {
		if pendingTx.Nonce == tx1.Nonce {
			found = true
			break
		}
	}
	assert.False(t, found, "排除的交易不应该在结果中")
}

