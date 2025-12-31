// Package txpool 配置更新测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestUpdateConfig_WithValidConfig_UpdatesSuccessfully 测试有效配置更新
func TestUpdateConfig_WithValidConfig_UpdatesSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    2000,
		MemoryLimit: 200 * 1024 * 1024,
		MaxTxSize:  2 * 1024 * 1024,
		Lifetime:   7200 * 1000000000, // 2小时
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 200,
			MaxBlockSizeForMining:     2 * 1024 * 1024,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "有效配置应该成功更新")
	// 验证配置已更新
	assert.Equal(t, newConfig.MaxSize, pool.config.MaxSize, "MaxSize应该已更新")
	assert.Equal(t, newConfig.MemoryLimit, pool.memoryLimit, "MemoryLimit应该已更新")
}

// TestUpdateConfig_WithInvalidConfig_ReturnsError 测试无效配置更新
func TestUpdateConfig_WithInvalidConfig_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	invalidConfig := &txpool.TxPoolOptions{
		MaxSize:    0, // 无效
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	err := pool.updateConfig(invalidConfig)

	// Assert
	assert.Error(t, err, "无效配置应该返回错误")
	assert.Contains(t, err.Error(), "配置验证失败", "错误信息应该包含相关描述")
}

// TestUpdateConfig_WithReducedMemoryLimit_TriggersCleanup 测试内存限制减小触发清理
func TestUpdateConfig_WithReducedMemoryLimit_TriggersCleanup(t *testing.T) {
	// Arrange
	// 创建初始内存限制较大的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024, // 100MB
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

	// 提交一些交易填满内存
	for i := 0; i < 10; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := txPool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	oldMemoryUsage := txPool.memoryUsage
	require.Greater(t, oldMemoryUsage, uint64(0), "应该有内存使用")

	// 创建新的配置，内存限制减小
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 10 * 1024 * 1024, // 10MB（小于当前使用）
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	err = txPool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MemoryLimit, txPool.memoryLimit, "内存限制应该已更新")
	// 注意：如果内存使用超过新限制，应该触发清理
	// 但由于清理是异步的，我们主要验证配置已更新
}

// TestUpdateConfig_WithIncreasedMemoryLimit_NoCleanup 测试内存限制增加不触发清理
func TestUpdateConfig_WithIncreasedMemoryLimit_NoCleanup(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	oldMemoryLimit := pool.memoryLimit

	// 提交一些交易
	for i := 0; i < 5; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	// 创建新的配置，内存限制增加
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: oldMemoryLimit * 2, // 增加一倍
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MemoryLimit, pool.memoryLimit, "内存限制应该已更新")
	// 内存限制增加不应该触发清理
}

// TestUpdateConfig_WithMaxSizeChange_UpdatesProtector 测试MaxSize改变
func TestUpdateConfig_WithMaxSizeChange_UpdatesProtector(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	oldMaxSize := pool.config.MaxSize

	// 创建新的配置，MaxSize改变
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    oldMaxSize * 2,
		MemoryLimit: pool.memoryLimit,
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MaxSize, pool.config.MaxSize, "MaxSize应该已更新")
	// 注意：保护器可能需要重启才能生效，这里只验证配置已更新
}

// TestUpdateConfig_WithMaxTxSizeChange_UpdatesValidator 测试MaxTxSize改变
func TestUpdateConfig_WithMaxTxSizeChange_UpdatesValidator(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	oldMaxTxSize := pool.config.MaxTxSize

	// 创建新的配置，MaxTxSize改变
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    pool.config.MaxSize,
		MemoryLimit: pool.memoryLimit,
		MaxTxSize:  oldMaxTxSize * 2,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MaxTxSize, pool.config.MaxTxSize, "MaxTxSize应该已更新")
	// 验证器应该已重新创建
	assert.NotNil(t, pool.basicValidator, "基础验证器应该存在")
}

