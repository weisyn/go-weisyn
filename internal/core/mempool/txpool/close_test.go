// Package txpool Close覆盖率提升测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestClose_WithMultipleCalls_NoError 测试多次关闭
func TestClose_WithMultipleCalls_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// Act - 第一次关闭
	err1 := pool.Close()
	
	// Act - 第二次关闭
	err2 := pool.Close()
	
	// Assert
	assert.NoError(t, err1, "第一次关闭应该成功")
	assert.NoError(t, err2, "第二次关闭应该成功（幂等操作）")
}

// TestClose_WithPendingOperations_AllowsCompletion 测试关闭后允许完成操作
func TestClose_WithPendingOperations_AllowsCompletion(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	txID, err := pool.SubmitTx(tx)
	require.NoError(t, err)
	
	// Act - 关闭交易池
	err = pool.Close()
	require.NoError(t, err)
	
	// Assert - 关闭后，某些操作应该仍然可以完成（取决于实现）
	// 这里我们主要验证关闭不会panic
	assert.NotPanics(t, func() {
		_ = pool.Close()
	})
	_ = txID // 避免未使用变量警告
}

// TestClose_WithTransactionsInPool_ClosesSuccessfully 测试有交易时关闭
func TestClose_WithTransactionsInPool_ClosesSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 添加多个交易
	numTxs := 5
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateSimpleTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}
	
	// Act
	err := pool.Close()
	
	// Assert
	assert.NoError(t, err, "有交易时应该成功关闭")
}

// TestClose_AfterClose_OperationsReturnError 测试关闭后的操作
func TestClose_AfterClose_OperationsReturnError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)
	
	// 关闭交易池
	err := pool.Close()
	require.NoError(t, err)
	
	// Act - 尝试添加交易
	txID, err := pool.AddTransaction(tx)
	
	// Assert
	assert.Error(t, err, "关闭后添加交易应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
	assert.Contains(t, err.Error(), "交易池已关闭", "错误信息应该包含'交易池已关闭'")
}

// TestClose_WithEventSink_NoPanic 测试关闭时事件下沉不panic
func TestClose_WithEventSink_NoPanic(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	mockSink := &MockTxEventSink{}
	pool.SetEventSink(mockSink)
	
	// Act & Assert
	assert.NotPanics(t, func() {
		err := pool.Close()
		assert.NoError(t, err)
	})
}

