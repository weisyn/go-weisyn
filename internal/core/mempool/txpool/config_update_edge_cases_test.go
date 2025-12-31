// Package txpool 配置更新边界情况测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestUpdateConfig_WithSameConfig_NoChange 测试相同配置更新
func TestUpdateConfig_WithSameConfig_NoChange(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	oldConfig := pool.config
	// 确保Lifetime有效（如果为0则设置默认值）
	lifetime := oldConfig.Lifetime
	if lifetime == 0 {
		lifetime = 3600 * 1000000000 // 1小时（纳秒）
	}
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    oldConfig.MaxSize,
		MemoryLimit: pool.memoryLimit,
		MaxTxSize:  oldConfig.MaxTxSize,
		Lifetime:   lifetime,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: oldConfig.Mining.MaxTransactionsForMining,
			MaxBlockSizeForMining:     oldConfig.Mining.MaxBlockSizeForMining,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "相同配置应该成功更新")
	assert.Equal(t, newConfig.MaxSize, pool.config.MaxSize, "MaxSize应该相同")
	assert.Equal(t, newConfig.MemoryLimit, pool.memoryLimit, "MemoryLimit应该相同")
}

// TestUpdateConfig_WithZeroMaxSize_ReturnsError 测试零MaxSize配置
func TestUpdateConfig_WithZeroMaxSize_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	invalidConfig := &txpool.TxPoolOptions{
		MaxSize:    0, // 无效
		MemoryLimit: pool.memoryLimit,
		MaxTxSize:  pool.config.MaxTxSize,
		Lifetime:   pool.config.Lifetime,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: pool.config.Mining.MaxTransactionsForMining,
			MaxBlockSizeForMining:     pool.config.Mining.MaxBlockSizeForMining,
		},
	}

	// Act
	err := pool.updateConfig(invalidConfig)

	// Assert
	assert.Error(t, err, "零MaxSize应该返回错误")
	assert.Contains(t, err.Error(), "配置验证失败", "错误信息应该包含相关描述")
}

// TestUpdateConfig_WithMemoryLimitEqualToUsage_NoCleanup 测试内存限制等于当前使用量
func TestUpdateConfig_WithMemoryLimitEqualToUsage_NoCleanup(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 提交一些交易
	for i := 0; i < 3; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		if err != nil {
			break
		}
	}

	currentUsage := pool.memoryUsage
	require.Greater(t, currentUsage, uint64(0), "应该有内存使用")

	// 确保MaxTxSize不超过新的MemoryLimit
	maxTxSize := pool.config.MaxTxSize
	if maxTxSize > currentUsage {
		maxTxSize = currentUsage
	}

	// 确保Lifetime有效
	lifetime := pool.config.Lifetime
	if lifetime == 0 {
		lifetime = 3600 * 1000000000 // 1小时（纳秒）
	}

	// 创建新配置，内存限制等于当前使用量
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    pool.config.MaxSize,
		MemoryLimit: currentUsage, // 等于当前使用量
		MaxTxSize:  maxTxSize,
		Lifetime:   lifetime,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: pool.config.Mining.MaxTransactionsForMining,
			MaxBlockSizeForMining:     pool.config.Mining.MaxBlockSizeForMining,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MemoryLimit, pool.memoryLimit, "内存限制应该已更新")
	// 由于内存限制等于使用量，不应该触发清理
}

// TestUpdateConfig_WithMaxTxSizeEqualToMemoryLimit_NoError 测试MaxTxSize等于MemoryLimit
func TestUpdateConfig_WithMaxTxSizeEqualToMemoryLimit_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	memoryLimit := pool.memoryLimit

	// 确保Lifetime有效
	lifetime := pool.config.Lifetime
	if lifetime == 0 {
		lifetime = 3600 * 1000000000 // 1小时（纳秒）
	}

	// 创建新配置，MaxTxSize等于MemoryLimit
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    pool.config.MaxSize,
		MemoryLimit: memoryLimit,
		MaxTxSize:  memoryLimit, // 等于MemoryLimit
		Lifetime:   lifetime,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: pool.config.Mining.MaxTransactionsForMining,
			MaxBlockSizeForMining:     pool.config.Mining.MaxBlockSizeForMining,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "MaxTxSize等于MemoryLimit应该成功")
	assert.Equal(t, newConfig.MaxTxSize, pool.config.MaxTxSize, "MaxTxSize应该已更新")
}

// TestUpdateConfig_WithNilProtector_NoError 测试nil保护器时的配置更新
func TestUpdateConfig_WithNilProtector_NoError(t *testing.T) {
	// Arrange
	// 创建MaxSize为0的交易池（不会创建保护器）
	config := &txpool.TxPoolOptions{
		MaxSize:    0, // 0表示不限制，不会创建保护器
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

	// 验证保护器为nil
	assert.Nil(t, txPool.protector, "保护器应该为nil")

	// 创建新配置
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 200 * 1024 * 1024,
		MaxTxSize:  2 * 1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 200,
			MaxBlockSizeForMining:     2 * 1024 * 1024,
		},
	}

	// Act
	err = txPool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "nil保护器时配置更新应该成功")
	assert.Equal(t, newConfig.MaxSize, txPool.config.MaxSize, "MaxSize应该已更新")
}

// TestUpdateConfig_WithNilBasicValidator_NoError 测试nil基础验证器时的配置更新
func TestUpdateConfig_WithNilBasicValidator_NoError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 确保Lifetime有效
	lifetime := pool.config.Lifetime
	if lifetime == 0 {
		lifetime = 3600 * 1000000000 // 1小时（纳秒）
	}

	// 注意：basicValidator在NewTxPoolWithCache中总是被创建，所以这个测试主要验证更新逻辑
	newConfig := &txpool.TxPoolOptions{
		MaxSize:    pool.config.MaxSize,
		MemoryLimit: pool.memoryLimit,
		MaxTxSize:  pool.config.MaxTxSize * 2, // 改变MaxTxSize
		Lifetime:   lifetime,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: pool.config.Mining.MaxTransactionsForMining,
			MaxBlockSizeForMining:     pool.config.Mining.MaxBlockSizeForMining,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MaxTxSize, pool.config.MaxTxSize, "MaxTxSize应该已更新")
	// 基础验证器应该已重新创建
	assert.NotNil(t, pool.basicValidator, "基础验证器应该存在")
}

// TestUpdateConfig_WithAllFieldsChanged_UpdatesAll 测试所有字段都改变
func TestUpdateConfig_WithAllFieldsChanged_UpdatesAll(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	oldMaxSize := pool.config.MaxSize
	oldMemoryLimit := pool.memoryLimit
	oldMaxTxSize := pool.config.MaxTxSize

	// 确保Lifetime有效
	lifetime := pool.config.Lifetime
	if lifetime == 0 {
		lifetime = 3600 * 1000000000 // 1小时（纳秒）
	}

	newConfig := &txpool.TxPoolOptions{
		MaxSize:    oldMaxSize * 2,
		MemoryLimit: oldMemoryLimit * 2,
		MaxTxSize:  oldMaxTxSize * 2,
		Lifetime:   lifetime * 2,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: pool.config.Mining.MaxTransactionsForMining * 2,
			MaxBlockSizeForMining:     pool.config.Mining.MaxBlockSizeForMining * 2,
		},
	}

	// Act
	err := pool.updateConfig(newConfig)

	// Assert
	assert.NoError(t, err, "配置更新应该成功")
	assert.Equal(t, newConfig.MaxSize, pool.config.MaxSize, "MaxSize应该已更新")
	assert.Equal(t, newConfig.MemoryLimit, pool.memoryLimit, "MemoryLimit应该已更新")
	assert.Equal(t, newConfig.MaxTxSize, pool.config.MaxTxSize, "MaxTxSize应该已更新")
}

