// Package candidatepool calcBlockHash覆盖率测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestCalcBlockHash_WithNilHashService_UsesSimpleHash 测试没有哈希服务时使用简单哈希
func TestCalcBlockHash_WithNilHashService_UsesSimpleHash(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    100,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()

	// 使用nil哈希服务
	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, nil, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	block := testutil.CreateSimpleTestBlock(100)

	// Act
	hash, err := cp.calcBlockHash(block)

	// Assert
	assert.NoError(t, err, "应该成功计算哈希")
	assert.NotNil(t, hash, "哈希不应为nil")
	// 简单哈希应该包含高度和时间戳
	assert.Contains(t, string(hash), "100", "简单哈希应该包含高度")
}

// TestCalcBlockHash_WithHashService_UsesService 测试使用哈希服务
func TestCalcBlockHash_WithHashService_UsesService(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)

	// Act
	hash, err := pool.calcBlockHash(block)

	// Assert
	assert.NoError(t, err, "应该成功计算哈希")
	assert.NotNil(t, hash, "哈希不应为nil")
	assert.Equal(t, 32, len(hash), "哈希长度应为32字节")
}

// TestCalcBlockHash_WithNilBlock_HandlesNil 测试nil区块
func TestCalcBlockHash_WithNilBlock_HandlesNil(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	hash, err := pool.calcBlockHash(nil)

	// Assert
	// 注意：根据实现，nil区块在使用哈希服务时，MockBlockHashService会返回哈希
	// 但在实际使用中，哈希服务可能会返回错误
	// 这里主要测试不会panic
	if err != nil {
		assert.Nil(t, hash, "哈希应为nil")
		assert.Contains(t, err.Error(), "计算区块哈希失败", "错误信息应该包含计算失败")
	} else {
		// MockBlockHashService返回了哈希（即使区块为nil）
		// 这是Mock的行为，实际服务可能会返回错误
		assert.NotNil(t, hash, "Mock服务返回了哈希")
	}
}

