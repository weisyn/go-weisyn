// Package txpool 过期交易清理测试
package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
)

// createTestTxPoolWithShortLifetime 创建生命周期很短的交易池用于测试过期
func createTestTxPoolWithShortLifetime(t *testing.T) *TxPool {
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   100 * time.Millisecond, // 100毫秒生命周期
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	pool, err := NewTxPoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	return pool.(*TxPool)
}

// TestSyncStatus_WithExpiredTxs_MarksAsExpired_ShortLifetime 测试同步状态时标记过期交易（短生命周期）
func TestSyncStatus_WithExpiredTxs_MarksAsExpired_ShortLifetime(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 验证初始状态是pending
	status, err := pool.GetTxStatus(txID)
	require.NoError(t, err)
	require.Equal(t, mempoolIfaces.TxStatusPending, status)

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// Act - 同步状态（应该标记过期交易）
	err = pool.SyncStatus(1, []byte("state_root"))

	// Assert
	assert.NoError(t, err, "同步状态应该成功")
	// 验证交易状态已更新为过期
	newStatus, err := pool.GetTxStatus(txID)
	if err == nil {
		// 如果交易仍然存在，状态应该是Expired
		assert.Equal(t, mempoolIfaces.TxStatusExpired, newStatus, "过期交易应该被标记为Expired")
	} else {
		// 或者交易可能已被清理
		assert.Error(t, err, "过期交易可能已被清理")
	}
}

// TestCleanExpiredTransactions_Indirectly_ThroughSyncStatus 间接测试清理过期交易
func TestCleanExpiredTransactions_Indirectly_ThroughSyncStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)
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

	// 验证所有交易都在pending状态
	pendingTxs, err := pool.GetAllPendingTransactions()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pendingTxs), numTxs, "提交后应该有pending交易")

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// Act - 同步状态（应该清理过期交易）
	err = pool.SyncStatus(1, []byte("state_root"))
	require.NoError(t, err)

	// Assert
	// 验证过期交易不再在pending列表中
	pendingTxs2, err := pool.GetAllPendingTransactions()
	assert.NoError(t, err)
	// 过期交易应该被移除或标记为过期
	assert.Less(t, len(pendingTxs2), len(pendingTxs), "过期交易应该被清理")
}

// TestRecomputePriorities_Indirectly_ThroughUpdateStatus 间接测试重新计算优先级
func TestRecomputePriorities_Indirectly_ThroughUpdateStatus(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 更新状态为rejected，然后恢复为pending（这会触发优先级重新计算）
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusRejected)
	require.NoError(t, err)

	// Act - 恢复为pending状态（应该重新计算优先级）
	err = pool.UpdateTransactionStatus(txID, mempoolIfaces.TxStatusPending)

	// Assert
	assert.NoError(t, err, "应该成功更新状态")
	// 验证交易回到pending状态
	status, err := pool.GetTxStatus(txID)
	assert.NoError(t, err)
	assert.Equal(t, mempoolIfaces.TxStatusPending, status, "交易应该回到pending状态")
	// 注意：优先级重新计算是内部行为，我们通过状态更新间接验证
}

// TestExpiredTransactions_NotInPendingList 测试过期交易不在pending列表中
func TestExpiredTransactions_NotInPendingList(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	_, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// Act - 同步状态
	err = pool.SyncStatus(1, []byte("state_root"))
	require.NoError(t, err)

	// Act - 获取pending交易列表
	pendingTxs, err := pool.GetAllPendingTransactions()

	// Assert
	assert.NoError(t, err, "应该成功获取pending交易列表")
	// 过期交易不应该在pending列表中
	found := false
	for _, pendingTx := range pendingTxs {
		if pendingTx.Nonce == tx.Nonce {
			found = true
			break
		}
	}
	assert.False(t, found, "过期交易不应该在pending列表中")
}

// TestExpiredTransactions_StatusIsExpired 测试过期交易状态为Expired
func TestExpiredTransactions_StatusIsExpired(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// Act - 同步状态
	err = pool.SyncStatus(1, []byte("state_root"))
	require.NoError(t, err)

	// Act - 获取交易状态
	status, err := pool.GetTxStatus(txID)

	// Assert
	// 如果交易仍然存在，状态应该是Expired
	if err == nil {
		assert.Equal(t, mempoolIfaces.TxStatusExpired, status, "过期交易状态应该是Expired")
	} else {
		// 或者交易已被清理（这是正常的，因为过期交易会被移除）
		assert.Error(t, err, "过期交易可能已被清理")
	}
	// 验证txID被使用（避免编译警告）
	_ = txID
}

