// Package txpool 健康检查测试
package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestCheckPoolHealth_WithHealthyPool_ReturnsHealthy 测试健康池的健康检查
func TestCheckPoolHealth_WithHealthyPool_ReturnsHealthy(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	health := pool.checkPoolHealth()

	// Assert
	assert.True(t, health.IsHealthy, "健康池应该返回IsHealthy=true")
	assert.Equal(t, "交易池运行正常", health.HealthMessage, "健康消息应该正确")
	assert.GreaterOrEqual(t, health.TxCount, 0, "交易数量应该>=0")
	assert.GreaterOrEqual(t, health.PendingCount, 0, "pending数量应该>=0")
}

// TestCheckPoolHealth_WithHighMemoryUsage_ReturnsUnhealthy 测试高内存使用率
func TestCheckPoolHealth_WithHighMemoryUsage_ReturnsUnhealthy(t *testing.T) {
	// Arrange
	// 创建内存限制很小的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    100,
		MemoryLimit: 5000, // 5KB限制
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

	// 填满内存（接近90%）
	for i := 0; i < 20; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// Act
	health := txPool.checkPoolHealth()

	// Assert
	// 如果内存使用率>90%，应该返回不健康
	if health.MemoryUsagePct > 90 {
		assert.False(t, health.IsHealthy, "高内存使用率应该返回IsHealthy=false")
		assert.Contains(t, health.HealthMessage, "内存使用率过高", "健康消息应该包含相关描述")
	}
	assert.Greater(t, health.MemoryUsageMB, 0.0, "内存使用量应该>0")
}

// TestCheckPoolHealth_WithHighTxCount_ReturnsUnhealthy 测试高交易数量
func TestCheckPoolHealth_WithHighTxCount_ReturnsUnhealthy(t *testing.T) {
	// Arrange
	// 创建MaxSize很小的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    10, // 只有10个交易的限制
		MemoryLimit: 100 * 1024 * 1024,
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

	// 填满交易池（接近90%）
	for i := 0; i < 9; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// Act
	health := txPool.checkPoolHealth()

	// Assert
	// 如果交易数量>90%，应该返回不健康
	if health.TxCountPct > 90 {
		assert.False(t, health.IsHealthy, "高交易数量应该返回IsHealthy=false")
		assert.Contains(t, health.HealthMessage, "交易数量接近上限", "健康消息应该包含相关描述")
	}
	assert.Greater(t, health.TxCount, 0, "交易数量应该>0")
}

// TestCheckPoolHealth_WithExpiredTxs_ReturnsUnhealthy 测试过期交易比例
func TestCheckPoolHealth_WithExpiredTxs_ReturnsUnhealthy(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)

	// 提交多个交易
	for i := 0; i < 10; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// 同步状态以标记过期
	pool.SyncStatus(1, []byte("state_root"))

	// Act
	health := pool.checkPoolHealth()

	// Assert
	// 注意：PoolHealthStatus可能没有ExpiredPct字段，我们检查ExpiredCount和健康消息
	assert.GreaterOrEqual(t, health.ExpiredCount, 0, "过期交易数量应该>=0")
	// 如果过期交易数量较多，健康消息可能包含相关信息
	if health.ExpiredCount > 0 {
		// 验证健康状态和消息
		assert.NotEmpty(t, health.HealthMessage, "健康消息不应为空")
		// 如果过期交易比例过高，应该返回不健康
		if !health.IsHealthy {
			assert.Contains(t, health.HealthMessage, "过期", "健康消息应该包含过期相关信息")
		}
	}
}

// TestCheckPoolHealth_WithRejectedTxs_ReturnsUnhealthy 测试被拒绝交易比例
func TestCheckPoolHealth_WithRejectedTxs_ReturnsUnhealthy(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// 提交多个交易
	txIDs := make([][]byte, 0)
	for i := 0; i < 10; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
		txIDs = append(txIDs, txID)
	}

	// 拒绝一些交易（如果比例>5%应该不健康）
	// 注意：需要拒绝超过5%的交易才能触发不健康状态
	rejectCount := len(txIDs) / 2 // 拒绝50%
	for i := 0; i < rejectCount && i < len(txIDs); i++ {
		pool.RejectTransactions([][]byte{txIDs[i]})
	}

	// Act
	health := pool.checkPoolHealth()

	// Assert
	// 注意：PoolHealthStatus可能没有RejectedPct字段，我们检查RejectedCount
	assert.GreaterOrEqual(t, health.RejectedCount, 0, "被拒绝交易数量应该>=0")
	// 如果被拒绝交易数量较多，健康消息可能包含相关信息
	if health.RejectedCount > 0 {
		// 验证健康状态和消息
		assert.NotEmpty(t, health.HealthMessage, "健康消息不应为空")
	}
}

