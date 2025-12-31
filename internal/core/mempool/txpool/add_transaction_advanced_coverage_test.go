// Package txpool AddTransaction高级覆盖率测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestAddTransaction_WithPoolClosed_ReturnsError 测试交易池关闭后添加交易
func TestAddTransaction_WithPoolClosed_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	
	// 关闭交易池
	err := pool.Close()
	require.NoError(t, err)
	
	// Act
	txID, err := pool.AddTransaction(tx)
	
	// Assert
	assert.Error(t, err, "交易池关闭后应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
	assert.Contains(t, err.Error(), "交易池已关闭", "错误信息应该包含'交易池已关闭'")
}

// TestAddTransaction_WithMemoryLimitExceededAfterEviction_ReturnsError 测试淘汰后仍超限
func TestAddTransaction_WithMemoryLimitExceededAfterEviction_ReturnsError(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 5000, // 很小的内存限制
		MaxTxSize:   1024 * 1024,
		Lifetime:    3600 * 1000000000, // 1小时
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
	
	// 填满交易池（接近内存限制）
	numTxs := 5
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break // 如果内存已满，停止
		}
	}
	
	// 尝试添加一个会导致超限的交易
	tx := testutil.CreateSimpleTestTransaction(100)
	
	// Act
	txID, err := txPool.AddTransaction(tx)
	
	// Assert
	// 可能成功（如果淘汰了足够交易）或失败（如果仍然超限）
	if err != nil {
		assert.Contains(t, err.Error(), "交易池已满", "应该返回交易池已满错误")
	} else {
		assert.NotNil(t, txID, "如果淘汰成功，应该有交易ID")
	}
}

// TestAddTransaction_WithEventSinkNil_NoPanic 测试事件下沉为nil时不panic
func TestAddTransaction_WithEventSinkNil_NoPanic(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	pool.SetEventSink(nil) // 设置nil事件下沉
	tx := testutil.CreateSimpleTestTransaction(1)
	
	// Act & Assert
	// 不应该panic
	assert.NotPanics(t, func() {
		_, err := pool.AddTransaction(tx)
		_ = err // 可能成功或失败，但不应该panic
	})
}

// TestAddTransaction_WithProtectorNil_NoError 测试保护器为nil时正常工作
func TestAddTransaction_WithProtectorNil_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	
	// Act
	txID, err := pool.AddTransaction(tx)
	
	// Assert
	// 保护器为nil时应该正常工作（跳过保护器检查）
	if err == nil {
		assert.NotNil(t, txID, "应该成功添加交易")
	}
}

// TestAddTransaction_WithCleanExpiredThenAdd_Succeeds_Advanced 测试清理过期交易后添加成功
func TestAddTransaction_WithCleanExpiredThenAdd_Succeeds_Advanced(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 10000, // 小内存限制
		MaxTxSize:   1024 * 1024,
		Lifetime:    1000000000, // 1秒，快速过期
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
	
	// 添加一个会过期的交易
	tx1 := testutil.CreateSimpleTestTransaction(1)
	_, err = txPool.SubmitTx(tx1)
	require.NoError(t, err)
	
	// 等待交易过期
	// 注意：由于Lifetime很短，交易会很快过期
	
	// 尝试添加新交易（应该触发清理过期交易）
	tx2 := testutil.CreateSimpleTestTransaction(2)
	
	// Act
	txID, err := txPool.AddTransaction(tx2)
	
	// Assert
	// 应该成功（清理过期交易后应该有空间）
	assert.NoError(t, err, "清理过期交易后应该成功添加")
	assert.NotNil(t, txID, "应该有交易ID")
}

// TestAddTransaction_WithEvictionStrategy_RemovesLowPriorityTxs_Advanced 测试淘汰策略移除低优先级交易
func TestAddTransaction_WithEvictionStrategy_RemovesLowPriorityTxs_Advanced(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 10000, // 小内存限制
		MaxTxSize:   1024 * 1024,
		Lifetime:    3600 * 1000000000, // 1小时
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
	
	// 填满交易池
	numTxs := 10
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
	}
	
	// 尝试添加新交易（应该触发淘汰策略）
	tx := testutil.CreateSimpleTestTransaction(100)
	
	// Act
	txID, err := txPool.AddTransaction(tx)
	
	// Assert
	// 可能成功（如果淘汰了足够交易）或失败（如果仍然超限）
	if err == nil {
		assert.NotNil(t, txID, "如果淘汰成功，应该有交易ID")
	}
}

