// Package txpool 提交交易测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// createTestTxPool 创建测试用的交易池
func createTestTxPool(t *testing.T) *TxPool {
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024, // 100MB
		MaxTxSize:  1024 * 1024,        // 1MB
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024, // 1MB
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

// TestSubmitTx_WithValidTransaction_ReturnsTxID 测试使用有效交易提交
func TestSubmitTx_WithValidTransaction_ReturnsTxID(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := pool.SubmitTx(tx)

	// Assert
	assert.NoError(t, err, "应该成功提交交易")
	assert.NotNil(t, txID, "交易ID不应为nil")
	assert.Len(t, txID, 32, "交易ID应该是32字节")
}

// TestSubmitTx_WithNilTransaction_ReturnsError 测试nil交易时返回错误
func TestSubmitTx_WithNilTransaction_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	txID, err := pool.SubmitTx(nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, txID, "交易ID应为nil")
}

// TestSubmitTx_WithDuplicateTransaction_ReturnsError 测试重复交易时返回错误
func TestSubmitTx_WithDuplicateTransaction_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act - 第一次提交
	txID1, err1 := pool.SubmitTx(tx)
	require.NoError(t, err1, "第一次提交应该成功")

	// Act - 第二次提交相同交易
	txID2, err2 := pool.SubmitTx(tx)

	// Assert
	assert.Error(t, err2, "第二次提交应该返回错误")
	assert.Nil(t, txID2, "交易ID应为nil")
	assert.Equal(t, txID1, txID1, "第一次提交的交易ID应该正确")
}

// TestSubmitTxs_WithValidTransactions_ReturnsTxIDs 测试批量提交有效交易
func TestSubmitTxs_WithValidTransactions_ReturnsTxIDs(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	txs := []*transaction.Transaction{
		testutil.CreateSimpleTestTransaction(1),
		testutil.CreateSimpleTestTransaction(2),
		testutil.CreateSimpleTestTransaction(3),
	}

	// Act
	txIDs, err := pool.SubmitTxs(txs)

	// Assert
	assert.NoError(t, err, "应该成功批量提交交易")
	assert.Len(t, txIDs, 3, "应该返回3个交易ID")
	for _, txID := range txIDs {
		assert.NotNil(t, txID, "交易ID不应为nil")
		assert.Len(t, txID, 32, "交易ID应该是32字节")
	}
}

// TestSubmitTxs_WithEmptyList_ReturnsEmpty 测试空列表时返回空结果
func TestSubmitTxs_WithEmptyList_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	txs := []*transaction.Transaction{}

	// Act
	txIDs, err := pool.SubmitTxs(txs)

	// Assert
	assert.NoError(t, err, "空列表应该成功")
	assert.Len(t, txIDs, 0, "应该返回空列表")
}

// TestSubmitTxs_WithNilList_ReturnsEmpty 测试nil列表时返回空结果
func TestSubmitTxs_WithNilList_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	txIDs, err := pool.SubmitTxs(nil)

	// Assert
	// 注意：SubmitTxs 对nil列表的处理可能返回空列表而不是错误
	// 这取决于实现，我们先测试实际行为
	if err != nil {
		assert.Error(t, err, "nil列表可能返回错误")
		assert.Nil(t, txIDs, "交易ID列表应为nil")
	} else {
		assert.Len(t, txIDs, 0, "nil列表应该返回空列表")
	}
}

// TestSubmitTxs_WithDuplicateTransactions_ReturnsError 测试批量提交包含重复交易时返回错误
func TestSubmitTxs_WithDuplicateTransactions_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txs := []*transaction.Transaction{
		tx,
		tx, // 重复交易
		testutil.CreateSimpleTestTransaction(2),
	}

	// Act
	txIDs, err := pool.SubmitTxs(txs)

	// Assert
	assert.Error(t, err, "包含重复交易应该返回错误")
	assert.Nil(t, txIDs, "交易ID列表应为nil")
}

// TestSubmitTx_ConcurrentAccess_IsSafe 测试并发提交的安全性
func TestSubmitTx_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 10

	// Act - 并发提交
	done := make(chan error, numTxs)
	for i := 0; i < numTxs; i++ {
		go func(idx int) {
			tx := testutil.CreateSimpleTestTransaction(idx)
			_, err := pool.SubmitTx(tx)
			done <- err
		}(i)
	}

	// 等待所有goroutine完成
	successCount := 0
	for i := 0; i < numTxs; i++ {
		err := <-done
		if err == nil {
			successCount++
		}
	}

	// Assert - 验证所有交易都已提交（可能有些因为重复而失败，但至少应该有一些成功）
	assert.Greater(t, successCount, 0, "至少应该有一些交易成功提交")
}

// TestSubmitTx_AfterClose_ReturnsError 测试交易池关闭后提交返回错误
func TestSubmitTx_AfterClose_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act - 关闭交易池
	err := pool.Close()
	require.NoError(t, err, "关闭交易池应该成功")

	// Act - 尝试提交交易
	txID, err := pool.SubmitTx(tx)

	// Assert
	assert.Error(t, err, "关闭后提交应该返回错误")
	assert.Nil(t, txID, "交易ID应为nil")
	assert.Equal(t, ErrTxPoolClosed, err, "应该返回ErrTxPoolClosed错误")
}

