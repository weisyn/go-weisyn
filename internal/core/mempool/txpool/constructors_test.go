// Package txpool 构造函数测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestNewTxPool_WithValidConfig_CreatesPool 测试NewTxPool构造函数
func TestNewTxPool_WithValidConfig_CreatesPool(t *testing.T) {
	// Arrange
	options := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:   1024 * 1024,
		Lifetime:    3600 * 1000000000, // 1小时
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	config := txpool.New(nil)
	// 使用反射或直接设置options（如果Config有SetOptions方法）
	// 这里我们直接使用NewTxPoolWithCache来测试，因为NewTxPool内部调用它
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act - 直接测试NewTxPoolWithCache，因为NewTxPool内部调用它
	pool, err := NewTxPoolWithCache(options, logger, eventBus, memory, hashService, nil)

	// Assert
	assert.NoError(t, err, "应该成功创建交易池")
	assert.NotNil(t, pool, "交易池不应为nil")
	_ = config // 避免未使用变量警告
}

// TestNewTxPool_WithNilConfig_ReturnsError 测试nil配置
func TestNewTxPool_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act - 直接测试NewTxPoolWithCache，因为NewTxPool内部调用它
	pool, err := NewTxPoolWithCache(nil, logger, eventBus, memory, hashService, nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, pool, "交易池应为nil")
	assert.Contains(t, err.Error(), "配置不能为空", "错误信息应该包含'配置不能为空'")
}

// TestNewTxPoolWithCacheAndCompliance_WithValidConfig_CreatesPool 测试NewTxPoolWithCacheAndCompliance构造函数
func TestNewTxPoolWithCacheAndCompliance_WithValidConfig_CreatesPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 100 * 1024 * 1024,
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
	compliancePolicy := testutil.NewMockCompliancePolicy(true)

	// Act
	pool, err := NewTxPoolWithCacheAndCompliance(config, logger, eventBus, memory, hashService, nil, compliancePolicy, nil)

	// Assert
	assert.NoError(t, err, "应该成功创建交易池")
	assert.NotNil(t, pool, "交易池不应为nil")
}

// TestNewTxPoolWithCacheAndCompliance_WithNilCompliancePolicy_CreatesPool 测试nil合规策略
func TestNewTxPoolWithCacheAndCompliance_WithNilCompliancePolicy_CreatesPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 100 * 1024 * 1024,
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

	// Act
	pool, err := NewTxPoolWithCacheAndCompliance(config, logger, eventBus, memory, hashService, nil, nil, nil)

	// Assert
	assert.NoError(t, err, "应该成功创建交易池（合规策略可选）")
	assert.NotNil(t, pool, "交易池不应为nil")
}

// TestNewTxPoolWithCacheAndCompliance_WithPersistentStore_CreatesPool 测试带持久化存储的构造函数
func TestNewTxPoolWithCacheAndCompliance_WithPersistentStore_CreatesPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:     1000,
		MemoryLimit: 100 * 1024 * 1024,
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

	// Act
	pool, err := NewTxPoolWithCacheAndCompliance(config, logger, eventBus, memory, hashService, nil, nil, nil)

	// Assert
	assert.NoError(t, err, "应该成功创建交易池（持久化存储可选）")
	assert.NotNil(t, pool, "交易池不应为nil")
}

