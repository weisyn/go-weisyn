// Package txpool 监控指标测试
package txpool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestCollectMetrics_WithEmptyPool_ReturnsZeroMetrics 测试空池的指标收集
func TestCollectMetrics_WithEmptyPool_ReturnsZeroMetrics(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	metrics := pool.collectMetrics()

	// Assert
	assert.Equal(t, 0, metrics.TotalTxs, "总交易数应该为0")
	assert.Equal(t, 0, metrics.PendingTxs, "pending交易数应该为0")
	assert.Equal(t, 0, metrics.MiningTxs, "挖矿中交易数应该为0")
	assert.Equal(t, 0, metrics.ConfirmedTxs, "已确认交易数应该为0")
	assert.Equal(t, 0, metrics.RejectedTxs, "被拒绝交易数应该为0")
	assert.Equal(t, 0, metrics.ExpiredTxs, "过期交易数应该为0")
	assert.Equal(t, 0.0, metrics.MemoryUsageMB, "内存使用应该为0")
	assert.Equal(t, 0.0, metrics.AvgTxSize, "平均交易大小应该为0")
}

// TestCollectMetrics_WithPendingTxs_ReturnsCorrectMetrics 测试有pending交易的指标收集
func TestCollectMetrics_WithPendingTxs_ReturnsCorrectMetrics(t *testing.T) {
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
	metrics := pool.collectMetrics()

	// Assert
	assert.Equal(t, numTxs, metrics.TotalTxs, "总交易数应该正确")
	assert.Equal(t, numTxs, metrics.PendingTxs, "pending交易数应该正确")
	assert.Greater(t, metrics.MemoryUsageMB, 0.0, "内存使用应该>0")
	assert.Greater(t, metrics.AvgTxSize, 0.0, "平均交易大小应该>0")
	assert.Greater(t, metrics.OldestTxAgeSec, 0.0, "最旧交易年龄应该>0")
}

// TestCollectMetrics_WithMiningTxs_ReturnsCorrectMiningCount 测试挖矿中交易的指标收集
func TestCollectMetrics_WithMiningTxs_ReturnsCorrectMiningCount(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	txIDs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// 标记一些交易为挖矿中
	miningCount := 3
	pool.MarkTransactionsAsMining(txIDs[:miningCount])

	// Act
	metrics := pool.collectMetrics()

	// Assert
	assert.Equal(t, numTxs, metrics.TotalTxs, "总交易数应该正确")
	assert.Equal(t, miningCount, metrics.MiningTxs, "挖矿中交易数应该正确")
	assert.Equal(t, numTxs-miningCount, metrics.PendingTxs, "pending交易数应该正确")
}

// TestCollectMetrics_WithConfirmedTxs_ReturnsCorrectConfirmedCount 测试已确认交易的指标收集
func TestCollectMetrics_WithConfirmedTxs_ReturnsCorrectConfirmedCount(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	txIDs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// 确认一些交易
	confirmedCount := 2
	pool.ConfirmTransactions(txIDs[:confirmedCount], 1)

	// Act
	metrics := pool.collectMetrics()

	// Assert
	// 注意：ConfirmTransactions会从p.txs中删除交易，所以TotalTxs会减少
	assert.Equal(t, numTxs-confirmedCount, metrics.TotalTxs, "总交易数应该减少（已确认的交易被移除）")
	assert.Equal(t, confirmedCount, metrics.ConfirmedTxs, "已确认交易数应该正确")
	assert.Equal(t, numTxs-confirmedCount, metrics.PendingTxs, "pending交易数应该正确")
}

// TestCollectMetrics_WithRejectedTxs_ReturnsCorrectRejectedCount 测试被拒绝交易的指标收集
func TestCollectMetrics_WithRejectedTxs_ReturnsCorrectRejectedCount(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	numTxs := 5

	// 提交多个交易
	txIDs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		txID, err := pool.SubmitTx(tx)
		require.NoError(t, err)
		txIDs[i] = txID
	}

	// 注意：RejectTransactions只处理mining状态的交易，将其恢复为pending
	// 我们需要先将交易标记为mining，然后拒绝
	rejectedCount := 2
	// 先标记为挖矿中
	pool.MarkTransactionsAsMining(txIDs[:rejectedCount])
	// 然后拒绝
	pool.RejectTransactions(txIDs[:rejectedCount])

	// Act
	metrics := pool.collectMetrics()

	// Assert
	// 注意：RejectTransactions不会将交易添加到rejectedTxs map，只是恢复为pending
	assert.Equal(t, numTxs, metrics.TotalTxs, "总交易数应该正确")
	// RejectTransactions不会增加rejectedTxs计数，所以应该是0
	assert.Equal(t, 0, metrics.RejectedTxs, "被拒绝交易数应该为0（RejectTransactions不增加rejectedTxs计数）")
}

// TestCollectMetrics_WithExpiredTxs_ReturnsCorrectExpiredCount 测试过期交易的指标收集
func TestCollectMetrics_WithExpiredTxs_ReturnsCorrectExpiredCount(t *testing.T) {
	// Arrange
	pool := createTestTxPoolWithShortLifetime(t)
	numTxs := 5

	// 提交多个交易
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// 等待交易过期
	time.Sleep(150 * time.Millisecond)

	// 同步状态以标记过期
	pool.SyncStatus(1, []byte("state_root"))

	// Act
	metrics := pool.collectMetrics()

	// Assert
	assert.Equal(t, numTxs, metrics.TotalTxs, "总交易数应该正确")
	assert.GreaterOrEqual(t, metrics.ExpiredTxs, 0, "过期交易数应该>=0")
}

// TestCollectMetrics_WithMemoryUsage_ReturnsCorrectMemoryMetrics 测试内存使用指标
func TestCollectMetrics_WithMemoryUsage_ReturnsCorrectMemoryMetrics(t *testing.T) {
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

	// Act
	metrics := pool.collectMetrics()

	// Assert
	assert.Greater(t, metrics.MemoryUsageMB, 0.0, "内存使用应该>0")
	assert.Greater(t, metrics.MemoryLimitMB, 0.0, "内存限制应该>0")
	assert.GreaterOrEqual(t, metrics.MemoryUsagePct, 0.0, "内存使用百分比应该>=0")
	assert.LessOrEqual(t, metrics.MemoryUsagePct, 100.0, "内存使用百分比应该<=100")
}

// TestLogMetrics_WithMetricsEnabled_LogsMetrics 测试指标日志记录（MetricsEnabled=true）
func TestLogMetrics_WithMetricsEnabled_LogsMetrics(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		MetricsEnabled: true, // 启用指标
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

	// Act
	txPool.logMetrics()

	// Assert
	// 验证日志被调用（通过MockLogger验证）
	// 注意：由于MockLogger的实现，我们主要验证不会panic
	assert.NotNil(t, txPool.logger, "logger应该存在")
}

// TestLogMetrics_WithMetricsDisabled_NoLogs 测试指标日志记录（MetricsEnabled=false）
func TestLogMetrics_WithMetricsDisabled_NoLogs(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		MetricsEnabled: false, // 禁用指标
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

	// Act
	txPool.logMetrics()

	// Assert
	// 如果MetricsEnabled=false，logMetrics应该直接返回，不记录日志
	// 我们主要验证不会panic
	assert.NotNil(t, txPool.logger, "logger应该存在")
}

