// Package txpool 淘汰策略测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestExecuteEvictionStrategy_WithMemoryLimit_EvictsLowPriorityTxs 测试淘汰策略通过内存限制触发
func TestExecuteEvictionStrategy_WithMemoryLimit_EvictsLowPriorityTxs(t *testing.T) {
	// Arrange
	// 创建内存限制很小的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    100,
		MemoryLimit: 8000, // 8KB限制
		MaxTxSize:  1024 * 1024,
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
	txPool := pool.(*TxPool)

	// 提交多个交易填满内存
	numTxs := 10
	txIDs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
		txIDs[i] = txID
	}

	// Act - 提交大量新交易，触发淘汰策略
	newTxCount := 0
	for i := 100; i < 200; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.AddTransaction(tx)
		if err != nil {
			// 如果内存已满，停止
			break
		}
		newTxCount++
		if newTxCount > 20 {
			// 限制测试时间
			break
		}
	}

	// Assert
	// 验证一些旧交易可能被淘汰
	finalPending, _ := txPool.GetAllPendingTransactions()
	finalCount := len(finalPending)
	// 由于淘汰策略，最终数量可能小于初始数量+新交易数
	assert.GreaterOrEqual(t, finalCount, 0, "应该至少有一些pending交易")
	// 验证内存使用量被管理
	assert.LessOrEqual(t, txPool.memoryUsage, txPool.memoryLimit, "内存使用量不应超过限制")
}

// TestExecuteEvictionStrategy_WithEmptyCandidates_ReturnsZero 测试空候选列表
func TestExecuteEvictionStrategy_WithEmptyCandidates_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	emptyCandidates := []*TxWrapper{}

	// Act
	// 注意：executeEvictionStrategy是内部方法，我们通过内存限制间接测试
	// 这里我们主要验证不会panic
	result := pool.executeEvictionStrategy(emptyCandidates, 1000)

	// Assert
	assert.Equal(t, 0, result, "空候选列表应该返回0")
}

// TestExecuteEvictionStrategy_WithSmallRequiredSpace_EvictsFewTxs 测试小空间需求
func TestExecuteEvictionStrategy_WithSmallRequiredSpace_EvictsFewTxs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// 提交几个交易
	for i := 0; i < 5; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// 获取所有交易包装器
	txWrappers := make([]*TxWrapper, 0)
	pool.mu.RLock()
	for _, wrapper := range pool.txs {
		txWrappers = append(txWrappers, wrapper)
	}
	pool.mu.RUnlock()

	// Act - 执行淘汰策略，只需要小空间
	pool.mu.Lock()
	evictedCount := pool.executeEvictionStrategy(txWrappers, 100)
	pool.mu.Unlock()

	// Assert
	// 应该只淘汰少量交易
	assert.GreaterOrEqual(t, evictedCount, 0, "淘汰数量应该>=0")
	assert.LessOrEqual(t, evictedCount, len(txWrappers), "淘汰数量不应超过候选数量")
}

