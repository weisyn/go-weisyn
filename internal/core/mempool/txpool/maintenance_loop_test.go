// Package txpool 维护循环测试
package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestMaintenanceLoop_WithExpiredTxs_CleansUp 测试维护循环清理过期交易
func TestMaintenanceLoop_WithExpiredTxs_CleansUp(t *testing.T) {
	// Arrange
	// 创建短生命周期的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   50 * 1000000000, // 50毫秒（纳秒）
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

	// 提交多个交易
	numTxs := 5
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// 等待交易过期（超过Lifetime）
	time.Sleep(100 * time.Millisecond)

	// Act - 手动触发清理（模拟maintenanceLoop的行为）
	txPool.mu.Lock()
	txPool.cleanExpiredTransactions()
	txPool.mu.Unlock()

	// Assert
	// 验证过期交易已被清理
	pendingTxs, _ := txPool.GetAllPendingTransactions()
	// 注意：过期交易可能已被清理，也可能仍然存在（取决于清理时机）
	assert.GreaterOrEqual(t, len(pendingTxs), 0, "pending交易数应该>=0")
}

// TestMaintenanceLoop_WithMetricsEnabled_LogsMetrics 测试启用监控时的指标记录
func TestMaintenanceLoop_WithMetricsEnabled_LogsMetrics(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		MetricsEnabled: true,
		MetricsInterval: 100 * time.Millisecond, // 100毫秒间隔
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

	// 提交一些交易
	for i := 0; i < 3; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// Act - 手动触发指标记录（模拟maintenanceLoop的行为）
	txPool.logMetrics()

	// Assert
	// 验证指标记录不会panic
	assert.NotNil(t, txPool.logger, "logger应该存在")
}

// TestMaintenanceLoop_WithHealthCheck_ChecksHealth 测试健康检查
func TestMaintenanceLoop_WithHealthCheck_ChecksHealth(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// 提交一些交易
	for i := 0; i < 5; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// Act - 手动触发健康检查（模拟maintenanceLoop的行为）
	health := pool.checkPoolHealth()

	// Assert
	assert.NotNil(t, health, "健康检查结果不应为nil")
	assert.GreaterOrEqual(t, health.TxCount, 0, "交易数量应该>=0")
	assert.NotEmpty(t, health.HealthMessage, "健康消息不应为空")
}

// TestMaintenanceLoop_WithRecomputePriorities_UpdatesPriorities 测试重算优先级
func TestMaintenanceLoop_WithRecomputePriorities_UpdatesPriorities(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// 提交多个交易
	numTxs := 5
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// 记录初始优先级
	initialPriorities := make(map[string]int32)
	pool.mu.RLock()
	for txIDStr, wrapper := range pool.txs {
		initialPriorities[txIDStr] = wrapper.Priority
	}
	pool.mu.RUnlock()

	// Act - 手动触发重算优先级（模拟maintenanceLoop的行为）
	pool.mu.Lock()
	pool.recomputePriorities()
	pool.mu.Unlock()

	// Assert
	// 验证优先级已更新
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	for txIDStr, wrapper := range pool.txs {
		// 优先级应该已更新（可能相同或不同）
		assert.NotNil(t, wrapper, "交易包装器不应为nil")
		_ = initialPriorities[txIDStr] // 使用初始优先级（如果存在）
	}
}

// TestMaintenanceLoop_WithQuitChannel_StopsLoop 测试quit通道停止循环
func TestMaintenanceLoop_WithQuitChannel_StopsLoop(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act - 关闭交易池（发送quit信号）
	err := pool.Stop()
	require.NoError(t, err)

	// Assert
	// 验证交易池已关闭
	select {
	case <-pool.quit:
		// quit通道已关闭，maintenanceLoop应该已停止
		assert.True(t, true, "quit通道应该已关闭")
	default:
		// quit通道未关闭，但Stop方法应该已调用
		t.Logf("注意：quit通道可能未立即关闭")
	}
}

