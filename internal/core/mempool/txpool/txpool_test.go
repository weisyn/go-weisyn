// Package txpool 测试文件
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestNewTxPoolWithCache_WithValidConfig_ReturnsPool 测试使用有效配置创建交易池
func TestNewTxPoolWithCache_WithValidConfig_ReturnsPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024, // 100MB
		MaxTxSize:  1024 * 1024,        // 1MB
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act
	pool, err := NewTxPoolWithCache(config, logger, eventBus, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "应该成功创建交易池")
	assert.NotNil(t, pool, "交易池实例不应为nil")
}

// TestNewTxPoolWithCache_WithNilConfig_ReturnsError 测试配置为nil时返回错误
func TestNewTxPoolWithCache_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act
	pool, err := NewTxPoolWithCache(nil, logger, eventBus, memory, hashService, nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, pool, "交易池实例应为nil")
	assert.Contains(t, err.Error(), "配置不能为空", "错误信息应该包含配置相关描述")
}

// TestNewTxPoolWithCache_WithNilLogger_ReturnsPool 测试logger为nil时仍能创建交易池
func TestNewTxPoolWithCache_WithNilLogger_ReturnsPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
	}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act
	pool, err := NewTxPoolWithCache(config, nil, eventBus, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "logger为nil时应该仍能创建交易池")
	assert.NotNil(t, pool, "交易池实例不应为nil")
}

// TestNewTxPoolWithCache_WithNilEventBus_ReturnsPool 测试eventBus为nil时仍能创建交易池
func TestNewTxPoolWithCache_WithNilEventBus_ReturnsPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
	}
	logger := &testutil.MockLogger{}
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	// Act
	pool, err := NewTxPoolWithCache(config, logger, nil, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "eventBus为nil时应该仍能创建交易池")
	assert.NotNil(t, pool, "交易池实例不应为nil")
}

// TestNewTxPoolWithCache_WithNilMemory_ReturnsPool 测试memory为nil时仍能创建交易池
func TestNewTxPoolWithCache_WithNilMemory_ReturnsPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	hashService := &testutil.MockTransactionHashService{}

	// Act
	pool, err := NewTxPoolWithCache(config, logger, eventBus, nil, hashService, nil)

	// Assert
	require.NoError(t, err, "memory为nil时应该仍能创建交易池")
	assert.NotNil(t, pool, "交易池实例不应为nil")
}

