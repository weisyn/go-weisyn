// Package txpool GetTransactionStatus边界情况测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// TestGetTransactionStatus_WithMiningStatus_ReturnsPending 测试挖矿中状态
func TestGetTransactionStatus_WithMiningStatus_ReturnsPending(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 标记为挖矿中
	err = pool.MarkTransactionsAsMining([][]byte{txID})
	require.NoError(t, err)

	// Act
	status, err := pool.GetTransactionStatus(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取状态")
	// 注意：根据实现，Mining状态现在返回Pending（因为交易仍在处理中）
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "Mining状态应该返回Pending")
}

// TestGetTransactionStatus_WithPendingConfirmStatus_ReturnsPending 测试待确认状态
func TestGetTransactionStatus_WithPendingConfirmStatus_ReturnsPending(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 标记为挖矿中，然后标记为待确认
	err = pool.MarkTransactionsAsMining([][]byte{txID})
	require.NoError(t, err)
	err = pool.MarkTransactionsAsPendingConfirm([][]byte{txID}, 100)
	require.NoError(t, err)

	// Act
	status, err := pool.GetTransactionStatus(txID)

	// Assert
	assert.NoError(t, err, "应该成功获取状态")
	// 注意：根据实现，PendingConfirm状态现在返回Pending（因为交易仍在处理中）
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "PendingConfirm状态应该返回Pending")
}

// TestGetTransactionStatus_WithAllStatuses_CoversAllCases 测试所有状态
func TestGetTransactionStatus_WithAllStatuses_CoversAllCases(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// 测试Pending状态
	tx1 := testutil.CreateSimpleTestTransaction(1)
	txID1, err := pool.SubmitTx(tx1)
	require.NoError(t, err)
	status1, err := pool.GetTransactionStatus(txID1)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status1, "应该是Pending状态")

	// 测试Rejected状态
	tx2 := testutil.CreateSimpleTestTransaction(2)
	txID2, err := pool.SubmitTx(tx2)
	require.NoError(t, err)
	err = pool.UpdateTransactionStatus(txID2, mempoolIfaces.TxStatusRejected)
	require.NoError(t, err)
	status2, err := pool.GetTransactionStatus(txID2)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusRejected, status2, "应该是Rejected状态")

	// 测试Confirmed状态
	// 注意：ConfirmTransactions会移除交易，所以无法通过GetTransactionStatus查询
	// 这里我们通过UpdateTransactionStatus来设置状态
	tx3 := testutil.CreateSimpleTestTransaction(3)
	txID3, err := pool.SubmitTx(tx3)
	require.NoError(t, err)
	err = pool.UpdateTransactionStatus(txID3, mempoolIfaces.TxStatusConfirmed)
	require.NoError(t, err)
	status3, err := pool.GetTransactionStatus(txID3)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusConfirmed, status3, "应该是Confirmed状态")

	// 测试Expired状态
	tx4 := testutil.CreateSimpleTestTransaction(4)
	txID4, err := pool.SubmitTx(tx4)
	require.NoError(t, err)
	err = pool.UpdateTransactionStatus(txID4, mempoolIfaces.TxStatusExpired)
	require.NoError(t, err)
	status4, err := pool.GetTransactionStatus(txID4)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusExpired, status4, "应该是Expired状态")
}

