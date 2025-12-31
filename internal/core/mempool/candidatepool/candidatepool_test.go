// Package candidatepool 测试文件
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestNewCandidatePoolWithCache_WithValidConfig_ReturnsPool 测试使用有效配置创建候选区块池
func TestNewCandidatePoolWithCache_WithValidConfig_ReturnsPool(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates: 100,
		MaxAge:        10 * time.Minute,
		MemoryLimit:   100 * 1024 * 1024, // 100MB
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	// Act
	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "应该成功创建候选区块池")
	assert.NotNil(t, pool, "候选区块池实例不应为nil")
}

// TestNewCandidatePoolWithCache_WithNilConfig_ReturnsError 测试配置为nil时返回错误
func TestNewCandidatePoolWithCache_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	// Act
	pool, err := NewCandidatePoolWithCache(nil, logger, eventBus, memory, hashService, nil)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, pool, "候选区块池实例应为nil")
	assert.Contains(t, err.Error(), "配置不能为空", "错误信息应该包含配置相关描述")
}

// TestNewCandidatePoolWithCache_WithNilLogger_ReturnsPool 测试logger为nil时仍能创建候选区块池
func TestNewCandidatePoolWithCache_WithNilLogger_ReturnsPool(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates: 100,
		MaxAge:        10 * time.Minute,
		MemoryLimit:   100 * 1024 * 1024,
	}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	// Act
	pool, err := NewCandidatePoolWithCache(config, nil, eventBus, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "logger为nil时应该仍能创建候选区块池")
	assert.NotNil(t, pool, "候选区块池实例不应为nil")
}

// TestNewCandidatePoolWithCache_WithNilEventBus_ReturnsPool 测试eventBus为nil时仍能创建候选区块池
func TestNewCandidatePoolWithCache_WithNilEventBus_ReturnsPool(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates: 100,
		MaxAge:        10 * time.Minute,
		MemoryLimit:   100 * 1024 * 1024,
	}
	logger := &testutil.MockLogger{}
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	// Act
	pool, err := NewCandidatePoolWithCache(config, logger, nil, memory, hashService, nil)

	// Assert
	require.NoError(t, err, "eventBus为nil时应该仍能创建候选区块池")
	assert.NotNil(t, pool, "候选区块池实例不应为nil")
}

// TestNewCandidatePoolWithCache_WithNilMemory_ReturnsPool 测试memory为nil时仍能创建候选区块池
func TestNewCandidatePoolWithCache_WithNilMemory_ReturnsPool(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates: 100,
		MaxAge:        10 * time.Minute,
		MemoryLimit:   100 * 1024 * 1024,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	hashService := &testutil.MockBlockHashService{}

	// Act
	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, nil, hashService, nil)

	// Assert
	require.NoError(t, err, "memory为nil时应该仍能创建候选区块池")
	assert.NotNil(t, pool, "候选区块池实例不应为nil")
}

