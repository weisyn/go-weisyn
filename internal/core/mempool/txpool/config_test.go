// Package txpool 配置管理测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/config/txpool"
)

// TestValidateConfig_WithValidConfig_ReturnsNoErrors 测试有效配置
func TestValidateConfig_WithValidConfig_ReturnsNoErrors(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000, // 1小时（纳秒）
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Len(t, errors, 0, "有效配置应该没有错误")
}

// TestValidateConfig_WithNilConfig_ReturnsError 测试nil配置
func TestValidateConfig_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	errors := pool.validateConfig(nil)

	// Assert
	assert.Len(t, errors, 1, "nil配置应该返回错误")
	assert.Contains(t, errors[0].Error(), "配置不能为空", "错误信息应该正确")
}

// TestValidateConfig_WithInvalidMaxSize_ReturnsError 测试无效MaxSize
func TestValidateConfig_WithInvalidMaxSize_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
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
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "无效MaxSize应该返回错误")
	found := false
	for _, err := range errors {
		if contains(err.Error(), "MaxSize") {
			found = true
			break
		}
	}
	assert.True(t, found, "错误信息应该包含MaxSize")
}

// TestValidateConfig_WithInvalidMemoryLimit_ReturnsError 测试无效MemoryLimit
func TestValidateConfig_WithInvalidMemoryLimit_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 0, // 无效
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "无效MemoryLimit应该返回错误")
	found := false
	for _, err := range errors {
		if contains(err.Error(), "MemoryLimit") {
			found = true
			break
		}
	}
	assert.True(t, found, "错误信息应该包含MemoryLimit")
}

// TestValidateConfig_WithInvalidMaxTxSize_ReturnsError 测试无效MaxTxSize
func TestValidateConfig_WithInvalidMaxTxSize_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  0, // 无效
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "无效MaxTxSize应该返回错误")
	found := false
	for _, err := range errors {
		if contains(err.Error(), "MaxTxSize") {
			found = true
			break
		}
	}
	assert.True(t, found, "错误信息应该包含MaxTxSize")
}

// TestValidateConfig_WithMaxTxSizeGreaterThanMemoryLimit_ReturnsError 测试MaxTxSize大于MemoryLimit
func TestValidateConfig_WithMaxTxSizeGreaterThanMemoryLimit_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  200 * 1024 * 1024, // 大于MemoryLimit
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "MaxTxSize大于MemoryLimit应该返回错误")
	found := false
	for _, err := range errors {
		if contains(err.Error(), "MaxTxSize") && contains(err.Error(), "MemoryLimit") {
			found = true
			break
		}
	}
	assert.True(t, found, "错误信息应该包含MaxTxSize和MemoryLimit")
}

// TestValidateConfig_WithInvalidLifetime_ReturnsError 测试无效Lifetime
func TestValidateConfig_WithInvalidLifetime_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   0, // 无效
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "无效Lifetime应该返回错误")
	found := false
	for _, err := range errors {
		if contains(err.Error(), "Lifetime") {
			found = true
			break
		}
	}
	assert.True(t, found, "错误信息应该包含Lifetime")
}

// TestValidateConfig_WithInvalidMiningConfig_ReturnsError 测试无效Mining配置
func TestValidateConfig_WithInvalidMiningConfig_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Lifetime:   3600 * 1000000000,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 0, // 无效
			MaxBlockSizeForMining:     0, // 无效
		},
	}

	// Act
	errors := pool.validateConfig(config)

	// Assert
	assert.Greater(t, len(errors), 0, "无效Mining配置应该返回错误")
	// 应该有两个错误：MaxTransactionsForMining和MaxBlockSizeForMining
	assert.GreaterOrEqual(t, len(errors), 2, "应该至少有两个错误")
}

// contains 辅助函数检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		indexOfSubstring(s, substr) >= 0)))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

