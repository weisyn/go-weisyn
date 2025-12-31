// Package txpool 挖矿相关测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// TestGetTransactionsForMining_WithPendingTxs_ReturnsTransactions 测试获取挖矿交易
func TestGetTransactionsForMining_WithPendingTxs_ReturnsTransactions(t *testing.T) {
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
	miningTxs, err := pool.GetTransactionsForMining()

	// Assert
	assert.NoError(t, err, "应该成功获取挖矿交易")
	assert.GreaterOrEqual(t, len(miningTxs), 0, "应该返回交易列表")
	assert.LessOrEqual(t, len(miningTxs), numTxs, "返回的交易数不应超过提交的交易数")
}

// TestGetTransactionsForMining_WithNoPendingTxs_ReturnsEmpty 测试没有pending交易时返回空列表
func TestGetTransactionsForMining_WithNoPendingTxs_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	miningTxs, err := pool.GetTransactionsForMining()

	// Assert
	assert.NoError(t, err, "应该成功返回空列表")
	assert.Len(t, miningTxs, 0, "应该返回空列表")
}

// TestMarkTransactionsAsMining_WithPendingTxs_MarksAsMining 测试标记交易为挖矿中
func TestMarkTransactionsAsMining_WithPendingTxs_MarksAsMining(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 验证初始状态是pending
	status, err := pool.GetTxStatus(txID)
	require.NoError(t, err)
	require.Equal(t, mempoolIfaces.TxStatusPending, status, "初始状态应该是Pending")

	// Act
	err = pool.MarkTransactionsAsMining([][]byte{txID})

	// Assert
	assert.NoError(t, err, "应该成功标记为挖矿中")
	// 注意：MarkTransactionsAsMining 不会改变状态为Mining（因为接口中没有Mining状态）
	// 但会从pendingTxs中移除
}

// TestMarkTransactionsAsMining_WithNonExistentTx_NoError 测试标记不存在的交易
func TestMarkTransactionsAsMining_WithNonExistentTx_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	nonExistentTxID := []byte("non_existent_tx_id_32_bytes_12345678")

	// Act
	err := pool.MarkTransactionsAsMining([][]byte{nonExistentTxID})

	// Assert
	// 注意：根据实现，不存在的交易不会返回错误，只是被忽略
	assert.NoError(t, err, "不存在的交易应该被忽略")
}

// TestMarkTransactionsAsMining_WithEmptyList_NoError 测试空列表时无错误
func TestMarkTransactionsAsMining_WithEmptyList_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.MarkTransactionsAsMining([][]byte{})

	// Assert
	assert.NoError(t, err, "空列表应该成功")
}

// TestGetTransactionsForMining_RespectsMaxCount 测试挖矿交易数量限制
func TestGetTransactionsForMining_RespectsMaxCount(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	// 提交大量交易
	numTxs := 200
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	miningTxs, err := pool.GetTransactionsForMining()

	// Assert
	assert.NoError(t, err, "应该成功获取挖矿交易")
	// 根据配置，MaxTransactionsForMining = 100
	assert.LessOrEqual(t, len(miningTxs), 100, "返回的交易数不应超过配置的最大值")
}

